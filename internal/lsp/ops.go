package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Request és el que demana l'eina: una operació sobre una posició.
// Line i Column són 1-based (com les mostra read); Symbol és el nom que hi
// ha a la línia, que és més fiable que una columna comptada pel model.
type Request struct {
	Op      string
	Path    string
	Line    int
	Column  int
	Symbol  string
	NewName string
}

// FileEdit és el resultat d'un rename en un fitxer.
type FileEdit struct {
	Path     string
	Old, New string
	Edits    int
}

// MaxResultats limita les referències que es llisten.
const MaxResultats = 80

// posicio troba la posició del símbol a la línia (o la columna donada). Sense
// línia però amb símbol, la primera línia on surt com a paraula sencera.
func posicio(text string, r Request) (Position, error) {
	linies := strings.Split(text, "\n")
	line := r.Line
	sym := strings.TrimSpace(r.Symbol)
	var re *regexp.Regexp
	if sym != "" {
		re = regexp.MustCompile(`(^|[^\pL\pN_])` + regexp.QuoteMeta(sym) + `($|[^\pL\pN_])`)
	}
	if line <= 0 {
		if re == nil {
			return Position{}, errors.New("give line (1-based) or symbol")
		}
		for i, l := range linies {
			if re.MatchString(l) {
				line = i + 1
				break
			}
		}
		if line <= 0 {
			return Position{}, fmt.Errorf("%q not found in the file", sym)
		}
	}
	if line > len(linies) {
		return Position{}, fmt.Errorf("line %d out of range (%d lines)", line, len(linies))
	}
	l := strings.TrimSuffix(linies[line-1], "\r")
	col := 0 // runes, 0-based
	switch {
	case re != nil:
		loc := re.FindStringIndex(l)
		if loc == nil {
			return Position{}, fmt.Errorf("%q is not on line %d: %s", sym, line, strings.TrimSpace(l))
		}
		// El patró inclou el caràcter de frontera d'abans, si n'hi ha.
		start := loc[0] + strings.Index(l[loc[0]:loc[1]], sym)
		col = len([]rune(l[:start]))
	case r.Column > 0:
		col = r.Column - 1
	default:
		col = len([]rune(l)) - len([]rune(strings.TrimLeft(l, " \t")))
	}
	return Position{Line: line - 1, Character: ColUTF16(l, col)}, nil
}

// Do executa una operació i torna el text per al model. Per a rename torna
// també els canvis (sense escriure'ls: ho fa qui crida, amb el diari i les
// fotos de sempre).
func (m *Manager) Do(ctx context.Context, root string, r Request) (string, []FileEdit, error) {
	path := r.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	c, s, err := m.clientPer(ctx, root, path)
	if err != nil {
		return "", nil, err
	}
	abans := c.diagVersio(URI(path))
	uri, text, err := m.obre(c, s, path)
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if r.Op == "diagnostics" {
		ds, nous := c.EsperaDiagnostics(uri, abans, 8*time.Second)
		return formatDiags(root, path, text, ds, nous), nil, nil
	}
	pos, err := posicio(text, r)
	if err != nil {
		return "", nil, err
	}
	td := map[string]any{"uri": uri}
	switch r.Op {
	case "definition":
		raw, err := c.Call(ctx, "textDocument/definition", map[string]any{"textDocument": td, "position": pos})
		if err != nil {
			return "", nil, err
		}
		return formatLocs(root, locations(raw), "no definition found"), nil, nil
	case "references":
		raw, err := c.Call(ctx, "textDocument/references", map[string]any{"textDocument": td, "position": pos, "context": map[string]any{"includeDeclaration": true}})
		if err != nil {
			return "", nil, err
		}
		return formatLocs(root, locations(raw), "no references found"), nil, nil
	case "hover":
		raw, err := c.Call(ctx, "textDocument/hover", map[string]any{"textDocument": td, "position": pos})
		if err != nil {
			return "", nil, err
		}
		return hoverText(raw), nil, nil
	case "rename":
		nou := strings.TrimSpace(r.NewName)
		if nou == "" {
			return "", nil, errors.New("rename needs new_name")
		}
		raw, err := c.Call(ctx, "textDocument/rename", map[string]any{"textDocument": td, "position": pos, "newName": nou})
		if err != nil {
			return "", nil, err
		}
		edits, err := aplicaWorkspaceEdit(root, raw)
		if err != nil {
			return "", nil, err
		}
		if len(edits) == 0 {
			return "the server found nothing to rename at that position", nil, nil
		}
		return "", edits, nil
	}
	return "", nil, fmt.Errorf("unknown operation %q (definition, references, hover, rename, diagnostics)", r.Op)
}

