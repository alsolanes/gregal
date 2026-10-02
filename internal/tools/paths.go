package tools

import (
	"path/filepath"
	"strings"
)

// IsRooted diu si p no és una ruta relativa innocent, mirant-ho amb els ulls
// de totes les plataformes alhora. filepath.IsAbs a Windows demana lletra
// d'unitat, de manera que "/etc/passwd" o "\Windows\System32" hi passen per
// relatives i s'escapen de qualsevol contenció feta amb Join: com que el
// binari també es distribueix per Windows, la comprovació no pot dependre del
// GOOS on corre.
func IsRooted(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	if filepath.IsAbs(p) {
		return true
	}
	if p[0] == '/' || p[0] == '\\' { // arrel POSIX, o arrel del volum a Windows
		return true
	}
	// "C:" i "C:fitxer": relatiu al directori actual d'una altra unitat.
	if len(p) >= 2 && p[1] == ':' {
		c := p[0]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}
