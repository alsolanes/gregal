package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Una línia més llarga que l'amplada s'embolcalla a la conversa en comptes
// de quedar tallada per la dreta: abans un pas del pla llarg només es
// podia llegir eixamplant la finestra.
func TestLesLiniesLlarguesFanSalt(t *testing.T) {
	m := modelGolden(t, 80, 30)
	m.treuBenvinguda()
	m.push("  1. " + strings.Repeat("paraula ", 20) + "FINAL")
	v := stripANSI(m.View())
	if !strings.Contains(v, "FINAL") {
		t.Fatalf("el final de la línia llarga ha de ser a la pantalla:\n%s", v)
	}
	for i, f := range strings.Split(v, "\n") {
		if w := lipgloss.Width(f); w > 80 {
			t.Fatalf("fila %d fa %d columnes, més que el terminal", i, w)
		}
	}
	// I la targeta del pla fa l'amplada de la conversa, amb els passos
	// llargs dins de la vora.
	card := planCard("1. "+strings.Repeat("pas llarg ", 15)+"\n2. curt", m.vp.Width)
	for _, f := range strings.Split(stripANSI(card), "\n") {
		if w := lipgloss.Width(f); w != m.vp.Width {
			t.Fatalf("la targeta del pla ha de fer %d columnes, en fa %d: %q", m.vp.Width, w, f)
		}
	}
	// En redimensionar, es torna a embolcallar amb la mida nova.
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	m2 := mod.(Model)
	if strings.Count(stripANSI(m2.vp.View()), "paraula") != 20 {
		t.Fatal("després de redimensionar la línia hi ha de ser sencera")
	}
}

// Un \r de la sortida de Windows no pot arribar al terminal: feia tornar
// el cursor a la columna 0 i la columna del cockpit d'aquella fila es
// pintava sobre la conversa. Els tabuladors tampoc: el terminal els
// eixampla al seu gust i la fila es torna més llarga del que compta lipgloss.
func TestNiRetornsDeCarroNiTabuladorsALaConversa(t *testing.T) {
	m := modelGolden(t, 140, 30)
	m.treuBenvinguda()
	m.push("v24.1.0\r")
	m.push("total\t59814")
	v := m.vp.View()
	if strings.Contains(v, "\r") || strings.Contains(v, "\t") {
		t.Fatalf("la vista porta \\r o \\t:\n%q", v)
	}
	if !strings.Contains(stripANSI(v), "total    59814") {
		t.Fatalf("el tabulador s'ha de convertir en espais:\n%s", stripANSI(v))
	}
}
