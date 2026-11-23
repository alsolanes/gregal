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
	"fmt"
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
// canviar per últim cop. Since és la versió en què es va crear: distingeix
// un bloc editat d'un d'inserit des de l'última versió que algú ha vist.
type Block struct {
	ID      string    `json:"id"`
	MD      string    `json:"md"`
	Version int       `json:"version"`
	Since   int       `json:"since,omitempty"`
	Author  string    `json:"author"`
	Updated time.Time `json:"updated"`
}

// Doc és un document viu.
type Doc struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Kind   string  `json:"kind"`
	Blocks []Block `json:"blocks"`
	// Deleted són les làpides dels blocs esborrats: la versió i l'autor
	// de l'esborrat, perquè l'agent sàpiga que la persona n'ha tret un.
	Deleted []Block   `json:"deleted,omitempty"`
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

// ConflictError és una edició d'un bloc que ha canviat després de la
// versió base de qui l'edita; Block és el bloc d'ara.
type ConflictError struct{ Block Block }

func (e *ConflictError) Error() string {
	return fmt.Sprintf("block %s changed after you last read it (now v%d by %s): read the document again before editing this block",
		e.Block.ID, e.Block.Version, e.Block.Author)
}

// conflict fa que ningú no trepitgi un canvi de la persona sense haver-lo
// llegit: una edició de la persona xoca amb qualsevol canvi del bloc
// posterior a base, una de l'agent només amb els de la persona.
func conflict(b Block, author string, base int) error {
	if b.Version > base && (author == AuthorUser || b.Author == AuthorUser) {
		return &ConflictError{Block: b}
	}
	return nil
}

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
		out = append(out, Block{ID: newID(8), MD: part, Version: v, Since: v, Author: author, Updated: at})
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
	// Còpia: Insert i Replace escriuen a l'array de blocs que d comparteix.
	before := append([]Block(nil), d.Blocks...)
	at := time.Now().UTC()
	if err := fn(&d, d.Version+1, at); err != nil {
		return Doc{}, err
	}
	d.Version++
	d.Updated = at
	if err := s.write(d); err != nil {
		return d, err
	}
	// Dins el bloqueig, perquè els que miren rebin els canvis en ordre de versió.
	s.notify(id, changes(before, d))
	return d, nil
}

// Change és el canvi d'un bloc tal com el reben els que miren el document.
// Un bloc esborrat només porta l'id; l'autor surt de la seva làpida.
type Change struct {
	Op      string `json:"op"` // insert, replace o delete
	Block   Block  `json:"block"`
	After   string `json:"after,omitempty"`
	Version int    `json:"version"`
	Author  string `json:"author"`
}

// deletedBy és l'autor de l'última làpida del bloc.
func deletedBy(d Doc, block string) string {
	for i := len(d.Deleted) - 1; i >= 0; i-- {
		if d.Deleted[i].ID == block {
			return d.Deleted[i].Author
		}
	}
	return ""
}

// changes compara els blocs d'abans del canvi amb el document d'ara.
func changes(before []Block, d Doc) []Change {
	var out []Change
	old, now := map[string]bool{}, map[string]bool{}
	for _, b := range before {
		old[b.ID] = true
	}
	prev := ""
	for _, b := range d.Blocks {
		now[b.ID] = true
		switch {
		case !old[b.ID]:
			out = append(out, Change{Op: "insert", Block: b, After: prev, Version: d.Version, Author: b.Author})
		case b.Version == d.Version:
			out = append(out, Change{Op: "replace", Block: b, Version: d.Version, Author: b.Author})
		}
		prev = b.ID
	}
	for _, b := range before {
		if !now[b.ID] {
			out = append(out, Change{Op: "delete", Block: Block{ID: b.ID}, Version: d.Version, Author: deletedBy(d, b.ID)})
		}
	}
	return out
}

// Els que miren cada document, per carpeta del projecte i id.
var (
	watchMu  sync.Mutex
	watchers = map[string]map[chan Change]bool{}
)

