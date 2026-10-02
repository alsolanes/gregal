package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Superposicions.
//
// Els diàlegs (selector, aprovació, pregunta, cerca, ajuda) es pinten
// SOBRE la conversa, centrats, amb el que hi ha a sota atenuat. Abans
// s'encabien entre el viewport i el composer i li prenien files: obrir un
// menú feia saltar tot el text amunt, i tancar-lo el tornava a baixar.
// Ara la conversa no es mou: només s'apaga mentre el diàleg hi és.

// maxDialeg és l'amplada màxima d'un diàleg. Més ample, un requadre
// deixa de semblar un diàleg i sembla una altra pantalla.
const maxDialeg = 72

// ampleDialeg és l'amplada d'un diàleg per a un terminal d'`ample`
// columnes: 72 o el que hi cap deixant dues columnes a cada costat.
func ampleDialeg(ample int) int {
	w := min(maxDialeg, ample-4)
	if w < 24 {
		w = 24
	}
	return w
}

// dialeg emmarca un diàleg: títol en negreta, cos, i una última fila
// amb les tecles. Tots els diàlegs passen per aquí perquè tinguin la
// mateixa vora, el mateix marge i la mateixa fila de tecles.
func dialeg(titol, cos, tecles string, ample int) string {
	w := ampleDialeg(ample)
	var b strings.Builder
	b.WriteString(headStyle.Render(titol) + "\n")
	if strings.TrimSpace(cos) != "" {
		b.WriteString(strings.TrimRight(cos, "\n") + "\n")
	}
	if tecles != "" {
		b.WriteString(faintStyle.Render(tecles))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(escuma).
		Background(night).
		Padding(0, 1).
		Width(w - 2). // Width és el contingut; la vora hi suma 2
		Render(strings.TrimRight(b.String(), "\n"))
}

// superposa pinta `caixa` centrada sobre `fons` (les files del viewport,
// d'`ample` columnes) i atenua el que queda a la vista del fons. El
// resultat té exactament les files del fons: el diàleg no fa créixer ni
// encongir res. Si la caixa és més alta que el fons, es retalla per
// baix; si és més ampla, per la dreta.
func superposa(fons, caixa string, ample int) string {
	files := strings.Split(fons, "\n")
	cl := strings.Split(strings.TrimRight(caixa, "\n"), "\n")
	if len(cl) > len(files) {
		cl = cl[:len(files)]
	}
	cw := 0
	for _, l := range cl {
		cw = max(cw, ansi.StringWidth(l))
	}
	if cw > ample {
		cw = ample
	}
	x0 := (ample - cw) / 2
	y0 := (len(files) - len(cl)) / 2
	out := make([]string, len(files))
	for i, l := range files {
		// El fons s'apaga tot sencer: sense colors, en text.tenue. El que
		// importa mentre el diàleg és obert és el diàleg.
		pla := ansi.Strip(l)
		if w := ansi.StringWidth(pla); w < ample {
			pla += strings.Repeat(" ", ample-w)
		} else if w > ample {
			pla = ansi.Truncate(pla, ample, "")
		}
		if i < y0 || i >= y0+len(cl) {
			out[i] = faintStyle.Render(pla)
			continue
		}
		fila := ansi.Truncate(cl[i-y0], cw, "")
		if w := ansi.StringWidth(fila); w < cw {
			fila += strings.Repeat(" ", cw-w)
		}
		out[i] = faintStyle.Render(ansi.Cut(pla, 0, x0)) + fila + faintStyle.Render(ansi.Cut(pla, x0+cw, ample))
	}
	return strings.Join(out, "\n")
}

// dialegObert diu quin diàleg toca pintar, si n'hi ha cap. Només un a la
// vegada, per aquest ordre: el que espera una resposta del model primer.
func (m Model) dialegObert() string {
	switch {
	case m.pendingQ != nil:
		return questionBox(m.pendingQ, m.vp.Width, m.input.Value())
	case m.pending != nil:
		return m.aprovacioBox(m.vp.Width)
	case m.menu != nil:
		return menuLine(m.menu, m.vp.Width)
	case m.histCerca != nil:
		return m.cercaHistBox(m.vp.Width)
	case m.ajuda:
		return ajudaTecles(m.vp.Width)
	}
	return ""
}

// ajudaTecles és el que obre `?`: les tecles i els modes, i prou. La
// llista sencera d'ordres és a /help, que s'escriu a la conversa perquè
// es pugui llegir amb calma.
func ajudaTecles(ample int) string {
	var b strings.Builder
	for _, g := range ajudaGrups {
		if g.clauTitol != "ajuda.tecles" && g.clauTitol != "ajuda.modes" {
			continue
		}
		b.WriteString(dimStyle.Render(T(g.clauTitol)) + "\n")
		// Les descripcions, alineades a la mateixa columna dins del grup.
		col := 0
		for _, f := range g.files {
			col = max(col, lipgloss.Width(T(f[0])))
		}
		for _, f := range g.files {
			et := T(f[0])
			b.WriteString("  " + cmdStyle.Render(et) + strings.Repeat(" ", col-lipgloss.Width(et)) + dimStyle.Render("  "+T(f[1])) + "\n")
		}
	}
	return dialeg("? "+T("ajuda.titol"), b.String(), T("ajuda.pistes"), ample)
}

// cercaHistBox és la cerca a l'historial com a diàleg: la consulta, les
// coincidències més recents i quina hi ha triada. El camp de sota mostra
// la triada sencera, com abans.
func (m Model) cercaHistBox(ample int) string {
	c := m.histCerca
	q := c.consulta
	if q == "" {
		q = "…"
	}
	var b strings.Builder
	if len(c.trobades) == 0 {
		b.WriteString(dimStyle.Render("  " + strings.TrimSpace(T("cerca.cap"))))
	}
	const finestra = 6
	ini := 0
	if c.sel >= finestra {
		ini = c.sel - finestra + 1
	}
	w := ampleDialeg(ample) - 6
	for i := ini; i < len(c.trobades) && i < ini+finestra; i++ {
		t := strings.ReplaceAll(m.histEntries[c.trobades[i]], "\n", " ")
		t = ansi.Truncate(t, w, "…")
		if i == c.sel {
			b.WriteString(menuHL.Width(w+2).Render("▸ "+t) + "\n")
		} else {
			b.WriteString(dimStyle.Render("  "+t) + "\n")
		}
	}
	titol := T("cerca.etiqueta") + q
	if len(c.trobades) > 1 {
		titol += "  " + fmtPos(c.sel+1, len(c.trobades))
	}
	return dialeg("⌕ "+titol, b.String(), strings.TrimSpace(T("cerca.pistes")), ample)
}
