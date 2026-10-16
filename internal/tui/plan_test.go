package tui

import (
	"errors"
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
	for _, want := range []string{"PLAN MODE", "NEVER call write", "Regla d'or.", dir, "LANGUAGE: Respon sempre"} {
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

// En esgotar el pressupost, el pla no s'atura en sec: demana la resposta
// final sense eines, la presenta com a PLA PARCIAL i [c] continua amb la
// conversa intacta (tirada de 2026-10-16: «límit de 10 passos exhaurit»).
func TestPlanEsgotatDonaPlaParcialIContinua(t *testing.T) {
	m := planTestModel(t)
	m.planning = true
	m.planHist = []llm.Message{{Role: "user", Content: "revisa a fons el projecte"}}
	m.planSteps = m.planLimit()
	var c llm.ToolCall
	c.ID, c.Function.Name, c.Function.Arguments = "c1", "read", `{"path":"x.go"}`
	m.planHist = append(m.planHist, llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{c}})
	mod, cmd := m.Update(agentExecMsg{results: []toolRes{{call: c, out: "contingut"}}})
	m = mod.(Model)
	if !m.planning || !m.planFinal || cmd == nil {
		t.Fatalf("en esgotar els passos cal la crida final: planning=%v final=%v", m.planning, m.planFinal)
	}
	mod, _ = m.Update(agentSintesiMsg{text: "1. Revisar loop.go\nPENDENT: web/"})
	m = mod.(Model)
	if m.planning || !m.planParcial || !strings.Contains(m.pendingPlan, "Revisar loop.go") {
		t.Fatalf("pla parcial esperat: planning=%v parcial=%v pla=%q", m.planning, m.planParcial, m.pendingPlan)
	}
	if !strings.Contains(m.status, "[c]ontinua") {
		t.Fatalf("status sense continuar: %q", m.status)
	}
	if !strings.Contains(strings.Join(m.lines, "\n"), "PLA PARCIAL") {
		t.Fatal("la targeta ha de dir que el pla és parcial")
	}
	mod, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = mod.(Model)
	if !m.planning || m.planSteps != 1 || m.planParcial || m.pendingPlan != "" || cmd == nil {
		t.Fatalf("c ha de reprendre l'exploració: planning=%v passos=%d", m.planning, m.planSteps)
	}
	if m.planHist[0].Content != "revisa a fons el projecte" {
		t.Fatal("continuar conserva la conversa")
	}
	if last := m.planHist[len(m.planHist)-1]; last.Role != "user" || !strings.Contains(last.Content, "Continue the exploration") {
		t.Fatalf("falta la consigna de continuar: %+v", last)
	}
}

func TestPlanSintesiFallidaDonaResumMecanic(t *testing.T) {
	m := planTestModel(t)
	m.planning, m.planFinal, m.planMotiu = true, true, "steps"
	var c llm.ToolCall
	c.ID, c.Function.Name, c.Function.Arguments = "c1", "grep", `{"pattern":"TODO"}`
	m.planHist = []llm.Message{{Role: "user", Content: "t"}, {Role: "assistant", ToolCalls: []llm.ToolCall{c}}}
	mod, _ := m.Update(agentSintesiMsg{err: errors.New("proveïdor caigut")})
	m = mod.(Model)
	if m.planning || !m.planParcial || !strings.Contains(m.pendingPlan, "grep") {
		t.Fatalf("resum mecànic esperat: %q", m.pendingPlan)
	}
}

func TestPlanErrorAMigCamiDemanaRespostaFinal(t *testing.T) {
	m := planTestModel(t)
	m.planning = true
	m.planSteps = 3
	m.planRetries = 2
	m.planHist = []llm.Message{{Role: "user", Content: "t"}}
	mod, cmd := m.Update(agentStepMsg{err: errors.New("model inactiu")})
	m = mod.(Model)
	if !m.planning || !m.planFinal || cmd == nil || !strings.Contains(m.planMotiu, "model inactiu") {
		t.Fatalf("amb exploració feta, l'error porta a la resposta final: final=%v motiu=%q", m.planFinal, m.planMotiu)
	}
}
