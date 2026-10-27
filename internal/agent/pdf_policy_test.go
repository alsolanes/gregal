package agent

import (
	"path/filepath"
	"testing"

	"gregal/internal/tools"
)

// pdf_read es llegeix com read (sense preguntar, també en paral·lel) i
// pdf_edit s'escriu com write: dins del projecte passa sol en code i
// autònom, fora pregunta, i els modes de només lectura el deneguen.
func TestPdfToolsPolicy(t *testing.T) {
	for _, n := range []string{"pdf_read", "pdf_edit"} {
		if !tools.EsNativa(n) || !tools.Configurable(n) {
			t.Fatalf("%s ha de ser al registre i configurable", n)
		}
	}
	if d, _ := PolicyFor("pdf_read", `{"path":"a.pdf"}`); d != "allow" {
		t.Fatalf("pdf_read=%s", d)
	}
	if d, _ := PolicyFor("pdf_edit", `{"path":"a.pdf","op":"rotate"}`); d != "ask" {
		t.Fatalf("pdf_edit=%s", d)
	}
	if !ParallelSafe("pdf_read") || ParallelSafe("pdf_edit") || !readOnlyParallel["pdf_read"] || readOnlyParallel["pdf_edit"] {
		t.Fatal("pdf_read és paral·lelitzable; pdf_edit no")
	}

	dir := t.TempDir()
	p := &Policy{ProjectDir: dir}
	dins := `{"path":"` + filepath.ToSlash(filepath.Join(dir, "a.pdf")) + `","op":"stamp"}`
	fora := `{"path":"` + filepath.ToSlash(filepath.Join(dir, "a.pdf")) + `","op":"extract","out":"` + filepath.ToSlash(filepath.Join(t.TempDir(), "b.pdf")) + `"}`
	for _, mode := range []string{ModeCode, ModeAutonomous} {
		if d, _ := p.Decide(mode, "pdf_edit", dins); d != "allow" {
			t.Fatalf("%s dins del projecte: %s", mode, d)
		}
	}
	if d, _ := p.Decide(ModeCode, "pdf_edit", fora); d != "ask" {
		t.Fatalf("code, out fora del projecte: %s", d)
	}
	for _, mode := range []string{ModeInspect, ModeChat, ModeGoal} {
		if d, _ := p.Decide(mode, "pdf_edit", dins); d != "deny" {
			t.Fatalf("%s: pdf_edit=%s", mode, d)
		}
		if d, _ := p.Decide(mode, "pdf_read", dins); d != "allow" {
			t.Fatalf("%s: pdf_read=%s", mode, d)
		}
	}
	// Un deny explícit del config mana.
	pd := &Policy{ProjectDir: dir, Tools: map[string]string{"pdf_edit": "deny"}}
	if d, _ := pd.Decide(ModeCode, "pdf_edit", dins); d != "deny" {
		t.Fatalf("config deny: %s", d)
	}
}
