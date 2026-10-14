package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Hit és un passatge trobat.
type Hit struct {
	CollectionID string  `json:"collection_id"`
	Collection   string  `json:"collection"`
	Scope        string  `json:"scope"`
	Path         string  `json:"path"`
	File         string  `json:"file,omitempty"` // ruta absoluta (fitxers)
	Loc          string  `json:"loc,omitempty"`
	Kind         string  `json:"kind,omitempty"`
	Snippet      string  `json:"snippet"`
	Score        float64 `json:"score"`
}

// SearchOpts acota una cerca.
type SearchOpts struct {
	Collection string // id o nom; buit = totes les activades
	K          int
	// Indexa sola les col·leccions amb fonts que encara no tenen índex.
	IndexMissing bool
}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
	rrfK   = 60.0
)

type loaded struct {
	c  Collection
	ix *Index
}

// Search busca a les col·leccions visibles des de w (BM25; híbrid si hi ha
// vectors i embeddings configurats).
func Search(ctx context.Context, w Where, query string, o SearchOpts) ([]Hit, error) {
	terms := QueryTerms(query)
	if len(terms) == 0 {
		return nil, errors.New("the query has no searchable words")
	}
	k := o.K
	if k <= 0 {
		k = 6
	}
	if k > 50 {
		k = 50
	}
	var cols []Collection
	if strings.TrimSpace(o.Collection) != "" {
		c, err := Resolve(w, o.Collection)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", err, o.Collection)
		}
		cols = []Collection{*c}
	} else {
		all, err := List(w)
		if err != nil {
			return nil, err
		}
		for _, c := range all {
			if c.Enabled {
				cols = append(cols, c)
			}
		}
	}
	var ls []loaded
	for _, c := range cols {
		ix, err := LoadIndex(w, c)
		if (ix == nil || err != nil) && o.IndexMissing && (len(c.Sources) > 0 || len(c.Notes) > 0) {
			ix, err = Reindex(ctx, w, c.ID, false)
		}
		if err != nil || ix == nil {
			continue
		}
		ls = append(ls, loaded{c, ix})
	}
	return rank(ctx, ls, terms, query, k), nil
}

type scored struct {
	li, ci int
	s      float64
}

func rank(ctx context.Context, ls []loaded, terms []string, query string, k int) []Hit {
	// Estadístiques conjuntes: així els resultats de col·leccions diferents
	// es poden comparar.
	var n int
	var total int64
	df := map[string]int{}
	for _, l := range ls {
		n += len(l.ix.Chunks)
		total += l.ix.TotalLen
		for _, t := range terms {
			df[t] += len(l.ix.Post[t])
		}
	}
	if n == 0 {
		return nil
	}
	avg := float64(total) / float64(n)
	if avg <= 0 {
		avg = 1
	}
	var bm []scored
	for li, l := range ls {
		acc := map[int]float64{}
		matched := map[int]int{}
		for _, t := range terms {
			post := l.ix.Post[t]
			if len(post) == 0 {
				continue
			}
			idf := math.Log(1 + (float64(n)-float64(df[t])+0.5)/(float64(df[t])+0.5))
			for _, p := range post {
				tf := float64(p.TF)
				dl := float64(l.ix.Lens[p.C])
				acc[int(p.C)] += idf * tf * (bm25K1 + 1) / (tf + bm25K1*(1-bm25B+bm25B*dl/avg))
				matched[int(p.C)]++
			}
		}
		for ci, s := range acc {
			// Lleuger premi per cobrir més paraules de la consulta.
			if len(terms) > 1 {
				s *= 1 + 0.25*float64(matched[ci]-1)/float64(len(terms)-1)
			}
			bm = append(bm, scored{li, ci, s})
		}
	}
	sort.Slice(bm, func(i, j int) bool {
		if bm[i].s != bm[j].s {
			return bm[i].s > bm[j].s
		}
		if bm[i].li != bm[j].li {
			return bm[i].li < bm[j].li
		}
		return bm[i].ci < bm[j].ci
	})
	final := bm
	if vec := semantic(ctx, ls, query); len(vec) > 0 {
		final = fuse(bm, vec)
	}
	var out []Hit
	perDoc := map[string]int{}
	for _, s := range final {
		if len(out) >= k {
			break
		}
		l := ls[s.li]
		ch := l.ix.Chunks[s.ci]
		var d DocInfo
		if int(ch.Doc) < len(l.ix.Docs) {
			d = l.ix.Docs[ch.Doc]
		}
		// Com a molt tres passatges del mateix document: més varietat.
		key := l.c.ID + "\x00" + d.Key
		if perDoc[key] >= 3 {
			continue
		}
		perDoc[key]++
		h := Hit{CollectionID: l.c.ID, Collection: l.c.Name, Scope: l.c.Scope, Path: d.Path, Loc: ch.Loc, Kind: d.Kind, Snippet: Snippet(ch.Text, terms, 480), Score: math.Round(s.s*1000) / 1000}
		if !strings.HasPrefix(d.Key, "note:") {
			h.File = d.Key
		}
		out = append(out, h)
	}
	return out
}

