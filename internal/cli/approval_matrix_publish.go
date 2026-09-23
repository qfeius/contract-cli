package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"cn.qfei/contract-cli/internal/openplatform"
	"github.com/gofrs/flock"
)

// matrixPublishBaseline 保存内容而非凭证；Binding 绑定实际调用环境与身份，Version 绑定预发布版本。
type matrixPublishBaseline struct {
	Binding string         `json:"binding"`
	Version string         `json:"prepared_version"`
	Rows    map[string]any `json:"rows"`
}

// matrixBaselineRecoveryHint 统一提示缺失或损坏基准的恢复入口，明确该入口不会重写规则行。
const matrixBaselineRecoveryHint = "run pre-release to create or read-only recover the local baseline without rewriting rules, then confirm release again"

/* matrixPublishJSON 严格解析接口信封并保留数字精度；body 为响应字节，返回 data 对象或错误。 */
func matrixPublishJSON(body []byte) (map[string]any, error) {
	var envelope map[string]any
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := d.Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope["code"] != json.Number("0") {
		return nil, fmt.Errorf("matrix query failed: %v", envelope["msg"])
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("matrix response missing data object")
	}
	return data, nil
}

/* matrixExactValues 递归规范化数值和无序部门集合，避免精度损失及接口返回换序误判；v 为 JSON 值，返回规范化值。 */
func matrixExactValues(v any) any {
	switch x := v.(type) {
	case json.Number:
		if n, ok := new(big.Rat).SetString(string(x)); ok {
			return map[string]any{"$number": n.RatString()}
		}
	case map[string]any:
		for k, item := range x {
			normalized := matrixExactValues(item)
			// 部门集合表达成员关系而非审批顺序；保留重复项，只排序以消除开平查询的非确定返回顺序。
			if k == "department_collection" {
				normalized = matrixSortedStringCollection(normalized)
			}
			x[k] = normalized
		}
	case []any:
		for i, item := range x {
			x[i] = matrixExactValues(item)
		}
	}
	return v
}

/* matrixSortedStringCollection 对字符串集合生成确定顺序；v 为待规范化 JSON 值，返回排序后的 []any，非字符串数组原样返回。 */
func matrixSortedStringCollection(v any) any {
	items, ok := v.([]any)
	if !ok {
		return v
	}
	values := make([]string, len(items))
	for i, item := range items {
		value, ok := item.(string)
		if !ok {
			// 非字符串成员不是合法部门 ID，保持原值让发布比较继续按严格模式阻断异常变化。
			return v
		}
		values[i] = value
	}
	sort.Strings(values)
	for i, value := range values {
		items[i] = value
	}
	return items
}

/*
matrixRowsPage 将一页规则行加入累计基准，并拒绝重复行、残缺数据和无效分页标记。
入参 data（map[string]any）为接口页数据，rows（map[string]any）为按行 ID 索引的累计基准；返回 string 为下一页标记，error 为页结构错误。
*/
func matrixRowsPage(data map[string]any, rows map[string]any) (string, error) {
	items, ok := data["table_rows"].([]any)
	if !ok {
		return "", fmt.Errorf("incomplete matrix rows page")
	}
	for _, item := range items {
		row, err := matrixNormalizedRow(item)
		if err != nil {
			return "", err
		}
		id := row["id"].(string)
		if _, exists := rows[id]; exists {
			return "", fmt.Errorf("duplicate row id %s", id)
		}
		rows[id] = row
	}
	more, ok := data["has_more"].(bool)
	if !ok {
		return "", fmt.Errorf("missing has_more")
	}
	if !more {
		return "", nil
	}
	next, ok := data["page_token"].(string)
	if !ok || strings.TrimSpace(next) == "" || len(items) == 0 {
		return "", fmt.Errorf("incomplete pagination token")
	}
	return next, nil
}

