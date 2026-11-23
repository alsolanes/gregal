// Package gdocs són els documents vius d'un projecte: blocs de markdown que
// l'agent i la persona editen alhora. Cada canvi d'un bloc en puja la versió
// i la del document i apunta qui l'ha fet. Un document és un JSON a
// <dades>/docs/<hash de l'arrel>/; l'arrel es resol com el backlog.
package gdocs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/backlog"
	"gregal/internal/profile"
)

// Qui ha fet un canvi.
const (
	AuthorAgent = "agent"
	AuthorUser  = "user"
)

// Block és un tros de markdown amb la versió del document en què va
// canviar per últim cop.
type Block struct {
	ID      string    `json:"id"`
	MD      string    `json:"md"`
	Version int       `json:"version"`
	Author  string    `json:"author"`
	Updated time.Time `json:"updated"`
}

// Doc és un document viu.
type Doc struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Kind    string    `json:"kind"`
	Blocks  []Block   `json:"blocks"`
	Version int       `json:"version"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Markdown és el document sencer, bloc darrere bloc.
func (d Doc) Markdown() string {
	parts := make([]string, len(d.Blocks))
	for i, b := range d.Blocks {
		parts[i] = b.MD
	}
	return strings.Join(parts, "\n\n")
}

// Errors que l'eina i l'API expliquen tal qual.
var (
	ErrNotFound      = errors.New("there is no document with that id in this project")
	ErrBlockNotFound = errors.New("there is no block with that id in the document (read the document for the ids)")
)

// Store són els documents d'un projecte.
type Store struct {
	Root string
	Dir  string
}

// Open troba els documents del projecte on és dir.
func Open(dir string) (*Store, error) {
	root := backlog.Root(dir)
	if root == "" {
		return nil, backlog.ErrNoDir
	}
	h := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	return &Store{Root: root, Dir: filepath.Join(profile.DataDir(), "docs", hex.EncodeToString(h[:6]))}, nil
}

// Un mutex per arrel: l'eina, el servidor web i els clients escriuen alhora.
var locks sync.Map

func (s *Store) lock() func() {
	m, _ := locks.LoadOrStore(strings.ToLower(s.Root), &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func newID(n int) string { return strings.ToLower(rand.Text()[:n]) }

func (s *Store) path(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\.`) {
		return "", ErrNotFound
	}
	return filepath.Join(s.Dir, id+".json"), nil
}

func (s *Store) read(id string) (Doc, error) {
	var d Doc
	p, err := s.path(id)
	if err != nil {
		return d, err
	}
	raw, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(raw, &d)
}

func (s *Store) write(d Doc) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(s.Dir, d.ID+".json")
	if err := os.WriteFile(p+".tmp", raw, 0o600); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

// Get torna un document.
func (s *Store) Get(id string) (Doc, error) {
	defer s.lock()()
	return s.read(id)
}

// List torna els documents del projecte, l'últim canviat primer.
func (s *Store) List() ([]Doc, error) {
	defer s.lock()()
	rutes, err := filepath.Glob(filepath.Join(s.Dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []Doc
	for _, r := range rutes {
		d, err := s.read(strings.TrimSuffix(filepath.Base(r), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}

// blocks fa blocs nous d'un markdown, a la versió v.
func blocks(md, author string, v int, at time.Time) []Block {
	out := []Block{}
	for _, part := range Split(md) {
		out = append(out, Block{ID: newID(8), MD: part, Version: v, Author: author, Updated: at})
	}
	return out
}

// Create fa un document nou amb md partit en blocs.
func (s *Store) Create(title, md, author string) (Doc, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Doc{}, errors.New("a document needs a title")
	}
	defer s.lock()()
	now := time.Now().UTC()
	d := Doc{ID: newID(12), Title: title, Kind: "doc", Blocks: blocks(md, author, 1, now), Version: 1, Created: now, Updated: now}
	return d, s.write(d)
}

// change aplica fn a un document i en puja la versió; fn rep la versió
// nova per als blocs que toca.
func (s *Store) change(id string, fn func(d *Doc, v int, at time.Time) error) (Doc, error) {
	defer s.lock()()
	d, err := s.read(id)
	if err != nil {
		return d, err
	}
	at := time.Now().UTC()
	if err := fn(&d, d.Version+1, at); err != nil {
		return Doc{}, err
	}
	d.Version++
	d.Updated = at
	return d, s.write(d)
}

func find(d *Doc, block string) int {
	for i := range d.Blocks {
		if d.Blocks[i].ID == block {
			return i
		}
	}
	return -1
}

// Insert posa els blocs de md després del bloc after ("" = al principi).
func (s *Store) Insert(id, after, md, author string) (Doc, error) {
	return s.change(id, func(d *Doc, v int, at time.Time) error {
		nous := blocks(md, author, v, at)
		if len(nous) == 0 {
			return errors.New("nothing to insert")
		}
		i := 0
		if after != "" {
			if i = find(d, after) + 1; i == 0 {
				return ErrBlockNotFound
			}
		}
		d.Blocks = append(d.Blocks[:i], append(nous, d.Blocks[i:]...)...)
		return nil
	})
}

// Replace canvia el markdown d'un bloc.
func (s *Store) Replace(id, block, md, author string) (Doc, error) {
	return s.change(id, func(d *Doc, v int, at time.Time) error {
		i := find(d, block)
		if i < 0 {
			return ErrBlockNotFound
		}
		d.Blocks[i] = Block{ID: block, MD: strings.TrimSpace(md), Version: v, Author: author, Updated: at}
		return nil
	})
}

// Delete treu un bloc.
func (s *Store) Delete(id, block string) (Doc, error) {
	return s.change(id, func(d *Doc, _ int, _ time.Time) error {
		i := find(d, block)
		if i < 0 {
			return ErrBlockNotFound
		}
		d.Blocks = append(d.Blocks[:i], d.Blocks[i+1:]...)
		return nil
	})
}

// Split parteix markdown en blocs: a cada línia en blanc i a cada títol
// (que queda sol en el seu bloc). Un bloc de codi no es parteix.
func Split(md string) []string {
	var out, cur []string
	tanca := func() {
		if t := strings.TrimSpace(strings.Join(cur, "\n")); t != "" {
			out = append(out, t)
		}
		cur = nil
	}
	codi := false
	for _, l := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "```"):
			if !codi {
				tanca()
			}
			cur = append(cur, l)
			if codi {
				tanca()
			}
			codi = !codi
		case codi:
			cur = append(cur, l)
		case t == "":
			tanca()
		case strings.HasPrefix(t, "#"):
			tanca()
			cur = append(cur, l)
			tanca()
		default:
			cur = append(cur, l)
		}
	}
	tanca()
	return out
}
