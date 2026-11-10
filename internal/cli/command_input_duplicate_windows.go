//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

func duplicateCommandInputFile(file *os.File) (*os.File, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	var duplicate windows.Handle
	var duplicateErr error
	if err := connection.Control(func(original uintptr) {
		process := windows.CurrentProcess()
		duplicateErr = windows.DuplicateHandle(process, windows.Handle(original), process, &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS)
	}); err != nil {
		return nil, err
	}
	if duplicateErr != nil {
		return nil, &os.PathError{Op: "duplicate", Path: file.Name(), Err: duplicateErr}
	}
	return os.NewFile(uintptr(duplicate), file.Name()), nil
}
