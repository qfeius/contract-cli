package credential

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"cn.qfei/contract-cli/internal/config"
)

func TestTaskCredentialsUsePrivateStateAndRandomKey(t *testing.T) {
	workspace, state := t.TempDir(), t.TempDir()
	t.Chdir(workspace)
	options := Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "task-a"}), StateDir: state}
	store, err := NewStore(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("contract", DeviceCredential{Token: &config.Token{AccessToken: "private-token"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".contract-cli")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credential created in project: %v", err)
	}
	reopened, err := NewStore(options)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := reopened.Load("contract")
	if err != nil || stored.Token == nil || stored.Token.AccessToken != "private-token" {
		t.Fatalf("private state cannot be reused: %v", err)
	}
	first := store.(*encryptedFileStore)
	other, err := NewStore(Options{LookupEnv: options.LookupEnv, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	second := other.(*encryptedFileStore)
	if bytes.Equal(first.key, second.key) {
		t.Fatal("credential key is still derivable from SESSION_ID")
	}
	raw, err := os.ReadFile(first.path("contract"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second.path("contract"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Load("contract"); err == nil {
		t.Fatal("another private root decrypted copied credential")
	}
}

func TestTaskStoreDoesNotImportLegacyProjectCredential(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	legacy := filepath.Join(workspace, ".contract-cli", "sessions", deviceSessionNamespace("task-a"), "credentials")
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, profileFileName("contract")), []byte("untrusted legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "task-a"}), StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("contract"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("legacy project state was consumed: %v", err)
	}
}

func TestConcurrentTaskKeyInitializationKeepsOneCompleteKey(t *testing.T) {
	workspace, state := t.TempDir(), t.TempDir()
	t.Chdir(workspace)
	const count = 8
	keys := make(chan []byte, count)
	failures := make(chan error, count)
	var group sync.WaitGroup
	for range count {
		group.Add(1)
		go func() {
			defer group.Done()
			store, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "same-task"}), StateDir: state})
			if err != nil {
				failures <- err
				return
			}
			keys <- store.(*encryptedFileStore).key
		}()
	}
	group.Wait()
	close(keys)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var first []byte
	seen := 0
	for key := range keys {
		seen++
		if first == nil {
			first = key
		}
		if len(key) != 32 || !bytes.Equal(first, key) {
			t.Fatal("concurrent stores use different or incomplete keys")
		}
	}
	if seen != count {
		t.Fatalf("only %d keys initialized", seen)
	}
}

func TestTaskPrivateStateTightensExistingStateRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not Windows ACLs")
	}
	workspace, state := t.TempDir(), t.TempDir()
	t.Chdir(workspace)
	if err := os.Chmod(state, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "same-task"}), StateDir: state}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(state)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("private root not secured: %v %v", info, err)
	}
}
