// Package gdocs són els documents vius d'un projecte: blocs de markdown que
// l'agent i la persona editen alhora. Cada canvi d'un bloc en puja la versió
// i la del document i apunta qui l'ha fet. Un document és un JSON a
// <dades>/docs/<hash de l'arrel>/; l'arrel es resol com el backlog.
package gdocs

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"

	"gregal/internal/backlog"
	"gregal/internal/ooxml"
	"gregal/internal/profile"
	"gregal/internal/xlsxdoc"
)

// Qui ha fet un canvi.
const (
	AuthorAgent = "agent"
	AuthorUser  = "user"
)

// Els tipus de document: de text (blocs de markdown), una presentació
// (un bloc per diapositiva) o un full de càlcul (un bloc per full).
const (
	KindDoc   = "doc"
	KindDeck  = "deck"
	KindSheet = "sheet"
)

// Block és un tros de markdown, una diapositiva en una presentació o un
// full en un full de càlcul, amb la versió del document en què va
// canviar per últim cop. Since és la versió en què es va crear: distingeix
// un bloc editat d'un d'inserit des de l'última versió que algú ha vist.
type Block struct {
	ID    string           `json:"id"`
	MD    string           `json:"md"`
	Slide *ooxml.SlideSpec `json:"slide,omitempty"`
	// Un full: el nom, l'entrada de cada cel·la (A1 → número, text o
	// fórmula amb =) i el valor que en surt, calculat pel servidor.
	Name    string            `json:"name,omitempty"`
	Cells   map[string]string `json:"cells,omitempty"`
	Values  map[string]string `json:"values,omitempty"`
	Version int               `json:"version"`
	Since   int               `json:"since,omitempty"`
	Author  string            `json:"author"`
	Updated time.Time         `json:"updated"`
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
	// ErrInvalid marca una edició que no encaixa amb el document (un altre
	// tipus de bloc, una diapositiva mal feta): les rutes en fan un 400.
	ErrInvalid = errors.New("invalid edit")
)

// invalid porta el missatge d'un error d'aquests i en fa un ErrInvalid.
type invalid struct{ error }

func (e invalid) Unwrap() []error { return []error{e.error, ErrInvalid} }

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
	return OpenAt(backlog.Root(dir))
}

// OpenAt obre els documents de root tal com és, sense pujar a l'arrel del
// repo: les carpetes personals no han de compartir els documents d'un repo
// que les contingui.
func OpenAt(root string) (*Store, error) {
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

// AddImage desa una imatge a la carpeta del document id i en torna la ruta
// que va al markdown, images/<nom>. Només PNG, JPEG o GIF: les que el Word i
// el PowerPoint saben posar.
func (s *Store) AddImage(id string, data []byte) (string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", invalid{errors.New("image: only PNG, JPEG or GIF images")}
	}
	if _, err := s.Get(id); err != nil {
		return "", err
	}
	dir := filepath.Join(s.Dir, id, "images")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := newID(16) + "." + format
	return "images/" + name, os.WriteFile(filepath.Join(dir, name), data, 0o600)
}

// ImagePath és on és al disc la imatge src (images/<nom>) del document id.
// Una imatge d'un document només pot ser de la seva carpeta: una ruta
// absoluta, amb .. o fora d'images és ErrInvalid.
func (s *Store) ImagePath(id, src string) (string, error) {
	name, ok := strings.CutPrefix(src, "images/")
	if _, err := s.path(id); err != nil || !ok || strings.ContainsAny(name, `/\:`) || !filepath.IsLocal(name) {
		return "", invalid{fmt.Errorf("image %q: a document's images are images/<name> in its own folder", src)}
	}
	return filepath.Join(s.Dir, id, "images", name), nil
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
	return s.create(title, KindDoc, func(at time.Time) []Block { return blocks(md, author, 1, at) })
}

// CreateDeck fa una presentació buida; les diapositives hi entren amb
// InsertSlide.
func (s *Store) CreateDeck(title string) (Doc, error) {
	return s.create(title, KindDeck, func(time.Time) []Block { return []Block{} })
}

