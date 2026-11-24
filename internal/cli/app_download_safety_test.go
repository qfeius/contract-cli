package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestAppDownloadsPreserveFilesOnFailure(t *testing.T) {
	for _, command := range [][]string{{"contract", "download-file", "1"}, {"mdm", "file", "download", "1"}, {"contract", "cooperation", "file", "download", "1"}} {
		for _, mode := range []string{"http", "short", "broken", "cancel", "success", "concurrent"} {
			for _, existing := range []bool{false, true} {
				t.Run(strings.Join(command, "-")+"/"+mode+map[bool]string{false: "/new", true: "/replace"}[existing], func(t *testing.T) {
					dir := t.TempDir()
					target := filepath.Join(dir, "saved.pdf")
					if existing {
						if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					store := config.NewStore(t.TempDir())
					if err := store.UpsertProfile(uploadProfile(config.IdentityApp), true); err != nil {
						t.Fatal(err)
					}
					calls := 0
					var out bytes.Buffer
					app := cli.New(cli.Options{Store: store, Stdout: &out, Stderr: io.Discard, HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						calls++
						if mode == "concurrent" && !existing {
							if err := os.WriteFile(target, []byte("concurrent"), 0600); err != nil {
								t.Fatal(err)
							}
						}
						status, size := 200, int64(4)
						reader := io.Reader(strings.NewReader("file"))
						switch mode {
						case "http":
							status = 500
						case "short":
							size = 20
						case "broken":
							reader = io.MultiReader(reader, appDownloadErrorReader{io.ErrUnexpectedEOF})
						case "cancel":
							reader = io.MultiReader(reader, appDownloadErrorReader{context.Canceled})
						}
						return &http.Response{StatusCode: status, Header: make(http.Header), ContentLength: size, Body: io.NopCloser(reader)}, nil
					})}})
					args := append(append([]string{}, command...), "--as", "app", "--output-file", target)
					if existing {
						args = append(args, "--force")
					}
					err := app.Run(context.Background(), args)
					wantSuccess := mode == "success" || (mode == "concurrent" && existing)
					if (err == nil) != wantSuccess || calls != 1 {
						t.Fatalf("err=%v HTTP=%d", err, calls)
					}
					body, readErr := os.ReadFile(target)
					switch {
					case wantSuccess:
						if readErr != nil || string(body) != "file" {
							t.Fatalf("successful download: %q %v", body, readErr)
						}
					case existing:
						if readErr != nil || string(body) != "original" {
							t.Fatalf("lost original: %q %v", body, readErr)
						}
					case mode == "concurrent":
						if readErr != nil || string(body) != "concurrent" {
							t.Fatalf("overwrote concurrent file: %q %v", body, readErr)
						}
					default:
						if !errors.Is(readErr, os.ErrNotExist) {
							t.Fatalf("left failed output: %q %v", body, readErr)
						}
					}
					parts, globErr := filepath.Glob(target + ".part-*")
					if globErr != nil || len(parts) != 0 {
						t.Fatalf("left temporary files: %v %v", parts, globErr)
					}
					if !wantSuccess && strings.Contains(out.String(), "Downloaded") {
						t.Fatal("reported failure as saved")
					}
				})
			}
		}
	}
}

type appDownloadErrorReader struct{ err error }

func (r appDownloadErrorReader) Read([]byte) (int, error) { return 0, r.err }
