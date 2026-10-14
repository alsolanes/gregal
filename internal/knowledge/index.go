package knowledge

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Estats d'un document a l'índex.
const (
	DocIndexed     = "indexed"
	DocEmpty       = "empty"       // llegit, però sense text
	DocSkipped     = "skipped"     // massa gran o fora de límits
	DocUnsupported = "unsupported" // format de document que no es llegeix
	DocError       = "error"
)

const indexVersion = 1

// DocInfo és un document de l'índex (un fitxer o una nota).
type DocInfo struct {
	Key     string    `json:"key"`  // ruta absoluta o note:<id>
	Path    string    `json:"path"` // per ensenyar i citar
	Source  string    `json:"source"`
	Kind    string    `json:"kind"`
	Size    int64     `json:"size"`
	ModTime int64     `json:"mtime"`
	Status  string    `json:"status"`
	Error   string    `json:"error,omitempty"`
	Chunks  int       `json:"chunks"`
	Indexed time.Time `json:"indexed"`
}

// ChunkRec és un fragment indexat.
type ChunkRec struct {
	Doc  int32
	Loc  string
	Text string
	Vec  []float32
}

// Posting: el fragment C conté el terme TF vegades.
type Posting struct {
	C  int32
	TF uint16
}

// Index és l'índex d'una col·lecció (index.gob).
type Index struct {
	Version    int
	Built      time.Time
	Docs       []DocInfo
	Chunks     []ChunkRec
	Post       map[string][]Posting
	Lens       []int32
	TotalLen   int64
	EmbedModel string
	EmbedError string
	Skipped    int  // fitxers no indexables descartats sense soroll
	Truncated  bool // s'ha tocat el límit de fitxers o MB
	Limit      string
}

// Stats és el resum d'un índex per a la UI i el prompt.
type Stats struct {
	Docs        int       `json:"docs"`
	Chunks      int       `json:"chunks"`
	Errors      int       `json:"errors"`
	Unsupported int       `json:"unsupported"`
	Skipped     int       `json:"skipped"`
	Truncated   bool      `json:"truncated,omitempty"`
	Limit       string    `json:"limit,omitempty"`
	Indexed     time.Time `json:"indexed,omitempty"`
	EmbedModel  string    `json:"embed_model,omitempty"`
	EmbedError  string    `json:"embed_error,omitempty"`
}

// Stats resumeix l'índex.
func (ix *Index) Stats() Stats {
	if ix == nil {
		return Stats{}
	}
	s := Stats{Chunks: len(ix.Chunks), Skipped: ix.Skipped, Truncated: ix.Truncated, Limit: ix.Limit, Indexed: ix.Built, EmbedModel: ix.EmbedModel, EmbedError: ix.EmbedError}
	for _, d := range ix.Docs {
		switch d.Status {
		case DocIndexed, DocEmpty:
			s.Docs++
		case DocError:
			s.Errors++
		case DocUnsupported:
			s.Unsupported++
		case DocSkipped:
			s.Skipped++
		}
	}
	return s
}

// ---- memòria cau d'índexs carregats ----

type cachedIndex struct {
	mod  time.Time
	size int64
	ix   *Index
}

var (
	cacheMu sync.Mutex
	cache   = map[string]cachedIndex{}
)

func forgetIndex(path string) {
	cacheMu.Lock()
	delete(cache, path)
	cacheMu.Unlock()
}

func loadIndexFile(path string) (*Index, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	cacheMu.Lock()
	if c, ok := cache[path]; ok && c.mod.Equal(st.ModTime()) && c.size == st.Size() {
		cacheMu.Unlock()
		return c.ix, nil
	}
	cacheMu.Unlock()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ix Index
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&ix); err != nil {
		return nil, fmt.Errorf("knowledge index %s: %w", path, err)
	}
	if ix.Version != indexVersion {
		return nil, errors.New("knowledge index has an old format: reindex it")
	}
	cacheMu.Lock()
	cache[path] = cachedIndex{st.ModTime(), st.Size(), &ix}
	cacheMu.Unlock()
	return &ix, nil
}

