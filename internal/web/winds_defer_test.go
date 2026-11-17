package web

import (
	"encoding/json"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func specNamed(n string, size int) llm.ToolSpec {
	return llm.ToolSpec{Name: n, Description: "Does " + n + ". " + strings.Repeat("More detail. ", size), Parameters: map[string]any{"type": "object"}}
}

func TestDeferWindSpecsKeepsCoreAndCatalogsTheRest(t *testing.T) {
	all := []llm.ToolSpec{specNamed("web_search", 1), specNamed("read", 1), specNamed("office_create", 200), specNamed("browser", 300)}
	active, def := deferWindSpecs(all)
	names := map[string]bool{}
	for _, a := range active {
		names[a.Name] = true
	}
	if !names["web_search"] || !names["read"] || !names[loadToolName] || names["browser"] || names["office_create"] {
		t.Fatalf("active: %v", names)
	}
	if len(def) != 2 {
		t.Fatalf("deferred: %v", def)
	}
	var load llm.ToolSpec
	for _, a := range active {
		if a.Name == loadToolName {
			load = a
		}
	}
	if !strings.Contains(load.Description, "- browser: Does browser.") || strings.Contains(load.Description, "More detail. More detail.") {
		t.Fatalf("catalog:\n%s", load.Description)
	}
	// Mida: molt més petit que enviar-ho tot.
	full, small := 0, 0
	for _, a := range all {
		b, _ := json.Marshal(a)
		full += len(b)
	}
	for _, a := range active {
		b, _ := json.Marshal(a)
		small += len(b)
	}
	if small >= full {
		t.Fatalf("no s'ha reduït: %d >= %d", small, full)
	}
}

func TestToolLoaderLoadsAndRejectsOnce(t *testing.T) {
	_, def := deferWindSpecs([]llm.ToolSpec{specNamed("read", 1), specNamed("browser", 3)})
	l := newWindToolLoader(def)
	// Crida sense carregar: no s'executa i torna la definició, un cop.
	msg, ok := l.Needs("browser")
	if !ok || !strings.Contains(msg, `"name":"browser"`) {
		t.Fatalf("needs: %v %q", ok, msg)
	}
	if _, ok := l.Needs("browser"); ok {
		t.Fatal("la segona crida ja ha de passar")
	}
	if _, ok := l.Needs("read"); ok {
		t.Fatal("una eina de base no es difereix")
	}
	// Carregar-la abans: la crida passa directa.
	l2 := newWindToolLoader(def)
	out := l2.Load(`{"names":["browser","nope","read"]}`)
	if !strings.Contains(out, `"name":"browser"`) || !strings.Contains(out, "nope: unknown") || !strings.Contains(out, "read: already") {
		t.Fatalf("load: %s", out)
	}
	if _, ok := l2.Needs("browser"); ok {
		t.Fatal("carregada: no s'ha de tornar a rebutjar")
	}
}
