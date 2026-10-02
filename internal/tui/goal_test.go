package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/llm"
)

func goalTestModel(t *testing.T) Model {
	t.Helper()
	old := gitBranchFn
	t.Cleanup(func() { gitBranchFn = old })
	gitBranchFn = func(string) string { return "main" }
	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "m", ContextWindow: 32768},
			"code": {Provider: "p", Model: "m", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1", APIKey: "k"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, filepath.Join(t.TempDir(), "config.yaml"), llm.New(), "test")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	m = asModel(t, mod)
	return m
}

// El goal passa per l'agent (amb eines de només lectura), no pel xat
// simple sense tools: si no, el model intenta llegir escrivint la crida
// en text i el torn mor.
func TestGoalPassaPerAgent(t *testing.T) {
	m := goalTestModel(t)
	m.mode = agent.ModeGoal
	mod, _ := m.submitText("mira el projecte i concreta")
	mm := asModel(t, mod)
	if !mm.agentActive {
		t.Fatal("en goal el text ha d'engegar l'agent, no el xat sense eines")
	}
}

// El bloc ```goal de la resposta final de l'agent es desa igual que
// abans (ara arriba per agentStepMsg, no per streamDoneMsg).
func TestGoalDesaBlocFinalAgent(t *testing.T) {
	m := goalTestModel(t)
	m.mode = agent.ModeGoal
	m = ambTorn(m, "concreta l'objectiu")
	reply := "Ho tinc.\n\n```goal\ntasca: Fer un videojoc\ncontext: web\ncriteris:\n- 2 nivells\n```\n"
	mod, _ := m.Update(agentStepMsg{content: reply})
	mm := asModel(t, mod)
	if mm.agentActive {
		t.Fatal("la resposta final ha de tancar el torn")
	}
	llista, err := goal.List(mm.goalDir(), mm.projectName())
	if err != nil {
		t.Fatal(err)
	}
	if len(llista) != 1 {
		t.Fatalf("esperava 1 objectiu desat, tinc %d", len(llista))
	}
}

// En goal l'agent és només-lectura: un write del model es denega amb
// missatge i no s'executa (la gràcia del canvi: eines sí, escriptura no).
func TestGoalDenegaEscriptura(t *testing.T) {
	m := goalTestModel(t)
	m.mode = agent.ModeGoal
	m.agentActive = true
	m.torn = m.nouTorn("escriu una cosa")
	prova := filepath.Join(t.TempDir(), "no-s'ha-de-crear.txt")
	call := llm.ToolCall{ID: "c-w1"}
	call.Function.Name = "write"
	call.Function.Arguments = `{"path":` + strconv.Quote(prova) + `,"content":"x"}`
	mod, _ := m.Update(agentStepMsg{content: "Ho escric.", calls: []llm.ToolCall{call}})
	mm := asModel(t, mod)
	if _, err := os.Stat(prova); !os.IsNotExist(err) {
		t.Fatal("en goal no es pot escriure cap fitxer")
	}
	ultim := mm.histAgent()[len(mm.histAgent())-1]
	if ultim.Role != "tool" || !strings.Contains(ultim.Content, "EINA BLOQUEJADA") {
		t.Fatalf("cal denegació a l'historial: %+v", ultim)
	}
}

// En desar-se el bloc s'obre el menú d'accions (fletxes + Enter):
// executar, editar, llistar, esborrar. Sense robar el teclat si
// estaves escrivint.
func TestRecordGoalObreMenuAccions(t *testing.T) {
	m := goalTestModel(t)
	m.mode = agent.ModeGoal
	m = ambTorn(m, "concreta l'objectiu")
	reply := "Fet.\n\n```goal\ntasca: Fer un joc\n```\n"
	mod, _ := m.Update(agentStepMsg{content: reply})
	mm := asModel(t, mod)
	if mm.menu == nil {
		t.Fatal("cal menú d'accions en desar l'objectiu")
	}
	var vals []string
	for _, it := range mm.menu.items {
		vals = append(vals, it.val)
	}
	want := []string{"executa", "mostra", "edita", "llista", "esborra"}
	if strings.Join(vals, ",") != strings.Join(want, ",") {
		t.Fatalf("accions=%v, volia %v", vals, want)
	}
}

// Si estaves escrivint, la targeta no obre cap menú.
func TestRecordGoalNoRobaTeclat(t *testing.T) {
	m := goalTestModel(t)
	m.mode = agent.ModeGoal
	m.input.SetValue("estic escrivint")
	mod, _ := m.Update(agentStepMsg{content: "Fet.\n\n```goal\ntasca: Fer un joc\n```\n"})
	if mod.(Model).menu != nil {
		t.Fatal("amb text al composer no s'ha d'obrir cap menú")
	}
}

