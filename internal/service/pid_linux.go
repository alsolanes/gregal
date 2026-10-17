package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// sameProgram diu si el PID viu executa aquest mateix binari. Un PID del
// mateix usuari pot ser reciclat per un altre programa (a Android, el
// mateix procÃ©s de l'app): llavors el lock Ã©s orfe. Si no es pot saber
// (sense /proc), es considera viu, com abans.
func sameProgram(pid int) bool {
	other, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return true
	}
	self, err := os.Executable()
	if err != nil {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	// Un binari substituït mentre corre (actualització) surt com «… (deleted)».
	other = strings.TrimSuffix(other, " (deleted)")
	return filepath.Clean(other) == filepath.Clean(self)
}
