package agent

import (
	"strings"
	"testing"

	"gregal/internal/llm"
)

func TestCompactAddsOfficeToolsOnlyForOfficeTasks(t *testing.T) {
	code := []llm.Message{{Role: "user", Content: "Create stats.py with a median function"}}
	office := []llm.Message{{Role: "user", Content: "Crea vendes.pptx amb un gràfic"}}
	has := func(hist []llm.Message, name string) bool {
		for _, s := range SpecsForHist(ModeCode, true, hist) {
			if s.Name == name {
				return true
			}
		}
		return false
	}
	if has(code, "office_create") || has(code, "skill") {
		t.Fatal("una tasca de codi no ha de veure les eines d'Office en mode compacte")
	}
	if !has(office, "office_create") || !has(office, "office_render") || !has(office, "skill") {
		t.Fatal("una tasca d'Office ha de tenir les eines d'Office")
	}
	if !OfficeAlHist([]llm.Message{{Role: "user", Content: "fes un full de càlcul"}}) || OfficeAlHist([]llm.Message{{Role: "user", Content: "refactor the parser"}}) {
		t.Fatal("detecció d'Office")
	}
}

func names(specs []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range specs {
		m[s] = true
	}
	return m
}

func specNames(mode string, compact bool) map[string]bool {
	var out []string
	for _, s := range SpecsFor(mode, compact) {
		out = append(out, s.Name)
	}
	return names(out)
}

func TestSpecsForModes(t *testing.T) {
	code := specNames(ModeCode, false)
	chat := specNames(ModeChat, false)
	if !code["write"] || !code["office_create"] {
		t.Fatal("el mode code ha de tenir eines d'escriptura")
	}
	if chat["write"] || chat["edit"] || chat["office_edit"] || chat["git_commit"] {
		t.Fatal("chat no ha d'ensenyar eines d'escriptura")
	}
	if !chat["read"] || !chat["office_render"] {
		t.Fatal("chat ha de poder llegir i revisar")
	}
	compact := specNames(ModeCode, true)
	if len(compact) >= len(code) || compact["browser"] || compact["gh_pr_create"] || compact["office_create"] || compact["skill"] || !compact["edit"] || !compact["bash"] {
		t.Fatalf("conjunt compacte: %v", compact)
	}
}

func TestPromptForModelCompact(t *testing.T) {
	full := PromptForModel("Ets Gregal.", ModeCode, false)
	short := PromptForModel("Ets Gregal.", ModeCode, true)
	if !strings.Contains(full, "CODE MODE RULES\n") || strings.Contains(full, "(short)") {
		t.Fatal("el prompt complet ha de portar les regles completes")
	}
	if !strings.Contains(short, "CODE MODE RULES (short)") || len(short) > len(full)-1000 {
		t.Fatalf("el compacte ha de ser bastant més curt: %d vs %d", len(short), len(full))
	}
	if PromptForModel("x", ModeChat, true) != PromptFor("x", ModeChat) {
		t.Fatal("el mode chat no canvia")
	}
}
