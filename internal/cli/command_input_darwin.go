//go:build darwin

package cli

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func pollCommandInputDescriptor(fd int) (bool, error) {
	// Darwin poll/kqueue can miss the last FIFO writer closing. Select observes
	// that EOF without involving Go's runtime poller or a blocking reader.
	if fd < 0 || fd >= unix.FD_SETSIZE {
		return false, fmt.Errorf("command input descriptor %d exceeds select limit %d", fd, unix.FD_SETSIZE)
	}
	var readable unix.FdSet
	readable.Set(fd)
	timeout := unix.NsecToTimeval(100_000_000)
	count, err := unix.Select(fd+1, &readable, nil, nil, &timeout)
	return count > 0, err
}