func saveIndexFile(path string, ix *Index) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ix); err != nil {
		return err
	}
	if err := writeAtomic(path, buf.Bytes()); err != nil {
		return err
	}
	if st, err := os.Stat(path); err == nil {
		cacheMu.Lock()
		cache[path] = cachedIndex{st.ModTime(), st.Size(), ix}
		cacheMu.Unlock()
	}
	return nil
}

// LoadIndex torna l'índex d'una col·lecció (nil, nil si no se n'ha fet cap).
func LoadIndex(w Where, c Collection) (*Index, error) {
	dir, err := indexDir(w, c)
	if err != nil {
		return nil, err
	}
	ix, err := loadIndexFile(filepath.Join(dir, "index.gob"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return ix, err
}

// ---- progrés ----

// Progress és l'estat d'una indexació en curs o acabada.
type Progress struct {
	Running  bool      `json:"running"`
	Phase    string    `json:"phase,omitempty"`
	Done     int       `json:"done"`
	Total    int       `json:"total"`
	Current  string    `json:"current,omitempty"`
	Started  time.Time `json:"started,omitempty"`
	Finished time.Time `json:"finished,omitempty"`
	Error    string    `json:"error,omitempty"`
}

var (
	progMu   sync.Mutex
	progress = map[string]*Progress{}
	locks    = map[string]*sync.Mutex{}
)

// ProgressOf torna una còpia del progrés d'una col·lecció.
func ProgressOf(id string) Progress {
	progMu.Lock()
	defer progMu.Unlock()
	if p := progress[id]; p != nil {
		return *p
	}
	return Progress{}
}

func setProgress(id string, fn func(p *Progress)) {
	progMu.Lock()
	defer progMu.Unlock()
	p := progress[id]
	if p == nil {
		p = &Progress{}
		progress[id] = p
	}
	fn(p)
}

func lockFor(id string) *sync.Mutex {
	progMu.Lock()
	defer progMu.Unlock()
	l := locks[id]
	if l == nil {
		l = &sync.Mutex{}
		locks[id] = l
	}
	return l
}

// ErrBusy: ja s'està indexant.
var ErrBusy = errors.New("this collection is already being indexed")

// StartReindex indexa en segon pla. Torna ErrBusy si ja hi ha una
// indexació en curs per a la col·lecció.
func StartReindex(w Where, id string, force bool) error {
	l := lockFor(id)
	if !l.TryLock() {
		return ErrBusy
	}
	setProgress(id, func(p *Progress) { *p = Progress{Running: true, Phase: "scanning", Started: time.Now().UTC()} })
	go func() {
		defer l.Unlock()
		_, _ = reindexLocked(context.Background(), w, id, force)
	}()
	return nil
}

// Reindex indexa ara (síncron). Incremental: els fitxers amb la mateixa
// mida i data reaprofiten fragments i vectors; force ho rellegeix tot.
func Reindex(ctx context.Context, w Where, id string, force bool) (*Index, error) {
	l := lockFor(id)
	l.Lock()
	defer l.Unlock()
	setProgress(id, func(p *Progress) { *p = Progress{Running: true, Phase: "scanning", Started: time.Now().UTC()} })
	return reindexLocked(ctx, w, id, force)
}

// carpetes que no s'indexen mai.
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true, "vendor": true, ".gregal": true,
	"dist": true, "build": true, "target": true, "__pycache__": true, ".venv": true, "venv": true,
	".idea": true, ".vs": true, ".vscode": true, "bin": true, "obj": true, ".next": true, ".cache": true,
	"coverage": true, ".terraform": true, ".gradle": true,
}

type candidate struct {
	abs, display, source string
	size                 int64
	mod                  int64
	kind                 string
}

func reindexLocked(ctx context.Context, w Where, id string, force bool) (ix *Index, err error) {
	defer func() {
		setProgress(id, func(p *Progress) {
			p.Running = false
			p.Finished = time.Now().UTC()
			p.Current = ""
			if err != nil {
				p.Error = err.Error()
				p.Phase = "failed"
			} else {
				p.Phase = "done"
			}
		})
	}()
	c, err := Get(w, id)
	if err != nil {
		return nil, err
	}
	dir, err := indexDir(w, *c)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "index.gob")
	old, _ := loadIndexFile(path)
	set := Current()

	// 1. Fitxers candidats de totes les fonts.
	var cands []candidate
	skipped := 0
	truncated, limit := false, ""
	var total int64
	seen := map[string]bool{}
	for _, src := range c.Sources {
		root := resolveSourcePath(*c, src, w)
		st, err := os.Stat(root)
		if err != nil {
			continue
		}
		add := func(abs string, rel string, info fs.FileInfo) bool {
			if seen[abs] {
				return true
			}
			kind := KindOf(abs)
			if kind == "skip" {
				skipped++
				return true
			}
			if len(cands) >= set.MaxFiles {
				truncated, limit = true, fmt.Sprintf("max %d files", set.MaxFiles)
				return false
			}
			if total+info.Size() > int64(set.MaxTotalMB)<<20 {
				truncated, limit = true, fmt.Sprintf("max %d MB", set.MaxTotalMB)
				return false
			}
			seen[abs] = true
			if info.Size() <= int64(set.MaxFileMB)<<20 {
				total += info.Size()
			}
			cands = append(cands, candidate{abs: abs, display: displayPath(w, root, rel, st.IsDir()), source: src.ID, size: info.Size(), mod: info.ModTime().UnixNano(), kind: kind})
			return true
		}
		if !st.IsDir() {
			add(root, filepath.Base(root), st)
			continue
		}
		stop := false
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || stop {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			name := d.Name()
			if d.IsDir() {
				if p != root && (skipDirs[strings.ToLower(name)] || strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if len(src.Include) > 0 && !matchAny(src.Include, rel) {
				return nil
			}
			if matchAny(src.Exclude, rel) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if !add(p, rel, info) {
				stop = true
				return filepath.SkipAll
			}
			return nil
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].display < cands[j].display })
	setProgress(id, func(p *Progress) { p.Phase = "reading"; p.Total = len(cands) })

	// 2. Llegir (o reaprofitar) cada fitxer.
	reuse := map[string]prevDoc{}
	if old != nil && !force {
		byDoc := make([][]ChunkRec, len(old.Docs))
		for _, ch := range old.Chunks {
			if int(ch.Doc) < len(byDoc) {
				byDoc[ch.Doc] = append(byDoc[ch.Doc], ch)
			}
		}
		for i, d := range old.Docs {
			reuse[d.Key] = prevDoc{d, byDoc[i]}
		}
	}
	nix := &Index{Version: indexVersion, Built: time.Now().UTC(), Skipped: skipped, Truncated: truncated, Limit: limit}
	addDoc := func(d DocInfo, pieces []ChunkRec) {
		di := int32(len(nix.Docs))
		d.Chunks = len(pieces)
		nix.Docs = append(nix.Docs, d)
		for _, p := range pieces {
			p.Doc = di
			nix.Chunks = append(nix.Chunks, p)
		}
	}
	for i, cd := range cands {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		setProgress(id, func(p *Progress) { p.Done = i; p.Current = cd.display })
		if od, ok := reuse[cd.abs]; ok && od.info.Size == cd.size && od.info.ModTime == cd.mod && (od.info.Status == DocIndexed || od.info.Status == DocEmpty || od.info.Status == DocUnsupported) {
			od.info.Path, od.info.Source = cd.display, cd.source
			addDoc(od.info, od.chunks)
			continue
		}
		d := DocInfo{Key: cd.abs, Path: cd.display, Source: cd.source, Kind: cd.kind, Size: cd.size, ModTime: cd.mod, Indexed: time.Now().UTC()}
		if cd.size > int64(set.MaxFileMB)<<20 {
			d.Status, d.Error = DocSkipped, fmt.Sprintf("larger than %d MB", set.MaxFileMB)
			addDoc(d, nil)
			continue
		}
		kind, secs, err := Extract(cd.abs)
		if kind != "" {
			d.Kind = kind
		}
		switch {
		case errors.Is(err, ErrSkip):
			nix.Skipped++
			continue
		case errors.Is(err, ErrUnsupported) || errors.Is(err, ErrPDFEncrypted):
			d.Status, d.Error = DocUnsupported, err.Error()
			addDoc(d, nil)
			continue
		case err != nil:
			d.Status, d.Error = DocError, err.Error()
			addDoc(d, nil)
			continue
		}
		pieces := Chunk(secs, set.ChunkChars, set.Overlap)
		recs := make([]ChunkRec, 0, len(pieces))
		for _, p := range pieces {
			recs = append(recs, ChunkRec{Loc: p.Loc, Text: p.Text})
		}
		d.Status = DocIndexed
		if len(recs) == 0 {
			d.Status = DocEmpty
		}
		addDoc(d, recs)
	}
	// 3. Notes.
	addNotes(nix, *c, reuse, set)
	// 4. Índex invertit.
	buildPostings(nix)
	setProgress(id, func(p *Progress) { p.Done = len(cands); p.Phase = "embedding" })
	// 5. Vectors (opcional).
	embedMissing(ctx, nix, old, set)
	setProgress(id, func(p *Progress) { p.Phase = "saving" })
	if err := saveIndexFile(path, nix); err != nil {
		return nil, err
	}
	return nix, nil
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if MatchGlob(g, rel) {
			return true
		}
	}
	return false
}

