//go:build windows

package tools

import (
	"io/fs"
	"syscall"
)

// Atributs que Windows posa als fitxers que només són al núvol (Nextcloud,
// OneDrive…): obrir-los en demana la baixada.
const (
	fileAttributeOffline            = 0x00001000
	fileAttributeRecallOnOpen       = 0x00040000
	fileAttributeRecallOnDataAccess = 0x00400000
)

func onlineOnly(info fs.FileInfo) bool {
	if a, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return a.FileAttributes&(fileAttributeOffline|fileAttributeRecallOnOpen|fileAttributeRecallOnDataAccess) != 0
	}
	return false
}
