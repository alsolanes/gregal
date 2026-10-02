// Package session desa i recupera converses a ~/.local/share/gregal/sessions.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gregal/internal/llm"
)

// Session és una conversa desada.
type Session struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	// Title és el nom llegible de la conversa, tret del primer missatge.
	// Name continua sent l'identificador del fitxer: les sessions d'abans
	// no en tenen i el llistat hi posa el Name.
	Title     string    `json:"title,omitempty"`
	SavedAt   time.Time `json:"saved_at"`
	Role      string    `json:"role"`
	Workspace string    `json:"workspace,omitempty"`
	Pinned    bool      `json:"pinned,omitempty"`
	Compacted string    `json:"compacted,omitempty"`
	// ModelOverrides conserva la tria de model d'aquesta conversa per rol.
	// Buida en sessions antigues; aquestes hereten els valors per defecte.
	ModelOverrides map[string]string `json:"model_overrides,omitempty"`
	Convo          []llm.Message     `json:"convo"`
	Activity       []Activity        `json:"activity,omitempty"`
}

// Activity desa la vista estructurada d'eines sense lligar session amb el TUI.
// Els camps nous són opcionals perquè les sessions antigues continuïn obrint.
type Activity struct {
	Name   string `json:"name"`
	Args   string `json:"args,omitempty"`
	Output string `json:"output,omitempty"`
	Failed bool   `json:"failed,omitempty"`
	Done   bool   `json:"done,omitempty"`
}

// Info resumeix una sessió per llistats.
type Info struct {
	Name      string
	Title     string
	SavedAt   time.Time
	Msgs      int
	Workspace string
	Pinned    bool
}

// TitleFrom tria el títol d'una conversa a partir del primer missatge de
// l'usuari, com fan ChatGPT i Claude: una conversa sense nom no es torna a
// obrir mai perquè no saps quina és.
func TitleFrom(convo []llm.Message) string {
	for _, m := range convo {
		if m.Role != "user" {
			continue
		}
		t := strings.TrimSpace(m.Content)
		// Les mencions expandides i els blocs adjunts van a part: el títol
		// surt de la primera línia del que va escriure la persona.
		if i := strings.Index(t, "\n\n[@"); i > 0 {
			t = t[:i]
		}
		if i := strings.IndexByte(t, '\n'); i > 0 {
			t = t[:i]
		}
		t = strings.TrimSpace(strings.TrimPrefix(t, "🤖 "))
		if t == "" {
			continue
		}
		r := []rune(t)
		if len(r) > 60 {
			return strings.TrimSpace(string(r[:59])) + "…"
		}
		return t
	}
	return ""
}

// DefaultDir retorna ~/.local/share/gregal/sessions.
func DefaultDir() string {
	return filepath.Join(baseDataDir(), "sessions")
}

// DirFor retorna la carpeta de converses d'un usuari. Cada usuari veu
// només les seves: sense això, dues persones comparteixen llista i es
// poden reprendre converses l'una de l'altra.
//
// Compte: la comprovació de buit va ABANS de slugify, perquè slugify("")
// torna «sessio» (és el nom que es dona a una conversa sense nom) i la
// carpeta de qui no té usuari seria «sessions/sessio» en comptes de
// «sessions».
func DirFor(user string) string {
	user = strings.TrimSpace(user)
	if user == "" {
		return DefaultDir()
	}
	return filepath.Join(DefaultDir(), slugify(user))
}

func baseDataDir() string {
	// GREGAL_DATA_DIR permet apuntar-ho tot (converses, tokens,
	// contrasenyes) a un altre lloc: és el que fan les proves per no
	// embrutar les converses de debò, i serveix per tenir dues instàncies.
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "gregal")
	}
	return filepath.Join(home, ".local", "share", "gregal")
}

func slugify(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "sessio"
	}
	return s
}

// NewName genera un nom de sessió amb prou precisió temporal per evitar
// col·lisions quan dues sessions es desen en el mateix segon.
func NewName() string {
	now := time.Now()
	return fmt.Sprintf("sessio-%s-%03d", now.Format("20060102-150405"), now.Nanosecond()/1_000_000)
}

// SaveSession desa la sessió de manera atòmica i retorna la ruta.
func SaveSession(dir string, s Session) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	slug := slugify(s.Name)
	if slug == "sessio" || s.Name == "" {
		slug = NewName()
	}
	s.Name = slug
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Title == "" {
		s.Title = TitleFrom(s.Convo)
	}
	if s.SavedAt.IsZero() {
		s.SavedAt = time.Now()
	}
	path := filepath.Join(dir, slug+".json")
	tmpPath := filepath.Join(dir, fmt.Sprintf(".%s.%d.tmp", slug, time.Now().UnixNano()))

	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return "", err
	}
	// Escriptura atòmica via rename
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmpPath, path); err2 != nil {
			_ = os.Remove(tmpPath)
			return "", err2
		}
	}
	return path, nil
}

// Save desa la conversa i retorna la ruta (mantenint retrocompatibilitat).
func Save(dir, name, role string, convo []llm.Message) (string, error) {
	return SaveSession(dir, Session{
		Name:  name,
		Role:  role,
		Convo: convo,
	})
}

// List retorna sessions ordenades (recents primer).
func List(dir string) ([]Info, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		title := s.Title
		if title == "" {
			// Sessions d'abans del títol, o desades sense cap missatge
			// d'usuari: se'n treu un del contingut si es pot.
			title = TitleFrom(s.Convo)
		}
		out = append(out, Info{
			Name:      s.Name,
			Title:     title,
			SavedAt:   s.SavedAt,
			Msgs:      len(s.Convo),
			Workspace: s.Workspace,
			Pinned:    s.Pinned,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt.After(out[j].SavedAt) })
	return out, nil
}

// Load carrega una sessió per nom (amb o sense .json).
func Load(dir, name string) (Session, error) {
	name = strings.TrimSuffix(slugify(name), ".json")
	raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return Session{}, fmt.Errorf("sessió %q no trobada", name)
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return Session{}, fmt.Errorf("sessió %q corrupta: %w", name, err)
	}
	return s, nil
}

// Delete esborra una sessió per nom (amb o sense .json).
func Delete(dir, name string) error {
	name = strings.TrimSuffix(slugify(name), ".json")
	path := filepath.Join(dir, name+".json")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("sessió %q no trobada", name)
	}
	return os.Remove(path)
}

// HistoryPath retorna ~/.local/share/gregal/history.
func HistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "gregal", "history")
	}
	return filepath.Join(home, ".local", "share", "gregal", "history")
}

// MaxHistory és el topall d'entrades desades.
const MaxHistory = 500

// LoadHistory llegeix l'historial (una entrada per línia, sense buides).
func LoadHistory() []string {
	raw, err := os.ReadFile(HistoryPath())
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	if len(out) > MaxHistory {
		out = out[len(out)-MaxHistory:]
	}
	return out
}

// SaveHistory desa l'historial (retalla a MaxHistory).
func SaveHistory(entries []string) error {
	if len(entries) > MaxHistory {
		entries = entries[len(entries)-MaxHistory:]
	}
	if err := os.MkdirAll(filepath.Dir(HistoryPath()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(HistoryPath(), []byte(strings.Join(entries, "\n")+"\n"), 0o600)
}
