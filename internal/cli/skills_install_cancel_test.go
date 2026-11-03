package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/skillsync"
)

func TestSkillsInstallCancellationStopsWritesAndSuccessOutput(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"after-metadata", "before-next-metadata", "during-staging"} {
		for _, format := range []string{"json", "default-text"} {
			t.Run(phase+"/"+format, func(t *testing.T) {
				t.Parallel()
				state := skillInstallCancelTempDir(t)
				target := filepath.Join(skillInstallCancelTempDir(t), "skills")
				registry := filepath.Join(state, "skill-locations.json")
				var originalRegistry []byte
				var originalFiles map[string][]byte
				if phase == "during-staging" {
					seed := skillsync.Service{StateDir: state, Version: "old-fixture", Source: fstest.MapFS{
						"auth/SKILL.md":            {Data: []byte("---\nname: auth\ndescription: original fixture\n---\noriginal skill\n")},
						"auth/references/guide.md": {Data: []byte("original resource\n")},
					}}
					if _, err := seed.Install(target, []string{"auth"}, true, true); err != nil {
						t.Fatal(err)
					}
					var err error
					originalRegistry, err = os.ReadFile(registry)
					if err != nil {
						t.Fatal(err)
					}
					originalFiles = skillInstallCancelSnapshot(t, target)
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				triggerPath := "z-new/SKILL.md"
				if phase == "during-staging" {
					triggerPath = "auth/references/guide.md"
				} else if phase == "before-next-metadata" {
					triggerPath = "auth/SKILL.md"
				}
				source := &skillInstallCancelFS{FS: fstest.MapFS{
					"auth/SKILL.md":            {Data: []byte("---\nname: auth\ndescription: replacement fixture\n---\nreplacement skill\n")},
					"auth/references/guide.md": {Data: []byte("replacement resource\n")},
					"z-new/SKILL.md":           {Data: []byte("---\nname: z-new\ndescription: additional fixture\n---\nnew skill\n")},
				}, triggerPath: triggerPath, cancel: cancel}
				stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
				app := cli.New(cli.Options{
					Store: config.NewStore(state), Secrets: config.NewSecretsStore(state),
					SkillsFS: source, Stdout: stdout, Stderr: stderr,
					LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
					HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						t.Error("local Skill installation unexpectedly accessed HTTP")
						return nil, errors.New("external HTTP disabled for cancellation regression")
					})},
				})
				args := []string{"skills", "install", "--target", target, "--force"}
				if format == "json" {
					args = append(args, "--output", "json", "--debug")
				}
				code := app.Execute(ctx, args)
				if !source.triggered || !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatalf("fixture did not cancel in %s", phase)
				}
				if phase == "before-next-metadata" && len(source.readsAfterCancel) != 0 {
					t.Errorf("metadata loading continued reading after cancellation: %v", source.readsAfterCancel)
				}
				if code != 130 {
					t.Errorf("cancelled installation exit=%d, want 130; stderr=%s", code, stderr)
				}
				if format == "json" {
					if stdout.Len() != 0 {
						t.Errorf("cancelled JSON installation printed a result: %s", stdout)
					}
					skillInstallCancelAssertJSONError(t, stderr.Bytes())
				} else if !strings.Contains(stderr.String(), "context canceled") {
					t.Errorf("text cancellation error missing: %s", stderr)
				}
				for _, success := range []string{"Installed skill:", "Skill 安装：成功", "本次目录登记：成功", "Skill installation completed", "skills install completed"} {
					if strings.Contains(stdout.String()+stderr.String(), success) {
						t.Errorf("cancelled installation printed success marker %q", success)
					}
				}

				if phase != "during-staging" {
					if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("cancellation after metadata created the target: %v", err)
					}
					if _, err := os.Stat(registry); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("cancellation after metadata created a registration: %v", err)
					}
				} else {
					if files := skillInstallCancelSnapshot(t, target); !reflect.DeepEqual(files, originalFiles) {
						t.Error("cancellation during staging changed existing Skill files or added the new Skill")
					}
					currentRegistry, err := os.ReadFile(registry)
					if err != nil || !bytes.Equal(currentRegistry, originalRegistry) {
						t.Errorf("cancellation during staging changed existing registration: %v", err)
					}
				}
			})
		}
	}
}

func TestSkillsInstallCancellationAfterCommitReportsInstalledOutcome(t *testing.T) {
	t.Parallel()
	for _, message := range []string{"Skill installation completed", "skills install completed", "Skill registration snapshot completed"} {
		t.Run(message, func(t *testing.T) {
			t.Parallel()
			testSkillsInstallCommittedCancellation(t, message)
		})
	}
}

