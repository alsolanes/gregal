package web

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"gregal/internal/llm"
)

// Eines diferides. Cada pas d'un torn envia l'esquema de les 54 eines
// (uns 13.000 tokens) encara que la majoria de torns només cerquin al web o
// llegeixin un fitxer, i en un model local això són minuts de prefill. Com a
// Claude Code, el model només porta completes les eines de cada dia; de la
// resta veu el nom i una línia (a la descripció de load_tool) i en demana
// l'esquema quan el necessita. La llista d'eines no canvia durant el torn,
// així el servidor local pot reaprofitar el prefix.

const loadToolName = "load_tool"

// windCoreTools són les que sempre porten l'esquema complet.
var windCoreTools = map[string]bool{
	"web_search": true, "web_fetch": true, "web_images": true, "read": true, "write": true,
	"edit": true, "glob": true, "grep": true, "bash": true, "bash_output": true,
	"question": true, "secret": true, "todowrite": true, "todoread": true,
	"remind": true, "send_file": true, "skill": true, "wind": true, loadToolName: true,
}

// windDeferTools diu si els vents difereixen les eines (per defecte sí;
// GREGAL_WIND_ALL_TOOLS=1 les torna a enviar totes, per comparar o desfer-ho).
func windDeferTools() bool { return os.Getenv("GREGAL_WIND_ALL_TOOLS") != "1" }

// firstSentence retalla una descripció a la primera frase, com a molt n lletres.
func firstSentence(d string, n int) string {
	d = strings.Join(strings.Fields(d), " ")
	if i := strings.Index(d, ". "); i > 0 && i < n {
		return d[:i+1]
	}
	r := []rune(d)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return d
}

// deferWindSpecs separa les eines: les de cada dia completes (més load_tool,
// que descriu la resta en una línia cada una) i les diferides, per nom.
func deferWindSpecs(all []llm.ToolSpec) (active []llm.ToolSpec, deferred map[string]llm.ToolSpec) {
	deferred = map[string]llm.ToolSpec{}
	var names []string
	for _, t := range all {
		if windCoreTools[t.Name] {
			active = append(active, t)
			continue
		}
		deferred[t.Name] = t
		names = append(names, t.Name)
	}
	if len(deferred) == 0 {
		return all, nil
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("Load the full definition of one or more extra tools before you use them. A tool in this list cannot be called until you have loaded it, and loading is quick: when the person's request needs one of them (documents, spreadsheets, PDFs, browser, notes, calendar, mail, apps, publishing, routines, delegation...), load it first and then call it. Available tools:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "- %s: %s\n", n, firstSentence(deferred[n].Description, 110))
	}
	active = append(active, llm.ToolSpec{
		Name:        loadToolName,
		Description: strings.TrimRight(b.String(), "\n"),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"names": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Names of the tools to load, from the list in this tool's description",
				},
			},
			"required": []string{"names"},
		},
	})
	return active, deferred
}

// windToolLoader recorda quines eines diferides ja s'han carregat en aquest torn.
type windToolLoader struct {
	mu       sync.Mutex
	deferred map[string]llm.ToolSpec
	loaded   map[string]bool
	extra    []llm.ToolSpec // carregades: entren a la llista d'eines del pas següent
}

func newWindToolLoader(deferred map[string]llm.ToolSpec) *windToolLoader {
	return &windToolLoader{deferred: deferred, loaded: map[string]bool{}}
}

func definitionOf(t llm.ToolSpec) string {
	b, _ := json.Marshal(map[string]any{"name": t.Name, "description": t.Description, "parameters": t.Parameters})
	return string(b)
}

// Load torna les definicions demanades i les marca com a carregades.
func (l *windToolLoader) Load(argsJSON string) string {
	var in struct {
		Names []string `json:"names"`
		Name  string   `json:"name"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &in)
	if in.Name != "" {
		in.Names = append(in.Names, in.Name)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, n := range in.Names {
		n = strings.TrimSpace(n)
		t, ok := l.deferred[n]
		if !ok {
			if windCoreTools[n] {
				out = append(out, n+": already available, call it directly.")
			} else {
				out = append(out, n+": unknown tool.")
			}
			continue
		}
		if !l.loaded[n] {
			l.extra = append(l.extra, t)
		}
		l.loaded[n] = true
		out = append(out, definitionOf(t))
	}
	if len(out) == 0 {
		return "Pass names, e.g. {\"names\":[\"browser\"]}."
	}
	return strings.Join(out, "\n\n") + "\n\nThe tool(s) are now available: call them directly with these parameters."
}

// Needs diu si la crida és a una eina diferida que encara no s'ha carregat;
// en aquest cas retorna la definició perquè el model repeteixi la crida bé.
func (l *windToolLoader) Needs(name string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.deferred[name]
	if !ok || l.loaded[name] {
		return "", false
	}
	l.loaded[name] = true
	l.extra = append(l.extra, t)
	return "NOT RUN: load this tool first. Its definition is below; call it again now with these parameters.\n" + definitionOf(t), true
}

// With afegeix a la llista base les eines carregades fins ara: els models
// només ometen crides a eines que són a la llista que reben, així que, un cop
// carregada, l'eina hi entra (la llista només canvia en els torns que en
// carreguen alguna).
func (l *windToolLoader) With(base []llm.ToolSpec) []llm.ToolSpec {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.extra) == 0 {
		return base
	}
	return append(append([]llm.ToolSpec(nil), base...), l.extra...)
}
