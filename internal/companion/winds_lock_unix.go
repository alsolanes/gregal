//go:build !windows

package companion

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockWindFile(f *os.File) error {
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX)
		switch err {
		case nil, unix.EINVAL, unix.ENOTSUP:
			return nil
		case unix.EINTR:
			continue
		default:
			if err == unix.EOPNOTSUPP {
				return nil
			}
			return err
		}
	}
}

func unlockWindFile(f *os.File) {
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
