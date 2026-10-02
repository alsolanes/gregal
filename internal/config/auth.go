package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Magatzem de claus d'API, separat del config.
//
// Fins ara una clau escrita al TUI només vivia en memòria: el missatge
// deia «per persistir usa ${VAR_ENTORN}» i, si no en tenies cap definida,
// cada arrencada te la tornava a demanar. El config tampoc és el lloc:
// és un fitxer que la gent edita, comparteix i enganxa als informes.
//
// Les claus van a ~/.config/gregal/auth.yaml amb permisos 0600. El
// config.yaml continua sense secrets: hi pots deixar ${VAR} si ho
// prefereixes, i aleshores mana la variable d'entorn.

// AuthPath retorna la ruta del magatzem de claus.
func AuthPath() string {
	// GREGAL_AUTH mou el magatzem: els tests l'apunten a una carpeta
	// temporal perquè mai no escriguin al fitxer de claus de debo.
	if p := strings.TrimSpace(os.Getenv("GREGAL_AUTH")); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "auth.yaml"
	}
	return filepath.Join(home, ".config", "gregal", "auth.yaml")
}

type authFile struct {
	Keys map[string]string `yaml:"keys"`
}

var authMu sync.Mutex

// LoadAuth llegeix les claus desades (mapa buit si no n'hi ha cap).
func LoadAuth() map[string]string {
	authMu.Lock()
	defer authMu.Unlock()
	return loadAuthLocked()
}

func loadAuthLocked() map[string]string {
	raw, err := os.ReadFile(AuthPath())
	if err != nil {
		return map[string]string{}
	}
	var a authFile
	if yaml.Unmarshal(raw, &a) != nil || a.Keys == nil {
		return map[string]string{}
	}
	return a.Keys
}

// SaveKey desa (o esborra, amb clau buida) la clau d'un provider.
func SaveKey(provider, key string) error {
	authMu.Lock()
	defer authMu.Unlock()
	keys := loadAuthLocked()
	if strings.TrimSpace(key) == "" {
		delete(keys, provider)
	} else {
		keys[provider] = key
	}
	path := AuthPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(authFile{Keys: keys})
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	// Com el config: encara que el fitxer ja existís amb permisos més
	// oberts, aquí hi ha secrets.
	return os.Chmod(path, 0o600)
}

// aplicaAuth omple les api_key buides amb les del magatzem. L'ordre de
// preferència és: el que digui el config (inclosa una ${VAR} resolta),
// i només si allò queda buit, la clau desada.
func (c *Config) aplicaAuth() {
	if c == nil || len(c.Providers) == 0 {
		return
	}
	keys := LoadAuth()
	if len(keys) == 0 {
		return
	}
	for name, p := range c.Providers {
		if strings.TrimSpace(p.APIKey) != "" {
			continue
		}
		if k, ok := keys[name]; ok && strings.TrimSpace(k) != "" {
			p.APIKey = k
			c.Providers[name] = p
		}
	}
}
