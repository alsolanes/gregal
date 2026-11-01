//go:build linux

package pty

import (
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// openSlave desbloqueja l'esclau i en torna el camí (/dev/pts/N).
func openSlave(master *os.File) (string, error) {
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		return "", err
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		return "", err
	}
	return "/dev/pts/" + strconv.Itoa(n), nil
}
