package credential

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func privateTaskKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, "credential.key")
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		tmp, err := os.CreateTemp(dir, ".key-*")
		if err != nil {
			return nil, err
		}
		defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
		if _, err := tmp.Write(key); err != nil {
			return nil, err
		}
		if err := tmp.Sync(); err != nil {
			return nil, err
		}
		if err := tmp.Close(); err != nil {
			return nil, err
		}
		// Publish only a complete key, without replacing a concurrent winner.
		if err := os.Link(tmp.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create private task key: %w", err)
		}
	} else if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != 32 {
		return nil, errors.New("private task key is invalid; reauthorize after repairing private task state")
	}
	if err := os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
