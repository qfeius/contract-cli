package credential

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cn.qfei/contract-cli/internal/config"
)

func TestPendingTransactionStatusDefaultsToPendingForLegacyCredentials(t *testing.T) {
	var pending PendingTransaction
	if err := json.Unmarshal([]byte(`{"device_code":"device-a"}`), &pending); err != nil {
		t.Fatal(err)
	}
	if pending.EffectiveStatus() != PendingStatusPending {
		t.Fatalf("effective status = %q, want pending", pending.EffectiveStatus())
	}
}

func TestDoubaoStoreEncryptsCredentialAndDoesNotPersistPlaintext(t *testing.T) {
	workspace := t.TempDir()
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	store, err := NewStore(Options{LookupEnv: envLookup(map[string]string{
		"SKILL_SESSION_WORKSPACE":        workspace,
		"CONTRACT_CLI_CREDENTIAL_KEY_V1": key,
	})})
	if err != nil {
		t.Fatal(err)
	}
	credential := DeviceCredential{
		Pending: &PendingTransaction{DeviceCode: "secret-device-code"},
		Token:   &config.Token{AccessToken: "secret-access", RefreshToken: "secret-refresh", Expiry: time.Now().Add(time.Hour)},
		DeviceProfile: &DeviceProfile{
			Name:                        "contract",
			Environment:                 "prod",
			OpenPlatformBaseURL:         "https://open.qfei.cn",
			Resource:                    "https://open.qfei.cn",
			BusinessType:                "contract",
			DeviceClientID:              "device-client",
			DeviceScope:                 "contract:full",
			DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/api/public/oauth/device-authorization/contract",
			TokenEndpoint:               "https://myaccount.qfei.cn/api/public/oauth/token/contract",
			RevocationEndpoint:          "https://myaccount.qfei.cn/api/public/oauth/revoke/contract",
		},
	}
	if err := store.Save("contract", credential); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(workspace, ".contract-cli", "credentials", profileFileName("contract"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{
		"secret-device-code",
		"secret-access",
		"secret-refresh",
		"https://open.qfei.cn",
		"device-client",
	} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("encrypted file contains %q", secret)
		}
	}
	loaded, err := store.Load("contract")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token == nil || loaded.Token.AccessToken != "secret-access" || loaded.Pending == nil || loaded.Pending.DeviceCode != "secret-device-code" {
		t.Fatalf("unexpected loaded credential: %+v", loaded)
	}
	if loaded.DeviceProfile == nil || loaded.DeviceProfile.Name != "contract" || loaded.DeviceProfile.DeviceScope != "contract:full" {
		t.Fatalf("unexpected loaded device profile: %+v", loaded.DeviceProfile)
	}
}

func TestDoubaoStoreRejectsMissingOrInvalidKey(t *testing.T) {
	for _, key := range []string{"", "not-base64"} {
		_, err := NewStore(Options{LookupEnv: envLookup(map[string]string{
			"SKILL_SESSION_WORKSPACE":        t.TempDir(),
			"CONTRACT_CLI_CREDENTIAL_KEY_V1": key,
		})})
		if err == nil {
			t.Fatalf("key %q should fail", key)
		}
	}
}

