//go:build !windows && !darwin

package tui

import "errors"

// copiaNatiu: a Linux i companyia no hi ha API única; es prova la cadena
// wl-copy → xclip → xsel → OSC52 de copyToClipboard.
func copiaNatiu(string) (string, error) {
	return "", errors.New("sense porta-retalls natiu")
}
