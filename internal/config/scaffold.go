package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitConfig escriu el config per defecte (només si no existeix, o amb force).
func InitConfig(path string, force bool) (string, error) {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); err == nil && !force {
		return "", fmt.Errorf("%s ja existeix (fes servir --force per sobreescriure)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(defaultYAML), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// AgentsTemplate és l'AGENTS.md que `gregal init` deixa al projecte.
const AgentsTemplate = `# AGENTS.md — gregal

Coding agent (Go + Bubble Tea). Read this before making changes.

## Comandes

- 'go build -o gregal .' — compila
- 'go test ./...' — tests (han de passar abans de commit)
- './gregal --check-config' — valida el config

## Convencions

- Reply in the user's chosen language (English by default; Catalan supported).
- Eines amb confirmació: escriure demana 's/n'; la denylist dura bloqueja sola.
- '/verify' crida el model reviewer amb el diff inclòs; sense veredicte
  explícit no hi ha aprovació.
- No commitegis 'config.yaml' (porta claus) ni binaris.
`
