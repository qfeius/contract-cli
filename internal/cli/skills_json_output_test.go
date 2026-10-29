package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"cn.qfei/contract-cli/internal/build"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/skillsync"
)

type skillsJSONTestOutput struct {
	OK   bool `json:"ok"`
	Data struct {
		CLI struct {
			Version    string `json:"version"`
			Executable string `json:"executable"`
		} `json:"cli"`
		Skills []struct {
			Directory   string `json:"directory"`
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
		} `json:"skills"`
		Target       string   `json:"target"`
		Installed    []string `json:"installed"`
		Skipped      []string `json:"skipped"`
		Registration struct {
			Path      string `json:"path"`
			Confirmed bool   `json:"confirmed"`
			Locations []struct {
				Target  string   `json:"target"`
				Status  string   `json:"status"`
				Managed []string `json:"managed_skills"`
			} `json:"locations"`
			ReadError string `json:"read_error"`
		} `json:"registration"`
		ClientReloadRequired bool `json:"client_reload_required"`
	} `json:"data"`
	Error struct {
		Type    string `json:"type"`
		Details struct {
			CLI struct {
				Version    string `json:"version"`
				Executable string `json:"executable"`
			} `json:"cli"`
			Target       string `json:"target"`
			Registration struct {
				Path      string `json:"path"`
				Confirmed bool   `json:"confirmed"`
				ReadError string `json:"read_error"`
			} `json:"registration"`
			Recovery struct {
				Status      string `json:"status"`
				Description string `json:"description"`
			} `json:"recovery"`
			Guidance string `json:"guidance"`
		} `json:"details"`
	} `json:"error"`
}

func skillsJSONTempDir(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return directory
}

func skillsJSONFixture(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	state := skillsJSONTempDir(t)
	out, diagnostic := &bytes.Buffer{}, &bytes.Buffer{}
	source := fstest.MapFS{
		"auth/SKILL.md":                  {Data: []byte("---\nname: auth\nversion: previous\ndescription: fixture auth\n---\n")},
		"auth/references/guide.md":       {Data: []byte("nested resource\n")},
		"contract-cli-rule/SKILL.md":     {Data: []byte("---\nname: contract-cli-rule\ndescription: fixture rule\n---\n")},
		"contract-cli-api-call/SKILL.md": {Data: []byte("---\nname: contract-cli-api-call\n---\n")},
	}
	app := New(Options{Store: config.NewStore(state), Secrets: config.NewSecretsStore(state), SkillsFS: source, Stdout: out, Stderr: diagnostic,
		LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
	})
	return app, out, diagnostic, state
}

func skillsJSONDecode(t *testing.T, body []byte) skillsJSONTestOutput {
	t.Helper()
	var decoded skillsJSONTestOutput
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("expected a single JSON document: %v body=%s", err, body)
	}
	return decoded
}

