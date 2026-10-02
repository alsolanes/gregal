package agent

import (
	"gregal/internal/config"
	"os"
	"path/filepath"
	"strings"
)

// memoryFiles són les memòries de projecte reconegudes (primer que existeixi).
var memoryFiles = []string{"AGENTS.md", "CLAUDE.md"}

// MaxMemoryChars topa la memòria injectada al prompt (un AGENTS.md normal
// en fa uns centenars; més enllà és soroll que es menja context).
const MaxMemoryChars = 4000

// LoadProjectMemory llegeix la memòria del projecte: AGENTS.md (o CLAUDE.md,
// autoritativa) més .gregal/memory.md (notes d'equip, via /nota). El topall
// de 4000 caràcters es reparteix: primer AGENTS.md, després notes.
func LoadProjectMemory(cwd string) string {
	dir := strings.TrimSpace(cwd)
	if dir == "" {
		return ""
	}
	var parts []string
	budget := MaxMemoryChars
	for _, name := range memoryFiles {
		p := filepath.Join(dir, name)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(raw))
		if text == "" {
			continue
		}
		text = capRunes(text, budget)
		budget -= len([]rune(text))
		parts = append(parts, "MEMÒRIA DEL PROJECTE ("+name+", autoritativa per a "+dir+"):\n"+text)
		break
	}
	if notes := config.LoadProjectNotes(dir); notes != "" && budget > 200 {
		parts = append(parts, "NOTES DEL PROJECTE (.gregal/memory.md):\n"+capRunes(notes, budget))
	}
	return strings.Join(parts, "\n\n")
}

func capRunes(text string, budget int) string {
	if r := []rune(text); len(r) > budget {
		return string(r[:budget]) + "\n…(retallat)"
	}
	return text
}
