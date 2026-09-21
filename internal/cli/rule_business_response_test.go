package cli_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

/*
TestRuleBusinessResponseExitError 验证矩阵 HTTP 200 下的非零业务码会转换为命令错误。
参数 t（*testing.T）为测试上下文；无返回值，覆盖失败响应保留和成功响应退出两条路径。
*/
func TestRuleBusinessResponseExitError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		code    string
		wantErr bool
	}{
		{name: "missing table", code: "20003", wantErr: true},
		{name: "missing row", code: "20021", wantErr: true},
		{name: "invalid parameter", code: "100000", wantErr: true},
		{name: "success", code: "0", wantErr: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
				t.Fatal(err)
			}
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Store: store, Stdout: stdout, Stderr: io.Discard,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return jsonResponse(`{"code":` + tc.code + `,"msg":"fixture detail","data":null}`), nil
				})},
			})
			err := app.Run(context.Background(), []string{
				"rule", "table", "row", "list",
				"--profile", "contract", "--as", "app",
				"--product-id", "contract", "--group-id", "approve_matrix", "--table-id", "table-1",
				"--page-size", "0",
			})
			if (err != nil) != tc.wantErr {
				t.Fatalf("Run() error = %v, want error = %v", err, tc.wantErr)
			}
			if !strings.Contains(stdout.String(), tc.code) || !strings.Contains(stdout.String(), "fixture detail") {
				t.Fatalf("response was not preserved: %s", stdout.String())
			}
			if err != nil && !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("error = %v, want business code %s", err, tc.code)
			}
		})
	}
}
