//go:build windows

package procs

import (
	"os/exec"
	"strconv"
)

// A Windows no hi ha grups de processos POSIX. taskkill /T atura també els
// descendents; els arguments es passen directament, sense cap intèrpret.
func setGroup(c *exec.Cmd) {}

func killGroup(c *exec.Cmd) {
	if c.Process == nil {
		return
	}
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(c.Process.Pid)).Run(); err != nil {
		_ = c.Process.Kill()
	}
}
