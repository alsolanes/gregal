package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func kanbanTestModel(t *testing.T) Model {
	t.Helper()
	cfg := &config.Config{Language: "en", Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "local", Model: "m", ContextWindow: 32768},
			"code": {Provider: "local", Model: "m", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"local": {BaseURL: "http://127.0.0.1:1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "manual"},
		Agent:       config.AgentCfg{MaxSteps: 5},
	}
	return New(cfg, "", llm.New(), "vtest")
}

func TestKanbanStateOrder(t *testing.T) {
	m := kanbanTestModel(t)
	if got := m.kanbanState(); got != KanbanIdle {
		t.Fatalf("model nou: %s", got)
	}
	m.lastPrompt = "fes x"
	if got := m.kanbanState(); got != KanbanDone {
		t.Fatalf("després d'un torn: %s", got)
	}
	m.busy = true
	if got := m.kanbanState(); got != KanbanWorking {
		t.Fatalf("busy: %s", got)
	}
	m.planning = true
	if got := m.kanbanState(); got != KanbanPlanning {
		t.Fatalf("planning: %s", got)
	}
	m.pendingPlan = "1. a"
	if got := m.kanbanState(); got != KanbanPlanReview {
		t.Fatalf("pla pendent: %s", got)
	}
	m.pendingQ = &questionPending{}
	if got := m.kanbanState(); got != KanbanQuestion {
		t.Fatalf("pregunta: %s", got)
	}
	m.pending = &pendingOp{desc: "bash"}
	if got := m.kanbanState(); got != KanbanApproval {
		t.Fatalf("aprovació: %s", got)
	}
}

func TestWithKanbanWithoutEnvIsPlainModel(t *testing.T) {
	t.Setenv(KanbanStatusEnv, "")
	if _, ok := WithKanban(kanbanTestModel(t)).(Model); !ok {
		t.Fatal("sense la variable, el TUI ha de quedar igual")
	}
}

func TestWithKanbanPublishesStatusAndSendsPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "st", "card.json")
	t.Setenv(KanbanStatusEnv, path)
	t.Setenv(KanbanPromptEnv, "/help")
	km := WithKanban(kanbanTestModel(t))
	km.Init()
	var st KanbanStatus
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &st) != nil || st.State != KanbanIdle || st.PID == 0 {
		t.Fatalf("estat inicial: %s %v", raw, err)
	}
	next, cmd := km.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd == nil {
		t.Fatal("hauria d'encuar la tasca inicial")
	}
	// La tasca s'envia pel camí de l'Enter quan arriba el missatge.
	next, _ = next.Update(kanbanPromptMsg{})
	k := next.(*kanbanModel)
	if !k.sent || k.m.input.Value() != "" {
		t.Fatalf("la tasca no s'ha enviat (input %q)", k.m.input.Value())
	}
}