func TestSkillsJSONListSuccess(t *testing.T) {
	app, out, diagnostic, state := skillsJSONFixture(t)
	if code := app.Execute(context.Background(), []string{"skills", "list", "--output", "json"}); code != 0 {
		t.Fatalf("list code=%d stderr=%s", code, diagnostic)
	}
	result := skillsJSONDecode(t, out.Bytes())
	if !result.OK || result.Data.CLI.Executable == "" || result.Data.CLI.Version != build.Current().Version || len(result.Data.Skills) != 2 {
		t.Fatalf("missing list metadata: %s", out)
	}
	for _, skill := range result.Data.Skills {
		if skill.Directory == "" || skill.Name == "contract-cli-api-call" || skill.Version != build.Current().Version || skill.Description == "" {
			t.Fatalf("wrong public Skill metadata: %s", out)
		}
	}
	if diagnostic.Len() != 0 {
		t.Fatalf("default JSON list emitted logs: %s", diagnostic)
	}
	if _, err := os.Stat(filepath.Join(state, "skill-locations.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("list changed registry: %v", err)
	}
}

func TestSkillsJSONInstallSuccessRegistrationAndResources(t *testing.T) {
	for _, mode := range []string{"complete", "selected", "skip"} {
		t.Run(mode, func(t *testing.T) {
			app, out, diagnostic, state := skillsJSONFixture(t)
			target := filepath.Join(skillsJSONTempDir(t), "skills")
			args := []string{"skills", "install", "--target", target, "--output=json"}
			if mode == "selected" {
				args = append(args, "--name", "auth")
			}
			if mode == "skip" {
				if err := os.MkdirAll(filepath.Join(target, "auth"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "auth", "SKILL.md"), []byte("keep local"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if code := app.Execute(context.Background(), args); code != 0 {
				t.Fatalf("install code=%d stderr=%s", code, diagnostic)
			}
			result := skillsJSONDecode(t, out.Bytes())
			wantInstalled, wantSkipped := 2, 0
			if mode != "complete" {
				wantInstalled = 1
			}
			if mode == "skip" {
				wantSkipped = 1
			}
			if !result.OK || result.Data.CLI.Executable == "" || result.Data.Target != target || len(result.Data.Installed) != wantInstalled || len(result.Data.Skipped) != wantSkipped || result.Data.Registration.Confirmed != (mode == "complete") || result.Data.Registration.Path != filepath.Join(state, "skill-locations.json") || !result.Data.ClientReloadRequired {
				t.Fatalf("incorrect structured install result: %s", out)
			}
			if result.Data.Installed == nil || result.Data.Skipped == nil || result.Data.Registration.Locations == nil {
				t.Fatalf("known lists must use arrays: %s", out)
			}
			if mode == "complete" {
				if len(result.Data.Registration.Locations) != 1 || len(result.Data.Registration.Locations[0].Managed) != 2 || result.Data.Registration.Locations[0].Status != "success" {
					t.Fatalf("registration missing: %s", out)
				}
			} else if len(result.Data.Registration.Locations) != 0 {
				t.Fatalf("partial install claimed full registration: %s", out)
			}
			if mode == "skip" {
				body, err := os.ReadFile(filepath.Join(target, "auth", "SKILL.md"))
				if err != nil || string(body) != "keep local" {
					t.Fatal("skipped Skill changed")
				}
			} else {
				body, err := os.ReadFile(filepath.Join(target, "auth", "references", "guide.md"))
				if err != nil || string(body) != "nested resource\n" {
					t.Fatalf("nested resources missing: %v", err)
				}
			}
			if _, err := os.Stat(filepath.Join(target, "contract-cli-api-call")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("hidden Skill installed: %v", err)
			}
			if diagnostic.Len() != 0 {
				t.Fatalf("JSON install mixed user text in diagnostics: %s", diagnostic)
			}
		})
	}
}

func TestSkillsJSONInstallFailureKeepsStructuredDiagnostics(t *testing.T) {
	for _, mode := range []string{"unknown-name", "corrupt-registration", "invalid-parameter", "invalid-positional"} {
		t.Run(mode, func(t *testing.T) {
			app, out, diagnostic, state := skillsJSONFixture(t)
			target := filepath.Join(skillsJSONTempDir(t), "skills")
			args := []string{"skills", "install", "--target", target, "--output", "json"}
			switch mode {
			case "unknown-name":
				args = append(args, "--name", "missing")
			case "invalid-parameter":
				args = append(args, "--unknown")
			case "invalid-positional":
				args = []string{"skills", "install", "--target", target, "unexpected", "--output", "json"}
			case "corrupt-registration":
				if err := os.WriteFile(filepath.Join(state, "skill-locations.json"), []byte("invalid registry"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if code := app.Execute(context.Background(), args); code != 1 {
				t.Fatalf("failure code=%d stderr=%s", code, diagnostic)
			}
			if out.Len() != 0 {
				t.Fatalf("failed JSON install wrote stdout: %s", out)
			}
			lines := bytes.Split(bytes.TrimSpace(diagnostic.Bytes()), []byte("\n"))
			for _, line := range lines {
				if !json.Valid(line) {
					t.Fatalf("diagnostics are not JSONL: %s", diagnostic)
				}
			}
			result := skillsJSONDecode(t, lines[len(lines)-1])
			details := result.Error.Details
			if result.OK || details.CLI.Executable == "" || details.CLI.Version != build.Current().Version || details.Registration.Path != filepath.Join(state, "skill-locations.json") || details.Recovery.Status != "unconfirmed" || details.Recovery.Description == "" || details.Guidance == "" {
				t.Fatalf("safe failure diagnostics lost: %s", diagnostic)
			}
			if mode != "invalid-parameter" && details.Target != target {
				t.Fatalf("known target lost: %s", diagnostic)
			}
			if mode == "corrupt-registration" && details.Registration.ReadError == "" {
				t.Fatalf("unreadable registry reported empty: %s", diagnostic)
			}
			if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed JSON install created target: %v", err)
			}
		})
	}
}

type skillsJSONReadDirFailure struct {
	fs.FS
	failure error
}

func (source skillsJSONReadDirFailure) ReadDir(string) ([]fs.DirEntry, error) {
	return nil, source.failure
}

func TestSkillsJSONInstallPreservesRecoveryAndCancellationErrorChain(t *testing.T) {
	for _, rollbackFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "failed"}[rollbackFails], func(t *testing.T) {
			app, out, diagnostic, _ := skillsJSONFixture(t)
			target := filepath.Join(skillsJSONTempDir(t), "skills")
			failure := &skillsync.TransactionFailure{Cause: context.Canceled}
			if rollbackFails {
				failure.RollbackErr = errors.New("fixture recovery failed")
			}
			app.skillsFS = skillsJSONReadDirFailure{FS: fstest.MapFS{}, failure: failure}
			err := app.Run(context.Background(), []string{"skills", "install", "--target", target, "--output", "json"})
			var preserved *skillsync.TransactionFailure
			var details commandFailureDetailer
			if !errors.Is(err, context.Canceled) || !errors.As(err, &preserved) || preserved != failure || !errors.As(err, &details) {
				t.Fatalf("structured error discarded cancellation/recovery chain: %v", err)
			}
			body, marshalErr := json.Marshal(map[string]any{"error": map[string]any{"details": details.commandFailureDetails()}})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			status := "restored"
			if rollbackFails {
				status = "failed"
			}
			decoded := skillsJSONDecode(t, body)
			if decoded.Error.Details.Recovery.Status != status || decoded.Error.Details.Target != target {
				t.Fatalf("wrong recovery evidence: %s", body)
			}
			if out.Len() != 0 || diagnostic.Len() != 0 {
				t.Fatalf("Run should leave failure output to Execute: stdout=%s stderr=%s", out, diagnostic)
			}
		})
	}
}

type skillsJSONCancelTransport func(*http.Request) (*http.Response, error)

func (transport skillsJSONCancelTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestSkillsJSONCancelledUpdateDoesNotInstall(t *testing.T) {
	app, out, diagnostic, state := skillsJSONFixture(t)
	target := filepath.Join(skillsJSONTempDir(t), "skills")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.updateVersion = "1.0.0"
	app.updateURL = "https://registry.example.invalid/package"
	app.lookupEnv = func(string) (string, bool) { return "", false }
	app.httpClient = &http.Client{Transport: skillsJSONCancelTransport(func(request *http.Request) (*http.Response, error) { cancel(); return nil, request.Context().Err() })}
	if code := app.Execute(ctx, []string{"skills", "install", "--target", target, "--output", "json"}); code != 130 {
		t.Fatalf("cancel code=%d stderr=%s", code, diagnostic)
	}
	if out.Len() != 0 || !json.Valid(diagnostic.Bytes()) || skillsJSONDecode(t, diagnostic.Bytes()).Error.Type != "cancelled" {
		t.Fatalf("cancelled output is not isolated: stdout=%s stderr=%s", out, diagnostic)
	}
	for _, path := range []string{target, filepath.Join(state, "skill-locations.json")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cancelled command installed/registered %s: %v", path, err)
		}
	}
}

func TestSkillsJSONDoesNotTreatFlagValueAsOutputFormat(t *testing.T) {
	app, out, diagnostic, _ := skillsJSONFixture(t)
	if code := app.Execute(context.Background(), []string{"skills", "install", "--target", "--output", "json"}); code != 1 {
		t.Fatalf("invalid positional argument accepted: %d", code)
	}
	if !strings.Contains(out.String(), "CLI 实际可执行文件：") || !strings.Contains(diagnostic.String(), "Skill 安装：失败") || json.Valid(diagnostic.Bytes()) {
		t.Fatalf("flag value misclassified as explicit JSON: stdout=%s stderr=%s", out, diagnostic)
	}
}
