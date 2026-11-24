// Package privatefile contains the shared private-state file replacement helper.
package privatefile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Write keeps the previous file intact until the complete private replacement
// is synced. It does not serialize read/modify/write transactions.
func Write(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("private state target must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".private-*")
	if err != nil {
		return err
	}
	defer func() { _ = temporary.Close(); _ = os.Remove(temporary.Name()) }()
	if err := temporary.Chmod(0600); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return Replace(temporary.Name(), path)
}
