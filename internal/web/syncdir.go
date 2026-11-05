package web

import (
	"errors"
	"os"
	"runtime"
	"syscall"
)

// syncDir fa durador un rename dins de dir (fsync del directori). A Windows
// no es pot obrir un directori per fer-hi Sync («Access is denied») i NTFS ja
// fa el canvi de nom de manera atòmica i registrada: no cal ni s'ha de fallar.
// Abans, aquest error impedia arrencar qualsevol colla a Windows.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	cerr := d.Close()
	// Alguns sistemes de fitxers (xarxa, FUSE) no admeten fsync de directoris.
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
		err = nil
	}
	if err != nil {
		return err
	}
	return cerr
}
