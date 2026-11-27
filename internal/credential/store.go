package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/privatefile"
	keyringlib "github.com/zalando/go-keyring"
)

const (
	envSkillSessionWorkspace = "SKILL_SESSION_WORKSPACE"
	envCredentialKey         = "CONTRACT_CLI_CREDENTIAL_KEY_V1"
	envWorkBuddySessionID    = "CODEBUDDY_SESSION_ID"
	envDoubaoWorkTaskSession = "SESSION_ID"
	keyringService           = "cn.qfei.contract-cli"
	localUserKeyringService  = "cn.qfei.contract-cli.local-user-v1"
	doubaoWorkTaskNSPurpose  = "cn.qfei.contract-cli/doubao-work-task/session-namespace/v1\x00"
)

var ErrCredentialNotFound = errors.New("device credential not found")

type PendingStatus string

const (
	PendingStatusPending      PendingStatus = "pending"
	PendingStatusChecking     PendingStatus = "checking"
	PendingStatusUncertain    PendingStatus = "uncertain"
	PendingStatusDenied       PendingStatus = "denied"
	PendingStatusExpired      PendingStatus = "expired"
	PendingStatusInvalidGrant PendingStatus = "invalid_grant"
)

type PendingTransaction struct {
	PollIntervalSeconds     int64         `json:"poll_interval_seconds,omitempty"`
	Status                  PendingStatus `json:"status,omitempty"`
	DeviceCode              string        `json:"device_code"`
	VerificationURIComplete string        `json:"verification_uri_complete,omitempty"`
	TokenEndpoint           string        `json:"token_endpoint"`
	ClientID                string        `json:"client_id"`
	ExpiresAt               time.Time     `json:"expires_at"`
}

func (p PendingTransaction) EffectiveStatus() PendingStatus {
	if p.Status == "" {
		return PendingStatusPending
	}
	return p.Status
}

type DeviceProfile struct {
	Name                           string   `json:"name"`
	Environment                    string   `json:"environment"`
	OpenPlatformBaseURL            string   `json:"open_platform_base_url"`
	ProtectedResourceMetadataURL   string   `json:"protected_resource_metadata_url,omitempty"`
	AuthorizationServerMetadataURL string   `json:"authorization_server_metadata_url,omitempty"`
	Resource                       string   `json:"resource"`
	Scopes                         []string `json:"scopes,omitempty"`
	BusinessType                   string   `json:"business_type"`
	ClientName                     string   `json:"client_name,omitempty"`
	DeviceClientID                 string   `json:"device_client_id"`
	DeviceScope                    string   `json:"device_scope"`
	DeviceAuthorizationEndpoint    string   `json:"device_authorization_endpoint"`
	TokenEndpoint                  string   `json:"token_endpoint"`
	RevocationEndpoint             string   `json:"revocation_endpoint,omitempty"`
}

type DeviceCredential struct {
	Pending       *PendingTransaction `json:"pending,omitempty"`
	Token         *config.Token       `json:"token,omitempty"`
	DeviceProfile *DeviceProfile      `json:"device_profile,omitempty"`
}

type Store interface {
	Load(profileName string) (DeviceCredential, error)
	Save(profileName string, credential DeviceCredential) error
	Delete(profileName string) error
}

type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}

type Options struct {
	LookupEnv func(string) (string, bool)
	Keyring   Keyring
	Runtime   *DeviceRuntime
	StateDir  string
}

type DeviceRuntimeKind string

const (
	DeviceRuntimeDoubaoCloud    DeviceRuntimeKind = "doubao_cloud"
	DeviceRuntimeWorkBuddy      DeviceRuntimeKind = "workbuddy"
	DeviceRuntimeDoubaoWorkTask DeviceRuntimeKind = "doubao_work_task"
	DeviceRuntimeLocalUser      DeviceRuntimeKind = "local_user"
)

type DeviceRuntime struct {
	Kind             DeviceRuntimeKind
	Workspace        string
	SessionID        string
	SessionNamespace string
	DataDir          string
}

func ResolveDeviceRuntime(lookupEnv func(string) (string, bool)) (DeviceRuntime, error) {
	return resolveDeviceRuntime(lookupEnv, os.Getwd)
}

