package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

func modelDeBarra(t *testing.T, ample int) Model {
	t.Helper()
	old := gitBranchFn
	t.Cleanup(func() { gitBranchFn = old })
	gitBranchFn = func(string) string { return "main" }
	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "local", Model: "un-model", ContextWindow: 32768},
			"code": {Provider: "local", Model: "un-model", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"local": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "manual"},
		Agent:       config.AgentCfg{MaxSteps: 40},
	}
	m := New(cfg, "", llm.New(), "vprova")
	m.cwd = filepath.Join("C", "projecte")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: ample, Height: 30})
	mm := mod.(Model)
	mm.usedTokens, mm.tokUp, mm.tokDown = 9200, 7100, 2100
	// El numerador de la barra surt de promptEst (context actual), no de
	// l'acumulat de sessió: per defecte, el posem igual perquè la barra diu
	// el que mostrava abans.
	mm.promptEst = 9200
	return mm
}

// ultimaLinia és la barra d'estat: l'última línia pintada.
func ultimaLinia(vista string) string {
	l := strings.Split(strings.TrimRight(vista, "\n"), "\n")
	return l[len(l)-1]
}

// Les pistes de teclat són ajuda fixa; el mesurador de context és
// informació viva. S'afegien primer, i a 110 columnes —una amplada de
// terminal de les normals— el context desapareixia perquè les pistes li
// havien pres el lloc: perdies el que canvia per guanyar el que sempre diu
// el mateix.
func TestElContextNoCedeixElLlocALesPistes(t *testing.T) {
	for _, ample := range []int{90, 100, 110, 120, 130, 160} {
		linia := ultimaLinia(modelDeBarra(t, ample).View())
		if !strings.Contains(linia, "ctx ") {
			t.Errorf("a %d columnes la barra es queda sense context:\n%s", ample, linia)
		}
	}
}

// La barra no pot ser mai més ampla que el terminal: el terminal
// l'embolcallaria i tota la pantalla saltaria una fila.
func TestLaBarraNoDesbordaMai(t *testing.T) {
	for ample := 40; ample <= 200; ample += 7 {
		m := modelDeBarra(t, ample)
		limit := m.vp.Width
		if m.columnaActiva() {
			limit += ampleColumna // la darrera fila porta la columna del cockpit
		}
		if got := lipgloss.Width(ultimaLinia(m.View())); got > limit {
			t.Errorf("a %d columnes (vp %d) la barra en fa %d", ample, m.vp.Width, got)
		}
	}
}

// El cadenat diu la postura de permisos d'un cop d'ull: tancat per
// defecte (cada cosa delicada demana permís), obert en permissiu (fa sol).
func TestCadenatPermissosALaBarra(t *testing.T) {
	m := modelDeBarra(t, 100)
	if linia := ultimaLinia(m.View()); !strings.Contains(linia, "■") || strings.Contains(linia, "□") {
		t.Fatalf("per defecte cadenat tancat:\n%s", linia)
	}
	m.permissive = true
	if linia := ultimaLinia(m.View()); !strings.Contains(linia, "□") || strings.Contains(linia, "■") {
		t.Fatalf("en permissiu cadenat obert:\n%s", linia)
	}
}

// Escriure no pot moure el text anterior: la vista ha de fer sempre les
// mateixes files, tant amb el composer d'una línia com multilínia,
// amb popup de suggeriments o sense, i amb pregunta/todos oberts. Abans
// el viewport no cedia el creixement del composer ni l'embolcall dels
// blocs, el total desbordava el terminal i tot ballava a cada tecla.
func TestLaVistaNoCanviaDAlcadaEnEscriure(t *testing.T) {
	estats := map[string]func(*Model){
		"buit":        func(m *Model) {},
		"text":        func(m *Model) { m.input.SetValue("hola, què tal"); m.fitInput() },
		"suggeriment": func(m *Model) { m.input.SetValue("/ver"); m.fitInput() },
		"ordre":       func(m *Model) { m.input.SetValue("/verify mode auto"); m.fitInput() },
		"multilinia":  func(m *Model) { m.input.SetValue("u\ndos\ntres\nquatre\ncinc"); m.fitInput() },
		"llarga":      func(m *Model) { m.input.SetValue(strings.Repeat("paraula ", 40)); m.fitInput() },
		"todos":       func(m *Model) { m.showTodos = true },
		"pregunta": func(m *Model) {
			m.pendingQ = &questionPending{query: "Quin?", options: []tools.QuestionOption{{Label: "A"}, {Label: "B"}}}
		},
		"feina": func(m *Model) { m.busy = true; m.status = "treballant…" },
	}
	var base int
	for nom, aplica := range estats {
		m := modelDeBarra(t, 100)
		aplica(&m)
		h := lipgloss.Height(m.View())
		if base == 0 {
			base = h
		}
		if h != base {
			t.Errorf("estat %q: la vista fa %d files, no %d", nom, h, base)
		}
	}
}

