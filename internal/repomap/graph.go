package repomap

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Graf de referències (com el repo map d'Aider).
//
// El rànquing lèxic troba els fitxers que comparteixen paraules amb la
// tasca, però no el codi del qual depenen: el tipus que fan servir, la
// funció que criden. Aquí cada fitxer apunta als fitxers que defineixen
// els identificadors que usa, i un PageRank personalitzat (que surt dels
// fitxers rellevants per a la tasca i dels que s'estan tocant) fa pujar
// aquestes dependències.

var identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)

// maxRefs limita els identificadors desats per fitxer.
const maxRefs = 3000

// paraules reservades i noms massa genèrics que no aporten arestes.
var refsBuides = map[string]bool{
	"func": true, "return": true, "package": true, "import": true, "const": true, "var": true, "type": true,
	"struct": true, "interface": true, "string": true, "error": true, "bool": true, "true": true, "false": true,
	"nil": true, "for": true, "range": true, "else": true, "switch": true, "case": true, "default": true,
	"break": true, "continue": true, "defer": true, "this": true, "self": true, "class": true, "def": true,
	"public": true, "private": true, "static": true, "void": true, "new": true, "None": true, "True": true,
	"False": true, "function": true, "let": true, "async": true, "await": true, "from": true, "export": true,
	"int": true, "float64": true, "len": true, "make": true, "append": true, "err": true, "Println": true,
	"Errorf": true, "Sprintf": true, "fmt": true, "strings": true, "context": true, "ctx": true,
}

// extractRefs torna els identificadors que apareixen al codi (sense
// repetir, fins a maxRefs).
func extractRefs(src []byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range identRe.FindAll(src, -1) {
		s := string(m)
		if refsBuides[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) >= maxRefs {
			break
		}
	}
	return out
}

// pageRank calcula un PageRank personalitzat sobre el graf de fitxers.
// pers dóna el pes de sortida de cada fitxer (sumarà 1 un cop normalitzat).
// mencionats són identificadors esmentats a la tasca (arestes ×10).
func pageRank(files []*File, pers []float64, mencionats map[string]bool) []float64 {
	n := len(files)
	if n == 0 {
		return nil
	}
	// Qui defineix cada nom.
	defs := map[string][]int{}
	for i, f := range files {
		vist := map[string]bool{}
		for _, s := range f.Symbols {
			if !vist[s.Name] {
				vist[s.Name] = true
				defs[s.Name] = append(defs[s.Name], i)
			}
		}
	}
	type aresta struct {
		to int
		w  float64
	}
	out := make([][]aresta, n)
	for i, f := range files {
		pes := map[int]float64{}
		for _, r := range f.Refs {
			ds := defs[r]
			if len(ds) == 0 {
				continue
			}
			w := 1.0
			if mencionats[strings.ToLower(r)] {
				w *= 10
			}
			if len(r) >= 8 && compost(r) {
				w *= 3
			}
			if strings.HasPrefix(r, "_") || len(ds) > 5 {
				w *= 0.1
			}
			for _, d := range ds {
				if d != i {
					pes[d] += w / float64(len(ds))
				}
			}
		}
		for d, w := range pes {
			out[i] = append(out[i], aresta{d, math.Sqrt(w)})
		}
		sort.Slice(out[i], func(a, b int) bool { return out[i][a].to < out[i][b].to })
	}
	// Personalització normalitzada (uniforme si no n'hi ha).
	p := make([]float64, n)
	tot := 0.0
	for i := range p {
		if i < len(pers) && pers[i] > 0 {
			p[i] = pers[i]
			tot += p[i]
		}
	}
	if tot == 0 {
		for i := range p {
			p[i] = 1
		}
		tot = float64(n)
	}
	for i := range p {
		p[i] /= tot
	}
	const d = 0.85
	pr := append([]float64(nil), p...)
	for it := 0; it < 25; it++ {
		next := make([]float64, n)
		dang := 0.0
		for i := 0; i < n; i++ {
			sum := 0.0
			for _, a := range out[i] {
				sum += a.w
			}
			if sum == 0 {
				dang += pr[i]
				continue
			}
			for _, a := range out[i] {
				next[a.to] += d * pr[i] * a.w / sum
			}
		}
		for i := range next {
			next[i] += (1-d)*p[i] + d*dang*p[i]
		}
		pr = next
	}
	return pr
}
