//go:build !unix && !windows

package cli

import (
	"fmt"
	"os"
)

func duplicateCommandInputFile(file *os.File) (*os.File, error) {
	return nil, &os.PathError{Op: "duplicate", Path: file.Name(), Err: fmt.Errorf("cancellable file input is unavailable on this platform")}
}
