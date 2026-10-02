package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func planTestModel(t *testing.T) Model {
	t.Helper()
	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "model-prova", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "manual"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "vtest")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	return mod.(Model)
}

func TestPlanSysPromptEsReadOnlyIMemoria(t *testing.T) {
	m := planTestModel(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Regla d'or."), 0o600); err != nil {
		t.Fatal(err)
	}
	m.cwd = dir
	p := m.planSysPrompt()
	for _, want := range []string{"mode PLA", "NO cridis mai write", "Regla d'or.", dir} {
		if !strings.Contains(p, want) {
			t.Fatalf("planSysPrompt sense %q:\n%s", want, p)
		}
	}
}

func TestFinishPlanPresentaIExecuta(t *testing.T) {
	m := planTestModel(t)
	m.planning = true
	mod, _ := m.Update(agentStepMsg{content: "1. Fer X\nVERIFICACIÓ: pytest"})
	m = mod.(Model)
	if m.planning {
		t.Fatal("finishPlan hauria d'aturar l'exploració")
	}
	if m.pendingPlan == "" {
		t.Fatal("cal pla pendent")
	}
	if !strings.Contains(m.status, "[e]xecuta") {
		t.Fatalf("status sense accions: %q", m.status)
	}
	// "e" executa el pla com a agent.
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = mod.(Model)
	if !m.agentActive || m.pendingPlan != "" {
		t.Fatalf("e hauria d'engegar l'agent: active=%v pendent=%q", m.agentActive, m.pendingPlan)
	}
}

func TestDescartaPla(t *testing.T) {
	m := planTestModel(t)
	m.planning = true
	mod, _ := m.Update(agentStepMsg{content: "1. Fer X"})
	m = mod.(Model)
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = mod.(Model)
	if m.pendingPlan != "" || m.agentActive {
		t.Fatal("d hauria de descartar sense actuar")
	}
}

func TestPlanSenseTascaAvisa(t *testing.T) {
	m := planTestModel(t)
	mod, _ := m.startPlan("   ")
	m = mod.(Model)
	if m.planning {
		t.Fatal("sense tasca no hi ha exploració")
	}
}
