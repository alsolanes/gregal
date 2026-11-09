package tui

import (
	"fmt"
	"strings"
	"testing"

	"gregal/internal/config"
)

// Les notes que el model decideix ja són al fitxer; el TUI només ho diu.
func TestAsideReportsNotesSavedByTheModel(t *testing.T) {
	m := planTestModel(t)
	m.cwd = t.TempDir()
	mod, _ := m.Update(asideMsg{notes: []string{"Run go vet first", "Keep CA/EN keys identical"}})
	m = mod.(Model)
	want := fmt.Sprintf(T("mem.desades"), 2, config.ProjectMemoryPath(m.cwd))
	if !strings.Contains(strings.Join(m.lines, "\n"), want) {
		t.Fatalf("lines=%q", strings.Join(m.lines, "\n"))
	}
}

// /btw amb el torn en marxa no s'encua: es respon al marge.
func TestBtwWhileBusyIsNotQueued(t *testing.T) {
	m := planTestModel(t)
	m.busy = true
	mod, cmd := m.btwCmd("what now?")
	if cmd == nil || len(mod.(Model).queued) != 0 {
		t.Fatalf("btw must answer on the side (cmd=%v queued=%v)", cmd != nil, mod.(Model).queued)
	}
	if out := m.agentsList(); out == "" {
		t.Fatal("agents list must say something")
	}
}
