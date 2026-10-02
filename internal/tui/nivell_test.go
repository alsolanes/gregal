package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// crida munta una tool_call per als tests.
func crida(id, name, args string) llm.ToolCall {
	var c llm.ToolCall
	c.ID = id
	c.Function.Name = name
	c.Function.Arguments = args
	return c
}

// nomesText treu els codis de color per poder comparar.
func nomesText(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestUnaFilaPerEina(t *testing.T) {
	m := planTestModel(t)
	abans := len(m.lines)
	m.pushToolCall("read", `{"path":"x.go"}`)
	if len(m.lines) != abans+1 {
		t.Fatalf("la crida ha d'ocupar una fila: %d", len(m.lines)-abans)
	}
	m.pushToolResult("read", "llegit x.go\n1\tpackage x", false)
	if len(m.lines) != abans+1 {
		t.Fatalf("el resultat d'una lectura ha de reescriure la mateixa fila, no afegir-ne: %d files", len(m.lines)-abans)
	}
	fila := nomesText(m.lines[abans])
	for _, vol := range []string{"✓", "read", "x.go"} {
		if !strings.Contains(fila, vol) {
			t.Fatalf("la fila hauria de dir %q: %q", vol, fila)
		}
	}
	// Un error sí que ensenya el cos: és el que es vol llegir.
	m.pushToolCall("bash", `{"command":"go test ./..."}`)
	n := len(m.lines)
	m.pushToolResult("bash", "ERROR: exit status 1\nundefined: X", true)
	if len(m.lines) <= n {
		t.Fatal("un error ha de deixar veure la sortida")
	}
	if !strings.Contains(nomesText(m.lines[n-1]), "✗") {
		t.Fatalf("la fila de l'eina fallida ha de dur ✗: %q", nomesText(m.lines[n-1]))
	}
}

func TestBenvingudaMarxaAlPrimerMissatge(t *testing.T) {
	m := planTestModel(t)
	if m.benvingudaN == 0 {
		t.Fatal("la benvinguda s'hauria d'haver marcat")
	}
	if !strings.Contains(nomesText(strings.Join(m.lines, "\n")), "GREGAL") {
		t.Fatal("la benvinguda hauria de ser a la pantalla")
	}
	mod, _ := m.runCommand("/help")
	m = mod.(Model)
	if m.benvingudaN != 0 {
		t.Fatal("la marca s'ha de netejar")
	}
	if strings.Contains(nomesText(strings.Join(m.lines, "\n")), "Preparat per treballar") {
		t.Fatal("la targeta de benvinguda no pot quedar-se després del primer missatge")
	}
}

func TestAprovacioEnsenyaElDiffAbansDeDecidir(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "x.go")
	if err := os.WriteFile(ruta, []byte("package x\n\nconst Timeout = 180\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := planTestModel(t)
	m.cwd = dir
	// Sense ProjectDir, escriure demana permís (que és el que es prova).
	m.policy = &agent.Policy{}
	m.agentActive = true
	m.torn = m.nouTorn("canvia el timeout")
	m.torn.Seguent() // primer pas
	m.torn.RepPas("", []llm.ToolCall{crida("1", "edit", `{"path":"x.go","old_string":"const Timeout = 180","new_string":"const Timeout = 240"}`)}, nil)
	mod, _ := m.avanca()
	m = mod.(Model)
	if m.pending == nil {
		t.Fatal("hauria de demanar confirmació")
	}
	prev := nomesText(m.pending.preview)
	if !strings.Contains(prev, "240") || !strings.Contains(prev, "180") {
		t.Fatalf("la vista prèvia ha de dur el diff: %q", prev)
	}
	if strings.Contains(m.pending.desc, "old_string") {
		t.Fatalf("la descripció no ha de ser el JSON cru: %q", m.pending.desc)
	}
	// Tres botons quan és l'agent qui demana; dos per a una ordre manual.
	if n := len(m.botonsConfirm()); n != 3 {
		t.Fatalf("amb l'agent actiu calen 3 botons (permet/sempre/denega): %d", n)
	}
	m.agentActive = false
	if n := len(m.botonsConfirm()); n != 2 {
		t.Fatalf("sense agent, «sempre» no té sentit: %d botons", n)
	}
}

func TestAprovacioNoTeCompteEnrere(t *testing.T) {
	// El TUI no denega sol: la web sí, i aquí la conseqüència era trobar
	// l'edició rebutjada per haver-se aixecat de la cadira.
	m := planTestModel(t)
	m.pending = &pendingOp{desc: "prova", run: func(ctx context.Context) (string, error) { return "", nil }}
	for i := 0; i < 4; i++ {
		mod, _ := m.Update(streamTickMsg{})
		m = mod.(Model)
	}
	if m.pending == nil {
		t.Fatal("l'aprovació ha d'esperar indefinidament")
	}
}

func TestCtrlCAturaAbansDeSortir(t *testing.T) {
	m := planTestModel(t)
	aturat := false
	m.cancel = func() { aturat = true }
	mod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = mod.(Model)
	if !aturat {
		t.Fatal("el primer Ctrl+C ha d'aturar la feina")
	}
	if cmd != nil {
		t.Fatal("el primer Ctrl+C no pot tancar el programa")
	}
	// El segon sí que surt, encara que la cancel·lació segueixi en marxa.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("el segon Ctrl+C ha de sortir")
	}
}

func TestClauNoApareixEnClar(t *testing.T) {
	m := planTestModel(t)
	m.pendingKeyFor = "p"
	for _, r := range "sk-secret-123" {
		mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mod.(Model)
	}
	if m.keyBuf != "sk-secret-123" {
		t.Fatalf("la clau s'ha d'acumular a keyBuf: %q", m.keyBuf)
	}
	if strings.Contains(m.input.Value(), "secret") {
		t.Fatal("la clau no pot entrar al composer")
	}
	pantalla := nomesText(m.View())
	if strings.Contains(pantalla, "sk-secret") {
		t.Fatal("la clau no pot sortir a la pantalla")
	}
	if !strings.Contains(pantalla, strings.Repeat("•", len("sk-secret-123"))) {
		t.Fatal("hauria de pintar rodonetes")
	}
	for _, h := range m.histEntries {
		if strings.Contains(h, "secret") {
			t.Fatal("la clau no pot anar a l'historial")
		}
	}
}

func TestMenuFiltraEscrivint(t *testing.T) {
	mn := &menuState{items: []menuItem{
		{text: "gpt-4o-mini", val: "a"},
		{text: "claude-sonnet", val: "b"},
		{text: "gpt-5", val: "c"},
	}}
	m := planTestModel(t)
	m.menu = mn
	for _, k := range []string{"g", "p", "t"} {
		m.menuKeys(k)
	}
	if n := len(m.menu.visibles()); n != 2 {
		t.Fatalf("«gpt» hauria de deixar 2 files: %d", n)
	}
	m.menuKeys("esc")
	if m.menu == nil || m.menu.filtre != "" {
		t.Fatal("el primer Esc neteja el filtre, no tanca el menú")
	}
}

func TestTopallAutonomTambeAlTUI(t *testing.T) {
	cfg := &config.Config{Language: "ca", Agent: config.AgentCfg{Autonomous: config.AutonomousCfg{MaxToolSteps: 3, MaxMinutes: 60}}}
	if motiu := agent.TopallAutonom(cfg, agent.EstatAutonom{Execs: 3, Inici: time.Now()}); motiu == "" {
		t.Fatal("hauria d'aturar-se en arribar al límit d'eines")
	}
	if motiu := agent.TopallAutonom(cfg, agent.EstatAutonom{Execs: 1, Inici: time.Now().Add(-2 * time.Hour)}); motiu == "" {
		t.Fatal("hauria d'aturar-se passat el límit de temps")
	}
	if motiu := agent.TopallAutonom(cfg, agent.EstatAutonom{Execs: 1, Inici: time.Now()}); motiu != "" {
		t.Fatalf("no hauria d'aturar-se: %q", motiu)
	}
}

func TestRegistreDEinesCoherent(t *testing.T) {
	// El config ha d'acceptar una política per a qualsevol eina que
	// l'agent sàpiga executar. Abans la llista del config era més curta i
	// «permissions.tools.office_edit: deny» no deixava arrencar.
	for _, nom := range tools.Configurables() {
		cfg := &config.Config{Language: "ca",
			Mode:        "code",
			Roles:       map[string]config.Role{"chat": {Provider: "p", Model: "m"}, "code": {Provider: "p", Model: "m"}, "reviewer": {Provider: "p", Model: "m"}},
			Providers:   map[string]config.Provider{"p": {BaseURL: "http://x"}},
			Verify:      config.VerifyCfg{Mode: "off"},
			Agent:       config.AgentCfg{MaxSteps: 10},
			Permissions: config.PermissionsCfg{Tools: map[string]string{nom: "deny"}},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("el config hauria d'acceptar %s: %v", nom, err)
		}
		if d, _ := agent.DefaultPolicy().For(nom, "{}"); d == "deny" && nom != "" {
			// Només comprovem que la política la reconegui (no «eina desconeguda»).
			if _, motiu := agent.DefaultPolicy().For(nom, "{}"); strings.Contains(motiu, "desconeguda") {
				t.Fatalf("la política no reconeix %s", nom)
			}
		}
	}
}
