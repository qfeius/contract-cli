package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
)

func TestBlueOpenWebCommandTraceAndDebug(t *testing.T) {
	for _, debug := range []bool{false, true} {
		t.Run(map[bool]string{false: "quiet", true: "debug-json"}[debug], func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			profile := productionProfileFixture(config.Profile{Name: "blue-web-trace", DefaultIdentity: config.IdentityUser})
			profile.Identities.User.Token = &config.Token{AccessToken: "blue-web-fixture-token", Expiry: time.Now().Add(time.Hour)}
			profile.CredentialBinding = config.CredentialTargetBinding(profile)
			if err := store.UpsertProfile(profile, true); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			var traceID string
			app := cli.New(cli.Options{Store: store, Stdout: &stdout, Stderr: &stderr,
				LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if request.URL.Path != "/open-web/web/clm/contract/info" || request.Header.Get("Authorization") != "Bearer blue-web-fixture-token" {
						t.Fatalf("unexpected Open Web request: %s", request.URL.Path)
					}
					traceID = request.Header.Get("X-Log-Id")
					parent := strings.Split(request.Header.Get("traceparent"), "-")
					if len(traceID) != 32 || len(parent) != 4 || parent[1] != traceID {
						t.Fatalf("Open Web correlation mismatch: trace=%q parent=%v", traceID, parent)
					}
					return jsonResponse(`{"code":0,"data":{"items":[]}}`), nil
				})},
			})
			args := []string{"api", "call", "--channel", "web", "--profile", profile.Name, "--as", "user", "--output", "json", "GET", "/open-web/web/clm/contract/info"}
			if debug {
				args = append(args, "--debug")
			}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if traceID == "" || !json.Valid(stdout.Bytes()) {
				t.Fatalf("missing request or JSON result: trace=%q stdout=%s", traceID, &stdout)
			}
			if !debug {
				if stderr.Len() != 0 {
					t.Fatalf("default Open Web request emitted diagnostic logs: %s", &stderr)
				}
				return
			}
			boundaries := map[string]bool{}
			for _, line := range bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n")) {
				var record map[string]any
				if err := json.Unmarshal(line, &record); err != nil {
					t.Fatalf("debug record is not JSON: %s", line)
				}
				message, _ := record["msg"].(string)
				if message == "command started" || message == "command completed" {
					boundaries[message] = record["trace_id"] == traceID
				}
			}
			if !boundaries["command started"] || !boundaries["command completed"] {
				t.Fatalf("Open Web command boundaries did not share request trace: %s", &stderr)
			}
		})
	}
}