/*
matrixNormalizedRow 校验单行结构并将单元格按列 ID 索引，供分页基准和单行回读共用。
入参 item（any）为服务端行对象；返回 map[string]any 为规范化行，error 为缺失或重复字段错误。
*/
func matrixNormalizedRow(item any) (map[string]any, error) {
	row, ok := item.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid matrix row")
	}
	id, ok := row["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("missing row id")
	}
	cells, ok := row["table_cells"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing row cells")
	}
	// 列 ID 是比较键；服务端返回顺序变化不应使内容快照失效。
	indexed := make(map[string]any, len(cells))
	for _, value := range cells {
		cell, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid cell")
		}
		column, ok := cell["table_column_id"].(string)
		if !ok || column == "" {
			return nil, fmt.Errorf("missing column id")
		}
		if _, exists := indexed[column]; exists {
			return nil, fmt.Errorf("duplicate column id")
		}
		if _, ok := cell["table_cell_content"].(map[string]any); !ok {
			return nil, fmt.Errorf("missing cell content")
		}
		indexed[column] = cell
	}
	row["table_cells"] = indexed
	return matrixExactValues(row).(map[string]any), nil
}

/*
readApprovalMatrixRows 完整分页读取规则行，严格拒绝残缺页和重复游标。
入参 ctx（context.Context）为请求上下文，client（*openplatform.Client）与 rc（openplatform.RequestContext）为已解析的调用身份，path（string）为矩阵路径；返回 map[string]any 为按行 ID 索引的行，error 为读取或解析错误。
*/
func readApprovalMatrixRows(ctx context.Context, client *openplatform.Client, rc openplatform.RequestContext, path string) (map[string]any, error) {
	rows := map[string]any{}
	seen := map[string]bool{}
	token := ""
	for page := 0; page < 10000; page++ {
		query := url.Values{"page_size": {"100"}}
		if token != "" {
			query.Set("page_token", token)
		}
		response, err := client.Do(ctx, rc, openplatform.Request{Method: http.MethodGet, Path: path + "/table_rows", Query: query, IdentityPolicy: openplatform.IdentityPolicyAny})
		if err != nil {
			return nil, err
		}
		data, err := matrixPublishJSON(response.Body)
		if err != nil {
			return nil, err
		}
		// 空表的服务端 DTO 可能把 table_rows、has_more 一并序列化为 null。
		_, hasRows := data["table_rows"]
		_, hasMore := data["has_more"]
		if page == 0 && hasRows && hasMore && data["table_rows"] == nil && data["has_more"] == nil {
			return rows, nil
		}
		next, err := matrixRowsPage(data, rows)
		if err != nil {
			return nil, err
		}
		if next == "" {
			return rows, nil
		}
		if seen[next] {
			return nil, fmt.Errorf("repeated pagination token")
		}
		seen[next] = true
		token = next
	}
	return nil, fmt.Errorf("matrix pagination limit exceeded")
}

