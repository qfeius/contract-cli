package cli_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
)

func TestReviewInterruptJSONInputBudget(t *testing.T) {
	const limit = 2 << 20
	for _, source := range []string{"data", "input-file"} {
		for _, extra := range []int{0, 1} {
			t.Run(source+strconv.Itoa(extra), func(t *testing.T) {
				body := alignmentStart()
				body += strings.Repeat(" ", limit+extra-len(body))
				value := body
				if source == "input-file" {
					value = filepath.Join(t.TempDir(), "input.json")
					if err := os.WriteFile(value, []byte(body), 0600); err != nil {
						t.Fatal(err)
					}
				}
				app, _ := alignmentApp(t, nil)
				err := app.Run(context.Background(), []string{"review", "task", "start", "--" + source, value, "--dry-run"})
				if extra == 0 && err != nil {
					t.Fatalf("exact 2 MiB JSON rejected: %v", err)
				}
				if extra > 0 && (err == nil || !strings.Contains(err.Error(), "JSON 输入不能超过 2 MiB")) {
					t.Fatalf("JSON limit or error changed: %v", err)
				}
			})
		}
	}
}

func TestReviewInterruptRemovedStdinUploadNeverReads(t *testing.T) {
	const limit = 50 << 20
	for _, size := range []int64{limit, limit + 4096} {
		t.Run(strconv.FormatInt(size, 10), func(t *testing.T) {
			reader := &reviewInterruptBudgetReader{remaining: size}
			calls := 0
			app := newReviewInterruptApp(t, reader, func(request *http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(`{"code":200,"data":{}}`), nil
			})
			err := app.Run(context.Background(), []string{"review", "file", "upload", "--stdin", "--name", "contract.pdf"})
			if err == nil || !strings.Contains(err.Error(), "仅支持 CLM 文件引用") || calls != 0 || reader.read != 0 {
				t.Fatalf("removed stdin upload performed I/O: calls=%d read=%d err=%v", calls, reader.read, err)
			}
		})
	}
}

func TestReviewInterruptCancelledCommandDoesNotReadRemovedStdin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()
	reader := &reviewInterruptBudgetReader{remaining: 1 << 20, cancel: cancel}
	app := newReviewInterruptApp(t, reader, nil)
	err := app.Run(ctx, []string{"review", "file", "upload", "--stdin", "--name", "contract.pdf"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("injected reader lost cancellation: %v", err)
	}
	if reader.read != 0 {
		t.Fatalf("injected reader kept reading after context cancellation: bytes=%d, err=%v", reader.read, err)
	}
}

type reviewInterruptBudgetReader struct {
	remaining, read int64
	cancel          context.CancelFunc
}

func (reader *reviewInterruptBudgetReader) Read(body []byte) (int, error) {
	if reader.remaining == 0 {
		return 0, io.EOF
	}
	size := min(int64(len(body)), reader.remaining)
	if reader.cancel != nil {
		size = min(size, 512)
		reader.cancel()
	}
	clear(body[:size])
	reader.remaining -= size
	reader.read += size
	return int(size), nil
}

func newReviewInterruptApp(t *testing.T, stdin io.Reader, transport func(*http.Request) (*http.Response, error)) *cli.App {
	t.Helper()
	store := config.NewStore(t.TempDir())
	if err := store.UpsertProfile(uploadProfile(config.IdentityUser), true); err != nil {
		t.Fatal(err)
	}
	if transport == nil {
		transport = func(*http.Request) (*http.Response, error) {
			t.Error("cancelled input reached a business request")
			return jsonResponse(`{"code":200,"data":{}}`), nil
		}
	}
	return cli.New(cli.Options{Store: store, Stdin: stdin, Stdout: io.Discard, Stderr: io.Discard,
		HTTPClient:         &http.Client{Transport: roundTripFunc(transport)},
		LookupEnv:          func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
	})
}
