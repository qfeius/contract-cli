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

/* matrixExactValues 递归规范化数值，避免 float64 损失及 1000/1000.0 误判；v 为 JSON 值，返回规范化值。 */
func matrixExactValues(v any) any {
	switch x := v.(type) {
	case json.Number:
		if n, ok := new(big.Rat).SetString(string(x)); ok {
			return map[string]any{"$number": n.RatString()}
		}
	case map[string]any:
		for k, item := range x {
			x[k] = matrixExactValues(item)
		}
	case []any:
		for i, item := range x {
			x[i] = matrixExactValues(item)
		}
	}
	return v
}

/* matrixRowsPage 将一页行按行列 ID 归一化并拒绝重复或残缺数据；data 为接口数据，rows 为累计基准，返回下一页标识或错误。 */
func matrixRowsPage(data map[string]any, rows map[string]any) (string, error) {
	items, ok := data["table_rows"].([]any)
	if !ok {
		return "", fmt.Errorf("incomplete matrix rows page")
	}
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return "", fmt.Errorf("invalid matrix row")
		}
		id, ok := row["id"].(string)
		if !ok || id == "" {
			return "", fmt.Errorf("missing row id")
		}
		if _, exists := rows[id]; exists {
			return "", fmt.Errorf("duplicate row id %s", id)
		}
		cells, ok := row["table_cells"].([]any)
		if !ok {
			return "", fmt.Errorf("missing row cells")
		}
		indexed := map[string]any{}
		for _, value := range cells {
			cell, ok := value.(map[string]any)
			if !ok {
				return "", fmt.Errorf("invalid cell")
			}
			column, ok := cell["table_column_id"].(string)
			if !ok || column == "" {
				return "", fmt.Errorf("missing column id")
			}
			if _, exists := indexed[column]; exists {
				return "", fmt.Errorf("duplicate column id")
			}
			if _, ok := cell["table_cell_content"].(map[string]any); !ok {
				return "", fmt.Errorf("missing cell content")
			}
			indexed[column] = cell
		}
		row["table_cells"] = indexed
		rows[id] = matrixExactValues(row)
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

/* publishMatrixWithRowBaseline 编排原有发布接口并保存行基准；ctx/options 为调用上下文，action/path 为动作与矩阵路径，body 为原请求体；返回执行或核验错误。 */
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
	readRows := func() (map[string]any, error) {
		rows := map[string]any{}
		seen := map[string]bool{}
		token := ""
		for page := 0; page < 10000; page++ {
			query := url.Values{"page_size": {"100"}}
			if token != "" {
				query.Set("page_token", token)
			}
			data, e := request(http.MethodGet, "/table_rows", query, nil)
			if e != nil {
				return nil, e
			}
			next, e := matrixRowsPage(data, rows)
			if e != nil {
				return nil, e
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
	baseline := matrixPublishBaseline{Binding: binding}
	if action == "pre-release" {
		// 先废弃旧基准，任何预发布失败都不允许复用历史确认。
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
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
		encoded, e := json.Marshal(baseline)
		if e != nil {
			return e
		}
		if e = os.WriteFile(file, encoded, 0600); e != nil {
			return e
		}
		return a.renderApprovalMatrixImportValue(options, map[string]any{"code": 0, "msg": "success", "data": map[string]any{"prepared_version": baseline.Version, "row_count": len(baseline.Rows), "requires_release_confirmation": true}})
	}
	encoded, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("no valid publication baseline; pre-release and confirm again: %w", err)
	}
	if json.Unmarshal(encoded, &baseline) != nil || baseline.Binding != binding || baseline.Version == "" || baseline.Rows == nil {
		return fmt.Errorf("invalid publication baseline; pre-release and confirm again")
	}
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
