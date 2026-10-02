package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// /verify on|off|toggle commuta la verificació automàtica i ho desa.
func TestVerifyToggleIDesat(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.cfgPath = filepath.Join(t.TempDir(), "config.yaml")
	m.cfg.Verify.Mode = "off"
	// Save valida: cal el trio chat/code/reviewer.
	m.cfg.Roles["reviewer"] = m.cfg.Roles["chat"]
	mod, _ := m.runCommand("/verify on")
	mm := mod.(Model)
	if mm.cfg.Verify.Mode != "auto" {
		t.Fatalf("/verify on ha de posar auto: %q", mm.cfg.Verify.Mode)
	}
	mod, _ = mm.runCommand("/verify toggle")
	mm = mod.(Model)
	if mm.cfg.Verify.Mode != "off" {
		t.Fatalf("toggle des d'auto ha de posar off: %q", mm.cfg.Verify.Mode)
	}
	mod, _ = mm.runCommand("/verify toggle")
	mm = mod.(Model)
	if mm.cfg.Verify.Mode != "auto" {
		t.Fatalf("toggle des d'off ha de posar auto: %q", mm.cfg.Verify.Mode)
	}
	raw, err := os.ReadFile(m.cfgPath)
	if err != nil || !strings.Contains(string(raw), "auto") {
		t.Fatalf("el mode s'ha de desar al config: %v", err)
	}
}

// Amb verificació automàtica el cockpit ho diu; apagada, res. Abans era
// una insignia a la barra d'estat, que ja anava plena.
func TestIndicadorVerifAlCockpit(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.cfg.Verify.Mode = "auto"
	if cock := stripANSI(m.cockpitBox(100)); !strings.Contains(cock, "revisor") || !strings.Contains(cock, "auto") {
		t.Fatalf("en auto el cockpit ha de dir el revisor:\n%s", cock)
	}
	if linia := stripANSI(ultimaLinia(m.View())); strings.Contains(linia, "verif") {
		t.Fatalf("la barra ja no porta l'indicador:\n%s", linia)
	}
	m.cfg.Verify.Mode = "off"
	if cock := stripANSI(m.cockpitBox(100)); strings.Contains(cock, "revisor") {
		t.Fatalf("en off no hi ha d'haver revisor:\n%s", cock)
	}
}

// Al mode xat el seleccionable també s'ha d'obrir: el circuit és el
// mateix que a code (ara el prompt també ho diu).
// ambTorn deixa el model amb un torn viu just després del primer pas,
// que és l'estat des del qual arriben els agentStepMsg.
func ambTorn(m Model, tasca string) Model {
	m.agentActive = true
	m.torn = m.nouTorn(tasca)
	m.torn.Seguent()
	return m
}

// ambPregunta deixa el model amb una pregunta oberta de debò: la fa el
// motor, que és qui després rep la resposta. Posar pendingQ a mà deixava
// la pregunta sense ningú a l'altra banda.
func ambPregunta(m Model, opcions ...string) Model {
	m = ambTorn(m, "una tasca")
	var opts []string
	for _, o := range opcions {
		opts = append(opts, `{"label":"`+o+`"}`)
	}
	args := `{"query":"Quin?","options":[` + strings.Join(opts, ",") + `]}`
	m.torn.RepPas("", []llm.ToolCall{crida("q1", "question", args)}, nil)
	mod, _ := m.avanca()
	return mod.(Model)
}

func TestPreguntaObreSeleccionableAlXat(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.mode = agent.ModeChat
	m = ambTorn(m, "una tasca")
	call := llm.ToolCall{ID: "c-qx"}
	call.Function.Name = "question"
	call.Function.Arguments = `{"query":"Quin vols?","options":[{"label":"A"},{"label":"B"}]}`
	mod, _ := m.Update(agentStepMsg{content: "Dubto.", calls: []llm.ToolCall{call}})
	mm := mod.(Model)
	if mm.pendingQ == nil {
		t.Fatal("al xat també cal pendingQ oberta")
	}
	if vista := mm.View(); !strings.Contains(vista, "Quin vols?") {
		t.Fatal("la pregunta s'ha de veure al View també en xat")
	}
}