// displayPath és la ruta per citar: relativa al projecte si hi és a dins;
// si no, el nom de la carpeta font més la ruta relativa.
func displayPath(w Where, root, rel string, rootIsDir bool) string {
	abs := root
	if rootIsDir {
		abs = filepath.Join(root, filepath.FromSlash(rel))
	}
	if r, ok := relInside(w.Project, abs); ok && r != "." {
		return r
	}
	if !rootIsDir {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(filepath.Join(filepath.Base(root), filepath.FromSlash(rel)))
}

// prevDoc és un document de l'índex anterior amb els seus fragments.
type prevDoc struct {
	info   DocInfo
	chunks []ChunkRec
}

func addNotes(nix *Index, c Collection, reuse map[string]prevDoc, set Settings) {
	for _, n := range c.Notes {
		key := "note:" + n.ID
		title := n.Title
		if title == "" {
			title = "note " + n.Created.Format("2006-01-02")
		}
		d := DocInfo{Key: key, Path: "note: " + title, Source: "notes", Kind: "note", Size: int64(len(n.Text)), ModTime: n.Created.UnixNano(), Status: DocIndexed, Indexed: time.Now().UTC()}
		if od, ok := reuse[key]; ok && od.info.ModTime == d.ModTime && od.info.Size == d.Size {
			di := int32(len(nix.Docs))
			od.info.Chunks = len(od.chunks)
			nix.Docs = append(nix.Docs, od.info)
			for _, ch := range od.chunks {
				ch.Doc = di
				nix.Chunks = append(nix.Chunks, ch)
			}
			continue
		}
		text := n.Text
		if n.Title != "" {
			text = n.Title + "\n" + text
		}
		pieces := Chunk([]Section{{Loc: title, Text: text}}, set.ChunkChars, set.Overlap)
		di := int32(len(nix.Docs))
		d.Chunks = len(pieces)
		nix.Docs = append(nix.Docs, d)
		for _, p := range pieces {
			nix.Chunks = append(nix.Chunks, ChunkRec{Doc: di, Loc: p.Loc, Text: p.Text})
		}
	}
}

// buildPostings construeix l'índex invertit. El nom del fitxer i la
// ubicació també compten (una vegada): «a l'spec» troba spec.pdf.
func buildPostings(ix *Index) {
	ix.Post = map[string][]Posting{}
	ix.Lens = make([]int32, len(ix.Chunks))
	ix.TotalLen = 0
	for ci, ch := range ix.Chunks {
		var extra string
		if int(ch.Doc) < len(ix.Docs) {
			extra = filepath.Base(ix.Docs[ch.Doc].Path) + " " + ch.Loc
		}
		tf := map[string]int{}
		n := 0
		for _, t := range Tokens(ch.Text) {
			tf[t]++
			n++
		}
		for _, t := range Tokens(extra) {
			if tf[t] == 0 {
				tf[t] = 1
				n++
			}
		}
		ix.Lens[ci] = int32(n)
		ix.TotalLen += int64(n)
		for t, f := range tf {
			if f > 65535 {
				f = 65535
			}
			ix.Post[t] = append(ix.Post[t], Posting{C: int32(ci), TF: uint16(f)})
		}
	}
}

// embedMissing calcula els vectors que falten (reaprofita els de l'índex
// anterior si el model és el mateix). Un error no fa fallar la indexació:
// la cerca continua amb BM25 i l'error queda a les estadístiques.
func embedMissing(ctx context.Context, nix, old *Index, set Settings) {
	if !set.EmbeddingsOn() {
		for i := range nix.Chunks {
			nix.Chunks[i].Vec = nil
		}
		return
	}
	model := set.Embed.Model
	if old != nil && old.EmbedModel != model {
		for i := range nix.Chunks {
			nix.Chunks[i].Vec = nil
		}
	}
	var idx []int
	var texts []string
	for i, ch := range nix.Chunks {
		if len(ch.Vec) == 0 {
			idx = append(idx, i)
			texts = append(texts, ch.Loc+"\n"+ch.Text)
		}
	}
	nix.EmbedModel = model
	if len(texts) == 0 {
		return
	}
	vecs, err := embedTexts(ctx, set.Embed, texts)
	for k, v := range vecs {
		nix.Chunks[idx[k]].Vec = v
	}
	if err != nil {
		nix.EmbedError = err.Error()
	}
}

// refreshNotes torna a indexar només les notes (sense recórrer carpetes).
func refreshNotes(w Where, c *Collection) error {
	l := lockFor(c.ID)
	l.Lock()
	defer l.Unlock()
	dir, err := indexDir(w, *c)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "index.gob")
	old, _ := loadIndexFile(path)
	set := Current()
	nix := &Index{Version: indexVersion, Built: time.Now().UTC()}
	reuse := map[string]prevDoc{}
	if old != nil {
		nix.Skipped, nix.Truncated, nix.Limit, nix.EmbedModel, nix.EmbedError = old.Skipped, old.Truncated, old.Limit, old.EmbedModel, old.EmbedError
		nix.Built = old.Built
		byDoc := make([][]ChunkRec, len(old.Docs))
		for _, ch := range old.Chunks {
			if int(ch.Doc) < len(byDoc) {
				byDoc[ch.Doc] = append(byDoc[ch.Doc], ch)
			}
		}
		for i, d := range old.Docs {
			if d.Kind == "note" {
				reuse[d.Key] = prevDoc{d, byDoc[i]}
				continue
			}
			di := int32(len(nix.Docs))
			nix.Docs = append(nix.Docs, d)
			for _, ch := range byDoc[i] {
				ch.Doc = di
				nix.Chunks = append(nix.Chunks, ch)
			}
		}
	}
	addNotes(nix, *c, reuse, set)
	buildPostings(nix)
	embedMissing(context.Background(), nix, old, set)
	return saveIndexFile(path, nix)
}
