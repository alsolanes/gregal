// Package repomap és el mapa de símbols del projecte: les declaracions de
// nivell superior de cada fitxer de codi (funcions, mètodes, tipus,
// classes...) amb la línia i una signatura curta.
//
// Serveix per a dues coses: el mapa del projecte que va enganxat a la tasca
// (internal/agent, mapa_projecte.go) hi afegeix els símbols més rellevants
// per a la tasca, i l'eina `symbols` hi busca definicions sense haver de fer
// grep. Go es llegeix amb go/parser; la resta de llenguatges (JS/TS,
// Python, Java, C#, Rust) amb expressions regulars per línia: res de cgo ni
// tree-sitter, i prou bo per orientar-se.
//
// L'índex viu en memòria per directori absolut, i cada fitxer es torna a
// llegir només si li han canviat la data o la mida. Només depèn de la
// biblioteca estàndard: agent i tools hi poden arribar sense cicles.
package repomap

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Symbol és una declaració.
type Symbol struct {
	Name     string // nom sol (sense receptor)
	Kind     string // func, method, type, struct, interface, class, enum, trait, impl, const, var, record
	Recv     string // receptor o contenidor (mètodes de Go, Python i Rust)
	Line     int
	Exported bool
	Sig      string // signatura curta d'una línia
}

// Display és el nom amb el receptor: «Policy.Decide».
func (s Symbol) Display() string {
	if s.Recv != "" {
		return s.Recv + "." + s.Name
	}
	return s.Name
}

// File són els símbols d'un fitxer. Un cop a l'índex no es modifica: si el
// fitxer canvia se'n fa un de nou, i així les instantànies es poden llegir
// sense candau.
type File struct {
	Path    string // relatiu al projecte, amb barres normals
	Lang    string
	Size    int64
	ModTime time.Time
	Symbols []Symbol
}

// Snapshot és l'estat de l'índex després d'una actualització.
type Snapshot struct {
	Dir     string
	Files   []*File // ordenats per camí; només els que tenen símbols
	Partial bool    // s'ha tallat per temps o per nombre de fitxers
	Elapsed time.Duration
	Parsed  int // fitxers llegits de nou en aquesta actualització
}

// Options governen una actualització.
type Options struct {
	Budget      time.Duration // temps màxim (0 = DefaultBudget)
	MaxFiles    int           // fitxers de codi com a màxim (0 = DefaultMaxFiles)
	MaxFileSize int64         // bytes per fitxer (0 = DefaultMaxFileSize)
	Priority    []string      // camins a processar primer (els modificats)
}

const (
	DefaultBudget      = 300 * time.Millisecond
	DefaultMaxFiles    = 5000
	DefaultMaxFileSize = 256 << 10
)

type entrada struct {
	mod  time.Time
	size int64
	f    *File // nil si el fitxer no té símbols
}

type index struct {
	mu       sync.Mutex
	entrades map[string]*entrada
}

var (
	indexMu sync.Mutex
	indexos = map[string]*index{}
)

func indexDe(dir string) *index {
	indexMu.Lock()
	defer indexMu.Unlock()
	ix := indexos[dir]
	if ix == nil {
		ix = &index{entrades: map[string]*entrada{}}
		indexos[dir] = ix
	}
	return ix
}

// Reset buida la memòria cau (per als tests i el benchmark en fred).
func Reset() {
	indexMu.Lock()
	indexos = map[string]*index{}
	indexMu.Unlock()
}

// LangOf diu el llenguatge pel nom del fitxer, o "" si no és codi que
// sabem llegir.
func LangOf(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "js"
	case ".ts", ".tsx", ".mts", ".cts":
		return "ts"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".rs":
		return "rust"
	}
	return ""
}

