package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
)

// TestHelperMCPPenjat fa de servidor MCP stdio que no respon mai (el
// llança el test de sota amb el mateix binari de test).
func TestHelperMCPPenjat(t *testing.T) {
	if os.Getenv("GREGAL_TEST_MCP_PENJAT") != "1" {
		t.Skip("només com a subprocés")
	}
	time.Sleep(10 * time.Minute)
}

// Un servidor MCP penjat no pot retardar l'arrencada del TUI: New no
// connecta res, la barra diu «connectant» i, quan un servidor acaba (aquí,
// fallant), en surt l'avís i la pista baixa.
func TestMCPNoBloquejaLArrencada(t *testing.T) {
	cfg := &config.Config{Language: "ca", Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "local", Model: "un-model", ContextWindow: 32768},
			"code": {Provider: "local", Model: "un-model", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"local": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Agent:       config.AgentCfg{MaxSteps: 40},
		MCP: map[string]config.MCPServerCfg{
			"penjat": {Command: os.Args[0], Args: []string{"-test.run=^TestHelperMCPPenjat$"},
				Env: map[string]string{"GREGAL_TEST_MCP_PENJAT": "1"}},
			"mort": {Command: "gregal-no-existeix-mcp"},
		},
	}
	start := time.Now()
	m := New(cfg, "", nil, "vprova")
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("New ha trigat %v: no ha d'esperar els servidors MCP", d)
	}
	defer m.mcp.Close()
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	m = mod.(Model)
	if linia := ultimaLinia(m.View()); !strings.Contains(linia, "connectant 2") {
		t.Fatalf("la barra no diu que connecta:\n%s", linia)
	}
	var mort tea.Cmd
	for _, c := range mcpSetupCmds(m.mcp) {
		mort = c // ordenats: mort, penjat
		break
	}
	msg := mort()
	if r, ok := msg.(mcpReadyMsg); !ok || r.server != "mort" || r.err == nil {
		t.Fatalf("msg=%#v", msg)
	}
	mod, _ = m.Update(msg)
	m = mod.(Model)
	if !strings.Contains(strings.Join(m.lines, "\n"), "✗ mcp mort") {
		t.Fatal("falta l'avís del servidor que no connecta")
	}
	if linia := ultimaLinia(m.View()); !strings.Contains(linia, "connectant 1") {
		t.Fatalf("la pista ha de comptar només el penjat:\n%s", linia)
	}
}