// Triar Executa al menú arrenca l'agent de debò (el Cmd arriba via
// deferred): abans es perdia i el torn quedava penjat en busy.
func TestMenuObjectiuExecutaArrenca(t *testing.T) {
	m := goalTestModel(t)
	g, ok := goal.Parse("```goal\ntasca: Fer un joc\n```\n", m.cwd)
	if !ok {
		t.Fatal("no s'ha pogut construir l'objectiu de prova")
	}
	if err := goal.Save(m.goalDir(), g); err != nil {
		t.Fatal(err)
	}
	m.lastGoalID = g.ID
	m.menu = goalActionMenu(g)
	mod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := asModel(t, mod)
	if cmd == nil {
		t.Fatal("l'execució ha d'arribar com a deferred")
	}
	if !mm.agentActive {
		t.Fatal("l'agent s'ha d'haver engegat")
	}
	if mm.menu != nil {
		t.Fatal("el menú s'ha de tancar en triar")
	}
	if mm.mode != agent.ModeCode {
		t.Fatalf("mode=%q, volia code", mm.mode)
	}
	// L'estat del pas (comptador, busy, stream) també ha de tornar al
	// model viu, no només el Cmd: si no, el torn va coix.
	if mm.passosAgent() != 1 || !mm.busy {
		t.Fatalf("estat del pas perdut: steps=%d busy=%v", mm.passosAgent(), mm.busy)
	}
}

// Edita carrega el cos al composer per retocar-lo.
func TestMenuObjectiuEditaCarregaCos(t *testing.T) {
	m := goalTestModel(t)
	g, ok := goal.Parse("```goal\ntasca: Fer un joc\n```\n", m.cwd)
	if !ok {
		t.Fatal("no s'ha pogut construir l'objectiu de prova")
	}
	m.menu = goalActionMenu(g)
	msg, next := m.menu.run(&m, "edita")
	if next != nil {
		t.Fatal("edita no obre submenú")
	}
	if got := m.input.Value(); !strings.Contains(got, "Fer un joc") {
		t.Fatalf("el cos ha d'anar al composer: %q (avís: %q)", got, msg)
	}
}

// El menú /goal també ha de lliurar el Cmd (deferred): abans el
// descartava i triar Executa deixava el torn penjat en busy per sempre.
func TestGoalMenuExecutaNoPenja(t *testing.T) {
	m := goalTestModel(t)
	g, ok := goal.Parse("```goal\ntasca: Fer un joc\n```\n", m.cwd)
	if !ok {
		t.Fatal("no s'ha pogut construir l'objectiu de prova")
	}
	if err := goal.Save(m.goalDir(), g); err != nil {
		t.Fatal(err)
	}
	m.lastGoalID = g.ID
	mn := goalMenu(&m)
	if _, next := mn.run(&m, "executa"); next != nil {
		t.Fatal("executa no obre submenú")
	}
	if m.deferred == nil {
		t.Fatal("el Cmd s'ha de lliurar via deferred o el torn no arrenca mai")
	}
	if !m.agentActive {
		t.Fatal("l'agent s'ha d'haver engegat")
	}
}

// asModel accepta tant Model com *Model (runCommand pot tornar-los tots dos).
func asModel(t *testing.T, x tea.Model) Model {
	t.Helper()
	switch v := x.(type) {
	case Model:
		return v
	case *Model:
		return *v
	}
	t.Fatalf("model inesperat: %T", x)
	return Model{}
}

func pressTab(t *testing.T, m Model) Model {
	t.Helper()
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	return asModel(t, mod)
}

func TestTabCyclesQuatreModes(t *testing.T) {
	m := goalTestModel(t)
	if m.mode != "code" {
		t.Fatalf("mode inicial=%q, volia code", m.mode)
	}
	// Tab ja no canvia de mode sol (provocava caure a "goal" sense eines i
	// semblava que l'agent no acabava). El cicle explícit és Shift+Tab.
	modes := []string{"inspect", "chat", "goal", "code"}
	for i, volia := range modes {
		m.cycleMode()
		if m.mode != volia {
			t.Fatalf("cicle %d=%q, volia %s", i+1, m.mode, volia)
		}
	}
	// I el Tab sense ordre a mitges no toca el mode.
	m2 := goalTestModel(t)
	m2 = pressTab(t, m2)
	if m2.mode != "code" {
		t.Fatalf("Tab sense completar ha canviat el mode a %q (hauria de quedar code)", m2.mode)
	}
}

func TestModeMenuInclouObjectiu(t *testing.T) {
	menu := modeMenu()
	var vals []string
	for _, it := range menu.items {
		vals = append(vals, it.val)
	}
	if len(vals) != 4 || vals[0] != "code" || vals[1] != "inspect" || vals[2] != "chat" || vals[3] != "goal" {
		t.Fatalf("modeMenu=%v", vals)
	}
}

func TestSlashModeObjectiuEsDesa(t *testing.T) {
	m := goalTestModel(t)
	mod, _ := m.runCommand("/mode goal")
	m = asModel(t, mod)
	if m.mode != "goal" {
		t.Fatalf("mode=%q", m.mode)
	}
	if m.cfg.Mode != "goal" {
		t.Fatalf("cfg.Mode=%q", m.cfg.Mode)
	}
	if d := m.goalDir(); d == "" {
		t.Fatal("no s'ha pogut desar: sense directori")
	}
}

