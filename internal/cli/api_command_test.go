package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestAPICommandIsTemporarilyUnavailable(t *testing.T) {
	t.Parallel()

	app := cli.New(cli.Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Store:  config.NewStore(t.TempDir()),
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("api command should not send HTTP request to %s", req.URL.String())
				return nil, nil
			}),
		},
	})

	err := app.Run(context.Background(), []string{
		"api", "call", "GET", "/open-apis/mdm/v1/vendors/1", "--profile", "contract-group",
	})
	if err == nil || !strings.Contains(err.Error(), "api call 暂未开放使用") {
		t.Fatalf("unexpected api unavailable error: %v", err)
	}
}

func TestAPICommandHelpIsNotExposed(t *testing.T) {
	t.Parallel()

	testCases := [][]string{
		{"help", "api"},
		{"help", "api", "call"},
	}
	for _, args := range testCases {
		args := args
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			t.Parallel()

			app := cli.New(cli.Options{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
				Store:  config.NewStore(t.TempDir()),
			})
			err := app.Run(context.Background(), args)
			if err == nil || !strings.Contains(err.Error(), "unknown help topic") {
				t.Fatalf("unexpected help error: %v", err)
			}
		})
	}
}
