package repomap

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Cerca de definicions per a l'eina `symbols`.

// Query és una cerca de símbols.
type Query struct {
	Name     string // subcadena (sense distingir majúscules) o expressió regular
	Regex    bool
	Kind     string // func, method, type, struct, interface, class, enum, trait, impl, const, var
	File     string // glob sobre el camí relatiu (admet **); sense «/» mira el nom del fitxer
	Exported bool   // només els públics
	Limit    int    // 0 = 50; màxim 200
}

// Hit és una definició trobada.
type Hit struct {
	Path string
	Symbol
}

// SearchBudget és el temps que l'eina dona a l'índex: més que el del mapa
// (una cerca incompleta enganya més que un mapa incomplet).
const SearchBudget = 3 * time.Second

// kindsAlias accepta els noms habituals de cada llenguatge.
var kindsAlias = map[string]string{
	"function": "func", "fn": "func", "def": "func", "func": "func",
	"method": "method", "type": "type", "struct": "struct", "interface": "interface", "iface": "interface",
	"class": "class", "record": "record", "enum": "enum", "trait": "trait", "impl": "impl",
	"const": "const", "constant": "const", "var": "var", "variable": "var", "static": "const",
}

// Search busca a l'índex del directori (l'actualitza abans).
func Search(dir string, q Query) ([]Hit, *Snapshot, error) {
	snap := Build(dir, nil, Options{Budget: SearchBudget})
	hits, err := SearchSnapshot(snap, q)
	return hits, snap, err
}

// SearchSnapshot busca en una instantània ja feta.
func SearchSnapshot(snap *Snapshot, q Query) ([]Hit, error) {
	var re *regexp.Regexp
	nom := strings.TrimSpace(q.Name)
	if q.Regex && nom != "" {
		var err error
		if re, err = regexp.Compile(nom); err != nil {
			return nil, fmt.Errorf("invalid regex: %v", err)
		}
	}
	kind := ""
	if k := strings.ToLower(strings.TrimSpace(q.Kind)); k != "" {
		var ok bool
		if kind, ok = kindsAlias[k]; !ok {
			return nil, fmt.Errorf("unknown kind %q (use func, method, type, struct, interface, class, enum, trait, impl, const or var)", q.Kind)
		}
	}
	var glob *regexp.Regexp
	if g := strings.TrimSpace(q.File); g != "" {
		glob = globRegex(g)
	}
	baixa := strings.ToLower(nom)
	var hits []Hit
	for _, f := range snap.Files {
		if glob != nil {
			obj := f.Path
			if !strings.Contains(q.File, "/") {
				obj = path.Base(f.Path)
			}
			if !glob.MatchString(obj) {
				continue
			}
		}
		for _, s := range f.Symbols {
			if kind != "" && s.Kind != kind && !(kind == "class" && s.Kind == "record") && !(kind == "type" && (s.Kind == "struct" || s.Kind == "interface")) {
				continue
			}
			if q.Exported && !s.Exported {
				continue
			}
			if nom != "" {
				disp := s.Display()
				if re != nil {
					if !re.MatchString(s.Name) && !re.MatchString(disp) {
						continue
					}
				} else if !strings.Contains(strings.ToLower(disp), baixa) {
					continue
				}
			}
			hits = append(hits, Hit{Path: f.Path, Symbol: s})
		}
	}
	// Primer el nom exacte, després el que hi comença, després la resta;
	// el codi abans que els tests.
	grau := func(h Hit) int {
		if nom == "" || re != nil {
			return 0
		}
		n, d := strings.ToLower(h.Name), strings.ToLower(h.Display())
		switch {
		case n == baixa || d == baixa:
			return 0
		case strings.HasPrefix(n, baixa) || strings.HasPrefix(d, baixa):
			return 1
		}
		return 2
	}
	sort.SliceStable(hits, func(i, j int) bool {
		gi, gj := grau(hits[i]), grau(hits[j])
		if gi != gj {
			return gi < gj
		}
		ti, tj := esTest(hits[i].Path), esTest(hits[j].Path)
		if ti != tj {
			return !ti
		}
		if hits[i].Path != hits[j].Path {
			return hits[i].Path < hits[j].Path
		}
		return hits[i].Line < hits[j].Line
	})
	return hits, nil
}

// FormatHits pinta els resultats per al model: «camí:línia kind nom — signatura».
func FormatHits(hits []Hit, snap *Snapshot, limit int) string {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var b strings.Builder
	if len(hits) == 0 {
		b.WriteString("no symbols found")
		if snap != nil && len(snap.Files) == 0 {
			b.WriteString(" (no Go/JS/TS/Python/Java/C#/Rust source files indexed here)")
		}
		b.WriteString("\n")
	}
	for i, h := range hits {
		if i >= limit {
			fmt.Fprintf(&b, "… %d more matches: narrow the search with kind, file or a longer name\n", len(hits)-limit)
			break
		}
		fmt.Fprintf(&b, "%s:%d %s %s", h.Path, h.Line, h.Kind, h.Display())
		if h.Sig != "" {
			b.WriteString(" — " + h.Sig)
		}
		b.WriteString("\n")
	}
	if snap != nil && snap.Partial {
		b.WriteString("(index incomplete: the repository is large; results may be missing — narrow with file)\n")
	}
	return b.String()
}

// globRegex converteix un glob (amb **) en una expressió regular sencera.
func globRegex(g string) *regexp.Regexp {
	g = strings.TrimPrefix(strings.ReplaceAll(g, "\\", "/"), "./")
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch {
		case c == '*' && i+1 < len(g) && g[i+1] == '*':
			i++
			if i+1 < len(g) && g[i+1] == '/' {
				i++
				b.WriteString("(?:.*/)?")
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile("(?i)" + b.String())
	if err != nil {
		return regexp.MustCompile("^$")
	}
	return re
}
