// Package profile diu on viu el perfil de cada producte: la carpeta de
// configuració (config.yaml, auth.yaml, memòria, ordres, estat al costat del
// config) i la de dades (sessions, comptes, tokens, descriptor de servei…).
//
// Gregal i Gregal Jocs són dues apps a part i no comparteixen res:
//
//	Gregal       ~/.config/gregal        ~/.local/share/gregal
//	Gregal Jocs  ~/.config/gregal-jocs   ~/.local/share/gregal-jocs
//
// El binari tria el producte un cop, en arrencar (cli.Main → SetProduct), i
// tot el que resol una ruta de perfil passa per aquí. Les variables d'entorn
// de sempre (GREGAL_DATA_DIR, GREGAL_AUTH, GREGAL_MEMORY i --config) manen
// per damunt, sigui quin sigui el producte. Les rutes de Gregal són les
// d'abans: cap migració.
//
// El paquet no importa res de Gregal: el pot fer servir qualsevol paquet.
package profile

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var product atomic.Value // string

// SetProduct tria el producte d'aquest procés ("" = Gregal; "jocs" =
// Gregal Jocs). Es crida en arrencar, abans de resoldre cap ruta.
func SetProduct(p string) { product.Store(strings.TrimSpace(p)) }

// Product és el producte d'aquest procés ("" = Gregal).
func Product() string {
	p, _ := product.Load().(string)
	return p
}

// DirNameFor és el nom de les carpetes del perfil del producte p: "gregal"
// per a Gregal i "gregal-<p>" per als altres ("gregal-jocs").
func DirNameFor(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "gregal"
	}
	return "gregal-" + p
}

// DirName és el nom de les carpetes del perfil d'aquest procés.
func DirName() string { return DirNameFor(Product()) }

// ConfigDirFor és la carpeta de configuració del producte p
// (~/.config/<nom>). Sense home, una ruta relativa (com abans).
func ConfigDirFor(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", DirNameFor(p))
	}
	return filepath.Join(home, ".config", DirNameFor(p))
}

// ConfigDir és la carpeta de configuració d'aquest procés.
func ConfigDir() string { return ConfigDirFor(Product()) }

// DefaultDataDirFor és la carpeta de dades del producte p sense mirar
// GREGAL_DATA_DIR (~/.local/share/<nom>).
func DefaultDataDirFor(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".local", "share", DirNameFor(p))
	}
	return filepath.Join(home, ".local", "share", DirNameFor(p))
}

// DataDir és la carpeta de dades d'aquest procés: GREGAL_DATA_DIR si hi
// és, si no ~/.local/share/<nom>.
func DataDir() string {
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return d
	}
	return DefaultDataDirFor(Product())
}

// UserConfigSubdir és <os.UserConfigDir()>/<nom> (l'estat que abans anava a
// %AppData%\gregal o ~/.config/gregal quan no hi havia config). "" si el
// sistema no en té.
func UserConfigSubdir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, DirName())
}

// CacheSubdir és <os.UserCacheDir()>/<nom>, o "" si el sistema no en té.
func CacheSubdir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, DirName())
}
