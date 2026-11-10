package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ImportedModel és el model que fa servir un altre perfil (el de Gregal,
// llegit des de Gregal Jocs): el proveïdor, el model i la clau. Serveix per
// a una còpia única cap al perfil propi; mai no queda cap enllaç viu.
type ImportedModel struct {
	Provider string   // nom del proveïdor ("openrouter")
	Model    string   // id del model
	Settings Provider // la definició del proveïdor, sense clau
	// RawKey és "${VAR}" quan l'altre perfil llegeix la clau de l'entorn
	// (es copia la referència, no el valor).
	RawKey string
	// Key és la clau literal (del config.yaml o de l'auth.yaml de l'altre
	// perfil). Buida si no n'hi ha o si és una ${VAR}.
	Key string
	// Role és el rol d'on surt (context_window, temperature…).
	Role Role
}

// EffectiveKey és la clau amb què respondria ara mateix (la ${VAR}
// resolta si cal).
func (m ImportedModel) EffectiveKey() string {
	if m.Key != "" {
		return m.Key
	}
	if m.RawKey != "" {
		return os.ExpandEnv(m.RawKey)
	}
	return ""
}

// profileFile és el que en llegim: només proveïdors, rols i mode.
type profileFile struct {
	Mode      string              `yaml:"mode"`
	Providers map[string]Provider `yaml:"providers"`
	Roles     map[string]Role     `yaml:"roles"`
}

// ErrNoProfileModel vol dir que l'altre perfil no té cap model triat.
var ErrNoProfileModel = errors.New("no model in that profile")

// ReadProfileModel llegeix, només lectura, el model del perfil que viu a dir
// (el seu config.yaml i, per a la clau, el seu auth.yaml). No crea ni
// modifica res. selections és el model que l'usuari hi ha triat a la UI per
// a cada rol ("proveïdor/model" o "model"; pot ser nil): mana per damunt del
// config, com fa el servidor.
func ReadProfileModel(dir string, selections map[string]string) (ImportedModel, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return ImportedModel{}, err
	}
	var f profileFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return ImportedModel{}, err
	}
	c := &Config{Mode: f.Mode, Roles: f.Roles}
	roleName := c.InitialRole()
	role, ok := f.Roles[roleName]
	if !ok {
		return ImportedModel{}, ErrNoProfileModel
	}
	if sel := strings.TrimSpace(selections[roleName]); sel != "" {
		if i := strings.Index(sel, "/"); i > 0 {
			if _, ok := f.Providers[sel[:i]]; ok {
				role.Provider, role.Model = sel[:i], sel[i+1:]
			}
		} else {
			role.Model = sel
		}
	}
	p, ok := f.Providers[role.Provider]
	if !ok || strings.TrimSpace(role.Model) == "" {
		return ImportedModel{}, ErrNoProfileModel
	}
	m := ImportedModel{Provider: role.Provider, Model: strings.TrimSpace(role.Model), Role: role}
	key := strings.TrimSpace(p.APIKey)
	p.APIKey = ""
	m.Settings = p
	switch {
	case strings.Contains(key, "${"):
		m.RawKey = key
	case key != "":
		m.Key = key
	}
	// Com Load (aplicaAuth): la clau desada només omple la que el config
	// deixa buida (o una ${VAR} sense valor).
	if m.EffectiveKey() == "" {
		if k := strings.TrimSpace(readAuthKeys(filepath.Join(dir, "auth.yaml"))[role.Provider]); k != "" {
			m.Key, m.RawKey = k, ""
		}
	}
	return m, nil
}

// readAuthKeys llegeix un auth.yaml qualsevol sense el candau del propi.
func readAuthKeys(path string) map[string]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var a authFile
	if yaml.Unmarshal(raw, &a) != nil {
		return nil
	}
	return a.Keys
}
