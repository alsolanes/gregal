//go:build windows

package procs

import "os/exec"

// A Windows no hi ha grups de processos POSIX: matem el procés directe.
func setGroup(c *exec.Cmd) {}

func killGroup(c *exec.Cmd) {
	if c.Process != nil {
		_ = c.Process.Kill()
	}
}
