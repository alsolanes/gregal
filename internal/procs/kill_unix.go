//go:build !windows

package procs

import (
	"os/exec"
	"syscall"
)

type unixProcessGroup struct{}

// newProcessGroup posa el procés en un grup propi: `sh -c "npm run dev"` crea
// fills, i matar només la shell deixaria els nets vius (i els pipes
// oberts, amb la sortida penjada per sempre).
func newProcessGroup(c *exec.Cmd) (processGroup, error) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &unixProcessGroup{}, nil
}

func (*unixProcessGroup) start(*exec.Cmd) error { return nil }

// kill mata tot el grup del procés.
func (*unixProcessGroup) kill(c *exec.Cmd) {
	if c.Process == nil {
		return
	}
	if err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL); err != nil {
		_ = c.Process.Kill()
	}
}

func (*unixProcessGroup) close() {}