// Els diàlegs (pregunta, menú) fan 72 columnes o el que hi cap, i tots
// la mateixa; la checklist, que viu apilada sobre el composer, fa el que
// fa el composer.
func TestElsDialegsFanLaMateixaAmplada(t *testing.T) {
	for _, ample := range []int{60, 90, 110, 160} {
		m := modelDeBarra(t, ample)
		vol := ampleDialeg(m.vp.Width)
		if vol > 72 || vol > m.vp.Width-4 {
			t.Fatalf("a %d columnes el diàleg fa %d", ample, vol)
		}
		q := &questionPending{query: "Quin format?", options: []tools.QuestionOption{{Label: "A"}, {Label: "B"}}}
		if got := lipgloss.Width(strings.Split(questionBox(q, m.vp.Width, ""), "\n")[0]); got != vol {
			t.Errorf("a %d columnes la pregunta fa %d i no %d", ample, got, vol)
		}
		mn := &menuState{title: "tria", items: []menuItem{{text: "a"}, {text: "b"}}}
		if got := lipgloss.Width(strings.Split(menuLine(mn, m.vp.Width), "\n")[0]); got != vol {
			t.Errorf("a %d columnes el menú fa %d i no %d", ample, got, vol)
		}
		composer := lipgloss.Width(strings.Split(m.inputBox(), "\n")[0])
		tools.TodoSet([]tools.TodoItem{{Title: "una", Status: "pending"}})
		if got := lipgloss.Width(strings.Split(todosBox(m.vp.Width), "\n")[0]); got != composer {
			t.Errorf("a %d columnes la checklist fa %d i el composer %d", ample, got, composer)
		}
		tools.TodoClear()
	}
}

// La vista ha d'ocupar exactament l'alçada del terminal. Si en sobra, la
// barra d'estat (i el composer) queden penjats a mig camí i el cursor
// balla a sota; si en falta, el terminal fa scroll i tot salta. Abans
// vpSize reservava 11 files quan la vista n'usa 9: en sobraven dues.
func TestLaVistaArribaFinsAlFinalDeLaPantalla(t *testing.T) {
	for _, h := range []int{20, 24, 30, 40, 55} {
		m := modelDeBarra(t, 110)
		mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: h})
		mm := mod.(Model)
		if got := lipgloss.Height(mm.View()); got != h {
			t.Errorf("alçada %d: la vista fa %d files, no %d", h, got, h)
		}
	}
}

// La barra diu el context com a percentatge del prompt ACTUAL, i prou:
// els comptadors (↑↓, acumulat, cost) viuen a /stats. Abans la barra deia
// "ctx 350k/65.5k ↑… ↓… ~$…" i no es llegia res.
func TestLaBarraDiuElContextEnPercentatge(t *testing.T) {
	m := modelDeBarra(t, 110)
	m.usedTokens = 999_000 // acumulat de sessió: no ha de sortir a la barra
	m.promptEst = 12_000   // sobre 32.768 de finestra → 36 %
	linia := stripANSI(ultimaLinia(m.View()))
	if !strings.Contains(linia, "ctx 36%") {
		t.Fatalf("cal el percentatge del prompt actual:\n%s", linia)
	}
	for _, no := range []string{"999k", "↑", "↓", "12k", "32.8k"} {
		if strings.Contains(linia, no) {
			t.Fatalf("%q no va a la barra (és a /stats):\n%s", no, linia)
		}
	}
	if stats := stripANSI(m.statsBlock()); !strings.Contains(stats, "12k") {
		t.Fatalf("el prompt actual ha de ser a /stats:\n%s", stats)
	}
}

// Quatre segments i prou: mode, context, estat i permisos. Abans n'hi
// havia nou més tres jocs de pistes, i a 110 columnes el context
// desapareixia perquè les pistes li prenien el lloc.
func TestLaBarraTeQuatreSegments(t *testing.T) {
	for _, ample := range []int{80, 100, 132, 170} {
		linia := stripANSI(ultimaLinia(modelDeBarra(t, ample).View()))
		for _, vol := range []string{"CODE", "ctx ", "llest", "■"} {
			if !strings.Contains(linia, vol) {
				t.Errorf("a %d columnes falta %q:\n%s", ample, vol, linia)
			}
		}
		for _, no := range []string{"^T", "^C", "⇧Tab", "historial", "verif"} {
			if strings.Contains(linia, no) {
				t.Errorf("a %d columnes la barra no porta pistes (%q):\n%s", ample, no, linia)
			}
		}
	}
}