func TestWorkBuddyStoreRequiresTaskIDAndUsesItAsNamespace(t *testing.T) {
	backend := &memoryKeyring{values: map[string]string{}}
	_, err := NewStore(Options{LookupEnv: envLookup(map[string]string{}), Keyring: backend})
	if err == nil || !strings.Contains(err.Error(), "SKILL_SESSION_WORKSPACE") || !strings.Contains(err.Error(), "CODEBUDDY_SESSION_ID") {
		t.Fatalf("error = %v", err)
	}

	storeA, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"CODEBUDDY_SESSION_ID": "task-a"}), Keyring: backend})
	if err != nil {
		t.Fatal(err)
	}
	storeB, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"CODEBUDDY_SESSION_ID": "task-b"}), Keyring: backend})
	if err != nil {
		t.Fatal(err)
	}
	if err := storeA.Save("contract", DeviceCredential{Token: &config.Token{AccessToken: "token-a"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := storeB.Load("contract"); err == nil {
		t.Fatal("task-b unexpectedly loaded task-a credential")
	}
	if _, ok := backend.values[keyringService+":task-a:contract"]; !ok {
		t.Fatal("WorkBuddy keyring account changed and would invalidate existing credentials")
	}
}

func TestDeviceRuntimeRejectsRemovedDoubaoLocalEnvironment(t *testing.T) {
	lookupEnv := envLookup(map[string]string{
		"DOUBAO_SESSION_ID": "removed-local-session",
	})
	_, err := ResolveDeviceRuntime(lookupEnv)
	if err == nil {
		t.Fatal("DOUBAO_SESSION_ID unexpectedly enabled the removed Doubao local runtime")
	}
	if !strings.Contains(err.Error(), "SKILL_SESSION_WORKSPACE") || !strings.Contains(err.Error(), "CODEBUDDY_SESSION_ID") {
		t.Fatalf("error = %v, want the two supported runtime contracts", err)
	}
	if _, err := NewStore(Options{LookupEnv: lookupEnv, Keyring: &memoryKeyring{values: map[string]string{}}}); err == nil {
		t.Fatal("DOUBAO_SESSION_ID unexpectedly created a Device CredentialStore")
	}
}

func TestDoubaoWorkTaskRuntimeUsesSessionAndCurrentDirectory(t *testing.T) {
	workspace := t.TempDir()
	runtimeContext, err := resolveDeviceRuntime(envLookup(map[string]string{
		"SESSION_ID": "doubao-task-a",
	}), func() (string, error) {
		return workspace, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if runtimeContext.Kind != DeviceRuntimeDoubaoWorkTask {
		t.Fatalf("runtime kind = %q, want %q", runtimeContext.Kind, DeviceRuntimeDoubaoWorkTask)
	}
	if runtimeContext.Workspace != workspace {
		t.Fatalf("runtime workspace = %q, want %q", runtimeContext.Workspace, workspace)
	}
	if runtimeContext.SessionID != "doubao-task-a" {
		t.Fatalf("runtime session = %q", runtimeContext.SessionID)
	}
	if runtimeContext.SessionNamespace != deviceSessionNamespace("doubao-task-a") {
		t.Fatalf("runtime namespace = %q", runtimeContext.SessionNamespace)
	}
}

func TestDoubaoWorkTaskRuntimeRejectsInvalidCurrentDirectory(t *testing.T) {
	lookupEnv := envLookup(map[string]string{"SESSION_ID": "doubao-task-a"})
	for name, currentDir := range map[string]func() (string, error){
		"lookup failure": func() (string, error) { return "", errors.New("cwd unavailable") },
		"relative path":  func() (string, error) { return "relative/workspace", nil },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := resolveDeviceRuntime(lookupEnv, currentDir); err == nil {
				t.Fatal("invalid current directory unexpectedly accepted")
			}
		})
	}
}

func TestDoubaoWorkTaskStoreEncryptsAndIsolatesSessions(t *testing.T) {
	workspace := t.TempDir()
	state := t.TempDir()
	t.Chdir(workspace)

	storeA, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "task-a"}), StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	stored := DeviceCredential{
		Pending: &PendingTransaction{DeviceCode: "secret-device-code"},
		Token:   &config.Token{AccessToken: "secret-access", RefreshToken: "secret-refresh"},
		DeviceProfile: &DeviceProfile{
			Name: "contract", Environment: "prod", OpenPlatformBaseURL: "https://open.qfei.cn",
			Resource: "https://open.qfei.cn", BusinessType: "contract", DeviceClientID: "device-client",
			DeviceScope: "contract:full", DeviceAuthorizationEndpoint: "https://myaccount.qfei.cn/device",
			TokenEndpoint: "https://myaccount.qfei.cn/token",
		},
	}
	if err := storeA.Save("contract", stored); err != nil {
		t.Fatal(err)
	}

	credentialPath := filepath.Join(
		state, "device-sessions-v2", deviceSessionNamespace("task-a"), "credentials", profileFileName("contract"),
	)
	raw, err := os.ReadFile(credentialPath)
	if err != nil {
		t.Fatal(err)
	}
	credentialInfo, err := os.Stat(credentialPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := credentialInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("credential permissions = %o, want 600", got)
	}
	sessionDir := filepath.Join(state, "device-sessions-v2", deviceSessionNamespace("task-a"))
	sessionInfo, err := os.Stat(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := sessionInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("session directory permissions = %o, want 700", got)
	}
	for _, plaintext := range []string{"task-a", "secret-device-code", "secret-access", "secret-refresh", "https://open.qfei.cn"} {
		if strings.Contains(string(raw), plaintext) {
			t.Fatalf("encrypted credential contains plaintext %q", plaintext)
		}
	}
	loaded, err := storeA.Load("contract")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token == nil || loaded.Token.AccessToken != "secret-access" || loaded.Pending == nil || loaded.Pending.DeviceCode != "secret-device-code" {
		t.Fatalf("unexpected loaded credential: %+v", loaded)
	}

	storeB, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "task-b"}), StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storeB.Load("contract"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("task-b load error = %v, want ErrCredentialNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".contract-cli", "sessions", "task-a")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("raw session id unexpectedly used as path: %v", err)
	}
}

