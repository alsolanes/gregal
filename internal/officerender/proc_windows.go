//go:build windows

package officerender

import (
	"os/exec"
	"strconv"
	"syscall"
)

// hideWindow evita finestres de consola i, en cancel·lar, mata tot
// l'arbre de processos (soffice.com engega soffice.bin).
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		k := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		return k.Run()
	}
}

func killStray() {}