// Al mode goal el seleccionable també s'ha d'obrir: el circuit és el
// mateix que a code i xat (ara el prompt també ho diu).
func TestPreguntaObreSeleccionableAlGoal(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.mode = agent.ModeGoal
	m = ambTorn(m, "una tasca")
	call := llm.ToolCall{ID: "c-qg"}
	call.Function.Name = "question"
	call.Function.Arguments = `{"query":"Quin gènere?","options":[{"label":"Plataformes"},{"label":"Puzzle"}]}`
	mod, _ := m.Update(agentStepMsg{content: "Concretem.", calls: []llm.ToolCall{call}})
	mm := mod.(Model)
	if mm.pendingQ == nil {
		t.Fatal("al goal també cal pendingQ oberta")
	}
	if vista := mm.View(); !strings.Contains(vista, "Quin gènere?") {
		t.Fatal("la pregunta s'ha de veure al View també en goal")
	}
}

// Quan el model crida l'eina question, el TUI ha d'obrir el seleccionable
// (pregunta + opcions numerades) en comptes de seguir sol.
func TestPreguntaObreSeleccionable(t *testing.T) {
	m := modelDeBarra(t, 100)
	m = ambTorn(m, "una tasca")
	call := llm.ToolCall{ID: "c-q1"}
	call.Function.Name = "question"
	call.Function.Arguments = `{"query":"Quin format vols?","options":[{"label":"Markdown","description":"llegible"},{"label":"JSON","description":"estructurat"}]}`
	mod, _ := m.Update(agentStepMsg{content: "Necessito decidir el format.", calls: []llm.ToolCall{call}})
	mm := mod.(Model)
	if mm.pendingQ == nil {
		t.Fatal("cal pendingQ oberta")
	}
	if mm.pendingQ.query != "Quin format vols?" || len(mm.pendingQ.options) != 2 {
		t.Fatalf("pregunta mal parsejada: %+v", mm.pendingQ)
	}
	if vista := mm.View(); !strings.Contains(vista, "Quin format vols?") || !strings.Contains(vista, "Markdown") {
		t.Fatal("la pregunta s'ha de veure al View")
	}
	// Triar la 2 respon el tool_call i continua el torn.
	mod, _ = mm.answerQuestion(1, "")
	mm = mod.(Model)
	if mm.pendingQ != nil {
		t.Fatal("després de triar no hi ha d'haver pendingQ")
	}
	ultim := mm.histAgent()[len(mm.histAgent())-1]
	if ultim.Role != "tool" || !strings.Contains(ultim.Content, "JSON") {
		t.Fatalf("cal tool_result amb l'opció triada: %+v", ultim)
	}
}

// El model escriu la crida d'eina al text: el torn NO s'acaba (abans sí,
// i l'usuari havia de tornar-ho a demanar a mà); es guia i continua sol.
func TestCridaTextReintentaEstructurada(t *testing.T) {
	m := modelDeBarra(t, 100)
	m = ambTorn(m, "una tasca")
	mod, _ := m.Update(agentStepMsg{content: "Miro el disc.\n<tool_call><function=glob><parameter=pattern>*</parameter></function></tool_call>"})
	mm := mod.(Model)
	if !mm.agentActive {
		t.Fatal("el torn s'hauria de reprendre, no tancar")
	}
	ultim := mm.histAgent()[len(mm.histAgent())-1]
	if ultim.Role != "user" || ultim.Content != agent.GuiaEinaText {
		t.Fatalf("cal guia a l'historial: %+v", ultim)
	}
	if tot := strings.Join(mm.lines, "\n"); !strings.Contains(tot, agent.AvisEinaText) {
		t.Fatal("l'usuari ha de veure què ha passat")
	}
}