func resolveDeviceRuntime(lookupEnv func(string) (string, bool), currentDir func() (string, error)) (DeviceRuntime, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if workspace, ok := lookupEnv(envSkillSessionWorkspace); ok && strings.TrimSpace(workspace) != "" {
		return DeviceRuntime{Kind: DeviceRuntimeDoubaoCloud, Workspace: strings.TrimSpace(workspace)}, nil
	}

	workBuddySessionID := lookupTrimmedEnv(lookupEnv, envWorkBuddySessionID)
	if workBuddySessionID != "" {
		return DeviceRuntime{Kind: DeviceRuntimeWorkBuddy, SessionID: workBuddySessionID}, nil
	}

	doubaoSessionID := lookupTrimmedEnv(lookupEnv, envDoubaoWorkTaskSession)
	if doubaoSessionID != "" {
		workspace, err := currentDir()
		if err != nil {
			return DeviceRuntime{}, fmt.Errorf("resolve Doubao work task directory: %w", err)
		}
		workspace = strings.TrimSpace(workspace)
		if !filepath.IsAbs(workspace) {
			return DeviceRuntime{}, fmt.Errorf("doubao work task directory must be absolute: %q", workspace)
		}
		info, err := os.Stat(workspace)
		if err != nil {
			return DeviceRuntime{}, fmt.Errorf("inspect Doubao work task directory: %w", err)
		}
		if !info.IsDir() {
			return DeviceRuntime{}, fmt.Errorf("doubao work task directory is not a directory: %q", workspace)
		}
		namespace := deviceSessionNamespace(doubaoSessionID)
		state, err := config.DefaultDir()
		if configured := lookupTrimmedEnv(lookupEnv, "CONTRACT_CLI_CONFIG_DIR"); configured != "" {
			state, err = filepath.Abs(configured)
		}
		if err != nil {
			return DeviceRuntime{}, fmt.Errorf("resolve private task state: %w", err)
		}
		return DeviceRuntime{
			Kind: DeviceRuntimeDoubaoWorkTask, Workspace: workspace, SessionID: doubaoSessionID,
			SessionNamespace: namespace,
			DataDir:          filepath.Join(state, "device-sessions-v2", namespace),
		}, nil
	}
	return DeviceRuntime{}, errors.New("SKILL_SESSION_WORKSPACE, CODEBUDDY_SESSION_ID, or SESSION_ID is required for Device credential isolation")
}

func lookupTrimmedEnv(lookupEnv func(string) (string, bool), name string) string {
	value, ok := lookupEnv(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func NewStore(options Options) (Store, error) {
	lookupEnv := options.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	runtimeContext, err := runtimeForStore(options.Runtime, lookupEnv)
	if err != nil {
		return nil, err
	}
	switch runtimeContext.Kind {
	case DeviceRuntimeDoubaoCloud:
		return newCloudStore(runtimeContext, lookupEnv)
	case DeviceRuntimeDoubaoWorkTask:
		if options.StateDir != "" {
			root, err := filepath.Abs(options.StateDir)
			if err != nil {
				return nil, err
			}
			runtimeContext.DataDir = filepath.Join(root, "device-sessions-v2", runtimeContext.SessionNamespace)
		}
		return newTaskStore(runtimeContext)
	case DeviceRuntimeWorkBuddy, DeviceRuntimeLocalUser:
		return newKeyringStore(runtimeContext, options.Keyring)
	}
	return nil, fmt.Errorf("unsupported Device credential runtime: %q", runtimeContext.Kind)
}

func runtimeForStore(resolved *DeviceRuntime, lookupEnv func(string) (string, bool)) (DeviceRuntime, error) {
	if resolved != nil {
		return *resolved, nil
	}
	return ResolveDeviceRuntime(lookupEnv)
}

func newCloudStore(runtimeContext DeviceRuntime, lookupEnv func(string) (string, bool)) (Store, error) {
	encodedKey, keyExists := lookupEnv(envCredentialKey)
	if !keyExists || strings.TrimSpace(encodedKey) == "" {
		return nil, fmt.Errorf("%s is required in the Doubao Skill environment", envCredentialKey)
	}
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be a base64-encoded 32-byte key", envCredentialKey)
	}
	return &encryptedFileStore{
		dir: filepath.Join(runtimeContext.Workspace, ".contract-cli", "credentials"),
		key: key,
	}, nil
}

func newTaskStore(runtimeContext DeviceRuntime) (Store, error) {
	dir := filepath.Join(runtimeContext.DataDir, "credentials")
	if err := ensureWritableCredentialDirectory(dir); err != nil {
		return nil, err
	}
	key, err := privateTaskKey(runtimeContext.DataDir)
	if err != nil {
		return nil, err
	}
	return &encryptedFileStore{
		dir: dir,
		key: key,
	}, nil
}

func newKeyringStore(runtimeContext DeviceRuntime, backend Keyring) (Store, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return nil, fmt.Errorf("local agent credential store is not supported on %s", runtime.GOOS)
	}
	if backend == nil {
		backend = systemKeyring{}
	}
	if runtimeContext.Kind == DeviceRuntimeLocalUser {
		return &localKeyringStore{agentName: "local user", service: localUserKeyringService, keyring: backend}, nil
	}
	return &localKeyringStore{
		agentName: "WorkBuddy", service: keyringService, accountPrefix: runtimeContext.SessionID + ":", keyring: backend,
	}, nil
}

type encryptedFileStore struct {
	dir string
	key []byte
}

