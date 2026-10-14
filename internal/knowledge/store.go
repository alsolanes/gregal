// Package knowledge són les col·leccions de coneixement: carpetes i fitxers
// (md, text, codi, PDF, Word, Excel, PowerPoint, HTML) i notes que l'agent
// pot consultar amb l'eina knowledge_search.
//
// Cada col·lecció té un abast:
//
//   - global: per a tots els projectes, a <dades>/knowledge
//     (GREGAL_DATA_DIR o ~/.local/share/gregal);
//   - project: viu dins del projecte, a .gregal/knowledge, i viatja amb el
//     repositori (els índexs, que depenen de la màquina, s'ignoren);
//   - session: lligada a una sola conversa, a <dades>/knowledge/sessions.
//
// La cerca és BM25 per paraules, sense xarxa. Si el config té
// knowledge.embeddings i el provider ofereix /embeddings, també es desen
// vectors i el rànquing és híbrid (fusió de rànquings).
//
// El paquet no depèn del config (el config el configura a Load) ni de
// l'agent: així agent, web i TUI hi poden arribar sense cicles.
package knowledge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Abasts d'una col·lecció.
const (
	ScopeGlobal  = "global"
	ScopeProject = "project"
	ScopeSession = "session"
)

// SessionNotesName és la col·lecció on van les notes de l'agent quan no en
// diu cap: la de la sessió, que es crea quan cal.
const SessionNotesName = "Session notes"

var (
	ErrNotFound   = errors.New("knowledge collection not found")
	ErrNoProject  = errors.New("no project folder for project-scoped knowledge")
	ErrNoSession  = errors.New("no session for session-scoped knowledge")
	ErrBadScope   = errors.New("scope must be global, project or session")
	ErrNameNeeded = errors.New("a collection needs a name")
	ErrDuplicate  = errors.New("a collection with that name already exists in this scope")
)

// Source és una carpeta o un fitxer enllaçat per ruta. Include i Exclude
// són globs sobre la ruta relativa a l'arrel (`*.md`, `docs/**`,
// `**/*.pdf`); sense Include entra tot el que se sap llegir.
type Source struct {
	ID      string    `json:"id"`
	Path    string    `json:"path"`
	Include []string  `json:"include,omitempty"`
	Exclude []string  `json:"exclude,omitempty"`
	Added   time.Time `json:"added"`
}

// Note és un text escrit a mà (per la persona o per l'agent).
type Note struct {
	ID      string    `json:"id"`
	Title   string    `json:"title,omitempty"`
	Text    string    `json:"text"`
	Author  string    `json:"author,omitempty"`
	Created time.Time `json:"created"`
}

