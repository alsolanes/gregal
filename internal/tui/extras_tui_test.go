package tui

import (
	"strings"
	"testing"

	"gregal/internal/config"
)

// Les notes proposades arriben com a asideMsg i només /keep les escriu.
func TestKeepWritesOnlyProposedNotes(t *testing.T) {
	m := planTestModel(t)
	m.cwd = t.TempDir()
	mod, _ := m.Update(asideMsg{notes: []string{"Run go vet first", "Keep CA/EN keys identical"}})
	m = mod.(Model)
	if len(config.Notes(m.cwd, "")) != 0 {
		t.Fatal("a proposal must not write by itself")
	}
	mod, _ = m.runCommand("/keep 2")
	m = mod.(Model)
	notes := config.Notes(m.cwd, "")
	if len(notes) != 1 || !strings.Contains(notes[0], "Keep CA/EN keys identical") || m.memNotes != nil {
		t.Fatalf("notes=%q pending=%v", notes, m.memNotes)
	}
	mod, _ = m.runCommand("/keep")
	if !strings.Contains(strings.Join(mod.(Model).lines, "\n"), T("mem.capProposta")) {
		t.Fatal("/keep without a proposal says so")
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
