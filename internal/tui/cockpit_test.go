package tui

import (
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/tools"
)

// A 120 columnes o més, el cockpit és una columna a la dreta: la vista
// segueix fent l'alçada del terminal, totes les files fan la mateixa
// amplada, i la conversa cedeix les 34 columnes de la dreta.
func TestCockpitEnColumnaQuanHiCap(t *testing.T) {
	m := modelDeBarra(t, 140)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = mod.(Model)
	tools.TodoSet([]tools.TodoItem{{Title: "Llegir el config", Status: "done"}, {Title: "Afegir el camp", Status: "working"}})
	t.Cleanup(tools.TodoClear)
	if !m.columnaActiva() {
		t.Fatal("a 140 columnes la columna ha d'estar activa per defecte")
	}
	if m.vp.Width != 140-ampleColumna-4 {
		t.Fatalf("el viewport ha de cedir la columna: fa %d", m.vp.Width)
	}
	v := m.View()
	if h := lipgloss.Height(v); h != 40 {
		t.Fatalf("la vista fa %d files, no 40", h)
	}
	pla := stripANSI(v)
	for _, want := range []string{"tasca  1/2", "Afegir el camp", "context", "validació"} {
		if !strings.Contains(pla, want) {
			t.Fatalf("la columna ha de dir %q:\n%s", want, pla)
		}
	}
	files := strings.Split(pla, "\n")
	amples := map[int]bool{}
	for _, f := range files {
		amples[lipgloss.Width(f)] = true
	}
	if len(amples) != 1 {
		t.Fatalf("totes les files han de fer la mateixa amplada, n'hi ha %d de diferents", len(amples))
	}
	// I la caixa vella no surt a sota: seria el mateix dues vegades.
	m.showTodos = true
	if strings.Count(stripANSI(m.View()), "tasca") != 1 {
		t.Fatal("amb la columna activa, la caixa del cockpit no s'ha de pintar a sota")
	}
}

// Ctrl+T commuta la columna i ho recorda al config quan hi cap; en un
// terminal estret commuta la caixa de sempre i no toca el config.
func TestCtrlTCommutaIRecordaLaColumna(t *testing.T) {
	m := modelDeBarra(t, 140)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = mod.(Model)
	ampleAmb := m.vp.Width
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = mod.(Model)
	if m.columnaActiva() || m.cfg.Cockpit != "off" {
		t.Fatalf("Ctrl+T ha d'amagar la columna i desar-ho (actiu=%v cfg=%q)", m.columnaActiva(), m.cfg.Cockpit)
	}
	if m.vp.Width != ampleAmb+ampleColumna {
		t.Fatalf("sense columna la conversa recupera l'amplada: %d vs %d", m.vp.Width, ampleAmb+ampleColumna)
	}
	if h := lipgloss.Height(m.View()); h != 40 {
		t.Fatalf("la vista fa %d files, no 40", h)
	}
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = mod.(Model)
	if !m.columnaActiva() || m.cfg.Cockpit != "on" {
		t.Fatal("el segon Ctrl+T la torna a treure i ho desa")
	}

	estret := modelDeBarra(t, 100)
	mod, _ = estret.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	estret = mod.(Model)
	mod, _ = estret.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	estret = mod.(Model)
	if !estret.showTodos || estret.cfg.Cockpit != "" || estret.columnaActiva() {
		t.Fatalf("a 100 columnes Ctrl+T obre la caixa i no toca el config (todos=%v cfg=%q)", estret.showTodos, estret.cfg.Cockpit)
	}
	if !strings.Contains(stripANSI(estret.View()), "tasca") {
		t.Fatal("la caixa del cockpit s'ha de veure a sota")
	}
}

// `cockpit: off` al config arrenca sense columna.
func TestCockpitOffAlConfig(t *testing.T) {
	m := modelDeBarra(t, 140)
	m.cfg.Cockpit = "off"
	m.cockpit = m.cfg.CockpitOn()
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = mod.(Model)
	if m.columnaActiva() || m.vp.Width != 140-4 {
		t.Fatalf("amb cockpit: off no hi ha columna (actiu=%v vp=%d)", m.columnaActiva(), m.vp.Width)
	}
}

// Les rutes del cockpit són relatives al projecte (també en la forma del
// Git Bash) i, si no hi caben, es tallen per l'esquerra: el final és el
// que distingeix un fitxer d'un altre.
func TestRutesCurtesAlCockpit(t *testing.T) {
	// Prova escrita a Windows: les rutes C:/... no volen dir el mateix
	// a Linux (alla "C:" es un nom de fitxer mes). Es queda per a on te sentit.
	if runtime.GOOS != "windows" {
		t.Skip("casos de rutes de Windows")
	}
	m := modelDeBarra(t, 140)
	m.cwd = "C:\\Users\\usera\\gregal-agent"
	if got := m.rutaCurta("C:/Users/usera/gregal-agent/src/joc.js", 40); got != "src/joc.js" {
		t.Fatalf("relativa: %q", got)
	}
	if got := m.rutaCurta("/c/Users/usera/gregal-agent/plataformes/src/nivells.js", 40); got != "plataformes/src/nivells.js" {
		t.Fatalf("forma git bash: %q", got)
	}
	if got := m.rutaCurta("/altre/lloc/molt/llarg/fitxer.go", 12); got != "…/fitxer.go" {
		t.Fatalf("tall per l'esquerra: %q", got)
	}
}