func TestObjectiuEsDesaEnRebreElBloc(t *testing.T) {
	m := goalTestModel(t)
	mod, _ := m.runCommand("/goal nou")
	m = asModel(t, mod)

	reply := "Em cal saber una cosa abans.\n\n```goal\ntasca: Afegir scroll al TUI\ncontext: l'historial no es mou\ncriteris:\n- la roda funciona\n```\n"
	mod, _ = m.Update(streamDoneMsg{reply: reply})
	m = asModel(t, mod)

	llista, err := goal.List(m.goalDir(), m.projectName())
	if err != nil {
		t.Fatal(err)
	}
	if len(llista) != 1 {
		t.Fatalf("esperava 1 objectiu desat, tinc %d", len(llista))
	}
	if llista[0].Title != "Afegir scroll al TUI" {
		t.Fatalf("títol=%q", llista[0].Title)
	}
	if m.lastGoalID != llista[0].ID {
		t.Fatalf("lastGoalID=%q, volia %q", m.lastGoalID, llista[0].ID)
	}
}

func TestObjectiuNoEsDesaEnModeCode(t *testing.T) {
	m := goalTestModel(t)
	reply := "```goal\ntasca: No s'havia de desar\n```\n"
	mod, _ := m.Update(streamDoneMsg{reply: reply})
	m = asModel(t, mod)
	llista, _ := goal.List(m.goalDir(), "")
	if len(llista) != 0 {
		t.Fatalf("en mode code no s'hauria de desar cap objectiu: %+v", llista)
	}
}

func TestExecutaUltimObjectiu(t *testing.T) {
	m := goalTestModel(t)
	g, ok := goal.Parse("```goal\ntasca: Afegir scroll al TUI\ncriteris:\n- la roda funciona\n```\n", m.cwd)
	if !ok {
		t.Fatal("no s'ha pogut construir l'objectiu de prova")
	}
	if err := goal.Save(m.goalDir(), g); err != nil {
		t.Fatal(err)
	}
	m.lastGoalID = g.ID

	mod, _ := m.runCommand("/goal executa")
	m = asModel(t, mod)
	if !m.agentActive {
		t.Fatal("l'agent no s'ha engegat")
	}
	if m.mode != "code" {
		t.Fatalf("en executar hauria de passar a mode code, tinc %q", m.mode)
	}
	if len(m.histAgent()) == 0 || !strings.Contains(m.histAgent()[len(m.histAgent())-1].Content, "Afegir scroll al TUI") {
		t.Fatalf("la tasca no conté l'objectiu: %+v", m.histAgent())
	}
}

func TestExecutaSenseObjectiusAvisa(t *testing.T) {
	m := goalTestModel(t)
	mod, _ := m.runCommand("/goal executa")
	m = asModel(t, mod)
	if m.agentActive {
		t.Fatal("no s'hauria d'engegar cap agent sense objectius")
	}
	if !strings.Contains(strings.Join(m.lines, "\n"), "cap objectiu") {
		t.Fatalf("esperava un avís de objectiu inexistent:\n%s", strings.Join(m.lines, "\n"))
	}
}

func TestGoalMenuObreAmbOrdre(t *testing.T) {
	m := goalTestModel(t)
	mod, _ := m.runCommand("/goal")
	m = asModel(t, mod)
	if m.menu == nil {
		t.Fatal("esperava el menú d'objectius")
	}
	if len(m.menu.items) < 4 {
		t.Fatalf("el menú ha de tenir les accions bàsiques: %d", len(m.menu.items))
	}
}

// /goal llista obre el selector interactiu d'objectius per poder-los
// seleccionar, veure detalls, executar o esborrar (igual que a la UI web).
func TestGoalListMenuInteractiu(t *testing.T) {
	m := goalTestModel(t)
	g, ok := goal.Parse("```goal\ntasca: Crear pantalla de login\n```\n", m.cwd)
	if !ok {
		t.Fatal("no s'ha pogut analitzar l'objectiu")
	}
	if err := goal.Save(m.goalDir(), g); err != nil {
		t.Fatal(err)
	}
	mod, _ := m.runCommand("/goal llista")
	m = asModel(t, mod)
	if m.menu == nil {
		t.Fatal("/goal llista ha d'obrir el menú interactiu")
	}
	if !strings.Contains(m.menu.title, "objectius") {
		t.Fatalf("títol menú inesperat: %q", m.menu.title)
	}
	// Seleccionar l'objectiu de la llista l'obre (goalActionMenu)
	_, actionMenu := m.menu.run(&m, g.ID)
	if actionMenu == nil || !strings.Contains(actionMenu.title, g.ID) {
		t.Fatalf("seleccionar l'objectiu ha d'obrir les seves accions: %v", actionMenu)
	}
	// L'acció "mostra" bolca la fitxa completa de l'objectiu
	actionMenu.run(&m, "mostra")
	found := false
	for _, l := range m.lines {
		if strings.Contains(l, "OBJECTIU") && strings.Contains(l, g.ID) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("l'acció 'mostra' hauria d'haver bolcat la targeta d'objectiu")
	}
}
