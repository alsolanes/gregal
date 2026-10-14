package repomap

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Rànquing i pintat del mapa de símbols per a una tasca.
//
// La rellevància és barata i explicable: coincidència d'identificadors
// entre la tasca i els noms de símbols i camins (partits per camelCase i
// snake_case, en minúscules), més un pes per als fitxers que s'estan
// tocant (git status) o s'han modificat fa poc. Els fitxers que la tasca
// esmenta pel nom d'un símbol surten primer.

// DefaultTokens és el pressupost per defecte del mapa de símbols: prou per
// orientar-se sense inflar cada torn.
const DefaultTokens = 1200

// RankOptions afinen el rànquing.
type RankOptions struct {
	Tokens   int             // pressupost (chars/4); 0 = DefaultTokens
	Modified map[string]bool // camins tocats segons git (els que acaben en / són carpetes)
	Now      time.Time       // per a la recència (zero = ara)
}

// Ranked és el resultat del rànquing pintat.
type Ranked struct {
	Text   string // la secció, o "" si no hi ha res a dir
	Strong bool   // la tasca anomena literalment algun símbol compost (p. ex. «MapaProjecte»)
	Files  int    // fitxers inclosos
}

var paraulesBuides = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "this": true, "that": true, "from": true,
	"into": true, "when": true, "then": true, "than": true, "what": true, "which": true, "where": true,
	"have": true, "has": true, "are": true, "was": true, "were": true, "will": true, "should": true,
	"can": true, "could": true, "would": true, "not": true, "but": true, "all": true, "any": true,
	"some": true, "use": true, "using": true, "make": true, "add": true, "fix": true, "new": true,
	"get": true, "set": true, "also": true, "only": true, "there": true, "their": true, "them": true,
	"how": true, "why": true, "you": true, "your": true, "please": true, "need": true, "want": true,
	"file": true, "files": true, "code": true, "func": true, "function": true, "test": true, "tests": true,
	"que": true, "per": true, "amb": true, "els": true, "les": true, "una": true, "uns": true,
	"del": true, "dels": true, "com": true, "més": true, "fer": true, "aquest": true, "aquesta": true,
	"fitxer": true, "codi": true, "go": true, "src": true, "internal": true, "pkg": true, "lib": true,
}

// Parts parteix un identificador en paraules en minúscules:
// «MapaProjecte» → mapa, projecte; «HTTPServer» → http, server;
// «llista_fitxers» → llista, fitxers.
func Parts(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if unicode.IsUpper(r) && len(cur) > 0 {
				prev := rs[i-1]
				seguentMinuscula := i+1 < len(rs) && unicode.IsLower(rs[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && seguentMinuscula) {
					flush()
				}
			}
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return out
}

// paraulesTasca són els identificadors que apareixen a la tasca, tal com
// hi són escrits.
func paraulesTasca(tasca string) []string {
	return strings.FieldsFunc(tasca, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.')
	})
}