// Collection és una base de coneixement.
type Collection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Scope       string    `json:"scope"`
	Enabled     bool      `json:"enabled"`
	Sources     []Source  `json:"sources"`
	Notes       []Note    `json:"notes,omitempty"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// Where diu des d'on es mira: el projecte (workspace) i la sessió. Les
// col·leccions de projecte i de sessió només existeixen si n'hi ha.
type Where struct {
	Project string
	Session string
}

type storeFile struct {
	Version     int          `json:"version"`
	Collections []Collection `json:"collections"`
}

// mu protegeix els collections.json (llegir-modificar-escriure).
var mu sync.Mutex

// DataDir és l'arrel de dades de Gregal (GREGAL_DATA_DIR o
// ~/.local/share/gregal), la mateixa que fan servir les sessions.
func DataDir() string {
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "gregal")
	}
	return filepath.Join(home, ".local", "share", "gregal")
}

// ScopeDir és el directori on viuen les col·leccions d'un abast.
func ScopeDir(scope string, w Where) (string, error) {
	switch scope {
	case ScopeGlobal:
		return filepath.Join(DataDir(), "knowledge"), nil
	case ScopeProject:
		if strings.TrimSpace(w.Project) == "" {
			return "", ErrNoProject
		}
		return filepath.Join(w.Project, ".gregal", "knowledge"), nil
	case ScopeSession:
		s := slug(w.Session)
		if s == "" {
			return "", ErrNoSession
		}
		return filepath.Join(DataDir(), "knowledge", "sessions", s), nil
	}
	return "", ErrBadScope
}

// scopeOfID treu l'abast del prefix de l'id (g-, p-, s-).
func scopeOfID(id string) string {
	switch {
	case strings.HasPrefix(id, "g-"):
		return ScopeGlobal
	case strings.HasPrefix(id, "p-"):
		return ScopeProject
	case strings.HasPrefix(id, "s-"):
		return ScopeSession
	}
	return ""
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ', r == '.':
			b.WriteRune('-')
		}
	}
	out := b.String()
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

func newID(prefix string) string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

func loadScope(dir string) (storeFile, error) {
	var sf storeFile
	raw, err := os.ReadFile(filepath.Join(dir, "collections.json"))
	if errors.Is(err, os.ErrNotExist) {
		return storeFile{Version: 1}, nil
	}
	if err != nil {
		return sf, err
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return sf, fmt.Errorf("knowledge: %s: %w", filepath.Join(dir, "collections.json"), err)
	}
	return sf, nil
}

func saveScope(dir, scope string, sf storeFile) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if scope == ScopeProject {
		// La definició viatja amb el repositori; els índexs no (rutes i
		// dates d'aquesta màquina, i poden pesar).
		gi := filepath.Join(dir, ".gitignore")
		if _, err := os.Stat(gi); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(gi, []byte("*/index.gob\n*/index.gob.tmp\n"), 0o644)
		}
	}
	sf.Version = 1
	raw, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "collections.json"), raw)
}

func writeAtomic(path string, raw []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// scopesOf són els abasts visibles des de w, del més general al més concret.
func scopesOf(w Where) []string {
	out := []string{ScopeGlobal}
	if strings.TrimSpace(w.Project) != "" {
		out = append(out, ScopeProject)
	}
	if slug(w.Session) != "" {
		out = append(out, ScopeSession)
	}
	return out
}

// List torna les col·leccions visibles des de w: globals, del projecte i de
// la sessió, en aquest ordre i per nom dins de cada abast.
func List(w Where) ([]Collection, error) {
	mu.Lock()
	defer mu.Unlock()
	var out []Collection
	for _, scope := range scopesOf(w) {
		dir, err := ScopeDir(scope, w)
		if err != nil {
			continue
		}
		sf, err := loadScope(dir)
		if err != nil {
			return out, err
		}
		cs := sf.Collections
		sort.SliceStable(cs, func(i, j int) bool { return strings.ToLower(cs[i].Name) < strings.ToLower(cs[j].Name) })
		for _, c := range cs {
			c.Scope = scope
			out = append(out, c)
		}
	}
	return out, nil
}

// Get torna una col·lecció per id.
func Get(w Where, id string) (*Collection, error) {
	scope := scopeOfID(id)
	if scope == "" {
		return nil, ErrNotFound
	}
	dir, err := ScopeDir(scope, w)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	sf, err := loadScope(dir)
	if err != nil {
		return nil, err
	}
	for _, c := range sf.Collections {
		if c.ID == id {
			c.Scope = scope
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

// Resolve troba una col·lecció per id o per nom (sense distingir
// majúscules). Amb noms repetits mana la més concreta: sessió, projecte,
// global.
func Resolve(w Where, ref string) (*Collection, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, ErrNotFound
	}
	if c, err := Get(w, ref); err == nil {
		return c, nil
	}
	all, err := List(w)
	if err != nil {
		return nil, err
	}
	for i := len(all) - 1; i >= 0; i-- {
		if strings.EqualFold(all[i].Name, ref) {
			c := all[i]
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

// Create crea una col·lecció buida i activada.
func Create(w Where, name, description, scope string) (*Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameNeeded
	}
	if len([]rune(name)) > 80 {
		return nil, errors.New("collection name is too long (80 characters at most)")
	}
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" {
		scope = ScopeProject
	}
	dir, err := ScopeDir(scope, w)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	sf, err := loadScope(dir)
	if err != nil {
		return nil, err
	}
	for _, c := range sf.Collections {
		if strings.EqualFold(c.Name, name) {
			return nil, ErrDuplicate
		}
	}
	now := time.Now().UTC()
	c := Collection{ID: newID(scope[:1] + "-"), Name: name, Description: strings.TrimSpace(description), Scope: scope, Enabled: true, Sources: []Source{}, Created: now, Updated: now}
	sf.Collections = append(sf.Collections, c)
	if err := saveScope(dir, scope, sf); err != nil {
		return nil, err
	}
	return &c, nil
}

// Update aplica fn a la col·lecció i la desa.
func Update(w Where, id string, fn func(*Collection) error) (*Collection, error) {
	scope := scopeOfID(id)
	if scope == "" {
		return nil, ErrNotFound
	}
	dir, err := ScopeDir(scope, w)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	sf, err := loadScope(dir)
	if err != nil {
		return nil, err
	}
	for i := range sf.Collections {
		if sf.Collections[i].ID != id {
			continue
		}
		c := sf.Collections[i]
		c.Scope = scope
		oldName := c.Name
		if err := fn(&c); err != nil {
			return nil, err
		}
		c.Name = strings.TrimSpace(c.Name)
		if c.Name == "" {
			return nil, ErrNameNeeded
		}
		if !strings.EqualFold(c.Name, oldName) {
			for j, o := range sf.Collections {
				if j != i && strings.EqualFold(o.Name, c.Name) {
					return nil, ErrDuplicate
				}
			}
		}
		c.ID, c.Scope = id, scope
		c.Updated = time.Now().UTC()
		sf.Collections[i] = c
		if err := saveScope(dir, scope, sf); err != nil {
			return nil, err
		}
		return &c, nil
	}
	return nil, ErrNotFound
}

// Delete esborra la col·lecció i el seu índex (no toca els fitxers font).
func Delete(w Where, id string) error {
	scope := scopeOfID(id)
	if scope == "" {
		return ErrNotFound
	}
	dir, err := ScopeDir(scope, w)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	sf, err := loadScope(dir)
	if err != nil {
		return err
	}
	kept := sf.Collections[:0]
	found := false
	for _, c := range sf.Collections {
		if c.ID == id {
			found = true
			continue
		}
		kept = append(kept, c)
	}
	if !found {
		return ErrNotFound
	}
	sf.Collections = kept
	if err := saveScope(dir, scope, sf); err != nil {
		return err
	}
	if id != "" && !strings.ContainsAny(id, `/\.`) {
		_ = os.RemoveAll(filepath.Join(dir, id))
	}
	forgetIndex(filepath.Join(dir, id, "index.gob"))
	return nil
}

// AddSource enllaça una carpeta o un fitxer. Dins d'un projecte, les fonts
// de projecte es desen relatives perquè viatgin amb el repositori.
func AddSource(w Where, id, path string, include, exclude []string) (*Source, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("a source needs a path")
	}
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if !filepath.IsAbs(path) && strings.TrimSpace(w.Project) != "" {
		path = filepath.Join(w.Project, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("source does not exist: %s", abs)
	}
	src := Source{ID: newID("src-"), Path: abs, Include: cleanGlobs(include), Exclude: cleanGlobs(exclude), Added: time.Now().UTC()}
	_, err = Update(w, id, func(c *Collection) error {
		if c.Scope == ScopeProject {
			if rel, ok := relInside(w.Project, abs); ok {
				src.Path = rel
			}
		}
		for _, s := range c.Sources {
			if filepath.Clean(resolveSourcePath(*c, s, w)) == filepath.Clean(abs) {
				return errors.New("that folder is already a source of this collection")
			}
		}
		c.Sources = append(c.Sources, src)
		return nil
	})
	if err != nil {
		return nil, err
	}
	src.Path = abs
	return &src, nil
}

// RemoveSource treu una font (l'índex s'actualitza al pròxim reindex).
func RemoveSource(w Where, id, sourceID string) error {
	_, err := Update(w, id, func(c *Collection) error {
		kept := c.Sources[:0]
		found := false
		for _, s := range c.Sources {
			if s.ID == sourceID {
				found = true
				continue
			}
			kept = append(kept, s)
		}
		if !found {
			return errors.New("source not found")
		}
		c.Sources = kept
		return nil
	})
	return err
}

// MaxNoteChars topa una nota: és per a fets i resums, no per a documents.
const MaxNoteChars = 8000

// AddNote afegeix una nota i l'indexa de seguida (les notes són petites).
func AddNote(w Where, id, title, text, author string) (*Note, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("a note needs text")
	}
	if len([]rune(text)) > MaxNoteChars {
		return nil, fmt.Errorf("note is too long (%d characters at most)", MaxNoteChars)
	}
	n := Note{ID: newID("n-"), Title: strings.TrimSpace(title), Text: text, Author: strings.TrimSpace(author), Created: time.Now().UTC()}
	c, err := Update(w, id, func(c *Collection) error {
		c.Notes = append(c.Notes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := refreshNotes(w, c); err != nil {
		return &n, err
	}
	return &n, nil
}

// RemoveNote esborra una nota.
func RemoveNote(w Where, id, noteID string) error {
	c, err := Update(w, id, func(c *Collection) error {
		kept := c.Notes[:0]
		found := false
		for _, n := range c.Notes {
			if n.ID == noteID {
				found = true
				continue
			}
			kept = append(kept, n)
		}
		if !found {
			return errors.New("note not found")
		}
		c.Notes = kept
		return nil
	})
	if err != nil {
		return err
	}
	return refreshNotes(w, c)
}

// SessionNotes torna (i crea si cal) la col·lecció de notes de la sessió.
func SessionNotes(w Where) (*Collection, error) {
	all, err := List(w)
	if err != nil {
		return nil, err
	}
	for _, c := range all {
		if c.Scope == ScopeSession && strings.EqualFold(c.Name, SessionNotesName) {
			return &c, nil
		}
	}
	return Create(w, SessionNotesName, "Notes saved during this conversation.", ScopeSession)
}

func resolveSourcePath(c Collection, s Source, w Where) string {
	if filepath.IsAbs(s.Path) || strings.TrimSpace(w.Project) == "" {
		return s.Path
	}
	return filepath.Join(w.Project, filepath.FromSlash(s.Path))
}

// SourcePath és la ruta absoluta d'una font vista des de w.
func SourcePath(c Collection, s Source, w Where) string { return resolveSourcePath(c, s, w) }

func relInside(root, path string) (string, bool) {
	if strings.TrimSpace(root) == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	if rel == "." {
		return ".", true
	}
	return filepath.ToSlash(rel), true
}

func cleanGlobs(in []string) []string {
	var out []string
	for _, g := range in {
		for _, p := range strings.FieldsFunc(g, func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, filepath.ToSlash(p))
			}
		}
	}
	return out
}

func indexDir(w Where, c Collection) (string, error) {
	dir, err := ScopeDir(c.Scope, w)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, c.ID), nil
}