// Si insisteix escrivint en text, el topall tanca el torn avisant. Sense
// la crida no queda res de resposta, i el motor demana una síntesi sense
// eines abans de tancar (abans el torn es tancava en blanc).
func TestCridaTextTopallTanca(t *testing.T) {
	m := modelDeBarra(t, 100)
	m = ambTorn(m, "una tasca")
	txt := "<tool_call><function=glob><parameter=pattern>*</parameter></function></tool_call>"
	for i := 0; i < 2; i++ { // gasta els dos reintents
		m.torn.RepPas(txt, nil, nil)
		m.torn.Seguent()
	}
	mod, _ := m.Update(agentStepMsg{content: txt})
	mm := mod.(Model)
	if tot := strings.Join(mm.lines, "\n"); !strings.Contains(tot, agent.AvisEinaText) {
		t.Fatal("l'usuari ha de veure què ha passat")
	}
	mod, _ = mm.Update(agentSintesiMsg{text: "No he pogut cridar l'eina; això és el que sé."})
	mm = mod.(Model)
	if mm.agentActive {
		t.Fatal("després de la síntesi, amb el topall exhaurit, el torn s'ha de tancar")
	}
}

// El model tanca el torn amb passos pendents: no es pot donar per
// acabat (el cas de l'usuari: anuncia què farà i s'atura amb 0/4).
func TestTodosPendentsContinuen(t *testing.T) {
	tools.TodoSet([]tools.TodoItem{{Title: "pas u", Status: "pending"}})
	t.Cleanup(tools.TodoClear)
	m := modelDeBarra(t, 100)
	m = ambTorn(m, "una tasca")
	mod, _ := m.Update(agentStepMsg{content: "Ara ho faig."})
	mm := mod.(Model)
	if !mm.agentActive {
		t.Fatal("amb passos pendents el torn ha de continuar")
	}
	ultim := mm.histAgent()[len(mm.histAgent())-1]
	if ultim.Role != "user" || ultim.Content != agent.GuiaTodosPendents {
		t.Fatalf("cal guia a l'historial: %+v", ultim)
	}
}

// Amb el checklist acabat, la resposta final sí que tanca el torn.
func TestTodosAcabatsTanquen(t *testing.T) {
	tools.TodoSet([]tools.TodoItem{{Title: "pas u", Status: "done"}})
	t.Cleanup(tools.TodoClear)
	m := modelDeBarra(t, 100)
	m = ambTorn(m, "una tasca")
	mod, _ := m.Update(agentStepMsg{content: "Fet."})
	if mod.(Model).agentActive {
		t.Fatal("amb tot fet el torn s'ha de tancar")
	}
}

// I el topall també tanca ( dues guies i prou).
func TestTodosPendentsTopallTanca(t *testing.T) {
	tools.TodoSet([]tools.TodoItem{{Title: "pas u", Status: "pending"}})
	t.Cleanup(tools.TodoClear)
	m := modelDeBarra(t, 100)
	m = ambTorn(m, "una tasca")
	for i := 0; i < 2; i++ { // gasta els dos reintents
		m.torn.RepPas("Ara ho faig.", nil, nil)
		m.torn.Seguent()
	}
	mod, _ := m.Update(agentStepMsg{content: "Ara ho faig."})
	if mod.(Model).agentActive {
		t.Fatal("amb el topall exhaurit el torn s'ha de tancar")
	}
}

// Doble Enter del mateix text mentre treballa: a la cua una sola vegada.
func TestEncuarNoDuplica(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.enqueue("fes-ho")
	m.enqueue("fes-ho")
	if len(m.queued) != 1 {
		t.Fatalf("encuat duplicat: %q", m.queued)
	}
	m.enqueue("una altra cosa")
	if len(m.queued) != 2 {
		t.Fatalf("la cua ha de créixer amb text nou: %q", m.queued)
	}
}

