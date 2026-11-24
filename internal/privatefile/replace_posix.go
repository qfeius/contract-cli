//go:build !windows

package privatefile

import (
	"os"
	"path/filepath"
)

// Replace commits a same-filesystem state file and syncs its parent directory.
func Replace(source, destination string) error {
	if err := os.Rename(source, destination); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}
