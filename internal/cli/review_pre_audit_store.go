package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gofrs/flock"
)

// A run contains only recovery metadata and necessary results, never credentials
// or source bytes. CLM file IDs and AI file IDs deliberately have separate fields.
type preAuditRun struct {
	SchemaVersion int    `json:"schemaVersion"`
	RunID         string `json:"runId"`
	Profile       string `json:"profile"`
	Origin        string `json:"origin"`
	TenantID      string `json:"tenantId"`
	UserID        string `json:"userId"`
	Stage         string `json:"workflowStage"`
	Name          string `json:"name"`
	SourceFileID  string `json:"sourceFileId,omitempty"`
	CategoryID    string `json:"categoryId,omitempty"`
	// 仅兼容读取旧本地记录，不生成、发送或用于创建重放。
	RequestID    string         `json:"requestId,omitempty"`
	PreAuditID   string         `json:"preAuditId,omitempty"`
	LatestFileID string         `json:"latestFileId,omitempty"`
	AIFileID     string         `json:"fileId,omitempty"`
	FileHash     string         `json:"fileHash,omitempty"`
	TaskID       string         `json:"taskId,omitempty"`
	Upload       map[string]any `json:"upload,omitempty"`
	Subjects     map[string]any `json:"subjects,omitempty"`
	Start        map[string]any `json:"start,omitempty"`
	Final        map[string]any `json:"final,omitempty"`
}

func newPreAuditID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func validPreAuditRunID(value string) bool {
	if len(value) != 32 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type preAuditStore struct{ dir string }

func (a *App) preAuditStore() preAuditStore {
	return preAuditStore{dir: filepath.Join(filepath.Dir(a.store.Path()), "preaudit-runs")}
}

func (s preAuditStore) secureDir() error {
	if err := os.Mkdir(s.dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("创建预审恢复目录: %w", err)
	}
	return checkPreAuditPath(s.dir, true)
}

func checkPreAuditPath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (directory && !info.IsDir()) || (!directory && !info.Mode().IsRegular()) {
		return fmt.Errorf("预审恢复路径不允许符号链接或特殊文件")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("预审恢复路径权限过宽，需要目录0700/文件0600")
	}
	return nil
}

// One process owns a run across persistence and network operations. A busy run
// fails immediately instead of starting a second creation/review.
func (s preAuditStore) lock(runID string) (func(), error) {
	if !validPreAuditRunID(runID) {
		return nil, fmt.Errorf("run-id 必须为32位小写十六进制")
	}
	if err := s.secureDir(); err != nil {
		return nil, err
	}
	path := filepath.Join(s.dir, runID+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		if err = f.Close(); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err := checkPreAuditPath(path, false); err != nil {
		return nil, err
	}
	lock := flock.New(path)
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("同一run正在执行，请等待原命令完成")
	}
	return func() { _ = lock.Unlock() }, nil
}

// Save before each business write; a failed Sync/replace stops the caller before
// any further network write. Never remove the previous record before replacing it.
func (s preAuditStore) save(run *preAuditRun) error {
	if !validPreAuditRunID(run.RunID) {
		return fmt.Errorf("无效预审runId")
	}
	if err := s.secureDir(); err != nil {
		return err
	}
	path := filepath.Join(s.dir, run.RunID+".json")
	if err := checkPreAuditPath(path, false); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	body, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".preaudit-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err := replacePreAuditFile(tmp.Name(), path); err != nil {
		return fmt.Errorf("保存预审恢复记录: %w", err)
	}
	return nil
}

func (s preAuditStore) load(runID string) (*preAuditRun, error) {
	path := filepath.Join(s.dir, runID+".json")
	if err := checkPreAuditPath(path, false); err != nil {
		return nil, fmt.Errorf("读取预审恢复记录: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	decoder := json.NewDecoder(io.LimitReader(f, 32<<20))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var run preAuditRun
	if err := decoder.Decode(&run); err != nil {
		return nil, fmt.Errorf("预审恢复记录损坏: %w", err)
	}
	if run.SchemaVersion != 1 || run.RunID != runID || run.Profile == "" {
		return nil, fmt.Errorf("预审恢复记录版本不支持或内容损坏")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("预审恢复记录包含多余内容")
	}
	if err := validatePreAuditRecord(&run); err != nil {
		return nil, err
	}
	return &run, nil
}

func validatePreAuditRecord(run *preAuditRun) error {
	invalid := func() error { return fmt.Errorf("预审恢复记录内容损坏或阶段不支持，已停止") }
	positiveID := func(id string) bool {
		value, err := strconv.ParseInt(id, 10, 64)
		return err == nil && value > 0 && strconv.FormatInt(value, 10) == id
	}
	origin, err := url.Parse(run.Origin)
	if err != nil || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || (origin.Scheme != "http" && origin.Scheme != "https") || !positiveID(run.TenantID) || !positiveID(run.UserID) || validateReviewName(run.Name) != nil {
		return invalid()
	}
	switch run.Stage {
	case "upload_unknown":
		return nil
	case "awaiting_category", "create_pending", "pre_audit_created", "reference_unknown", "reference_not_ready", "awaiting_config", "start_unknown", "start_rejected", "running", "review_failed", "review_succeeded", "completed":
	default:
		return invalid()
	}
	if !positiveID(run.SourceFileID) {
		return invalid()
	}
	if run.Stage == "awaiting_category" {
		return nil
	}
	if !positiveID(run.CategoryID) || (run.RequestID != "" && !validPreAuditRunID(run.RequestID)) {
		return invalid()
	}
	if run.Stage == "create_pending" {
		return nil
	}
	if !positiveID(run.PreAuditID) || !positiveID(run.LatestFileID) {
		return invalid()
	}
	switch run.Stage {
	case "pre_audit_created", "reference_unknown", "reference_not_ready":
		return nil
	}
	if _, err := normalizedReviewHash(run.FileHash); !positiveID(run.AIFileID) || err != nil {
		return invalid()
	}
	switch run.Stage {
	case "running", "review_failed", "review_succeeded", "completed":
		if !positiveID(run.TaskID) {
			return invalid()
		}
	}
	if (run.Stage == "review_succeeded" || run.Stage == "completed") && run.Final == nil {
		return invalid()
	}
	return nil
}