// Amb pregunta oberta i composer buit, les fletxes mouen el ressaltat
// (amb volta) i l'Enter el confirma: no cal escriure el número.
func TestFletxesTriaOpcioPregunta(t *testing.T) {
	nova := func(t *testing.T) Model {
		t.Helper()
		return ambPregunta(modelDeBarra(t, 100), "A", "B", "C")
	}
	m := nova(t)
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mod.(Model)
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mod.(Model)
	if m.pendingQ.sel != 2 {
		t.Fatalf("dues ↓ han de deixar sel=2: %d", m.pendingQ.sel)
	}
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mod.(Model)
	if m.pendingQ.sel != 0 {
		t.Fatalf("al final dóna la volta: %d", m.pendingQ.sel)
	}
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mod.(Model)
	if m.pendingQ.sel != 2 {
		t.Fatalf("↑ des de 0 va a l'última: %d", m.pendingQ.sel)
	}
	if vista := m.View(); !strings.Contains(vista, "▸") {
		t.Fatal("l'opció ressaltada s'ha de marcar amb ▸")
	}
	// Enter buit confirma la ressaltada (C), no renya.
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := mod.(Model)
	if mm.pendingQ != nil {
		t.Fatal("l'Enter ha de resoldre la pregunta")
	}
	ultim := mm.histAgent()[len(mm.histAgent())-1]
	if !strings.Contains(ultim.Content, "C") {
		t.Fatalf("havia de triar C: %+v", ultim)
	}
}

// Amb text escrit, les fletxes són del composer i l'Enter envia text
// lliure: el ressaltat no es mou.
func TestTextLliureNoMouRessaltat(t *testing.T) {
	m := ambPregunta(modelDeBarra(t, 100), "A", "B")
	m.input.SetValue("jo vull D")
	m.fitInput()
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	mm := mod.(Model)
	if mm.pendingQ.sel != 0 {
		t.Fatalf("amb text les fletxes no toquen la tria: %d", mm.pendingQ.sel)
	}
	mod, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = mod.(Model)
	if mm.pendingQ != nil {
		t.Fatal("l'Enter amb text ha de resoldre la pregunta")
	}
	ultim := mm.histAgent()[len(mm.histAgent())-1]
	if !strings.Contains(ultim.Content, "jo vull D") {
		t.Fatalf("havia de ser resposta lliure: %+v", ultim)
	}
}

// L'última fila del seleccionable és el camp de text lliure: sense
// número (no és triable), amb suggeriment si està buit i ressò del que
// escrius si no. I no mou el layout per molt llarg que sigui.
func TestCampLliureRessoIEstable(t *testing.T) {
	q := &questionPending{query: "Quin?", options: []tools.QuestionOption{{Label: "A"}, {Label: "B"}}}
	buida := questionBox(q, 100, "")
	if !strings.Contains(buida, T("pregunta.lliure")) {
		t.Fatalf("buit ha de suggerir escriure:\n%s", buida)
	}
	plena := questionBox(q, 100, "la meva idea")
	if !strings.Contains(plena, "la meva idea") {
		t.Fatalf("ha de ressonar el draft:\n%s", plena)
	}
	llarga := questionBox(q, 100, strings.Repeat("paraula ", 100))
	if lipgloss.Height(llarga) != lipgloss.Height(buida) {
		t.Fatalf("un draft llarg no pot moure l'alçada: %d vs %d",
			lipgloss.Height(llarga), lipgloss.Height(buida))
	}
	// La lliure no porta número: cada opció "N · label" hi és una vegada,
	// i la fila ✎ n'hi ha una de sola.
	for i, o := range q.options {
		marca := fmt.Sprintf("%d · %s", i+1, o.Label)
		if strings.Count(plena, marca) != 1 {
			t.Fatalf("opció %q ha de sortir una vegada:\n%s", marca, plena)
		}
	}
	if strings.Count(plena, "✎") != 1 {
		t.Fatalf("la fila lliure n'hi ha d'haver una de sola:\n%s", plena)
	}
}

// Escrivint amb pregunta oberta, la caixa mostra el draft en directe.
func TestDraftEsVeuAlView(t *testing.T) {
	m := ambTorn(modelDeBarra(t, 100), "una tasca")
	m.pendingQ = &questionPending{query: "Quin?", options: []tools.QuestionOption{{Label: "A"}, {Label: "B"}}}
	m.input.SetValue("jo vull C")
	m.fitInput()
	if vista := m.View(); !strings.Contains(vista, "jo vull C") {
		t.Fatal("el draft s'ha de veure a la caixa de pregunta")
	}
}

