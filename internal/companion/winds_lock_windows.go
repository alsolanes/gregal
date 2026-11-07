package companion

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockWindFile(f *os.File) error {
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &windows.Overlapped{})
	if err == windows.ERROR_NOT_SUPPORTED || err == windows.ERROR_INVALID_FUNCTION {
		return nil
	}
	return err
}

func unlockWindFile(f *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{})
}
