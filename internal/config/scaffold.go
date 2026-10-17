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
	if err := os.WriteFile(path, []byte(defaultTemplate()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// AgentsTemplate és l'AGENTS.md que `gregal init` deixa al projecte.
const AgentsTemplate = `# AGENTS.md — gregal

Coding agent (Go + Bubble Tea). Read this before making changes.

## Commands

- 'go build -o gregal .' — build
- 'go test ./...' — tests (must pass before committing)
- './gregal --check-config' — validate the config

## Conventions

- Reply in the user's chosen language (English by default; Catalan supported).
- Tools that write ask for confirmation; the hard denylist blocks on its own.
- '/verify' calls the reviewer model with the diff included; without an
  explicit verdict there is no approval.
- Do not commit 'config.yaml' (it holds keys) or binaries.
`