// semantic torna els fragments per similitud amb la consulta, si hi ha
// embeddings configurats i vectors del mateix model. Qualsevol error →
// res (i la cerca queda en BM25).
func semantic(ctx context.Context, ls []loaded, query string) []scored {
	set := Current()
	if !set.EmbeddingsOn() {
		return nil
	}
	has := false
	for _, l := range ls {
		if l.ix.EmbedModel == set.Embed.Model && len(l.ix.Chunks) > 0 && len(l.ix.Chunks[0].Vec) > 0 {
			has = true
		}
	}
	if !has {
		return nil
	}
	vecs, err := embedTexts(ctx, set.Embed, []string{query})
	if err != nil || len(vecs) != 1 {
		return nil
	}
	q := vecs[0]
	var out []scored
	for li, l := range ls {
		if l.ix.EmbedModel != set.Embed.Model {
			continue
		}
		for ci, ch := range l.ix.Chunks {
			if len(ch.Vec) == 0 {
				continue
			}
			out = append(out, scored{li, ci, dot(q, ch.Vec)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].s > out[j].s })
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

// fuse combina dos rànquings amb Reciprocal Rank Fusion.
func fuse(a, b []scored) []scored {
	type key struct{ li, ci int }
	sc := map[key]float64{}
	for i, s := range a {
		if i >= 100 {
			break
		}
		sc[key{s.li, s.ci}] += 1 / (rrfK + float64(i+1))
	}
	for i, s := range b {
		sc[key{s.li, s.ci}] += 1 / (rrfK + float64(i+1))
	}
	out := make([]scored, 0, len(sc))
	for k, v := range sc {
		out = append(out, scored{k.li, k.ci, v * 100})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].s != out[j].s {
			return out[i].s > out[j].s
		}
		if out[i].li != out[j].li {
			return out[i].li < out[j].li
		}
		return out[i].ci < out[j].ci
	})
	return out
}

// ---- eines de l'agent ----

// SearchTool és l'eina knowledge_search: {query, collection?, k?}.
func SearchTool(ctx context.Context, w Where, argsJSON string) (string, error) {
	var a struct {
		Query      string `json:"query"`
		Collection string `json:"collection"`
		K          int    `json:"k"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Query) == "" {
		return "", errors.New("knowledge_search needs a query")
	}
	if a.K <= 0 {
		a.K = 6
	}
	if a.K > 20 {
		a.K = 20
	}
	all, err := List(w)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "No knowledge collections exist yet. The user can create them in the Knowledge page (or /knowledge in the terminal UI).", nil
	}
	hits, err := Search(ctx, w, a.Query, SearchOpts{Collection: a.Collection, K: a.K, IndexMissing: true})
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		var names []string
		for _, c := range all {
			if c.Enabled {
				names = append(names, c.Name)
			}
		}
		return fmt.Sprintf("No passages found for %q in: %s. Try other words (synonyms, the exact term used in the documents) or a narrower collection.", a.Query, strings.Join(names, ", ")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d passages for %q (cite them as path · location):\n", len(hits), a.Query)
	for i, h := range hits {
		loc := h.Path
		if h.Loc != "" {
			loc += " · " + h.Loc
		}
		fmt.Fprintf(&b, "\n[%d] %s — %s (%s)\n%s\n", i+1, loc, h.Collection, h.Scope, h.Snippet)
	}
	return b.String(), nil
}

// NoteTool és l'eina knowledge_note: {text, title?, collection?}. Sense
// col·lecció va a les notes de la sessió.
func NoteTool(w Where, argsJSON string) (string, error) {
	var a struct {
		Text       string `json:"text"`
		Title      string `json:"title"`
		Collection string `json:"collection"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	c, err := noteTarget(w, a.Collection)
	if err != nil {
		return "", err
	}
	if _, err := AddNote(w, c.ID, a.Title, a.Text, "agent"); err != nil {
		return "", err
	}
	return fmt.Sprintf("note saved to %s (%s)", c.Name, c.Scope), nil
}

func noteTarget(w Where, ref string) (*Collection, error) {
	if strings.TrimSpace(ref) == "" {
		return SessionNotes(w)
	}
	c, err := Resolve(w, ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, ref)
	}
	return c, nil
}

// NoteScope diu a quin abast aniria una nota (per a la política de
// permisos): global, project o session. Una col·lecció desconeguda compta
// com a global, la més restrictiva.
func NoteScope(w Where, argsJSON string) string {
	var a struct {
		Collection string `json:"collection"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.Collection) == "" {
		return ScopeSession
	}
	c, err := Resolve(w, a.Collection)
	if err != nil {
		return ScopeGlobal
	}
	return c.Scope
}

// PromptIndex és el bloc del prompt de sistema amb les col·leccions
// activades ("" si no n'hi ha cap).
func PromptIndex(w Where) string {
	all, err := List(w)
	if err != nil {
		return ""
	}
	var lines []string
	for _, c := range all {
		if !c.Enabled {
			continue
		}
		if len(lines) >= 20 {
			lines = append(lines, "- …")
			break
		}
		line := "- " + c.Name + " [" + c.Scope + "]"
		if c.Description != "" {
			d := []rune(c.Description)
			if len(d) > 160 {
				d = append(d[:160], '…')
			}
			line += " — " + string(d)
		}
		if ix, _ := LoadIndex(w, c); ix != nil {
			st := ix.Stats()
			line += fmt.Sprintf(" (%d documents)", st.Docs)
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return "KNOWLEDGE COLLECTIONS (the user's documents and notes, searchable with knowledge_search). When a question may be covered by them, search before answering, base the answer on the passages found and cite each source you use as `path · location`. If the search finds nothing relevant, say so instead of guessing:\n" + strings.Join(lines, "\n")
}
