package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// modelAmbText és un model llest amb la conversa substituïda per files
// conegudes i el viewport a dalt de tot.
func modelAmbText(t *testing.T, files ...string) Model {
	t.Helper()
	m := modelDeBarra(t, 100)
	m.lines = files
	m.refresh()
	m.vp.GotoTop()
	return m
}

// filaPantalla és la fila de pantalla on es veu la primera fila visual que
// conté marca.
func filaPantalla(t *testing.T, m Model, marca string) int {
	t.Helper()
	vp, _, _, _, _, _ := m.composicio(dimStyle)
	for i, f := range strings.Split(m.visual(), "\n") {
		if strings.Contains(ansi.Strip(f), marca) {
			return filaConversa + i - vp.YOffset
		}
	}
	t.Fatalf("%q no és a la conversa", marca)
	return 0
}

func ratoli(m Model, x, y int, accio tea.MouseAction, boto tea.MouseButton) (Model, tea.Cmd) {
	mod, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Action: accio, Button: boto})
	return mod.(Model), cmd
}

func TestSeleccioArrossegarCopiaElRang(t *testing.T) {
	m := modelAmbText(t, "alfa beta gamma", "delta epsilon", "zeta")
	y0, y1 := filaPantalla(t, m, "alfa"), filaPantalla(t, m, "delta")
	m, cmd := ratoli(m, strings.Index("alfa beta gamma", "beta"), y0, tea.MouseActionPress, tea.MouseButtonLeft)
	if cmd != nil || !m.sel.arrossega {
		t.Fatalf("el clic a la conversa ha d'iniciar la selecció: %+v", m.sel)
	}
	m, _ = ratoli(m, 10, y0, tea.MouseActionMotion, tea.MouseButtonLeft)
	m, _ = ratoli(m, len("delta")-1, y1, tea.MouseActionMotion, tea.MouseButtonLeft)
	if !m.sel.actiu {
		t.Fatal("arrossegar ha de deixar una selecció activa")
	}
	if got := m.textSeleccionat(); got != "beta gamma\ndelta" {
		t.Fatalf("text=%q", got)
	}
	m, cmd = ratoli(m, len("delta")-1, y1, tea.MouseActionRelease, tea.MouseButtonLeft)
	if cmd == nil || m.sel.arrossega || !m.sel.actiu {
		t.Fatalf("en deixar anar s'ha de copiar i mantenir el ressaltat: cmd=%v sel=%+v", cmd != nil, m.sel)
	}
	// El resultat de la còpia surt a la barra.
	mod, _ := m.Update(copiatSelMsg{n: 16, how: "Windows"})
	if st := mod.(Model).status; !strings.Contains(st, "16") || !strings.Contains(st, "Windows") {
		t.Fatalf("status=%q", st)
	}
}

// Arrossegar cap amunt (cap abans que l'àncora) dona el mateix rang.
func TestSeleccioAlReves(t *testing.T) {
	m := modelAmbText(t, "alfa beta gamma", "delta epsilon")
	y0, y1 := filaPantalla(t, m, "alfa"), filaPantalla(t, m, "delta")
	m, _ = ratoli(m, 4, y1, tea.MouseActionPress, tea.MouseButtonLeft)
	m, _ = ratoli(m, 5, y0, tea.MouseActionMotion, tea.MouseButtonLeft)
	if got := m.textSeleccionat(); got != "beta gamma\ndelta" {
		t.Fatalf("text=%q", got)
	}
}

// Un clic sense moure no copia res ni deixa ressaltat.
func TestSeleccioClicSolNoCopia(t *testing.T) {
	m := modelAmbText(t, "alfa beta gamma")
	y := filaPantalla(t, m, "alfa")
	m, _ = ratoli(m, 3, y, tea.MouseActionPress, tea.MouseButtonLeft)
	m, cmd := ratoli(m, 3, y, tea.MouseActionRelease, tea.MouseButtonLeft)
	if cmd != nil || m.sel.actiu || m.sel.arrossega {
		t.Fatalf("cmd=%v sel=%+v", cmd != nil, m.sel)
	}
}

