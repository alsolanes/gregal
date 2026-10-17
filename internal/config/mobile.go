package config

import (
	"os"
	"runtime"
	"strings"
)

// Configuració per defecte al mòbil.
//
// L'app d'Android executa aquest mateix backend com a servidor local. Un
// telèfon no té un shell de desenvolupament ni un navegador que l'agent
// pugui controlar, i el workspace és una carpeta privada de l'app: per
// defecte s'hi treballa amb documents, web, MCP, coneixement i xat, i les
// eines que necessiten un ordinador queden denegades. Qui en vulgui alguna
// la pot tornar a permetre editant el config (permissions.tools), com a
// qualsevol altra plataforma.

// mobilePermissionsFrom és la línia del config per defecte que se
// substitueix; el test comprova que la plantilla encara la té.
const mobilePermissionsFrom = "permissions:\n  tools: {write: ask, edit: ask}\n"

const mobilePermissionsTo = `permissions:
  # Mobile defaults: no local shell, browser automation or git on the device.
  # Allow them here if your device can run them.
  tools: {write: ask, edit: ask, bash: deny, bash_background: deny, browser: deny, git_commit: deny, gh_pr_create: deny}
`

// MobileDevice diu si el backend corre en un mòbil: compilat per a Android,
// o forçat amb GREGAL_PLATFORM=android (tests i proves a l'escriptori).
func MobileDevice() bool {
	if runtime.GOOS == "android" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("GREGAL_PLATFORM")), "android")
}

// MobileYAML adapta una plantilla de config al mòbil.
func MobileYAML(src string) string {
	return strings.Replace(src, mobilePermissionsFrom, mobilePermissionsTo, 1)
}

// defaultTemplate és el config que s'escriu la primera vegada en aquesta
// plataforma.
func defaultTemplate() string {
	if MobileDevice() {
		return MobileYAML(defaultYAML)
	}
	return defaultYAML
}
