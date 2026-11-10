//go:build unix && !darwin

package cli

import "golang.org/x/sys/unix"

func pollCommandInputDescriptor(fd int) (bool, error) {
	poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	_, err := unix.Poll(poll, 100)
	if poll[0].Revents&unix.POLLNVAL != 0 {
		return false, unix.EBADF
	}
	return poll[0].Revents != 0, err
}