// La roda continua desplaçant amb la selecció activa.
func TestSeleccioRodaContinuaDesplacant(t *testing.T) {
	var files []string
	for i := range 200 {
		files = append(files, fmt.Sprintf("fila %03d", i))
	}
	m := modelAmbText(t, files...)
	y := filaPantalla(t, m, "fila 000")
	m, _ = ratoli(m, 0, y, tea.MouseActionPress, tea.MouseButtonLeft)
	m, _ = ratoli(m, 6, y+1, tea.MouseActionMotion, tea.MouseButtonLeft)
	m, _ = ratoli(m, 6, y+1, tea.MouseActionRelease, tea.MouseButtonLeft)
	abans := m.vp.YOffset
	m, _ = ratoli(m, 5, y+2, tea.MouseActionPress, tea.MouseButtonWheelDown)
	if m.vp.YOffset <= abans {
		t.Fatalf("la roda no ha desplaçat: %d → %d", abans, m.vp.YOffset)
	}
	if !m.sel.actiu || m.textSeleccionat() != "fila 000\nfila 00" {
		t.Fatalf("la selecció ha de quedar ancorada al text: %q", m.textSeleccionat())
	}
}

// Arrossegar per sota de la conversa la desplaça per seguir seleccionant.
func TestSeleccioArrossegarAlMargeDesplaca(t *testing.T) {
	var files []string
	for i := range 200 {
		files = append(files, fmt.Sprintf("fila %03d", i))
	}
	m := modelAmbText(t, files...)
	y := filaPantalla(t, m, "fila 000")
	m, _ = ratoli(m, 0, y, tea.MouseActionPress, tea.MouseButtonLeft)
	vp, _, _, _, _, _ := m.composicio(dimStyle)
	sota := filaConversa + vp.Height + 1
	for range 5 {
		m, _ = ratoli(m, 3, sota, tea.MouseActionMotion, tea.MouseButtonLeft)
	}
	if m.vp.YOffset != 5 {
		t.Fatalf("YOffset=%d, volia 5", m.vp.YOffset)
	}
	if got := strings.Count(m.textSeleccionat(), "\n"); got != vp.Height-1+5 {
		t.Fatalf("files seleccionades=%d", got+1)
	}
}

// Un moviment sense botó a mig arrossegament vol dir que l'alliberament
// s'ha perdut: es tanca la selecció i es copia igualment.
func TestSeleccioAlliberamentPerdut(t *testing.T) {
	m := modelAmbText(t, "alfa beta gamma")
	y := filaPantalla(t, m, "alfa")
	m, _ = ratoli(m, 0, y, tea.MouseActionPress, tea.MouseButtonLeft)
	m, _ = ratoli(m, 3, y, tea.MouseActionMotion, tea.MouseButtonLeft)
	m, cmd := ratoli(m, 9, y, tea.MouseActionMotion, tea.MouseButtonNone)
	if cmd == nil || m.sel.arrossega || m.textSeleccionat() != "alfa" {
		t.Fatalf("cmd=%v sel=%+v text=%q", cmd != nil, m.sel, m.textSeleccionat())
	}
}

// El rang seleccionat es pinta en vídeo invers, sense moure ni una
// columna la resta de la fila.
func TestSeleccioEsPintaRessaltada(t *testing.T) {
	perfil := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(perfil) })
	m := modelAmbText(t, "alfa beta gamma")
	// New() applies the configured theme and honors NO_COLOR, so force ANSI
	// styling after model setup to keep this rendering assertion deterministic.
	lipgloss.SetColorProfile(termenv.TrueColor)
	y := filaPantalla(t, m, "alfa")
	m, _ = ratoli(m, 5, y, tea.MouseActionPress, tea.MouseButtonLeft)
	m, _ = ratoli(m, 8, y, tea.MouseActionMotion, tea.MouseButtonLeft)
	files := strings.Split(m.View(), "\n")
	fila := files[y]
	if !strings.Contains(fila, "\x1b[7mbeta") {
		t.Fatalf("no hi ha ressaltat: %q", fila)
	}
	if got := ansi.Strip(fila); !strings.HasPrefix(got, "alfa beta gamma") || ansi.StringWidth(got) != ansi.StringWidth(ansi.Strip(strings.Split(modelAmbText(t, "alfa beta gamma").View(), "\n")[y])) {
		t.Fatalf("la fila ha canviat d'amplada o de text: %q", got)
	}
}
