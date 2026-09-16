package cli_test

import (
	"bytes"
	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

/* TestMatrixPublishBaseline 覆盖分页发布基准、精度及异常阻断；t 为测试上下文，无返回值。 */
func TestMatrixPublishBaseline(t *testing.T) {
	for _, identity := range []config.IdentityKind{config.IdentityUser, config.IdentityApp} {
		for _, scenario := range []string{"unchanged", "changed", "added", "deleted", "version", "pagination", "duplicate", "query-error", "identity", "matrix", "missing", "pre-failed", "release-failed"} {
			t.Run(string(identity)+"/"+scenario, func(t *testing.T) {
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(identity), true); err != nil {
					t.Fatal(err)
				}
				releasing := false
				published := false
				writes := 0
				row := func(id, amount string) string {
					return fmt.Sprintf(`{"id":%q,"table_cells":[{"table_column_id":"c","table_cell_content_type":"NUMBER","table_cell_content":{"number":%s}},{"table_column_id":"p","table_cell_content_type":"EMPLOYEE_COLLECTION","table_cell_content":{"employee_collection":[1069134472156283974]}}]}`, id, amount)
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
						rows := row(id, amount)
						if releasing && scenario == "deleted" && !first {
							rows = ""
						}
						data = fmt.Sprintf(`{"table_rows":[%s],"has_more":%v,"page_token":%s}`, rows, more, token)
					default:
						version := "V1.0"
						if releasing && scenario == "version" {
							version = "V2.0"
						}
						release := ""
						if published {
							release = "V1.0"
						}
						data = fmt.Sprintf(`{"prepared_version":%q,"release_version":%q,"status":0}`, version, release)
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
				if scenario == "unchanged" {
					if err != nil || writes != 1 {
						t.Fatalf("release: %v writes %d", err, writes)
					}
				} else {
					if err == nil {
						t.Fatal("expected blocked release")
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
