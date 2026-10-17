//go:build !windows && !linux

package service

// sameProgram: sense /proc no es pot comprovar; un PID viu compta com a nostre.
func sameProgram(int) bool { return true }