func TestDoubaoWorkTaskStoreRejectsUnwritableCredentialRoot(t *testing.T) {
	workspace := t.TempDir()
	state := t.TempDir()
	t.Chdir(workspace)
	if err := os.WriteFile(filepath.Join(state, "device-sessions-v2"), []byte("blocks directory creation"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "task-a"}), StateDir: state}); err == nil {
		t.Fatal("unwritable credential root unexpectedly accepted")
	}
}

func TestDoubaoWorkTaskStoreTightensExistingSessionDirectoryPermissions(t *testing.T) {
	workspace := t.TempDir()
	state := t.TempDir()
	t.Chdir(workspace)
	sessionDir := filepath.Join(state, "device-sessions-v2", deviceSessionNamespace("task-a"))
	credentialDir := filepath.Join(sessionDir, "credentials")
	if err := os.MkdirAll(credentialDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(credentialDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := NewStore(Options{LookupEnv: envLookup(map[string]string{"SESSION_ID": "task-a"}), StateDir: state}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{sessionDir, credentialDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Fatalf("directory %q permissions = %o, want 700", dir, got)
		}
	}
}

func TestDeviceRuntimeKeepsDoubaoCloudPrecedenceOverWorkBuddy(t *testing.T) {
	workspace := t.TempDir()
	runtimeContext, err := ResolveDeviceRuntime(envLookup(map[string]string{
		"SKILL_SESSION_WORKSPACE": workspace,
		"CODEBUDDY_SESSION_ID":    "workbuddy-task",
		"SESSION_ID":              "doubao-task",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if runtimeContext.Kind != DeviceRuntimeDoubaoCloud || runtimeContext.Workspace != workspace {
		t.Fatalf("runtime = %+v, want Doubao cloud workspace", runtimeContext)
	}
}

func TestDeviceRuntimeKeepsWorkBuddyPrecedenceOverDoubaoWorkTask(t *testing.T) {
	runtimeContext, err := ResolveDeviceRuntime(envLookup(map[string]string{
		"CODEBUDDY_SESSION_ID": "workbuddy-task",
		"SESSION_ID":           "doubao-task",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if runtimeContext.Kind != DeviceRuntimeWorkBuddy || runtimeContext.SessionID != "workbuddy-task" {
		t.Fatalf("runtime = %+v, want WorkBuddy", runtimeContext)
	}
}

type memoryKeyring struct{ values map[string]string }

func (m *memoryKeyring) Get(service, user string) (string, error) {
	value, ok := m.values[service+":"+user]
	if !ok {
		return "", ErrCredentialNotFound
	}
	return value, nil
}
func (m *memoryKeyring) Set(service, user, value string) error {
	m.values[service+":"+user] = value
	return nil
}
func (m *memoryKeyring) Delete(service, user string) error {
	delete(m.values, service+":"+user)
	return nil
}

func envLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
