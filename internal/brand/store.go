package brand

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gregal/internal/ooxml"
	"gregal/internal/tema"
)

// Dir és on viuen els paquets instal·lats: <dades>/brands.
func Dir() string {
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return filepath.Join(d, "brands")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "gregal", "brands")
	}
	return filepath.Join(home, ".local", "share", "gregal", "brands")
}

var mu sync.Mutex

// ErrNotFound és un paquet que no està instal·lat.
var ErrNotFound = errors.New("branding not installed")

func path(id string) string { return filepath.Join(Dir(), Slug(id)+".json") }

// List torna els paquets instal·lats, per nom. Els fitxers que no es
// poden llegir se salten (un fitxer trencat no pot tombar l'app).
func List() []*Pack {
	ents, err := os.ReadDir(Dir())
	if err != nil {
		return nil
	}
	var out []*Pack
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(Dir(), e.Name()))
		if err != nil {
			continue
		}
		p, err := Parse(raw)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// Get torna un paquet instal·lat.
func Get(id string) (*Pack, error) {
	raw, err := os.ReadFile(path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Install desa el paquet (substitueix el que tingui el mateix id) i el
// registra com a tema de l'app i dels documents.
func Install(p *Pack) error {
	if err := p.Normalize(); err != nil {
		return err
	}
	raw, err := p.Marshal()
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	tmp := path(p.ID) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path(p.ID)); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("install branding: %w", err)
	}
	register(p)
	return nil
}

// Remove desinstal·la un paquet.
func Remove(id string) error {
	mu.Lock()
	defer mu.Unlock()
	err := os.Remove(path(id))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	tema.Treu(ThemeName(Slug(id)))
	ooxml.UnregisterTheme(ThemeName(Slug(id)))
	return nil
}

func register(p *Pack) {
	tema.Registra(p.Tema())
	ooxml.RegisterTheme(p.Office())
}

// Setup registra tots els paquets instal·lats i activa el que digui el
// config per als documents ("" = cap). Es crida en arrencar cada front.
func Setup(active string) {
	for _, p := range List() {
		register(p)
	}
	Activate(active)
}

// Activate fa que els documents nous facin servir aquest paquet quan no
// se'n demana cap tema ("" o un id que no hi és = els de sempre). Desar-ho
// al config és cosa de qui crida.
func Activate(id string) {
	id = Slug(id)
	if id == "" {
		ooxml.SetDefaultTheme("")
		return
	}
	ooxml.SetDefaultTheme(ThemeName(id))
}

// Active és l'id del paquet que fan servir els documents ("" = cap).
func Active() string {
	n := ooxml.DefaultThemeName()
	if strings.HasPrefix(n, tema.PrefixBrand) {
		return strings.TrimPrefix(n, tema.PrefixBrand)
	}
	return ""
}

// ActivePack és el paquet actiu (nil = cap).
func ActivePack() *Pack {
	id := Active()
	if id == "" {
		return nil
	}
	p, err := Get(id)
	if err != nil {
		return nil
	}
	return p
}

// BodyFontCSS i HeadingFontCSS són les llistes CSS de lletra del paquet per
// a documents que es llegeixen fora de l'app ("" = la de sempre).
func (p *Pack) BodyFontCSS() string {
	return fontStack(p.Fonts.Body, `system-ui,-apple-system,"Segoe UI",Roboto,sans-serif`)
}

func (p *Pack) HeadingFontCSS() string {
	return fontStack(p.Fonts.Heading, `system-ui,-apple-system,"Segoe UI",Roboto,sans-serif`)
}

// AllCSS són els blocs de tots els paquets instal·lats, per al web.
func AllCSS() string {
	var b strings.Builder
	for _, p := range List() {
		b.WriteString(p.CSS())
	}
	return b.String()
}
