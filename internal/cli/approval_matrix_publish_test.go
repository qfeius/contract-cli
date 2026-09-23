package cli_test

import (
	"bytes"
	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/* TestMatrixPublishBaseline 覆盖分页发布基准、精度及异常阻断；t 为测试上下文，无返回值。 */
func TestMatrixPublishBaseline(t *testing.T) {
	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		for _, scenario := range []string{"unchanged", "changed", "department-order-pre", "department-order", "legacy-department-order", "department-changed", "added", "deleted", "version", "pagination", "duplicate", "query-error", "identity", "matrix", "missing", "pre-failed", "release-failed"} {
			t.Run(string(identity)+"/"+scenario, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
					t.Fatal(err)
				}
				releasing := false
				prepared := false
				published := false
				writes := 0
				row := func(id, amount, departments string) string {
					return fmt.Sprintf(`{"id":%q,"table_cells":[{"table_column_id":"c","table_cell_content_type":"NUMBER","table_cell_content":{"number":%s}},{"table_column_id":"p","table_cell_content_type":"EMPLOYEE_COLLECTION","table_cell_content":{"employee_collection":[1069134472156283974]}},{"table_column_id":"d","table_cell_content_type":"DEPARTMENT_COLLECTION","table_cell_content":{"department_collection":%s}}]}`, id, amount, departments)
				}
				app := cli.New(cli.Options{Store: store, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if got := r.URL.Query().Get("user_id"); got != "" {
						t.Fatalf("matrix request must use authenticated editor, user_id query = %q", got)
					}
					data := `{}`
					code := 0
					switch {
					case strings.HasSuffix(r.URL.Path, "/pre_release"):
						if r.Method != http.MethodPatch {
							t.Fatal("expected PATCH")
						}
						if scenario == "pre-failed" {
							code = 1
						} else {
							prepared = true
						}
					case strings.HasSuffix(r.URL.Path, "/release"):
						if r.Method != http.MethodPatch {
							t.Fatal("expected PATCH")
						}
						writes++
						published = true
						if scenario == "release-failed" {
							return nil, fmt.Errorf("connection lost")
						}
					case strings.HasSuffix(r.URL.Path, "/table_rows"):
						if releasing && scenario == "query-error" {
							return nil, fmt.Errorf("read failed")
						}
						first := r.URL.Query().Get("page_token") == ""
						id := "a"
						amount := "100000000000000000000000000001.12345678"
						departments := `["od-a","od-b"]`
						more := true
						token := `"next"`
						if !first {
							id = "b"
							more = false
							token = `null`
						}
						if releasing && scenario == "unchanged" {
							if first {
								id = "b"
							} else {
								id = "a"
							}
							amount += "0"
						}
						if releasing && scenario == "changed" {
							amount = "100000000000000000000000000001.12345679"
						}
						// 部门集合由开平 flatMap 聚合，返回顺序不稳定；仅换序不应被识别为规则变化。
						if (prepared && scenario == "department-order-pre") || (releasing && scenario == "department-order") {
							departments = `["od-b","od-a"]`
						}
						// 集合成员真实变化仍必须触发发布保护。
						if releasing && scenario == "department-changed" {
							departments = `["od-a","od-c"]`
						}
						if releasing && scenario == "added" {
							id += "new"
						}
						if releasing && scenario == "duplicate" {
							id = "a"
						}
						if releasing && scenario == "pagination" {
							token = `null`
							more = true
						}
						rows := row(id, amount, departments)
						if releasing && scenario == "deleted" && !first {
							rows = ""
						}
						data = fmt.Sprintf(`{"table_rows":[%s],"has_more":%v,"page_token":%s}`, rows, more, token)
					default:
						version := "V1.0"
						if releasing && scenario == "version" {
							version = "V2.0"
						}
						status := 2
						if prepared {
							status = 0
						}
						release := ""
						if published {
							status = 1
							release = "V1.0"
						}
						data = fmt.Sprintf(`{"prepared_version":%q,"release_version":%q,"status":%d}`, version, release, status)
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"code":%d,"data":%s}`, code, data)))}, nil
				})}})
				args := func(action string) []string {
					return []string{"rule", "table", action, "--profile", "contract", "--as", string(identity), "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "t", "--user-id", "another-user"}
				}
				if scenario != "missing" {
					err := app.Run(context.Background(), args("pre-release"))
					if scenario == "pre-failed" {
						if err == nil {
							t.Fatal("expected pre failure")
						}
					} else if err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "legacy-department-order" {
					// 模拟旧版本保存的未规范化基准，确保升级后仍可按集合语义比较已有快照。
					dir := filepath.Join(filepath.Dir(store.Path()), "approval-matrix-publish")
					entries, err := os.ReadDir(dir)
					if err != nil {
						t.Fatalf("read legacy baseline: entries=%d err=%v", len(entries), err)
					}
					path := ""
					for _, entry := range entries {
						if filepath.Ext(entry.Name()) == ".json" {
							path = filepath.Join(dir, entry.Name())
						}
					}
					if path == "" {
						t.Fatal("legacy publication baseline not found")
					}
					content, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					legacy := strings.Replace(string(content), `["od-a","od-b"]`, `["od-b","od-a"]`, 1)
					if legacy == string(content) {
						t.Fatal("baseline does not contain department collection")
					}
					if err = os.WriteFile(path, []byte(legacy), 0600); err != nil {
						t.Fatal(err)
					}
				}
				releasing = true
				releaseArgs := args("release")
				if scenario == "identity" {
					if identity == config.IdentityUser {
						releaseArgs[6] = "app"
					} else {
						releaseArgs[6] = "user"
					}
				}
				if scenario == "matrix" {
					releaseArgs[len(releaseArgs)-3] = "other"
				}
				err := app.Run(context.Background(), releaseArgs)
				if scenario == "unchanged" || scenario == "department-order-pre" || scenario == "department-order" || scenario == "legacy-department-order" {
					if err != nil || writes != 1 {
						t.Fatalf("release: %v writes %d", err, writes)
					}
				} else {
					if err == nil {
						t.Fatal("expected blocked release")
					}
					if scenario == "missing" && !strings.Contains(err.Error(), "read-only recover the local baseline without rewriting rules") {
						t.Fatalf("missing baseline recovery guidance = %v", err)
					}
					if scenario != "release-failed" && writes != 0 {
						t.Fatal("unsafe release request")
					}
				}
				if err := app.Run(context.Background(), releaseArgs); err == nil {
					t.Fatal("baseline must not permit repeated release")
				}
			})
		}
	}
}

/*
TestMatrixPreReleaseRecoversServerPreparedBaseline 验证本地基准丢失而服务端仍待发布时，只读恢复基准即可继续发布。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestMatrixPreReleaseRecoversServerPreparedBaseline(t *testing.T) {
	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		t.Run(string(identity), func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
				t.Fatal(err)
			}

			// 模拟 profile 重建后的真实状态：本地没有基准，服务端已持有 V1.0 待发布版本。
			published := false
			detailReads := 0
			rowReads := 0
			preReleaseWrites := 0
			rowWrites := 0
			releaseWrites := 0
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Store:  store,
				Stdout: stdout,
				Stderr: &bytes.Buffer{},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					data := `{}`
					code := 0
					msg := "success"
					switch {
					case strings.HasSuffix(r.URL.Path, "/pre_release"):
						preReleaseWrites++
						code = 20026
						msg = "当前审批矩阵在预发布状态不能再次预发布"
					case strings.HasSuffix(r.URL.Path, "/release"):
						releaseWrites++
						published = true
					case strings.HasSuffix(r.URL.Path, "/table_rows"):
						if r.Method != http.MethodGet {
							rowWrites++
						}
						rowReads++
						data = `{"table_rows":[{"id":"row-1","table_cells":[{"table_column_id":"condition-1","table_cell_content_type":"COLLECTION","table_cell_content":{"collection":["category-1"]}}]}],"has_more":false,"page_token":null}`
					default:
						detailReads++
						status := 0
						releaseVersion := ""
						if published {
							status = 1
							releaseVersion = "V1.0"
						}
						data = fmt.Sprintf(`{"prepared_version":"V1.0","release_version":%q,"status":%d}`, releaseVersion, status)
					}
					return jsonResponse(fmt.Sprintf(`{"code":%d,"msg":%q,"data":%s}`, code, msg, data)), nil
				})},
			})

			args := func(action string) []string {
				return []string{"rule", "table", action, "--profile", "contract", "--as", string(identity), "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "batch_import_test_20260922_1130"}
			}
			if err := app.Run(context.Background(), args("pre-release")); err != nil {
				t.Fatalf("recover pre-release baseline: %v", err)
			}
			if preReleaseWrites != 0 || rowWrites != 0 {
				t.Fatalf("recovery mutated server state: pre_release=%d row_writes=%d", preReleaseWrites, rowWrites)
			}
			if detailReads != 2 || rowReads != 2 {
				t.Fatalf("recovery must double-read stable state: details=%d rows=%d", detailReads, rowReads)
			}
			if !strings.Contains(stdout.String(), `"baseline_recovered": true`) {
				t.Fatalf("recovery output = %s", stdout.String())
			}

			stdout.Reset()
			if err := app.Run(context.Background(), args("release")); err != nil {
				t.Fatalf("release after recovery: %v", err)
			}
			if releaseWrites != 1 || rowWrites != 0 {
				t.Fatalf("release writes = %d, row writes = %d", releaseWrites, rowWrites)
			}
		})
	}
}

/*
TestMatrixPreReleaseRecoveryRejectsUnstableState 验证恢复期间版本、状态或规则行变化时不保存可发布基准。
入参 t（*testing.T）为测试上下文；返回值为空。
*/
func TestMatrixPreReleaseRecoveryRejectsUnstableState(t *testing.T) {
	for _, scenario := range []string{"status", "version", "rows"} {
		t.Run(scenario, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}

			// 两轮只读快照之间只改变一个维度，确保每种不稳定状态都阻断基准落盘。
			detailReads := 0
			rowReads := 0
			preReleaseWrites := 0
			releaseWrites := 0
			app := cli.New(cli.Options{
				Store:  store,
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					data := `{}`
					switch {
					case strings.HasSuffix(r.URL.Path, "/pre_release"):
						preReleaseWrites++
					case strings.HasSuffix(r.URL.Path, "/release"):
						releaseWrites++
					case strings.HasSuffix(r.URL.Path, "/table_rows"):
						rowReads++
						rowID := "row-1"
						if scenario == "rows" && rowReads == 2 {
							rowID = "row-2"
						}
						data = fmt.Sprintf(`{"table_rows":[{"id":%q,"table_cells":[]}],"has_more":false,"page_token":null}`, rowID)
					default:
						detailReads++
						status := 0
						version := "V1.0"
						if detailReads == 2 && scenario == "status" {
							status = 2
						}
						if detailReads == 2 && scenario == "version" {
							version = "V2.0"
						}
						data = fmt.Sprintf(`{"prepared_version":%q,"release_version":"","status":%d}`, version, status)
					}
					return jsonResponse(fmt.Sprintf(`{"code":0,"msg":"success","data":%s}`, data)), nil
				})},
			})

			args := func(action string) []string {
				return []string{"rule", "table", action, "--profile", "contract", "--as", "app", "--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "batch_import_test_20260922_1130"}
			}
			err := app.Run(context.Background(), args("pre-release"))
			if err == nil || !strings.Contains(err.Error(), "matrix changed while recovering publication baseline") {
				t.Fatalf("unstable recovery error = %v", err)
			}
			if preReleaseWrites != 0 {
				t.Fatalf("unstable recovery sent pre-release writes = %d", preReleaseWrites)
			}

			// 恢复失败后 release 必须在本地阻断，证明没有遗留可发布基准。
			err = app.Run(context.Background(), args("release"))
			if err == nil || !strings.Contains(err.Error(), "read-only recover the local baseline without rewriting rules") {
				t.Fatalf("release after unstable recovery = %v", err)
			}
			if releaseWrites != 0 {
				t.Fatalf("release request sent after unstable recovery = %d", releaseWrites)
			}
		})
	}
}