// El pressupost i les ampliacions (quan es demana continuar, quan es fa
// síntesi directa, què compta com a progrés) es proven sobre el motor a
// internal/agent/motor_test.go, que no necessita ni terminal ni model.
// Aquí només es comprova el cablejat: que el TUI faci la feina que el
// motor demana i ho digui a la pantalla.
func TestAmpliacioDemanadaEsVeuIContinua(t *testing.T) {
	m := ambTorn(modelDeBarra(t, 100), "una tasca")
	mod, cmd := m.Update(agentExtensioMsg{resp: "CONTINUA 5"})
	mm := mod.(Model)
	if cmd == nil || !mm.agentActive {
		t.Fatal("després de CONTINUA el torn ha de seguir")
	}
	if tot := strings.Join(mm.lines, "\n"); !strings.Contains(tot, "5") {
		t.Fatalf("cal dir quants passos més:\n%s", tot)
	}
}

func TestAmpliacioFinalFaSintesi(t *testing.T) {
	m := ambTorn(modelDeBarra(t, 100), "una tasca")
	mod, cmd := m.Update(agentExtensioMsg{resp: "FINAL"})
	mm := mod.(Model)
	if cmd == nil || !mm.busy || mm.status != T("est.sintesi") {
		t.Fatalf("FINAL ha de fer síntesi: busy=%v status=%q", mm.busy, mm.status)
	}
}

func TestAmpliacioCancelladaAtura(t *testing.T) {
	m := ambTorn(modelDeBarra(t, 100), "una tasca")
	m.busy = true
	m.cancelRequested = true
	mod, _ := m.Update(agentExtensioMsg{resp: "CONTINUA 5"})
	mm := mod.(Model)
	if mm.agentActive || mm.busy {
		t.Fatal("cancel·lat ha d'aturar el torn")
	}
}

// Escriure lletres no mou la conversa: el KeyMap de pager del viewport
// (u/d/k/j/b/f/espai…) està buit i l'scroll va per dreceres explícites.
func TestEscriureNoMouLaConversa(t *testing.T) {
	m := modelDeBarra(t, 100)
	for i := 0; i < 100; i++ {
		m.push("línia de farciment")
	}
	max := m.vp.YOffset
	if max <= 0 {
		t.Fatal("cal contingut amb scroll per provar")
	}
	for _, r := range []rune{'u', 'd', 'k', 'j', 'b', 'f', ' ', 'u'} {
		mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mod.(Model)
		if m.vp.YOffset != max {
			t.Fatalf("escriure %q ha mogut el viewport a %d", r, m.vp.YOffset)
		}
	}
	if got := m.input.Value(); got != "udkjbf u" {
		t.Fatalf("el text ha d'arribar sencer al composer: %q", got)
	}
}

// Ctrl+U/D són mitja pàgina només amb el composer buit; amb text són
// edició del composer i no toquen la lectura.
func TestCtrlUMouNomesEnBuit(t *testing.T) {
	m := modelDeBarra(t, 100)
	for i := 0; i < 100; i++ {
		m.push("línia de farciment")
	}
	max := m.vp.YOffset
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = mod.(Model)
	if m.vp.YOffset >= max {
		t.Fatalf("ctrl+u en buit ha de pujar (YOffset=%d de %d)", m.vp.YOffset, max)
	}
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = mod.(Model)
	if m.vp.YOffset != max {
		t.Fatalf("ctrl+d ha de tornar al final (YOffset=%d de %d)", m.vp.YOffset, max)
	}
	m.input.SetValue("hola")
	m.fitInput()
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	mm := mod.(Model)
	if mm.vp.YOffset != max {
		t.Fatalf("ctrl+u amb text no pot moure la lectura (YOffset=%d)", mm.vp.YOffset)
	}
}

// Tab a mig escriure no pinta res; en buit ensenya la pista.
func TestTabNomesEnsenyaEnBuit(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.input.SetValue("hola")
	m.fitInput()
	n := len(m.lines)
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if len(mod.(Model).lines) != n {
		t.Fatal("Tab amb text no pot afegir línies")
	}
	m2 := modelDeBarra(t, 100)
	n2 := len(m2.lines)
	mod2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	if len(mod2.(Model).lines) != n2+1 {
		t.Fatal("Tab en buit ha d'ensenyar la pista")
	}
}