func (s *encryptedFileStore) Load(profileName string) (DeviceCredential, error) {
	data, err := os.ReadFile(s.path(profileName))
	if errors.Is(err, os.ErrNotExist) {
		return DeviceCredential{}, ErrCredentialNotFound
	}
	if err != nil {
		return DeviceCredential{}, fmt.Errorf("read encrypted credential: %w", err)
	}
	plaintext, err := decryptCredential(s.key, data)
	if err != nil {
		return DeviceCredential{}, err
	}
	var credential DeviceCredential
	if err := json.Unmarshal(plaintext, &credential); err != nil {
		return DeviceCredential{}, fmt.Errorf("decode encrypted credential: %w", err)
	}
	return credential, nil
}

func (s *encryptedFileStore) Save(profileName string, credential DeviceCredential) (saveErr error) {
	plaintext, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode credential: %w", err)
	}
	ciphertext, err := encryptCredential(s.key, plaintext)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	if err := os.Chmod(s.dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.dir, ".credential-*")
	if err != nil {
		return fmt.Errorf("create credential temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			saveErr = errors.Join(saveErr, fmt.Errorf("remove credential temporary file: %w", err))
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set credential file permissions: %w", err)
	}
	if _, err := temporary.Write(ciphertext); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write encrypted credential: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync encrypted credential: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close encrypted credential: %w", err)
	}
	if err := privatefile.Replace(temporaryPath, s.path(profileName)); err != nil {
		return fmt.Errorf("replace encrypted credential: %w", err)
	}
	return nil
}

func (s *encryptedFileStore) Delete(profileName string) error {
	err := os.Remove(s.path(profileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *encryptedFileStore) path(profileName string) string {
	return filepath.Join(s.dir, profileFileName(profileName))
}

type localKeyringStore struct {
	agentName     string
	service       string
	accountPrefix string
	keyring       Keyring
}

func (s *localKeyringStore) Load(profileName string) (DeviceCredential, error) {
	value, err := s.keyring.Get(s.service, s.user(profileName))
	if err != nil {
		if errors.Is(err, ErrCredentialNotFound) || errors.Is(err, keyringlib.ErrNotFound) {
			return DeviceCredential{}, ErrCredentialNotFound
		}
		return DeviceCredential{}, fmt.Errorf("read %s credential from %s secure storage: %w", s.agentName, runtime.GOOS, err)
	}
	var credential DeviceCredential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return DeviceCredential{}, fmt.Errorf("decode %s credential: %w", s.agentName, err)
	}
	return credential, nil
}

func (s *localKeyringStore) Save(profileName string, credential DeviceCredential) error {
	data, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode %s credential: %w", s.agentName, err)
	}
	if err := s.keyring.Set(s.service, s.user(profileName), string(data)); err != nil {
		return fmt.Errorf("write %s credential to %s secure storage: %w", s.agentName, runtime.GOOS, err)
	}
	return nil
}

func (s *localKeyringStore) Delete(profileName string) error {
	err := s.keyring.Delete(s.service, s.user(profileName))
	if errors.Is(err, keyringlib.ErrNotFound) || errors.Is(err, ErrCredentialNotFound) {
		return nil
	}
	return err
}

func (s *localKeyringStore) user(profileName string) string {
	return s.accountPrefix + profileName
}

type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) { return keyringlib.Get(service, user) }
func (systemKeyring) Set(service, user, value string) error {
	return keyringlib.Set(service, user, value)
}
func (systemKeyring) Delete(service, user string) error { return keyringlib.Delete(service, user) }

func encryptCredential(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize credential gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate credential nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptCredential(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize credential gcm: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("encrypted credential is truncated")
	}
	nonce, payload := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, errors.New("encrypted credential authentication failed")
	}
	return plaintext, nil
}

func profileFileName(profileName string) string {
	digest := sha256.Sum256([]byte(profileName))
	return hex.EncodeToString(digest[:]) + ".json.enc"
}

func deviceSessionNamespace(sessionID string) string {
	digest := sha256.Sum256([]byte(doubaoWorkTaskNSPurpose + sessionID))
	return hex.EncodeToString(digest[:])
}

func ensureWritableCredentialDirectory(dir string) error {
	session := filepath.Dir(dir)
	states := filepath.Dir(session)
	root := filepath.Dir(states)
	namespace, decodeErr := hex.DecodeString(filepath.Base(session))
	if filepath.Base(states) != "device-sessions-v2" || decodeErr != nil || len(namespace) != sha256.Size || filepath.Dir(root) == root {
		return errors.New("invalid private task state layout")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Doubao work task credential directory: %w", err)
	}
	for _, privateDir := range []string{root, states, session, dir} {
		if err := os.Chmod(privateDir, 0o700); err != nil {
			return fmt.Errorf("secure Doubao work task credential directory: %w", err)
		}
	}
	probe, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return fmt.Errorf("verify Doubao work task credential directory: %w", err)
	}
	probePath := probe.Name()
	if err := probe.Chmod(0o600); err != nil {
		_ = probe.Close()
		_ = os.Remove(probePath)
		return fmt.Errorf("secure Doubao work task credential probe: %w", err)
	}
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("close Doubao work task credential probe: %w", err)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("remove Doubao work task credential probe: %w", err)
	}
	return nil
}