func testSkillsInstallCommittedCancellation(t *testing.T, message string) {
	t.Helper()
	state := skillInstallCancelTempDir(t)
	target := filepath.Join(skillInstallCancelTempDir(t), "skills")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	observer := &skillInstallCommitCancellationWriter{output: stderr, cancel: cancel, triggerMessage: message}
	app := cli.New(cli.Options{
		Store: config.NewStore(state), Secrets: config.NewSecretsStore(state),
		SkillsFS: fstest.MapFS{
			"auth/SKILL.md":            {Data: []byte("---\nname: auth\ndescription: committed fixture\n---\ncommitted skill\n")},
			"auth/references/guide.md": {Data: []byte("committed resource\n")},
			"z-new/SKILL.md":           {Data: []byte("---\nname: z-new\ndescription: new committed fixture\n---\nnew committed skill\n")},
		},
		Stdout: stdout, Stderr: stderr,
		Logger:    slog.New(slog.NewJSONHandler(observer, nil)),
		LookupEnv: func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("local Skill installation unexpectedly accessed HTTP")
			return nil, errors.New("external HTTP disabled for cancellation regression")
		})},
	})
	code := app.Execute(ctx, []string{"skills", "install", "--target", target, "--force", "--output", "json"})
	if !observer.triggered || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("fixture did not cancel after the service committed the installation")
	}
	if code != 130 || stdout.Len() != 0 {
		t.Errorf("late cancellation must return 130 without a success result: code=%d stdout=%s", code, stdout)
	}
	skillInstallCancelAssertJSONError(t, stderr.Bytes())
	lines := bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte("\n"))
	var failure struct {
		Error struct {
			Details struct {
				Recovery struct {
					Status string `json:"status"`
				} `json:"recovery"`
				Installation struct {
					Installed []string `json:"installed"`
					Confirmed bool     `json:"confirmed"`
				} `json:"installation"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(lines[len(lines)-1], &failure); err != nil {
		t.Fatal(err)
	}
	if failure.Error.Details.Recovery.Status != "committed" || !failure.Error.Details.Installation.Confirmed || !reflect.DeepEqual(failure.Error.Details.Installation.Installed, []string{"auth", "z-new"}) {
		t.Errorf("late cancellation hid committed installation details: %s", lines[len(lines)-1])
	}
	resource, err := os.ReadFile(filepath.Join(target, "auth", "references", "guide.md"))
	if err != nil || string(resource) != "committed resource\n" {
		t.Errorf("late cancellation rolled back the committed resource: error=%v", err)
	}
	newSkill, err := os.ReadFile(filepath.Join(target, "z-new", "SKILL.md"))
	if err != nil || !bytes.Contains(newSkill, []byte("new committed skill")) {
		t.Errorf("late cancellation removed the committed new Skill: error=%v", err)
	}
	registryBody, err := os.ReadFile(filepath.Join(state, "skill-locations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry skillsync.Registry
	if err := json.Unmarshal(registryBody, &registry); err != nil || len(registry.Locations) != 1 {
		t.Fatalf("committed registration missing or invalid: %s (%v)", registryBody, err)
	}
	location := registry.Locations[0]
	if location.Target != target || location.Status != "success" || !reflect.DeepEqual(location.Managed, []string{"auth", "z-new"}) || location.Transaction == "" {
		t.Errorf("late cancellation changed the committed registration: %#v", location)
	}
	markerBody, err := os.ReadFile(filepath.Join(target, ".contract-cli-managed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var marker struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(markerBody, &marker); err != nil || marker.ID == "" || marker.ID != location.ID {
		t.Errorf("committed marker and registration disagree: marker=%s error=%v", markerBody, err)
	}
}

type skillInstallCommitCancellationWriter struct {
	output         *bytes.Buffer
	cancel         context.CancelFunc
	triggerMessage string
	triggered      bool
}

func (writer *skillInstallCommitCancellationWriter) Write(body []byte) (int, error) {
	count, err := writer.output.Write(body)
	var record struct {
		Message string `json:"msg"`
	}
	if json.Unmarshal(body, &record) == nil && record.Message == writer.triggerMessage && !writer.triggered {
		writer.triggered = true
		writer.cancel()
	}
	return count, err
}

// fs.ReadFile dispatches here for both metadata and resources, so cancellation
// occurs at a deterministic I/O boundary while the read itself still succeeds.
type skillInstallCancelFS struct {
	fs.FS
	triggerPath      string
	cancel           context.CancelFunc
	triggered        bool
	readsAfterCancel []string
}

func (source *skillInstallCancelFS) ReadFile(name string) ([]byte, error) {
	if source.triggered {
		source.readsAfterCancel = append(source.readsAfterCancel, name)
	}
	body, err := fs.ReadFile(source.FS, name)
	if err == nil && name == source.triggerPath && !source.triggered {
		source.triggered = true
		source.cancel()
	}
	return body, err
}

func skillInstallCancelTempDir(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return directory
}

func skillInstallCancelSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		files[relative] = body
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func skillInstallCancelAssertJSONError(t *testing.T, body []byte) {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil || record == nil {
			t.Errorf("cancellation stderr contained non-JSON output: %q (%v)", line, err)
		}
	}
	var last struct {
		OK    *bool `json:"ok"`
		Error struct {
			Type      string `json:"type"`
			Cancelled bool   `json:"cancelled"`
		} `json:"error"`
	}
	if err := json.Unmarshal(lines[len(lines)-1], &last); err != nil || last.OK == nil || *last.OK || last.Error.Type != "cancelled" || !last.Error.Cancelled {
		t.Errorf("last stderr JSON is not the cancellation error: %s (%v)", lines[len(lines)-1], err)
	}
}
