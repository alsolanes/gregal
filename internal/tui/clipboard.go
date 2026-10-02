package tui

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// copyToClipboard prova wl-copy → xclip → xsel → OSC52 (/dev/tty).
// Torna com ho ha fet ("wl-copy", "xclip", "xsel", "OSC52") o error honest
// (p. ex. instal·la wl-clipboard o xclip).
func copyToClipboard(text string) (string, error) {
	run := func(name string, args ...string) error {
		// Cada backend necessita el seu context. Si, per exemple, wl-copy
		// es queda penjat fins al timeout, reutilitzar aquell context faria
		// que xclip, xsel i l'OSC52 naixessin ja cancel·lats.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, name, args...)
		w, err := c.StdinPipe()
		if err != nil {
			return err
		}
		if err := c.Start(); err != nil {
			return err
		}
		if _, err := w.Write([]byte(text)); err != nil {
			_ = c.Wait()
			return err
		}
		w.Close()
		return c.Wait()
	}
	if err := run("wl-copy"); err == nil {
		return "wl-copy", nil
	}
	if err := run("xclip", "-selection", "clipboard"); err == nil {
		return "xclip", nil
	}
	if err := run("xsel", "--clipboard", "--input"); err == nil {
		return "xsel", nil
	}
	// OSC52: funciona per SSH i a la majoria de terminals moderns.
	seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return "", errors.New(T("clip.sense"))
	}
	defer f.Close()
	if _, err := f.WriteString(seq); err != nil {
		return "", fmt.Errorf("%s", fmt.Sprintf(T("clip.osc52"), err))
	}
	return "OSC52", nil
}