// Una ordre repescada (comença per "/") no pot segrestar les fletxes:
// mentre es navega, ↑/↓ són sempre de l'historial, mai del popup.
func TestOrdreRepescarNoSegrestaFletxes(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.histEntries = []string{"text vell", "/retry"}
	vols := []struct {
		tecla tea.KeyMsg
		want  string
	}{
		{tea.KeyMsg{Type: tea.KeyUp}, "/retry"},
		{tea.KeyMsg{Type: tea.KeyUp}, "text vell"},
		{tea.KeyMsg{Type: tea.KeyDown}, "/retry"},
		{tea.KeyMsg{Type: tea.KeyDown}, ""},
	}
	for i, c := range vols {
		mod, _ := m.Update(c.tecla)
		m = mod.(Model)
		if got := m.input.Value(); got != c.want {
			t.Fatalf("pas %d: volem %q, tenim %q", i+1, c.want, got)
		}
	}
}

// Sense navegar, escriure "/" continua movent el popup, no l'historial.
func TestBarraOrdreMouPopupNoHistorial(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.histEntries = []string{"text vell"}
	m.input.SetValue("/ret")
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := mod.(Model)
	if got := mm.input.Value(); got != "/ret" {
		t.Fatalf("escrivint /ret la ↓ no ha de repescar: %q", got)
	}
	if mm.histIdx != -1 {
		t.Fatalf("escrivint /ret no s'ha de navegar: idx=%d", mm.histIdx)
	}
}

// La ↑ avança EXACTAMENT una entrada per pulsació, sense salts ni
// repeticions: [A,B,C] → C,B,A,A… i ↓ desfà el camí fins a l'esborrany.
func TestFletxaAvançaDUnaEnUna(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.histEntries = []string{"ordre A", "ordre B", "ordre C"}
	amunt := []string{"ordre C", "ordre B", "ordre A", "ordre A"}
	for i, vol := range amunt {
		mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = mod.(Model)
		if got := m.input.Value(); got != vol {
			t.Fatalf("↑ %d: volem %q, tenim %q", i+1, vol, got)
		}
	}
	avall := []string{"ordre B", "ordre C", ""}
	for i, vol := range avall {
		mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mod.(Model)
		if got := m.input.Value(); got != vol {
			t.Fatalf("↓ %d: volem %q, tenim %q", i+1, vol, got)
		}
	}
}

// Drenar la cua no pot duplicar l'historial: l'Enter ja ho havia desat.
// Abans [A,B] encuats acabaven en [A,B,A] i la ↑ passava per repetits.
func TestDrenarCuaNoDuplicaHistorial(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.histEntries = []string{"tasca A", "tasca B"}
	m.queued = []string{"tasca A", "tasca B"}
	mod, _ := m.drainQueue()
	mm := mod.(Model)
	if len(mm.histEntries) != 2 || mm.histEntries[0] != "tasca A" || mm.histEntries[1] != "tasca B" {
		t.Fatalf("l'historial no s'ha de tocar en drenar: %q", mm.histEntries)
	}
	if len(mm.queued) != 1 || mm.queued[0] != "tasca B" {
		t.Fatalf("n'ha de sortir una de la cua: %q", mm.queued)
	}
}

// Una entrada multilínia no pot encallar la navegació: un cop navegant,
// les fletxes continuen navegant.
func TestMultiliniaNoEncalllaNavegacio(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.histEntries = []string{"ordre A", "línia u\nlínia dos", "ordre C"}
	vols := []string{"ordre C", "línia u\nlínia dos", "ordre A"}
	for i, vol := range vols {
		mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = mod.(Model)
		if got := m.input.Value(); got != vol {
			t.Fatalf("↑ %d: volem %q, tenim %q", i+1, vol, got)
		}
	}
}

