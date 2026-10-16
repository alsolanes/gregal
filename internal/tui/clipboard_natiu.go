//go:build windows || darwin

package tui

import (
	"runtime"

	"github.com/atotto/clipboard"
)

// copiaNatiu fa servir el porta-retalls del sistema: l'API Win32 a Windows
// (no hi ha /dev/tty per a l'OSC52 ni wl-copy/xclip) i pbcopy a macOS.
func copiaNatiu(text string) (string, error) {
	if err := clipboard.WriteAll(text); err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return "Windows", nil
	}
	return "pbcopy", nil
}
