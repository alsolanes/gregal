package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Selecció de text amb el ratolí dins de l'app. Amb el ratolí capturat
// (roda per desplaçar) el terminal no fa la seva selecció nativa: clic i
// arrossegar sobre la conversa ressalta el text i, en deixar anar, es
// copia al porta-retalls. La roda continua desplaçant, també a mig
// arrossegar. Shift+arrossegar continua passant al terminal on ho suporta
// (Windows Terminal, iTerm2, kitty…).

// filaConversa és la primera fila de pantalla del viewport: capçalera,
// onada i una fila en blanc (vegeu View i clicSessio).
const filaConversa = 3

// puntSel és una cel·la del contingut de la conversa: fila VISUAL (ja
// embolcallada, com les del viewport) i columna en cel·les.
type puntSel struct{ fila, col int }

func (p puntSel) abans(q puntSel) bool {
	return p.fila < q.fila || (p.fila == q.fila && p.col < q.col)
}

type seleccio struct {
	arrossega bool // botó premut des d'un clic a la conversa
	actiu     bool // hi ha text seleccionat (es pinta ressaltat)
	ancora    puntSel
	cap       puntSel
}

// rang torna els extrems ordenats (inici inclòs, final inclòs).
func (s seleccio) rang() (puntSel, puntSel) {
	if s.cap.abans(s.ancora) {
		return s.cap, s.ancora
	}
	return s.ancora, s.cap
}

// copiatSelMsg és el resultat de copiar la selecció (asíncron: algun
// backend del porta-retalls pot trigar).
type copiatSelMsg struct {
	n   int
	how string
	err error
}

var selStyle = lipgloss.NewStyle().Reverse(true)

// puntSota tradueix una posició de pantalla a una cel·la del contingut,
// enganxada als marges del viewport. dins diu si queia dins de la conversa.
func (m Model) puntSota(x, y int) (p puntSel, dins bool) {
	vp, _, _, _, _, _ := m.composicio(dimStyle)
	rel := y - filaConversa
	dins = rel >= 0 && rel < vp.Height && x >= 0 && x < vp.Width
	rel = max(0, min(rel, vp.Height-1))
	x = max(0, min(x, vp.Width-1))
	return puntSel{fila: vp.YOffset + rel, col: x}, dins
}

// ratoliSeleccio gestiona clic, arrossegament i alliberament del botó
// esquerre. Torna handled=false per als esdeveniments que no són seus
// (roda, clics fora de la conversa), que segueixen el camí de sempre.
func (m Model) ratoliSeleccio(msg tea.MouseMsg) (Model, tea.Cmd, bool) {
	if m.dialegObert() != "" {
		return m, nil, false
	}
	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		p, dins := m.puntSota(msg.X, msg.Y)
		m.sel = seleccio{}
		if !dins {
			return m, nil, false
		}
		m.sel = seleccio{arrossega: true, ancora: p, cap: p}
		return m, nil, true
	case msg.Action == tea.MouseActionMotion && m.sel.arrossega && msg.Button != tea.MouseButtonLeft:
		// Moviment sense botó: s'ha perdut l'alliberament (p. ex. s'ha
		// deixat anar fora de la finestra). Es tanca com si hagués arribat.
		return m.ratoliSeleccio(tea.MouseMsg{X: msg.X, Y: msg.Y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	case msg.Action == tea.MouseActionMotion && m.sel.arrossega:
		// Arrossegar per sobre o per sota de la conversa la desplaça una
		// fila per esdeveniment: així es poden seleccionar trossos més
		// llargs que la pantalla.
		rel := msg.Y - filaConversa
		if rel < 0 {
			m.vp.ScrollUp(1)
		} else if vp, _, _, _, _, _ := m.composicio(dimStyle); rel >= vp.Height {
			m.vp.ScrollDown(1)
		}
		m.sel.cap, _ = m.puntSota(msg.X, msg.Y)
		m.sel.actiu = m.sel.cap != m.sel.ancora
		return m, nil, true
	case msg.Action == tea.MouseActionRelease && m.sel.arrossega:
		m.sel.arrossega = false
		if !m.sel.actiu {
			return m, nil, true
		}
		text := m.textSeleccionat()
		if text == "" {
			m.sel.actiu = false
			return m, nil, true
		}
		return m, func() tea.Msg {
			how, err := copyToClipboard(text)
			return copiatSelMsg{n: len([]rune(text)), how: how, err: err}
		}, true
	}
	return m, nil, false
}

// textSeleccionat és el text pla (sense estils) del rang seleccionat, amb
// els espais finals de cada fila retallats.
func (m Model) textSeleccionat() string {
	if !m.sel.actiu {
		return ""
	}
	a, b := m.sel.rang()
	files := strings.Split(m.visual(), "\n")
	var out []string
	for f := a.fila; f <= b.fila && f < len(files); f++ {
		if f < 0 {
			continue
		}
		pla := ansi.Strip(files[f])
		c0, c1 := 0, ansi.StringWidth(pla)
		if f == a.fila {
			c0 = a.col
		}
		if f == b.fila {
			c1 = min(c1, b.col+1)
		}
		out = append(out, strings.TrimRight(ansi.Cut(pla, c0, c1), " "))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// pintaSeleccio ressalta el rang seleccionat sobre la conversa ja pintada.
// offset és la primera fila del contingut que es veu (vp.YOffset).
func (m Model) pintaSeleccio(conversa string, offset, ample int) string {
	if !m.sel.actiu {
		return conversa
	}
	a, b := m.sel.rang()
	files := strings.Split(conversa, "\n")
	for i, fila := range files {
		f := offset + i
		if f < a.fila || f > b.fila {
			continue
		}
		c0, c1 := 0, ample
		if f == a.fila {
			c0 = a.col
		}
		if f == b.fila {
			c1 = b.col + 1
		}
		if c1 <= c0 {
			continue
		}
		mig := ansi.Strip(ansi.Cut(fila, c0, c1))
		files[i] = ansi.Cut(fila, 0, c0) + selStyle.Render(mig) + ansi.Cut(fila, c1, max(ample, c1))
	}
	return strings.Join(files, "\n")
}
