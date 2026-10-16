//go:build !windows

package officerender

import (
	"os/exec"
	"syscall"
)

// hideWindow posa el procés en un grup propi perquè, en cancel·lar, es
// pugui matar tot l'arbre (soffice engega soffice.bin).
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

func killStray() {}
