package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// superposa no canvia ni les files ni l'amplada del fons, centra la caixa
// i deixa el text del fons llegible (atenuat, però hi és) fora de la
// caixa.
func TestSuperposaCentraISenseMoureElFons(t *testing.T) {
	var fons []string
	for i := 0; i < 10; i++ {
		fons = append(fons, strings.Repeat("abcdefghij", 4)) // 40 columnes
	}
	caixa := "╭────╮\n│ ok │\n╰────╯"
	got := strings.Split(superposa(strings.Join(fons, "\n"), caixa, 40), "\n")
	if len(got) != 10 {
		t.Fatalf("el resultat fa %d files, el fons en feia 10", len(got))
	}
	for i, l := range got {
		if w := ansi.StringWidth(l); w != 40 {
			t.Fatalf("fila %d: %d columnes, no 40: %q", i, w, ansi.Strip(l))
		}
	}
	// Caixa de 3 files i 6 columnes: files 3-5, columnes 17-22.
	mig := ansi.Strip(got[4])
	if !strings.Contains(mig, "│ ok │") || strings.Index(mig, "│") != 17 {
		t.Fatalf("la caixa no és centrada: %q", mig)
	}
	if ansi.Strip(got[0]) != fons[0] || ansi.Strip(got[9]) != fons[9] {
		t.Fatalf("el fons fora de la caixa ha de ser el mateix text: %q", ansi.Strip(got[0]))
	}
	if !strings.HasPrefix(mig, "abcdefghijabcdefg") {
		t.Fatalf("el fons a l'esquerra de la caixa s'ha de conservar: %q", mig)
	}
}

// Obrir un menú no mou la conversa: la vista fa les mateixes files, i el
// text fora del requadre és el mateix amb el menú obert i tancat.
func TestElMenuNoMouLaConversa(t *testing.T) {
	m := modelDeBarra(t, 100)
	for i := 0; i < 30; i++ {
		m.push("fila de conversa " + itoa(i))
	}
	tancat := m.View()
	m.menu = &menuState{title: "tria", items: []menuItem{{text: "a"}, {text: "b"}}}
	obert := m.View()
	if lipgloss.Height(tancat) != lipgloss.Height(obert) {
		t.Fatalf("el menú canvia l'alçada: %d vs %d", lipgloss.Height(tancat), lipgloss.Height(obert))
	}
	if !strings.Contains(obert, "tria") || !strings.Contains(obert, "▸ a") {
		t.Fatalf("el menú no es veu:\n%s", stripANSI(obert))
	}
	ft, fo := strings.Split(stripANSI(tancat), "\n"), strings.Split(stripANSI(obert), "\n")
	iguals := 0
	for i := range ft {
		if strings.TrimRight(ft[i], " ") == strings.TrimRight(fo[i], " ") {
			iguals++
		}
	}
	// El requadre del menú ocupa unes vuit files: la resta han de ser
	// idèntiques (capçalera, conversa de dalt i de baix, barra).
	if iguals < len(ft)-10 {
		t.Fatalf("només %d de %d files iguals: la conversa s'ha mogut", iguals, len(ft))
	}
}

// ? obre l'ajuda superposada sense escriure res a la conversa, i la
// primera tecla la tanca sense fer res més.
func TestInterrogantObreITancaLAjuda(t *testing.T) {
	m := modelDeBarra(t, 100)
	abans := len(m.lines)
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = mod.(Model)
	if !m.ajuda || len(m.lines) != abans {
		t.Fatalf("? ha d'obrir l'ajuda sense tocar la conversa (ajuda=%v, files %d→%d)", m.ajuda, abans, len(m.lines))
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "Shift+Tab") || !strings.Contains(v, "CONSULTA") {
		t.Fatalf("l'ajuda ha de dir les tecles i els modes:\n%s", v)
	}
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mod.(Model)
	if m.ajuda || m.busy || len(m.lines) != abans {
		t.Fatalf("la primera tecla tanca l'ajuda i no fa res més (ajuda=%v busy=%v)", m.ajuda, m.busy)
	}
}