// compost diu si una paraula és clarament un identificador (té guions
// baixos o majúscules a dins) i no una paraula de la frase.
func compost(w string) bool {
	if len(w) < 5 {
		return false
	}
	if strings.ContainsAny(w, "_") {
		return true
	}
	for i, r := range w {
		if i > 0 && unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

type candidat struct {
	f       *File
	score   float64
	matched map[int]bool // índexs de símbols que coincideixen
}

// Rank ordena els fitxers per rellevància per a la tasca i pinta la secció
// dins del pressupost.
func Rank(snap *Snapshot, tasca string, opt RankOptions) Ranked {
	if snap == nil || len(snap.Files) == 0 {
		return Ranked{}
	}
	if opt.Tokens <= 0 {
		opt.Tokens = DefaultTokens
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	// Fitxes de la tasca: paraules senceres (exactes) i les seves parts.
	exactes := map[string]bool{}  // en minúscules
	literals := map[string]bool{} // compostos tal com estan escrits
	fitxes := map[string]bool{}
	for _, w := range paraulesTasca(tasca) {
		w = strings.Trim(w, ".")
		if w == "" {
			continue
		}
		if compost(w) {
			literals[w] = true
		}
		for _, sub := range strings.Split(w, ".") {
			if len(sub) >= 4 {
				exactes[strings.ToLower(sub)] = true
			}
			if compost(sub) {
				literals[sub] = true
			}
		}
		if strings.Contains(w, ".") && len(w) >= 4 {
			exactes[strings.ToLower(w)] = true
		}
		for _, p := range Parts(w) {
			if len(p) >= 3 && !paraulesBuides[p] {
				fitxes[p] = true
			}
		}
	}
	// Pes de cada fitxa per raresa (IDF): «sense» o «get» surten a mig
	// repositori i no diuen gaire; «projecte» en un sol fitxer, molt.
	df := map[string]int{}
	for _, f := range snap.Files {
		vist := map[string]bool{}
		for _, s := range f.Symbols {
			for _, p := range Parts(s.Name) {
				if fitxes[p] && !vist[p] {
					vist[p] = true
					df[p]++
				}
			}
		}
	}
	nf := float64(len(snap.Files))
	pesFitxa := func(p string) float64 {
		return 6 * math.Log(1+nf/float64(df[p]+1)) / math.Log(1+nf)
	}
	strong := false
	cands := make([]*candidat, 0, len(snap.Files))
	for _, f := range snap.Files {
		c := &candidat{f: f, matched: map[int]bool{}}
		fitxesSimbols := map[string]bool{}
		exact := 0.0
		for i, s := range f.Symbols {
			disp := s.Display()
			if literals[s.Name] || literals[disp] {
				exact += 12
				c.matched[i] = true
				strong = true
			} else if exactes[strings.ToLower(s.Name)] || exactes[strings.ToLower(disp)] {
				exact += 6
				c.matched[i] = true
			}
			for _, p := range Parts(s.Name) {
				if fitxes[p] {
					fitxesSimbols[p] = true
					c.matched[i] = true
				}
			}
		}
		if exact > 36 {
			exact = 36
		}
		fitxesCami := 0
		for _, p := range Parts(f.Path) {
			if fitxes[p] {
				fitxesCami++
			}
		}
		c.score = exact + 3*float64(fitxesCami)
		for p := range fitxesSimbols {
			c.score += pesFitxa(p)
		}
		if tocat(f.Path, opt.Modified) {
			c.score += 3
		}
		switch edat := opt.Now.Sub(f.ModTime); {
		case edat < 2*time.Hour:
			c.score += 1.5
		case edat < 24*time.Hour:
			c.score += 0.5
		}
		if esTest(f.Path) {
			c.score *= 0.5
		}
		cands = append(cands, c)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		// Empat: abans el codi que el test, i el que té més API pública.
		ti, tj := esTest(cands[i].f.Path), esTest(cands[j].f.Path)
		if ti != tj {
			return !ti
		}
		ei, ej := exportats(cands[i].f), exportats(cands[j].f)
		if ei != ej {
			return ei > ej
		}
		return cands[i].f.Path < cands[j].f.Path
	})
	maxChars := opt.Tokens * 4
	var b strings.Builder
	b.WriteString(capcalera)
	n := 0
	for _, c := range cands {
		// Els tests només si tenen a veure amb la tasca.
		if esTest(c.f.Path) && len(c.matched) == 0 {
			continue
		}
		linia := liniaFitxer(c, maxChars-b.Len())
		if linia == "" {
			if b.Len() > maxChars-40 {
				break
			}
			continue
		}
		b.WriteString(linia)
		n++
	}
	if n == 0 {
		return Ranked{}
	}
	if snap.Partial {
		b.WriteString("(symbol index incomplete: the repository is large or the time budget ran out)\n")
	}
	return Ranked{Text: b.String(), Strong: strong, Files: n}
}

const capcalera = "Symbols (most relevant to the task first; kinds: f func, m method, t type, s struct, i interface, c class, e enum, r trait, x impl, k const, v var; the symbols tool returns definitions with line numbers):\n"

var lletraKind = map[string]string{
	"func": "f", "method": "m", "type": "t", "struct": "s", "interface": "i", "class": "c",
	"record": "c", "enum": "e", "trait": "r", "impl": "x", "const": "k", "var": "v",
}

// maxSimbolsLinia topa els símbols per fitxer al mapa.
const (
	maxSimbolsLinia      = 14
	maxContextLinia      = 4 // símbols sense coincidència al costat dels que sí
	maxSenseCoincidencia = 8 // per fitxer que només omple el pressupost
)

// liniaFitxer pinta «camí: A(f) B.c(m) …» dins de lloc caràcters, o "" si
// no hi cap ni amb un símbol.
func liniaFitxer(c *candidat, lloc int) string {
	f := c.f
	idx := make([]int, 0, len(f.Symbols))
	for i, s := range f.Symbols {
		if c.matched[i] || mostrable(f, s) {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return ""
	}
	// Primer els que coincideixen amb la tasca, després els públics; dins
	// de cada grup, l'ordre del fitxer.
	pes := func(i int) int {
		switch {
		case c.matched[i]:
			return 0
		case f.Symbols[i].Exported:
			return 1
		}
		return 2
	}
	sort.SliceStable(idx, func(a, b int) bool { return pes(idx[a]) < pes(idx[b]) })
	// Dels que no coincideixen, només uns quants: el lloc és per als
	// fitxers rellevants, i el fitxer sencer es pot obrir.
	maxLinia := maxSimbolsLinia
	if n := len(c.matched); n > 0 {
		maxLinia = min(maxSimbolsLinia, n+maxContextLinia)
	} else {
		maxLinia = maxSenseCoincidencia
	}
	var b strings.Builder
	b.WriteString(f.Path + ":")
	posats := 0
	for k, i := range idx {
		if posats >= maxLinia {
			break
		}
		s := f.Symbols[i]
		tros := " " + s.Display() + "(" + lletraKind[s.Kind] + ")"
		resta := ""
		if k+1 < len(idx) {
			resta = fmt.Sprintf(" +%d", len(idx)-k-1)
		}
		if b.Len()+len(tros)+len(resta)+1 > lloc {
			break
		}
		b.WriteString(tros)
		posats++
	}
	if posats == 0 {
		return ""
	}
	if posats < len(idx) {
		fmt.Fprintf(&b, " +%d", len(idx)-posats)
	}
	b.WriteString("\n")
	if b.Len() > lloc {
		return ""
	}
	return b.String()
}

// mostrable: al mapa sense coincidència només hi va el que val la pena.
// Fora les constants i variables privades de Go i les funcions de test.
func mostrable(f *File, s Symbol) bool {
	if f.Lang == "go" && !s.Exported && (s.Kind == "const" || s.Kind == "var") {
		return false
	}
	if esTest(f.Path) && (strings.HasPrefix(s.Name, "Test") || strings.HasPrefix(s.Name, "Benchmark")) {
		return false
	}
	return true
}

func exportats(f *File) int {
	n := 0
	for _, s := range f.Symbols {
		if s.Exported {
			n++
		}
	}
	return n
}

func esTest(p string) bool {
	b := strings.ToLower(p)
	if i := strings.LastIndex(b, "/"); i >= 0 {
		b = b[i+1:]
	}
	return strings.HasSuffix(b, "_test.go") || strings.Contains(b, ".test.") || strings.Contains(b, ".spec.") ||
		strings.HasPrefix(b, "test_") || strings.HasSuffix(b, "_test.py") || strings.HasSuffix(b, "tests.cs") ||
		strings.HasSuffix(b, "test.java")
}

// tocat diu si el camí és a la llista de git (o dins d'una carpeta nova).
func tocat(p string, mod map[string]bool) bool {
	if len(mod) == 0 {
		return false
	}
	if mod[p] {
		return true
	}
	for d := p; ; {
		i := strings.LastIndex(d, "/")
		if i < 0 {
			return false
		}
		d = d[:i]
		if mod[d+"/"] {
			return true
		}
	}
}

// ModifiedFromPorcelain llegeix la sortida de `git status --porcelain`
// (amb o sense --branch) i torna els camins tocats.
func ModifiedFromPorcelain(s string) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if len(l) < 4 || strings.HasPrefix(l, "##") || l[2] != ' ' {
			continue
		}
		p := l[3:]
		if i := strings.Index(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			out[p] = true
		}
	}
	return out
}