// La ↑ ha de repescar l'entrada anterior també mentre l'agent treballa:
// el que s'escrigui llavors s'encua, i sense historial no hi ha res a encuar.
func TestFletxaAmuntFuncionaMentreTreballa(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.histEntries = []string{"primera ordre", "segona ordre"}
	m.busy = true
	m.agentActive = true
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	mm := mod.(Model)
	if got := mm.input.Value(); got != "segona ordre" {
		t.Fatalf("↑ en feina ha de mostrar l'última entrada, tenim %q", got)
	}
}

// Amb text multilínia la ↑ és del cursor, no de l'historial.
func TestFletxaAmuntNoTocaHistorialMultilinia(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.histEntries = []string{"ordre vella"}
	m.input.SetValue("línia u\nlínia dos")
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	mm := mod.(Model)
	if got := mm.input.Value(); got != "línia u\nlínia dos" {
		t.Fatalf("↑ multilínia no ha de repescar historial, tenim %q", got)
	}
}

// /retry sense res previ ho ha de dir, no penjar-se.
func TestRetrySenseHistorialHoDiu(t *testing.T) {
	m := modelDeBarra(t, 100)
	mod, _ := m.runCommand("/retry")
	mm := mod.(Model)
	ultima := mm.lines[len(mm.lines)-1]
	if !strings.Contains(ultima, T("app.resRetry")) {
		t.Fatalf("sense lastPrompt cal avís, tenim %q", ultima)
	}
}

// /retry no duplica l'usuari al transcript (via cortesia local, sense model).
func TestRetryNoDuplicaUsuari(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.lastPrompt = "hola"
	m.convo = []llm.Message{{Role: "user", Content: "hola"}}
	mod, _ := m.retryLast()
	mm := mod.(Model)
	users := 0
	for _, msg := range mm.convo {
		if msg.Role == "user" {
			users++
		}
	}
	if users != 1 {
		t.Fatalf("l'usuari ha de sortir una sola vegada, en surt %d: %+v", users, mm.convo)
	}
	if len(mm.convo) != 2 || mm.convo[1].Role != "assistant" {
		t.Fatalf("cal resposta local de cortesia: %+v", mm.convo)
	}
}

// El text que arriba es pinta com a markdown, no com a cua retallada:
// el que el model escriu es llegeix des del primer moment. El que no pot
// passar és que hi quedi el rail de feina (el text no és feina, és la
// resposta) ni el marcador ◌ quan ja hi ha text.
func TestBlocEnViuEsMarkdown(t *testing.T) {
	m := modelDeBarra(t, 100)
	s := &streamer{}
	m.stream = s
	m.push(workRail(dimStyle.Render("◌ " + T("est.escrivint"))))
	m.streamLine = len(m.lines) - 1
	s.add("## Pla\n\n1. Llegir el config\n2. Afegir el camp\n")
	mod, _ := m.Update(streamTickMsg{})
	mm := mod.(Model)
	viu := stripANSI(mm.lines[mm.streamLine])
	for _, vol := range []string{"Pla", "Llegir el config", "Afegir el camp"} {
		if !strings.Contains(viu, vol) {
			t.Fatalf("el text en viu ha de dir %q: %q", vol, viu)
		}
	}
	if strings.Contains(viu, "│") || strings.Contains(viu, "◌") {
		t.Fatalf("el text del model no és feina: sense rail ni ◌: %q", viu)
	}
	if strings.Contains(viu, "##") {
		t.Fatalf("els títols s'han de pintar, no sortir en cru: %q", viu)
	}
}

// El pas d'agent sense text no deixa mai la línia en blanc.
func TestPasAgentSenseTextNoDeixaForat(t *testing.T) {
	m := modelDeBarra(t, 100)
	m.push(workRail(dimStyle.Render("◌ " + T("est.escrivint"))))
	m.streamLine = len(m.lines) - 1
	m = ambTorn(m, "una tasca")
	call := llm.ToolCall{ID: "c-bloquejada"}
	call.Function.Name = "bash"
	call.Function.Arguments = `{"command":"rm -rf /"}`
	mod, _ := m.Update(agentStepMsg{content: "", calls: []llm.ToolCall{call}})
	mm := mod.(Model)
	if strings.TrimSpace(mm.lines[mm.streamLine]) == "" {
		t.Fatal("la línia en viu no pot quedar buida")
	}
}
