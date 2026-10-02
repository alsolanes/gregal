//go:build !windows

package procs

import (
	"os/exec"
	"syscall"
)

// setGroup posa el procés en un grup propi: `sh -c "npm run dev"` crea
// fills, i matar només la shell deixaria els nets vius (i els pipes
// oberts, amb la sortida penjada per sempre).
func setGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup mata tot el grup del procés.
func killGroup(c *exec.Cmd) {
	if c.Process == nil {
		return
	}
	if err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL); err != nil {
		_ = c.Process.Kill()
	}
}
