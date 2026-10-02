package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Cerca a l'historial d'entrada (Ctrl+R).
//
// Amb ↑/↓ es recorre l'historial una a una, i prou. Per a la cosa que fas
// de debò —«què era aquella ordre llarga de fa dos dies»— això vol dir
// picar la fletxa vint vegades mirant si passa de llarg. Ctrl+R és la
// drecera que ja tens als dits de la shell i de tots els agents de
// terminal, i aquí no hi era.
//
// Funciona com la del bash: escrius i va ensenyant la coincidència més
// recent; Ctrl+R torna a la d'abans; Enter la posa al camp perquè la
// puguis retocar abans d'enviar-la (no l'envia sola: una ordre repescada
// que s'executa sense mirar-la és com s'esborren coses); Esc deixa el que
// hi havia.

type histCerca struct {
	consulta string
	// trobades són els índexs de m.histEntries que casen, de més recent a
	// més antic. Es recalculen a cada tecla.
	trobades []int
	sel      int
	// abans és el que hi havia al camp, per si es cancel·la.
	abans string
}

// obreCercaHist engega la cerca amb el que ja hi hagi escrit com a
// consulta inicial: si has començat a escriure i te n'adones que ja ho
// havies fet, no ho has de tornar a picar.
func (m *Model) obreCercaHist() {
	c := &histCerca{abans: m.input.Value()}
	c.consulta = strings.TrimSpace(m.input.Value())
	m.histCerca = c
	c.busca(m.histEntries)
	m.aplicaCercaHist()
}

func (c *histCerca) busca(entrades []string) {
	c.trobades = c.trobades[:0]
	q := strings.ToLower(c.consulta)
	for i := len(entrades) - 1; i >= 0; i-- {
		if q == "" || strings.Contains(strings.ToLower(entrades[i]), q) {
			c.trobades = append(c.trobades, i)
		}
	}
	if c.sel >= len(c.trobades) {
		c.sel = len(c.trobades) - 1
	}
	if c.sel < 0 {
		c.sel = 0
	}
}

// actual torna l'entrada seleccionada, o "" si no n'hi ha cap.
func (c *histCerca) actual(entrades []string) string {
	if len(c.trobades) == 0 || c.sel >= len(c.trobades) {
		return ""
	}
	return entrades[c.trobades[c.sel]]
}

// aplicaCercaHist posa la coincidència al camp perquè es vegi sencera.
func (m *Model) aplicaCercaHist() {
	if t := m.histCerca.actual(m.histEntries); t != "" {
		m.input.SetValue(t)
		m.fitInput()
	}
}

// tecleaCercaHist processa una tecla mentre la cerca és oberta. Torna
// false si la tecla no és seva i l'ha de mirar el camí normal.
func (m *Model) tecleaCercaHist(tecla string, runes []rune) bool {
	c := m.histCerca
	switch tecla {
	case "esc", "ctrl+g":
		m.histCerca = nil
		m.input.SetValue(c.abans)
		m.fitInput()
		return true
	case "enter":
		// Es queda al camp, sense enviar: una ordre repescada que
		// s'executa sense mirar-la és com s'esborren coses. El valor
		// s'agafa ABANS de tancar: aplicaCercaHist mira m.histCerca.
		if t := c.actual(m.histEntries); t != "" {
			m.input.SetValue(t)
			m.fitInput()
		}
		m.histCerca = nil
		return true
	case "ctrl+r", "up":
		if c.sel+1 < len(c.trobades) {
			c.sel++
		}
		m.aplicaCercaHist()
		return true
	case "down":
		if c.sel > 0 {
			c.sel--
		}
		m.aplicaCercaHist()
		return true
	case "backspace":
		if r := []rune(c.consulta); len(r) > 0 {
			c.consulta = string(r[:len(r)-1])
		}
		c.sel = 0
		c.busca(m.histEntries)
		m.aplicaCercaHist()
		return true
	case "ctrl+c":
		return false
	}
	if len(runes) > 0 && tecla != "tab" {
		c.consulta += string(runes)
		c.sel = 0
		c.busca(m.histEntries)
		m.aplicaCercaHist()
		return true
	}
	return false
}

// etiquetaCercaHist és el que es llegeix sobre el camp mentre cerques.
func (m Model) etiquetaCercaHist() string {
	c := m.histCerca
	q := c.consulta
	if q == "" {
		q = "…"
	}
	cap := T("cerca.etiqueta") + q
	if len(c.trobades) == 0 {
		return warnStyle.Render(cap) + faintStyle.Render(T("cerca.cap"))
	}
	pos := lipgloss.NewStyle().Render("")
	if len(c.trobades) > 1 {
		pos = faintStyle.Render(" " + fmtPos(c.sel+1, len(c.trobades)))
	}
	// Sense pistes: són al diàleg superposat (overlay.go).
	return okStyle.Render(cap) + pos
}

func fmtPos(i, n int) string {
	return itoa(i) + "/" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