// Build actualitza l'índex del directori i en torna una instantània.
// fitxers és la llista del projecte (relativa, com la de ListFiles); si és
// nil es calcula. Amb el temps exhaurit torna el que té (Partial): la
// propera crida continua on s'havia quedat, perquè el que s'ha llegit ja
// queda a la memòria cau.
func Build(dir string, fitxers []string, opt Options) *Snapshot {
	t0 := time.Now()
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if opt.Budget <= 0 {
		opt.Budget = DefaultBudget
	}
	if opt.MaxFiles <= 0 {
		opt.MaxFiles = DefaultMaxFiles
	}
	if opt.MaxFileSize <= 0 {
		opt.MaxFileSize = DefaultMaxFileSize
	}
	if fitxers == nil {
		fitxers, _ = ListFiles(dir)
	}
	snap := &Snapshot{Dir: dir}
	codi := make([]string, 0, len(fitxers))
	for _, f := range fitxers {
		if LangOf(f) != "" && !esGenerat(f) && !saltaCami(f) {
			codi = append(codi, f)
		}
	}
	if len(codi) == 0 {
		snap.Elapsed = time.Since(t0)
		return snap
	}
	// Primer els prioritaris (els que s'estan tocant), després la resta en
	// ordre: així un índex a mitges ja té el que més importa.
	if len(opt.Priority) > 0 {
		pri := map[string]bool{}
		for _, p := range opt.Priority {
			pri[p] = true
		}
		sort.SliceStable(codi, func(i, j int) bool { return pri[codi[i]] && !pri[codi[j]] })
	}
	if len(codi) > opt.MaxFiles {
		codi = codi[:opt.MaxFiles]
		snap.Partial = true
	}
	ix := indexDe(dir)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	limit := t0.Add(opt.Budget)
	vistos := make(map[string]bool, len(codi))
	for i, rel := range codi {
		// El rellotge es mira cada pocs fitxers: time.Now no és gratis.
		if i%16 == 0 && time.Now().After(limit) {
			snap.Partial = true
			break
		}
		vistos[rel] = true
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() {
			delete(ix.entrades, rel)
			continue
		}
		e := ix.entrades[rel]
		if e != nil && e.size == info.Size() && e.mod.Equal(info.ModTime()) {
			if e.f != nil {
				snap.Files = append(snap.Files, e.f)
			}
			continue
		}
		e = &entrada{mod: info.ModTime(), size: info.Size()}
		if info.Size() > 0 && info.Size() <= opt.MaxFileSize {
			if src, err := os.ReadFile(abs); err == nil {
				if syms := Extract(rel, src); len(syms) > 0 {
					e.f = &File{Path: rel, Lang: LangOf(rel), Size: info.Size(), ModTime: info.ModTime(), Symbols: syms}
				}
			}
			snap.Parsed++
		}
		ix.entrades[rel] = e
		if e.f != nil {
			snap.Files = append(snap.Files, e.f)
		}
	}
	// Els que ja no hi són surten de l'índex (només amb una passada
	// sencera: a mitges no sabem si hi són).
	if !snap.Partial {
		for rel := range ix.entrades {
			if !vistos[rel] {
				delete(ix.entrades, rel)
			}
		}
	}
	sort.Slice(snap.Files, func(i, j int) bool { return snap.Files[i].Path < snap.Files[j].Path })
	snap.Elapsed = time.Since(t0)
	return snap
}

// Extract treu els símbols d'un fitxer pel seu llenguatge.
func Extract(rel string, src []byte) []Symbol {
	switch LangOf(rel) {
	case "go":
		return extractGo(rel, src)
	case "":
		return nil
	}
	if minificat(src) {
		return nil
	}
	return extractRegex(LangOf(rel), src)
}

// esGenerat salta el que no és codi escrit a mà: còpies minificades i
// tipus generats que només fan soroll al mapa.
func esGenerat(p string) bool {
	b := strings.ToLower(filepath.Base(p))
	return strings.HasSuffix(b, ".min.js") || strings.HasSuffix(b, ".bundle.js") ||
		strings.HasSuffix(b, ".pb.go") || strings.HasSuffix(b, "_gen.go") ||
		strings.HasSuffix(b, ".generated.cs") || strings.HasSuffix(b, ".designer.cs")
}

// minificat: línies llarguíssimes de mitjana vol dir un paquet generat.
func minificat(src []byte) bool {
	n := 1
	for _, c := range src {
		if c == '\n' {
			n++
		}
	}
	return len(src)/n > 300
}

// ListFiles torna els fitxers del projecte (camins relatius, barres
// normals) i si és un repositori git. Amb git mana el que git veu (sense
// els ignorats); sense, un recorregut que salta el que no és del projecte.
func ListFiles(dir string) ([]string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		var fs []string
		for _, l := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				fs = append(fs, l)
			}
		}
		sort.Strings(fs)
		return fs, true
	}
	var out []string
	salta := map[string]bool{".git": true, "node_modules": true, "vendor": true, "__pycache__": true,
		"dist": true, "build": true, "target": true, ".venv": true, "venv": true, ".idea": true, ".vscode": true}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && (salta[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= 5000 {
			return filepath.SkipAll
		}
		if rel, err := filepath.Rel(dir, p); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, false
}

// saltaCami diu si un camí de git ls-files és d'un directori que no és del
// projecte encara que git el segueixi (vendor, node_modules).
func saltaCami(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "vendor", "node_modules", "third_party", "__pycache__":
			return true
		}
	}
	return false
}
