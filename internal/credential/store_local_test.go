package credential

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cn.qfei/contract-cli/internal/config"
)

func TestLocalUserStoreSharesAcrossAgentsAndSessions(t *testing.T) {
	backend := &memoryKeyring{values: map[string]string{}}
	storeA := desktopStore(t, backend, map[string]string{"SESSION_ID": "doubao-task-a"})
	storeB := desktopStore(t, backend, map[string]string{"SESSION_ID": "doubao-task-b"})
	storeC := desktopStore(t, backend, map[string]string{"CODEBUDDY_SESSION_ID": "workbuddy-task"})
	stored := DeviceCredential{Token: &config.Token{AccessToken: "local-access", RefreshToken: "local-refresh"}}
	if err := storeA.Save("contract", stored); err != nil {
		t.Fatal(err)
	}
	for _, store := range []Store{storeB, storeC} {
		loaded, err := store.Load("contract")
		if err != nil || loaded.Token == nil || loaded.Token.AccessToken != "local-access" {
			t.Fatalf("shared credential = %+v, error = %v", loaded, err)
		}
		if _, err := store.Load("another-profile"); !errors.Is(err, ErrCredentialNotFound) {
			t.Fatalf("profile isolation error = %v", err)
		}
	}
	if err := storeC.Delete("contract"); err != nil {
		t.Fatal(err)
	}
	if _, err := storeA.Load("contract"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("deleted credential remains visible to another task: %v", err)
	}
}

func TestLocalUserStoreDoesNotReadOrMigrateLegacyCredentials(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	backend := &memoryKeyring{values: map[string]string{}}
	for _, environment := range []map[string]string{
		{"SESSION_ID": "task-a"},
		{"CODEBUDDY_SESSION_ID": "task-a"},
		{"CODEBUDDY_SESSION_ID": "local-user-v1"},
	} {
		legacy, err := NewStore(Options{LookupEnv: envLookup(environment), Keyring: backend, StateDir: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		if err := legacy.Save("contract", DeviceCredential{Token: &config.Token{AccessToken: "legacy-token"}}); err != nil {
			t.Fatal(err)
		}
		local := desktopStore(t, backend, environment)
		if _, err := local.Load("contract"); !errors.Is(err, ErrCredentialNotFound) {
			t.Fatalf("legacy credential migrated unexpectedly: %v", err)
		}
		if err := local.Save("contract", DeviceCredential{Token: &config.Token{AccessToken: "local-token"}}); err != nil {
			t.Fatal(err)
		}
		loaded, err := legacy.Load("contract")
		if err != nil || loaded.Token == nil || loaded.Token.AccessToken != "legacy-token" {
			t.Fatalf("legacy token changed: %+v, %v", loaded, err)
		}
		if err := local.Delete("contract"); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := legacy.Delete("contract"); err != nil {
				t.Fatalf("legacy deletion must remain idempotent: %v", err)
			}
		}
	}
}

func TestLocalUserStoreUsesResolvedRuntimeWithoutRedetecting(t *testing.T) {
	runtimeContext := DeviceRuntime{Kind: DeviceRuntimeLocalUser}
	backend := &memoryKeyring{values: map[string]string{}}
	store, err := NewStore(Options{Runtime: &runtimeContext, LookupEnv: envLookup(nil), Keyring: backend})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("contract", DeviceCredential{Token: &config.Token{AccessToken: "saved"}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load("contract")
	if err != nil || loaded.Token == nil || loaded.Token.AccessToken != "saved" {
		t.Fatalf("credential = %+v, error = %v", loaded, err)
	}
}

func TestLocalUserKeyringFailuresNeverFallBackToFiles(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	failure := errors.New("secure storage unavailable")
	store := desktopStore(t, failingKeyring{failure}, map[string]string{"SESSION_ID": "task-a"})
	if _, err := store.Load("contract"); !errors.Is(err, failure) {
		t.Fatalf("load error = %v", err)
	}
	if err := store.Save("contract", DeviceCredential{Token: &config.Token{AccessToken: "secret"}}); !errors.Is(err, failure) {
		t.Fatalf("save error = %v", err)
	}
	if err := store.Delete("contract"); !errors.Is(err, failure) {
		t.Fatalf("delete error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".contract-cli")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("keyring failure unexpectedly created fallback directory: %v", err)
	}
}

func TestNewStoreRejectsUnknownResolvedRuntime(t *testing.T) {
	runtimeContext := DeviceRuntime{Kind: "future"}
	if _, err := NewStore(Options{Runtime: &runtimeContext, Keyring: &memoryKeyring{values: map[string]string{}}}); err == nil {
		t.Fatal("unknown runtime unexpectedly created a store")
	}
}

func TestLocalUserStoreRejectsCorruptCredentialWithoutTaskFallback(t *testing.T) {
	backend := &memoryKeyring{values: map[string]string{
		localUserKeyringService + ":contract": "corrupt credential",
		keyringService + ":task-a:contract":   `{"token":{"access_token":"legacy-secret"}}`,
	}}
	store := desktopStore(t, backend, map[string]string{"CODEBUDDY_SESSION_ID": "task-a"})
	if _, err := store.Load("contract"); err == nil || errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("corruption must fail explicitly without reading task token: %v", err)
	}
}

func TestLocalUserStoreDeletionIsIdempotent(t *testing.T) {
	store := desktopStore(t, failingKeyring{ErrCredentialNotFound}, nil)
	if err := store.Delete("contract"); err != nil {
		t.Fatalf("missing credential deletion must succeed: %v", err)
	}
}

func desktopStore(t *testing.T, backend Keyring, environment map[string]string) Store {
	t.Helper()
	runtimeContext, err := ResolveDeviceRuntimeWithEvidence(envLookup(environment), desktopReport())
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(Options{Runtime: &runtimeContext, LookupEnv: envLookup(environment), Keyring: backend})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

type failingKeyring struct{ err error }

func (f failingKeyring) Get(string, string) (string, error) { return "", f.err }
func (f failingKeyring) Set(string, string, string) error   { return f.err }
func (f failingKeyring) Delete(string, string) error        { return f.err }
