package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestContractMCPApprovalBusinessFailuresReturnErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		code string
	}{
		{"process detail", []string{"approval", "get", "112233"}, "110125"},
		{"comment list", []string{"approval", "comment", "list", "112233"}, "110125"},
		{"comment create", []string{"approval", "comment", "create", "112233", "--data", `{"content":"review fixture"}`}, "110603"},
		{"task list", []string{"approval", "task", "list"}, "110000"},
		{"task approve", []string{"approval", "task", "approve", "112233"}, "110125"},
		{"task reject", []string{"approval", "task", "reject", "112233", "--comment", "review fixture"}, "110507"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := `{"code":` + tc.code + `,"msg":"private server detail","success":false,"data":null}`
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := cli.New(cli.Options{
				Stdout: stdout, Stderr: stderr, Store: store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					return jsonResponse(body), nil
				})},
			})
			args := append([]string{"contract"}, tc.args...)
			args = append(args, "--profile", "contract", "--as", "user")
			err := app.Run(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("Run() error = %v, want business code %s", err, tc.code)
			}
			if calls != 1 {
				t.Fatalf("business failure sent %d requests, want 1", calls)
			}
			if !strings.Contains(stdout.String(), `"success": false`) || !strings.Contains(stdout.String(), "private server detail") {
				t.Fatalf("error response was lost: %s", stdout.String())
			}
			if strings.Contains(stderr.String()+err.Error(), "private server detail") {
				t.Fatal("business error leaks response content into logs or error summary")
			}
		})
	}
}

func TestContractMCPResponseValidatesEnvelopeAndPreservesOutput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"success", `{"code":0,"success":true,"data":{"contract_id":7420000000000000001}}`, false},
		{"optional success", `{"code":0,"data":null}`, false},
		{"false overrides zero", `{"code":0,"success":false}`, true},
		{"nonzero overrides true", `{"code":110125,"success":true}`, true},
		{"nonzero without success", `{"code":110125}`, true},
		{"missing code", `{"success":true}`, true},
		{"null code", `{"code":null,"success":true}`, true},
		{"string code", `{"code":"0"}`, true},
		{"wrong success type", `{"code":0,"success":"true"}`, true},
		{"empty response", "", true},
		{"null response", `null`, true},
		{"array response", `[]`, true},
		{"malformed response", `{"code":0`, true},
		{"trailing response", `{"code":0}{"code":110125}`, true},
	}
	for _, tc := range cases {
		for _, format := range []string{"json", "yaml", "raw"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				t.Parallel()
				store := config.NewStore(t.TempDir())
				if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
					t.Fatal(err)
				}
				stdout := &bytes.Buffer{}
				app := cli.New(cli.Options{
					Stdout: stdout, Stderr: io.Discard, Store: store,
					HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						return jsonResponse(tc.body), nil
					})},
				})
				args := []string{"contract", "approval", "task", "list", "--profile", "contract"}
				if format == "raw" {
					args = append(args, "--raw")
				} else {
					args = append(args, "--output", format)
				}
				err := app.Run(context.Background(), args)
				if (err != nil) != tc.wantErr {
					t.Fatalf("Run() error = %v, wantErr %v", err, tc.wantErr)
				}
				if format == "raw" && stdout.String() != tc.body {
					t.Fatalf("raw response = %q, want %q", stdout.String(), tc.body)
				}
				if strings.Contains(tc.body, "7420000000000000001") && !strings.Contains(stdout.String(), "7420000000000000001") {
					t.Fatalf("long ID was changed: %s", stdout.String())
				}
				if json.Valid([]byte(tc.body)) && strings.Contains(tc.body, "110125") && !strings.Contains(stdout.String(), "110125") {
					t.Fatalf("business code missing from output: %s", stdout.String())
				}
			})
		}
	}
}

func TestContractMCPResponseKeepsNonDownloadLegacyBehavior(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		identity config.IdentityKind
		args     []string
		path     string
	}{
		{"app approval", config.IdentityApp, []string{"approval", "get", "112233"}, "/open-apis/contract/v1/process_instances/112233"},
		{"app download", config.IdentityApp, []string{"download-file", "112233", "--raw"}, "/open-apis/contract/v1/files/112233"},
		{"existing user command", config.IdentityUser, []string{"get", "112233"}, "/open-apis/contract/v1/mcp/contracts/112233"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(tc.identity), true); err != nil {
				t.Fatal(err)
			}
			stdout := &bytes.Buffer{}
			app := cli.New(cli.Options{
				Stdout: stdout, Stderr: io.Discard, Store: store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != tc.path {
						t.Fatalf("path = %q, want %q", req.URL.Path, tc.path)
					}
					return jsonResponse(`{"code":110125,"success":false}`), nil
				})},
			})
			args := append([]string{"contract"}, tc.args...)
			args = append(args, "--profile", "contract")
			err := app.Run(context.Background(), args)
			if tc.name == "app download" {
				if err == nil || !strings.Contains(err.Error(), "business code 110125") || stdout.Len() != 0 {
					t.Fatalf("download business failure was not suppressed: err=%v stdout=%s", err, stdout)
				}
				return
			}
			if err != nil {
				t.Fatalf("legacy response behavior changed: %v", err)
			}
			if !strings.Contains(stdout.String(), "110125") {
				t.Fatalf("legacy response missing: %s", stdout.String())
			}
		})
	}
}

func TestContractMCPResponsePropagatesOutputFailure(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"code":0}`, `{"code":110125}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			store := config.NewStore(t.TempDir())
			if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
				t.Fatal(err)
			}
			outputErr := errors.New("output unavailable")
			app := cli.New(cli.Options{
				Stdout: failingMCPOutput{err: outputErr}, Stderr: io.Discard, Store: store,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return jsonResponse(body), nil
				})},
			})
			err := app.Run(context.Background(), []string{"contract", "approval", "task", "list", "--profile", "contract"})
			if !errors.Is(err, outputErr) {
				t.Fatalf("Run() error = %v, want output error", err)
			}
		})
	}
}

type failingMCPOutput struct{ err error }

func (w failingMCPOutput) Write([]byte) (int, error) { return 0, w.err }