// Watch rep els canvis del document id d'ara endavant, fins que es crida
// stop. Si qui mira no buida el canal a temps, se li tanca: ha de tornar a
// llegir el document sencer.
func (s *Store) Watch(id string) (<-chan Change, func()) {
	k := filepath.Join(s.Dir, id)
	ch := make(chan Change, 64)
	watchMu.Lock()
	if watchers[k] == nil {
		watchers[k] = map[chan Change]bool{}
	}
	watchers[k][ch] = true
	watchMu.Unlock()
	return ch, func() {
		watchMu.Lock()
		defer watchMu.Unlock()
		if watchers[k][ch] {
			delete(watchers[k], ch)
			close(ch)
		}
		if len(watchers[k]) == 0 {
			delete(watchers, k)
		}
	}
}

func (s *Store) notify(id string, cs []Change) {
	k := filepath.Join(s.Dir, id)
	watchMu.Lock()
	defer watchMu.Unlock()
	for ch := range watchers[k] {
		if !send(ch, cs) {
			delete(watchers[k], ch)
			close(ch)
		}
	}
}

// send no espera mai: un client lent no pot aturar els que escriuen.
func send(ch chan Change, cs []Change) bool {
	for _, c := range cs {
		select {
		case ch <- c:
		default:
			return false
		}
	}
	return true
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

// Replace canvia el markdown d'un bloc; base és la versió del document
// que l'autor ha llegit (vegeu conflict).
func (s *Store) Replace(id, block, md, author string, base int) (Doc, error) {
	return s.change(id, func(d *Doc, v int, at time.Time) error {
		i := find(d, block)
		if i < 0 {
			return ErrBlockNotFound
		}
		if err := conflict(d.Blocks[i], author, base); err != nil {
			return err
		}
		d.Blocks[i] = Block{ID: block, MD: strings.TrimSpace(md), Version: v, Since: d.Blocks[i].Since, Author: author, Updated: at}
		return nil
	})
}

// Delete treu un bloc i en deixa la làpida; base com a Replace.
func (s *Store) Delete(id, block, author string, base int) (Doc, error) {
	return s.change(id, func(d *Doc, v int, at time.Time) error {
		i := find(d, block)
		if i < 0 {
			return ErrBlockNotFound
		}
		if err := conflict(d.Blocks[i], author, base); err != nil {
			return err
		}
		l := d.Blocks[i]
		l.Version, l.Author, l.Updated = v, author, at
		d.Deleted = append(d.Deleted, l)
		d.Blocks = append(d.Blocks[:i], d.Blocks[i+1:]...)
		return nil
	})
}

// seenPath és el registre d'una sessió: quina versió de cada document ha
// vist el seu agent. Va en un subdirectori perquè List no el prengui per
// un document.
func (s *Store) seenPath(session string) string {
	h := sha256.Sum256([]byte(session))
	return filepath.Join(s.Dir, "seen", hex.EncodeToString(h[:6])+".json")
}

func (s *Store) seen(session string) (map[string]int, error) {
	m := map[string]int{}
	raw, err := os.ReadFile(s.seenPath(session))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	return m, json.Unmarshal(raw, &m)
}

// Seen torna, per id de document, l'última versió que l'agent de la sessió
// ha llegit o escrit. Un document que no hi és, la sessió no l'ha tocat.
func (s *Store) Seen(session string) (map[string]int, error) {
	defer s.lock()()
	return s.seen(session)
}

// MarkSeen apunta que l'agent de la sessió ha vist el document a la versió
// v. Mai no tira enrere: una lectura vella que arriba tard no torna a fer
// veure canvis ja vistos.
func (s *Store) MarkSeen(session, id string, v int) error {
	defer s.lock()()
	m, err := s.seen(session)
	if err != nil {
		return err
	}
	if m[id] >= v {
		return nil
	}
	m[id] = v
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	p := s.seenPath(session)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(p+".tmp", raw, 0o600); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
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
