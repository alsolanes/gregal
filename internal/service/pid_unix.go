//go:build !windows

package service

import "syscall"

func processAlive(pid int) bool {
	// EPERM vol dir que el PID existeix però és d'un altre usuari. El lock
	// viu a la carpeta de dades (0700) d'aquest usuari, que cap altre no
	// pot haver escrit: és un PID reciclat, no el nostre servidor. A
	// Android passa sovint (els PID es reutilitzen de pressa) i abans el
	// servidor local es negava a arrencar per sempre.
	if err := syscall.Kill(pid, syscall.Signal(0)); err != nil {
		return false
	}
	return sameProgram(pid)
}