// CreateSheet fa un full de càlcul amb un full buit que es diu name
// (Sheet1 si és buit).
func (s *Store) CreateSheet(title, name, author string) (Doc, error) {
	if strings.TrimSpace(name) == "" {
		name = "Sheet1"
	}
	return s.create(title, KindSheet, func(at time.Time) []Block {
		return []Block{{ID: newID(8), Name: strings.TrimSpace(name), Version: 1, Since: 1, Author: author, Updated: at}}
	})
}

func (s *Store) create(title, kind string, mk func(at time.Time) []Block) (Doc, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Doc{}, errors.New("a document needs a title")
	}
	defer s.lock()()
	now := time.Now().UTC()
	d := Doc{ID: newID(12), Title: title, Kind: kind, Blocks: mk(now), Version: 1, Created: now, Updated: now}
	if err := calc(&d); err != nil {
		return Doc{}, err
	}
	return d, s.write(d)
}

// sameKind fa que una edició de text no entri en una presentació ni al revés.
func sameKind(d *Doc, kind string) error {
	if d.Kind != kind {
		return invalid{fmt.Errorf("document %s is a %s: this edit is for a %s (decks take slides, sheets take cells, docs take md)", d.ID, d.Kind, kind)}
	}
	return nil
}

// checkSlide accepta només els layouts que el constructor de presentacions
// coneix: una presentació viva no té plantilla amb layouts propis.
func checkSlide(sl ooxml.SlideSpec) error {
	if !ooxml.SlideLayout(sl.Layout) {
		return invalid{fmt.Errorf("slide layout %q: use title, section, content, two_columns, comparison, big_number, image, title_only, blank or closing", sl.Layout)}
	}
	if sl.Chart != nil {
		if err := ooxml.CheckChart(*sl.Chart); err != nil {
			return invalid{err}
		}
	}
	return nil
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
	if err := calc(&d); err != nil {
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
	values := map[string]map[string]string{}
	for _, b := range before {
		old[b.ID] = true
		values[b.ID] = b.Values
	}
	prev := ""
	for _, b := range d.Blocks {
		now[b.ID] = true
		switch {
		case !old[b.ID]:
			out = append(out, Change{Op: "insert", Block: b, After: prev, Version: d.Version, Author: b.Author})
		// Un full que no s'ha tocat també canvia si les seves fórmules
		// miren cel·les d'un altre que sí.
		case b.Version == d.Version, !maps.Equal(values[b.ID], b.Values):
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
	return s.insert(id, after, KindDoc, func(v int, at time.Time) []Block { return blocks(md, author, v, at) })
}

// InsertSlide posa una diapositiva després del bloc after ("" = al principi).
func (s *Store) InsertSlide(id, after string, sl ooxml.SlideSpec, author string) (Doc, error) {
	if err := checkSlide(sl); err != nil {
		return Doc{}, err
	}
	return s.insert(id, after, KindDeck, func(v int, at time.Time) []Block {
		return []Block{{ID: newID(8), Slide: &sl, Version: v, Since: v, Author: author, Updated: at}}
	})
}

// InsertSheet posa un full buit que es diu name després del bloc after.
func (s *Store) InsertSheet(id, after, name, author string) (Doc, error) {
	return s.insert(id, after, KindSheet, func(v int, at time.Time) []Block {
		return []Block{{ID: newID(8), Name: strings.TrimSpace(name), Version: v, Since: v, Author: author, Updated: at}}
	})
}

func (s *Store) insert(id, after, kind string, mk func(v int, at time.Time) []Block) (Doc, error) {
	return s.change(id, func(d *Doc, v int, at time.Time) error {
		if err := sameKind(d, kind); err != nil {
			return err
		}
		nous := mk(v, at)
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
	return s.replace(id, block, KindDoc, author, base, Block{MD: strings.TrimSpace(md)})
}

// ReplaceSlide canvia una diapositiva sencera; base com a Replace.
func (s *Store) ReplaceSlide(id, block string, sl ooxml.SlideSpec, author string, base int) (Doc, error) {
	if err := checkSlide(sl); err != nil {
		return Doc{}, err
	}
	return s.replace(id, block, KindDeck, author, base, Block{Slide: &sl})
}

// replace posa el contingut de nou al bloc block.
func (s *Store) replace(id, block, kind, author string, base int, nou Block) (Doc, error) {
	return s.change(id, func(d *Doc, v int, at time.Time) error {
		if err := sameKind(d, kind); err != nil {
			return err
		}
		i := find(d, block)
		if i < 0 {
			return ErrBlockNotFound
		}
		if err := conflict(d.Blocks[i], author, base); err != nil {
			return err
		}
		nou.ID, nou.Version, nou.Since, nou.Author, nou.Updated = block, v, d.Blocks[i].Since, author, at
		d.Blocks[i] = nou
		return nil
	})
}

// SetCells escriu cel·les d'un full: cel·la → entrada; una entrada buida
// buida la cel·la. base com a Replace.
func (s *Store) SetCells(id, block string, cells map[string]string, author string, base int) (Doc, error) {
	if len(cells) == 0 {
		return Doc{}, errors.New("no cells to set")
	}
	return s.change(id, func(d *Doc, v int, at time.Time) error {
		if err := sameKind(d, KindSheet); err != nil {
			return err
		}
		i := find(d, block)
		if i < 0 {
			return ErrBlockNotFound
		}
		b := d.Blocks[i]
		if err := conflict(b, author, base); err != nil {
			return err
		}
		// Un mapa nou: el d'abans el comparteix la còpia que fa change.
		nou := map[string]string{}
		maps.Copy(nou, b.Cells)
		for c, in := range cells {
			// $B$2, b2 i B02 són B2: una sola clau per cel·la.
			col, row, err := excelize.CellNameToCoordinates(strings.TrimSpace(c))
			if err != nil {
				return invalid{fmt.Errorf("cell %q: name cells like B2", c)}
			}
			if c, err = excelize.CoordinatesToCellName(col, row); err != nil {
				return err
			}
			if err := checkFormula(in); err != nil {
				return invalid{fmt.Errorf("cell %s: %w", c, err)}
			}
			if strings.TrimSpace(in) == "" {
				delete(nou, c)
			} else {
				nou[c] = in
			}
		}
		if len(nou) > maxSheetCells {
			return invalid{fmt.Errorf("a sheet holds at most %d cells", maxSheetCells)}
		}
		b.Cells, b.Version, b.Author, b.Updated = nou, v, author, at
		d.Blocks[i] = b
		return nil
	})
}

// Límits d'un full: les fórmules venen del model o de qualsevol persona, i
// es calculen amb el document bloquejat.
const (
	maxRangeCells = 100_000         // cel·les d'un rang A1:B2 d'una fórmula
	maxSheetCells = 50_000          // cel·les amb contingut d'un full
	maxValue      = 32_767          // caràcters d'un valor, el límit d'Excel
	calcDeadline  = 3 * time.Second // el recàlcul sencer; després, #CALC!
)

// Workbook fa el llibre d'Excel d'un full de càlcul, amb les fórmules.
// No porta els valors desats: Excel el recalcula en obrir-lo.
func Workbook(d Doc) (*excelize.File, error) {
	f := excelize.NewFile()
	noms := map[string]bool{}
	for i, b := range d.Blocks {
		if noms[strings.ToLower(b.Name)] {
			return nil, errors.Join(fmt.Errorf("there is already a sheet named %q", b.Name), f.Close())
		}
		noms[strings.ToLower(b.Name)] = true
		var err error
		if i == 0 {
			err = f.SetSheetName("Sheet1", b.Name)
		} else {
			_, err = f.NewSheet(b.Name)
		}
		if err != nil {
			return nil, errors.Join(fmt.Errorf("sheet %q: %w", b.Name, err), f.Close())
		}
		for c, in := range b.Cells {
			if err := xlsxdoc.SetString(f, b.Name, c, in); err != nil {
				return nil, errors.Join(fmt.Errorf("%s!%s: %w", b.Name, c, err), f.Close())
			}
		}
	}
	yes := true
	if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &yes}); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return f, nil
}

// calc calcula amb excelize el valor de cada cel·la dels fulls, perquè
// tothom vegi el mateix resultat; refusa un nom de full que Excel no admet.
func calc(d *Doc) error {
	if d.Kind != KindSheet {
		return nil
	}
	// El llibre només falla per un nom de full o de cel·la que Excel no admet.
	f, err := Workbook(*d)
	if err != nil {
		return invalid{err}
	}
	type valor struct {
		i              int
		sheet, cell, v string
	}
	var cells []valor
	for i := range d.Blocks {
		b := &d.Blocks[i]
		b.Values = map[string]string{}
		for c := range b.Cells {
			b.Values[c] = "#CALC!"
			cells = append(cells, valor{i: i, sheet: b.Name, cell: c})
		}
	}
	// CalcCellValue no es pot aturar a mitges: si passa del termini, la
	// cel·la que calcula acaba de fons (amb prou lloc al canal per no
	// quedar penjada, i sense tocar d), no se'n comencen més, i les que no
	// han arribat queden en #CALC!.
	out := make(chan valor, len(cells))
	prou := make(chan struct{})
	defer close(prou)
	id := d.ID
	go func() {
		defer close(out)
		defer func() {
			if err := f.Close(); err != nil {
				log.Printf("gdocs: tancar el llibre de %s: %v", id, err)
			}
		}()
		for _, cl := range cells {
			select {
			case <-prou:
				return
			default:
			}
			v, err := f.CalcCellValue(cl.sheet, cl.cell)
			if err != nil {
				v = err.Error()
				// excelize diu «not support X function» d'una funció que
				// no coneix; Excel hi posa #NAME?, com a un full que no hi és.
				if !strings.HasPrefix(v, "#") {
					v = "#NAME?"
				}
			}
			if len(v) > maxValue {
				v = strings.ToValidUTF8(v[:maxValue], "")
			}
			cl.v = v
			out <- cl
		}
	}()
	termini := time.After(calcDeadline)
	for {
		select {
		case r, ok := <-out:
			if !ok {
				return nil
			}
			d.Blocks[r.i].Values[r.cell] = r.v
		case <-termini:
			return nil
		}
	}
}

// rangRe troba els rangs de cel·la a cel·la d'una fórmula (A1:B20, $a$1:$b$20).
var rangRe = regexp.MustCompile(`(?i)\$?([A-Z]{1,3})\$?(\d+):\$?([A-Z]{1,3})\$?(\d+)`)

// checkFormula refusa una fórmula amb un rang massa gran per calcular-lo:
// excelize només retalla a l'àrea usada els rangs que acaben a l'última
// fila o columna (A:A, 1:1), no un A1:Z1000000.
func checkFormula(in string) error {
	if !strings.HasPrefix(strings.TrimSpace(in), "=") {
		return nil
	}
	for _, m := range rangRe.FindAllStringSubmatch(in, -1) {
		c0, r0, err0 := excelize.CellNameToCoordinates(m[1] + m[2])
		c1, r1, err1 := excelize.CellNameToCoordinates(m[3] + m[4])
		if err0 != nil || err1 != nil {
			continue // no és un rang de cel·les, p. ex. dins d'un text
		}
		if (max(c0, c1)-min(c0, c1)+1)*(max(r0, r1)-min(r0, r1)+1) > maxRangeCells {
			return fmt.Errorf("range %s:%s covers more than %d cells", m[1]+m[2], m[3]+m[4], maxRangeCells)
		}
	}
	return nil
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
// alineacio és la línia {align=…} d'un bloc (format ampliat, 078).
var alineacio = regexp.MustCompile(`^\{align=(left|center|right|justify)\}$`)

func Split(md string) []string {
	var out, cur []string
	tanca := func() {
		if t := strings.TrimSpace(strings.Join(cur, "\n")); t != "" {
			out = append(out, t)
		}
		cur = nil
	}
	codi := false
	titol := false // el bloc obert és un títol: només hi pot anar la seva alineació
	for _, l := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(l)
		if titol {
			titol = false
			if alineacio.MatchString(t) {
				cur = append(cur, l)
				tanca()
				continue
			}
			tanca()
		}
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
			titol = true
		default:
			cur = append(cur, l)
		}
	}
	tanca()
	return out
}