// /stats ha de dir d'on surt la finestra (config/model/defecte) perquè
// "65.5k" no sembli una constant misteriosa.
func TestStatsMostraLaFontDeLaFinestra(t *testing.T) {
	m := modelDeBarra(t, 110) // els rols de prova fixen ContextWindow 32768
	out := stripANSI(m.statsBlock())
	if !strings.Contains(out, "finestra") || !strings.Contains(out, "config") {
		t.Fatalf("cal la finestra i la font:\n%s", out)
	}
	if !strings.Contains(out, "prompt útil") {
		t.Fatalf("cal el pressupost de prompt:\n%s", out)
	}
}

// La carpeta de treball ha de ser a la barra: és l'única cosa que diu on
// treballa l'agent. Amb el camí llarg només hi cap el nom del projecte;
// amb la barra ampla, el camí sencer.
func TestLaBarraDiuLaCarpetaDeTreball(t *testing.T) {
	m := modelDeBarra(t, 110)
	m.cwd = "/c/Users/usera/gregal-agent"
	linia := stripANSI(ultimaLinia(m.View()))
	if !strings.Contains(linia, "gregal-agent") {
		t.Fatalf("cal el nom de la carpeta:\n%s", linia)
	}
	if !strings.Contains(linia, "abast: projecte") {
		t.Fatalf("cal l'abast de permisos:\n%s", linia)
	}
	// Amb la barra ampla hi cap el camí sencer.
	m = modelDeBarra(t, 170)
	m.cwd = "/c/Users/usera/gregal-agent"
	if linia := stripANSI(ultimaLinia(m.View())); !strings.Contains(linia, "/c/Users/usera/gregal-agent") {
		t.Fatalf("a 170 columnes cap el camí sencer:\n%s", linia)
	}
}

// Permissiu vol dir que l'agent pot escriure fora del projecte: la barra
// ho ha de cridar, no pas amagar-ho.
func TestLaBarraAdverteixDeLAbastTotal(t *testing.T) {
	m := modelDeBarra(t, 140)
	m.cwd = "/c/Users/usera/gregal-agent"
	m.permissive = true
	linia := stripANSI(ultimaLinia(m.View()))
	if !strings.Contains(linia, "abast: tot el disc") {
		t.Fatalf("cal l'avís d'abast total:\n%s", linia)
	}
}

// La carpeta no pot desbordar la barra ni fer fora el context: a
// qualsevol amplada, la línia cap i el ctx hi és.
func TestLaCarpetaNoMenjaElContext(t *testing.T) {
	for _, ample := range []int{60, 80, 100, 132, 170} {
		m := modelDeBarra(t, ample)
		m.cwd = "/c/Users/sample.user/una-carpeta-de-treball-llarguíssima"
		linia := stripANSI(ultimaLinia(m.View()))
		if w := lipgloss.Width(linia); w > ample {
			t.Errorf("a %d columnes la barra en fa %d:\n%s", ample, w, linia)
		}
		if !strings.Contains(linia, "ctx ") {
			t.Errorf("a %d columnes el context ha desaparegut:\n%s", ample, linia)
		}
		if !strings.Contains(linia, "CODE") {
			t.Errorf("a %d columnes falta el mode:\n%s", ample, linia)
		}
	}
}

// Durant una aprovació l'estat és el que s'ha de llegir: la carpeta
// s'adapta a l'espai que queda, i no a l'inrevés. Amb la reserva fixa
// d'abans, a 80 columnes sortia «◌ espera la t» i sense el ■.
func TestLaCarpetaCedeixALEstat(t *testing.T) {
	for _, ample := range []int{60, 80, 100} {
		m := modelDeBarra(t, ample)
		m.cwd = "C:/Users/usera/tui-agent"
		m.pending = &pendingOp{}
		linia := stripANSI(ultimaLinia(m.View()))
		if !strings.Contains(linia, "espera la teva aprovació") {
			t.Errorf("a %d columnes l'estat d'aprovació ha de sortir sencer:\n%s", ample, linia)
		}
		if !strings.HasSuffix(strings.TrimRight(linia, " "), "■") {
			t.Errorf("a %d columnes falta l'indicador de permisos:\n%s", ample, linia)
		}
		if ample >= 80 && !strings.Contains(linia, "tui-agent") {
			t.Errorf("a %d columnes encara hi cap el nom del projecte:\n%s", ample, linia)
		}
	}
}

// Sense carpeta (model sense cwd) la barra no ha de quedar amb un forat
// estrany ni petar.
func TestLaBarraSenseCarpeta(t *testing.T) {
	m := modelDeBarra(t, 110)
	m.cwd = ""
	linia := stripANSI(ultimaLinia(m.View()))
	if strings.Contains(linia, "abast:") {
		t.Fatalf("sense carpeta no hi ha abast a dir:\n%s", linia)
	}
	if !strings.Contains(linia, "ctx ") {
		t.Fatalf("la resta de la barra ha de quedar-se:\n%s", linia)
	}
}
