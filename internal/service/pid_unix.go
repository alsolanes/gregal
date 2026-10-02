//go:build !windows

package service

import "syscall"

func processAlive(pid int) bool {
	err := syscall.Kill(pid, syscall.Signal(0))
	// EPERM vol dir que el procés existeix però no tenim permís per sondar-lo.
	return err == nil || err == syscall.EPERM
}