// locations accepta Location, []Location i []LocationLink.
func locations(raw json.RawMessage) []Location {
	var one Location
	if json.Unmarshal(raw, &one) == nil && one.URI != "" {
		return []Location{one}
	}
	var many []struct {
		URI         string `json:"uri"`
		Range       Range  `json:"range"`
		TargetURI   string `json:"targetUri"`
		TargetRange Range  `json:"targetSelectionRange"`
	}
	if json.Unmarshal(raw, &many) != nil {
		return nil
	}
	var out []Location
	for _, l := range many {
		if l.TargetURI != "" {
			out = append(out, Location{URI: l.TargetURI, Range: l.TargetRange})
		} else if l.URI != "" {
			out = append(out, Location{URI: l.URI, Range: l.Range})
		}
	}
	return out
}

func relatiu(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

// liniaDe torna el text d'una línia (0-based) d'un fitxer, per ensenyar el
// context de cada resultat.
func liniaDe(cache map[string][]string, path string, line int) string {
	ls, ok := cache[path]
	if !ok {
		raw, err := os.ReadFile(path)
		if err == nil {
			ls = strings.Split(string(raw), "\n")
		}
		cache[path] = ls
	}
	if line < 0 || line >= len(ls) {
		return ""
	}
	t := strings.TrimSpace(strings.TrimSuffix(ls[line], "\r"))
	if r := []rune(t); len(r) > 160 {
		t = string(r[:160]) + "…"
	}
	return t
}

func formatLocs(root string, locs []Location, buit string) string {
	if len(locs) == 0 {
		return buit
	}
	sort.SliceStable(locs, func(i, j int) bool {
		if locs[i].URI != locs[j].URI {
			return locs[i].URI < locs[j].URI
		}
		return locs[i].Range.Start.Line < locs[j].Range.Start.Line
	})
	cache := map[string][]string{}
	var b strings.Builder
	if len(locs) > 1 {
		fmt.Fprintf(&b, "%d results\n", len(locs))
	}
	for i, l := range locs {
		if i >= MaxResultats {
			fmt.Fprintf(&b, "… and %d more\n", len(locs)-i)
			break
		}
		p := Path(l.URI)
		text := liniaDe(cache, p, l.Range.Start.Line)
		col := ColRunes(text, l.Range.Start.Character) + 1
		fmt.Fprintf(&b, "%s:%d:%d: %s\n", relatiu(root, p), l.Range.Start.Line+1, col, text)
	}
	return strings.TrimRight(b.String(), "\n")
}

func hoverText(raw json.RawMessage) string {
	var h struct {
		Contents json.RawMessage `json:"contents"`
	}
	if json.Unmarshal(raw, &h) != nil || len(h.Contents) == 0 || string(h.Contents) == "null" {
		return "no information at that position"
	}
	var s string
	if json.Unmarshal(h.Contents, &s) == nil {
		return strings.TrimSpace(s)
	}
	var mc struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(h.Contents, &mc) == nil && mc.Value != "" {
		return strings.TrimSpace(mc.Value)
	}
	var parts []json.RawMessage
	if json.Unmarshal(h.Contents, &parts) == nil {
		var out []string
		for _, p := range parts {
			if json.Unmarshal(p, &s) == nil {
				out = append(out, s)
			} else if json.Unmarshal(p, &mc) == nil {
				out = append(out, mc.Value)
			}
		}
		return strings.TrimSpace(strings.Join(out, "\n\n"))
	}
	return "no information at that position"
}

var severitats = map[int]string{1: "error", 2: "warning", 3: "info", 4: "hint"}

func formatDiags(root, path, text string, ds []Diagnostic, nous bool) string {
	if len(ds) == 0 {
		if !nous {
			return "no diagnostics received from the server yet (it may still be loading the project)"
		}
		return "no problems reported in " + relatiu(root, path)
	}
	linies := strings.Split(text, "\n")
	var b strings.Builder
	for i, d := range ds {
		if i >= MaxResultats {
			fmt.Fprintf(&b, "… and %d more\n", len(ds)-i)
			break
		}
		col := d.Range.Start.Character + 1
		if d.Range.Start.Line < len(linies) {
			col = ColRunes(strings.TrimSuffix(linies[d.Range.Start.Line], "\r"), d.Range.Start.Character) + 1
		}
		sev := severitats[d.Severity]
		if sev == "" {
			sev = "error"
		}
		fmt.Fprintf(&b, "%s:%d:%d: %s: %s", relatiu(root, path), d.Range.Start.Line+1, col, sev, strings.TrimSpace(d.Message))
		if d.Source != "" {
			fmt.Fprintf(&b, " (%s)", d.Source)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// aplicaWorkspaceEdit calcula el contingut nou de cada fitxer d'un
// WorkspaceEdit. Es refusen els canvis fora de l'arrel i les operacions
// sobre fitxers (crear, moure, esborrar), que un rename de símbol no
// necessita.
func aplicaWorkspaceEdit(root string, raw json.RawMessage) ([]FileEdit, error) {
	var we struct {
		Changes         map[string][]TextEdit `json:"changes"`
		DocumentChanges []json.RawMessage     `json:"documentChanges"`
	}
	if err := json.Unmarshal(raw, &we); err != nil {
		return nil, err
	}
	perFitxer := map[string][]TextEdit{}
	for uri, es := range we.Changes {
		perFitxer[Path(uri)] = append(perFitxer[Path(uri)], es...)
	}
	for _, dc := range we.DocumentChanges {
		var t struct {
			Kind         string `json:"kind"`
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Edits []TextEdit `json:"edits"`
		}
		if json.Unmarshal(dc, &t) != nil {
			continue
		}
		if t.Kind != "" {
			return nil, fmt.Errorf("the rename also wants to %s a file; do it by hand", t.Kind)
		}
		p := Path(t.TextDocument.URI)
		perFitxer[p] = append(perFitxer[p], t.Edits...)
	}
	arrel, _ := filepath.Abs(root)
	var out []FileEdit
	for p, es := range perFitxer {
		abs, _ := filepath.Abs(p)
		if rel, err := filepath.Rel(arrel, abs); err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("the rename touches %s, outside the workspace; refused", p)
		}
		raw, err := os.ReadFile(abs)
		if err != nil {
			return nil, err
		}
		vell := string(raw)
		type tros struct {
			ini, fi int
			text    string
		}
		var ts []tros
		for _, e := range es {
			ts = append(ts, tros{offset(vell, e.Range.Start), offset(vell, e.Range.End), e.NewText})
		}
		sort.Slice(ts, func(i, j int) bool { return ts[i].ini > ts[j].ini })
		nou := vell
		for _, t := range ts {
			if t.ini > t.fi || t.fi > len(nou) {
				return nil, fmt.Errorf("invalid edit range in %s", p)
			}
			nou = nou[:t.ini] + t.text + nou[t.fi:]
		}
		if nou != vell {
			out = append(out, FileEdit{Path: abs, Old: vell, New: nou, Edits: len(es)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
