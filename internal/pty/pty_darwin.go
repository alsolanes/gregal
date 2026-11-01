//go:build darwin

package pty

import (
	"bytes"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openSlave fa el grantpt/unlockpt/ptsname de macOS amb ioctl.
func openSlave(master *os.File) (string, error) {
	fd := master.Fd()
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, unix.TIOCPTYGRANT, 0); e != 0 {
		return "", e
	}
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, unix.TIOCPTYUNLK, 0); e != 0 {
		return "", e
	}
	buf := make([]byte, 128)
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&buf[0]))); e != 0 {
		return "", e
	}
	if i := bytes.IndexByte(buf, 0); i >= 0 {
		buf = buf[:i]
	}
	return string(buf), nil
}
