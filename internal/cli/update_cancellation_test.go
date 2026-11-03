package cli_test

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
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/invocation"
)

func TestCancelledUpdateStopsLocalWrites(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"logout", "switch-identity", "install-skills"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			fixture := newUpdateCancellationFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			app := fixture.app(func(req *http.Request) (*http.Response, error) {
				calls++
				cancel()
				return nil, req.Context().Err()
			})
			code := app.Execute(ctx, fixture.args(command))
			if code != 130 || calls != 1 || fixture.stdout.Len() != 0 || !strings.Contains(fixture.stderr.String(), "context canceled") {
				t.Fatalf("cancelled update continued dispatch: code=%d calls=%d stdout=%s stderr=%s", code, calls, fixture.stdout, fixture.stderr)
			}
			fixture.assertUnchanged(t)
		})
	}
}

func TestPreCancelledContextStopsLocalWrites(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"logout", "switch-identity", "install-skills", "internal-sync"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			fixture := newUpdateCancellationFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			app := fixture.app(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("pre-cancelled command started HTTP request: %s", req.URL)
				return nil, context.Canceled
			})
			if code := app.Execute(ctx, fixture.args(command)); code != 130 || fixture.stdout.Len() != 0 {
				t.Fatalf("pre-cancelled command returned success or dispatched: code=%d stdout=%s stderr=%s", code, fixture.stdout, fixture.stderr)
			}
			fixture.assertUnchanged(t)
		})
	}
}

func TestPreCancelledRunClearsPreviousUpdateNotice(t *testing.T) {
	t.Parallel()
	fixture := newUpdateCancellationFixture(t)
	calls := 0
	app := fixture.app(func(*http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(`{"dist-tags":{"latest":"1.0.1"}}`), nil
	})
	if code := app.Execute(context.Background(), []string{"unknown-command", "--output", "json"}); code != 1 || !bytes.Contains(fixture.stderr.Bytes(), []byte(`"_notice"`)) {
		t.Fatalf("first execution did not prepare notice: code=%d stderr=%s", code, fixture.stderr)
	}
	fixture.stdout.Reset()
	fixture.stderr.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := app.Execute(ctx, []string{"unknown-command", "--output", "json"})
	if code != 130 || calls != 1 || !json.Valid(fixture.stderr.Bytes()) || bytes.Contains(fixture.stderr.Bytes(), []byte(`"_notice"`)) {
		t.Fatalf("pre-cancelled run leaked prior notice or started work: code=%d calls=%d stderr=%s", code, calls, fixture.stderr)
	}
	fixture.assertUnchanged(t)
}

func TestUpdateCheckFailureStillAllowsLocalWrites(t *testing.T) {
	t.Parallel()
	for _, checkErr := range []error{errors.New("registry unavailable"), context.DeadlineExceeded} {
		for _, command := range []string{"logout", "switch-identity", "install-skills"} {
			t.Run(command+"/"+checkErr.Error(), func(t *testing.T) {
				t.Parallel()
				fixture := newUpdateCancellationFixture(t)
				calls := 0
				app := fixture.app(func(*http.Request) (*http.Response, error) { calls++; return nil, checkErr })
				if code := app.Execute(context.Background(), fixture.args(command)); code != 0 || calls != 1 || fixture.stderr.Len() != 0 {
					t.Fatalf("update failure blocked local command: code=%d calls=%d stderr=%s", code, calls, fixture.stderr)
				}
				fixture.assertExecuted(t, command)
			})
		}
	}
}

func TestUpdateCheckOwnTimeoutDoesNotCancelLocalCommand(t *testing.T) {
	t.Parallel()
	fixture := newUpdateCancellationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := 0
	app := fixture.app(func(req *http.Request) (*http.Response, error) {
		calls++
		<-req.Context().Done()
		if !errors.Is(req.Context().Err(), context.DeadlineExceeded) {
			t.Fatalf("update request did not time out independently: %v", req.Context().Err())
		}
		return nil, req.Context().Err()
	})
	if code := app.Execute(ctx, fixture.args("switch-identity")); code != 0 || calls != 1 || ctx.Err() != nil {
		t.Fatalf("update timeout cancelled parent command: code=%d calls=%d parent=%v stderr=%s", code, calls, ctx.Err(), fixture.stderr)
	}
	fixture.assertExecuted(t, "switch-identity")
}

type updateCancellationFixture struct {
	store          *config.Store
	target         string
	initialProfile []byte
	stdout, stderr *bytes.Buffer
	options        cli.Options
}

func newUpdateCancellationFixture(t *testing.T) updateCancellationFixture {
	t.Helper()
	state := t.TempDir()
	store := config.NewStore(state)
	profile := uploadProfile(config.IdentityUser)
	profile.Identities.User.AuthMode = config.UserAuthModeAuthorizationCode
	if err := store.UpsertProfile(profile, true); err != nil {
		t.Fatal(err)
	}
	initial, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return updateCancellationFixture{store: store, target: filepath.Join(t.TempDir(), "skills"), initialProfile: initial, stdout: stdout, stderr: stderr,
		options: cli.Options{Store: store, Secrets: config.NewSecretsStore(state), Stdout: stdout, Stderr: stderr,
			SkillsFS:             fstest.MapFS{"fixture/SKILL.md": {Data: []byte("---\nname: fixture\ndescription: regression fixture\n---\nfixture\n")}},
			UpdateCurrentVersion: "1.0.0", UpdateRegistryURL: "https://registry.example.test/package",
			LookupEnv:          func(string) (string, bool) { return "", false },
			InspectEnvironment: func(context.Context, int) invocation.Result { return invocation.Result{} },
		},
	}
}

func (fixture updateCancellationFixture) app(transport roundTripFunc) *cli.App {
	options := fixture.options
	options.HTTPClient = &http.Client{Transport: transport}
	return cli.New(options)
}

func (fixture updateCancellationFixture) args(command string) []string {
	switch command {
	case "logout":
		return []string{"auth", "logout", "--profile", "contract", "--as", "user"}
	case "switch-identity":
		return []string{"auth", "use", "--profile", "contract", "--as", "app"}
	case "internal-sync":
		return []string{"__internal", "skill-sync-on-install"}
	default:
		return []string{"skills", "install", "--target", fixture.target, "--force"}
	}
}

func (fixture updateCancellationFixture) assertUnchanged(t *testing.T) {
	t.Helper()
	current, err := os.ReadFile(fixture.store.Path())
	if err != nil || !bytes.Equal(current, fixture.initialProfile) {
		t.Fatalf("cancelled command changed persisted profile: %v", err)
	}
	for _, path := range []string{fixture.target, filepath.Join(filepath.Dir(fixture.store.Path()), "skill-locations.json")} {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("cancelled command wrote Skill files or registration: %s: %v", path, err)
		}
	}
}

func (fixture updateCancellationFixture) assertExecuted(t *testing.T, command string) {
	t.Helper()
	profile, err := fixture.store.GetProfile("contract")
	if err != nil {
		t.Fatal(err)
	}
	switch command {
	case "logout":
		if profile.Identities.User.Token != nil {
			t.Fatal("logout did not clear the user token")
		}
	case "switch-identity":
		if profile.DefaultIdentity != config.IdentityApp {
			t.Fatal("identity selection did not persist")
		}
	case "install-skills":
		for _, path := range []string{filepath.Join(fixture.target, "fixture", "SKILL.md"), filepath.Join(filepath.Dir(fixture.store.Path()), "skill-locations.json")} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("normal Skill installation or registration failed: %v", err)
			}
		}
	}
}