/*
publishMatrixWithRowBaseline 编排预发布、待发布基准恢复和正式发布，并保证恢复过程不改写业务规则行。
入参 ctx（context.Context）为请求上下文；options（commandOptions）为 CLI 公共选项；action/path（string）为发布动作与矩阵路径；body（[]byte）为原请求体。
返回值 error 表示查询、基准持久化、发布或发布后核验错误，成功时返回 nil。
*/
func (a *App) publishMatrixWithRowBaseline(ctx context.Context, options commandOptions, action, path string, body []byte) error {
	client, rc, err := a.openPlatformClientAndContextForOptions(options, path, openplatform.IdentityPolicyAny)
	if err != nil {
		return err
	}
	// 路径包含产品、规则组和矩阵编码；身份指纹含环境及用户凭证摘要，不保存原始凭证。
	binding := approvalMatrixFingerprint([]string{rc.Profile.Name, string(rc.Identity), path, approvalMatrixContextFingerprint(rc)})
	dir := filepath.Join(filepath.Dir(a.store.Path()), "approval-matrix-publish")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file := filepath.Join(dir, binding+".json")
	lock := flock.New(file + ".lock")
	acquired, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("matrix publication already running")
	}
	defer lock.Unlock()
	request := func(method, suffix string, query url.Values, input []byte) (map[string]any, error) {
		r, e := client.Do(ctx, rc, openplatform.Request{Method: method, Path: path + suffix, Query: query, Body: input, IdentityPolicy: openplatform.IdentityPolicyAny})
		if e != nil {
			return nil, e
		}
		return matrixPublishJSON(r.Body)
	}
	readRows := func() (map[string]any, error) { return readApprovalMatrixRows(ctx, client, rc, path) }
	baseline := matrixPublishBaseline{Binding: binding}
	persistBaseline := func(recovered bool) error {
		encoded, e := json.Marshal(baseline)
		if e != nil {
			return e
		}
		if e = os.WriteFile(file, encoded, 0600); e != nil {
			return e
		}
		data := map[string]any{"prepared_version": baseline.Version, "row_count": len(baseline.Rows), "requires_release_confirmation": true}
		if recovered {
			data["baseline_recovered"] = true
		}
		return a.renderApprovalMatrixImportValue(options, map[string]any{"code": 0, "msg": "success", "data": data})
	}
	if action == "pre-release" {
		// 先废弃旧基准，任何预发布失败都不允许复用历史确认。
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
		// profile 重建会丢失本地基准；服务端已处于待发布状态时，以稳定双读恢复基准，不制造规则行更新来重置状态。
		before, e := request(http.MethodGet, "", nil, nil)
		if e != nil {
			return e
		}
		if before["status"] == json.Number("0") {
			baseline.Version, _ = before["prepared_version"].(string)
			if baseline.Version == "" {
				return fmt.Errorf("waiting-publish matrix returned no prepared version; no baseline recovered")
			}
			baseline.Rows, e = readRows()
			if e != nil {
				return e
			}
			after, e := request(http.MethodGet, "", nil, nil)
			if e != nil {
				return e
			}
			current, e := readRows()
			if e != nil {
				return e
			}
			if after["status"] != json.Number("0") || after["prepared_version"] != baseline.Version || !reflect.DeepEqual(current, baseline.Rows) {
				return fmt.Errorf("matrix changed while recovering publication baseline; query and confirm again")
			}
			return persistBaseline(true)
		}
		baseline.Rows, err = readRows()
		if err != nil {
			return err
		}
		if _, err = request(http.MethodPatch, "/pre_release", nil, body); err != nil {
			return err
		}
		detail, e := request(http.MethodGet, "", nil, nil)
		if e != nil {
			return e
		}
		baseline.Version, _ = detail["prepared_version"].(string)
		if baseline.Version == "" {
			return fmt.Errorf("pre-release returned no prepared version; no baseline saved")
		}
		// 预发布期间有数据变化时不建立可发布基准。
		current, e := readRows()
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(current, baseline.Rows) {
			return fmt.Errorf("rows changed during pre-release; pre-release and confirm again")
		}
		return persistBaseline(false)
	}
	encoded, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("no valid publication baseline; %s: %w", matrixBaselineRecoveryHint, err)
	}
	if json.Unmarshal(encoded, &baseline) != nil || baseline.Binding != binding || baseline.Version == "" || baseline.Rows == nil {
		return fmt.Errorf("invalid publication baseline; %s", matrixBaselineRecoveryHint)
	}
	// 读取时再次规范化，兼容旧版本已保存但部门成员顺序未排序的有效发布基准。
	matrixExactValues(baseline.Rows)
	// 一旦开始正式发布检查即消费基准；失败或结果不确定均需重新确认，禁止盲目重试。
	if err = os.Remove(file); err != nil {
		return err
	}
	detail, err := request(http.MethodGet, "", nil, nil)
	if err != nil {
		return err
	}
	if detail["prepared_version"] != baseline.Version || detail["status"] != json.Number("0") {
		return fmt.Errorf("prepared version changed; pre-release and confirm again")
	}
	rows, err := readRows()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(rows, baseline.Rows) {
		return fmt.Errorf("matrix rows added, deleted or changed; pre-release and confirm again")
	}
	if _, err = request(http.MethodPatch, "/release", nil, body); err != nil {
		return err
	}
	detail, err = request(http.MethodGet, "", nil, nil)
	if err != nil {
		return err
	}
	if detail["release_version"] != baseline.Version {
		return fmt.Errorf("release result uncertain: version verification failed; query before further actions")
	}
	return a.renderApprovalMatrixImportValue(options, map[string]any{"code": 0, "msg": "success", "data": map[string]any{"release_version": baseline.Version}})
}
