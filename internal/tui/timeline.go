package tui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"gregal/internal/agent"
	"gregal/internal/session"
	"gregal/internal/tools"
)

// timelineEvent és una operació de l'agent i el seu resultat. No es desa a la
// sessió: és una lent navegable sobre el torn actual, no una segona conversa.
type timelineEvent struct {
	// linia és la fila de la conversa on s'ha pintat la crida (-1 si no
	// se'n va pintar cap): el resultat hi torna a escriure a sobre.
	linia    int
	name     string
	args     string
	output   string
	failed   bool
	done     bool
	expanded bool
}

func timelineToSession(in []timelineEvent) []session.Activity {
	out := make([]session.Activity, 0, len(in))
	for _, e := range in {
		out = append(out, session.Activity{Name: e.name, Args: e.args, Output: e.output, Failed: e.failed, Done: e.done})
	}
	return out
}

func timelineFromSession(in []session.Activity) []timelineEvent {
	out := make([]timelineEvent, 0, len(in))
	for _, e := range in {
		out = append(out, timelineEvent{name: e.Name, args: e.Args, output: e.Output, failed: e.Failed, done: e.Done})
	}
	return out
}

func (m *Model) pushToolCall(name, args string) {
	m.push(toolCallLine(name, args))
	// linia és on s'ha pintat: el resultat hi tornarà a escriure a sobre
	// en comptes d'afegir-ne una de nova. Amb vint eines en un torn, dues
	// files per eina és la diferència entre veure el torn sencer i no.
	m.timeline = append(m.timeline, timelineEvent{name: name, args: args, linia: len(m.lines) - 1})
	m.timelineSel = len(m.timeline) - 1
}

func (m *Model) pushToolResult(name, output string, failed bool) {
	// Les eines poden acabar fora d'ordre. Casa el resultat amb l'última crida
	// pendent del mateix nom; si no hi és, conserva'l igualment com a esdeveniment.
	linia, args := -1, ""
	matched := false
	for i := len(m.timeline) - 1; i >= 0; i-- {
		if m.timeline[i].name == name && !m.timeline[i].done {
			m.timeline[i].output = output
			m.timeline[i].failed = failed
			m.timeline[i].done = true
			linia, args = m.timeline[i].linia, m.timeline[i].args
			matched = true
			break
		}
	}
	if !matched {
		m.timeline = append(m.timeline, timelineEvent{name: name, output: output, failed: failed, done: true, linia: -1})
	}
	m.timelineSel = len(m.timeline) - 1
	// La crida ja té la seva fila: s'hi escriu el resultat a sobre.
	if linia >= 0 && linia < len(m.lines) {
		m.lines[linia] = toolEstatLine(name, args, output, failed)
		if cos := toolResultCos(name, output, failed); cos != "" {
			m.push(cos)
			return
		}
		m.refresh()
		return
	}
	m.push(toolResultLine(name, output, failed))
}

func (m *Model) timelineMove(delta int) {
	if len(m.timeline) == 0 {
		return
	}
	m.timelineSel = (m.timelineSel + delta + len(m.timeline)) % len(m.timeline)
}

func (m *Model) timelineToggle() {
	if m.timelineSel < 0 || m.timelineSel >= len(m.timeline) {
		return
	}
	m.timeline[m.timelineSel].expanded = !m.timeline[m.timelineSel].expanded
}

func timelineOneLine(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes-1]) + "…"
	}
	return s
}

func (m Model) timelineBox(width int) string {
	var b strings.Builder
	done, failed := 0, 0
	for _, e := range m.timeline {
		if e.done {
			done++
		}
		if e.failed {
			failed++
		}
	}
	head := fmt.Sprintf("≋ activitat  %d/%d", done, len(m.timeline))
	if failed > 0 {
		head += fmt.Sprintf(" · %d errors", failed)
	}
	b.WriteString(headStyle.Render(head) + "\n")
	b.WriteString(faintStyle.Render("↑↓ navega · Enter desplega · Esc tanca") + "\n")
	if len(m.timeline) == 0 {
		b.WriteString(dimStyle.Render("  Encara no hi ha eines en aquest torn."))
	} else {
		start := max(0, m.timelineSel-4)
		end := min(len(m.timeline), start+7)
		if end-start < 7 {
			start = max(0, end-7)
		}
		for i := start; i < end; i++ {
			e := m.timeline[i]
			cursor := "  "
			if i == m.timelineSel {
				cursor = "▸ "
			}
			mark := warnStyle.Render("◌")
			if e.done {
				mark = okStyle.Render("✓")
			}
			if e.failed {
				mark = badStyle.Render("✗")
			}
			detail := timelineOneLine(e.args, max(18, width-24))
			row := cursor + mark + " " + toolNameStyle.Render(e.name)
			if detail != "" {
				row += dimStyle.Render("  " + detail)
			}
			b.WriteString(row + "\n")
			if e.expanded {
				out := strings.TrimSpace(e.output)
				if out == "" && !e.done {
					out = "en curs…"
				}
				if out == "" {
					out = "sense sortida"
				}
				out = capText(out, 1600)
				for _, line := range strings.Split(out, "\n") {
					b.WriteString(toolRailStyle.Render("│   ") + dimStyle.MaxWidth(max(width-10, 10)).Render(line) + "\n")
				}
			}
		}
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(onada).Padding(0, 1).Width(max(width-2, 10)).
		Render(strings.TrimRight(b.String(), "\n"))
}

// queueBox fa visible l'orientació que l'usuari ha escrit mentre l'agent
// treballa. Abans només hi havia un comptador a la barra i era impossible
// recordar què s'enviaria després.
func (m Model) queueBox(width int) string {
	if len(m.queued) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(headStyle.Render(fmt.Sprintf("↳ després  %d", len(m.queued))) + "\n")
	start := max(0, len(m.queued)-3)
	for i := start; i < len(m.queued); i++ {
		text := timelineOneLine(m.queued[i], max(18, width-12))
		b.WriteString(fmt.Sprintf("  %d · %s\n", i+1, text))
	}
	b.WriteString(faintStyle.Render("s’enviarà quan acabi el torn actual"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(voraApagada).Padding(0, 1).Width(max(width-2, 10)).
		Render(strings.TrimRight(b.String(), "\n"))
}

func timelinePath(args string) string {
	var v map[string]any
	if json.Unmarshal([]byte(args), &v) != nil {
		return ""
	}
	for _, key := range []string{"path", "file", "filename"} {
		if s, ok := v[key].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// cockpitDades és el que el cockpit resumeix del torn: els fitxers
// tocats (escriptures acabades) i l'última validació (un bash).
type cockpitDades struct {
	fitxers   []string
	validacio string // "pendent" o el nom de l'eina
	fallada   bool
}

func (m Model) cockpitDades() cockpitDades {
	d := cockpitDades{validacio: "pendent"}
	files := map[string]bool{}
	for _, e := range m.timeline {
		if e.done && !e.failed {
			switch e.name {
			case "write", "edit", "patch", "office_edit":
				if p := timelinePath(e.args); p != "" {
					files[p] = true
				}
			}
		}
		if e.done && (e.name == "bash" || e.name == "bash_output") {
			d.validacio = e.name
			d.fallada = e.failed
		}
	}
	for f := range files {
		d.fitxers = append(d.fitxers, f)
	}
	sort.Strings(d.fitxers)
	return d
}

// cockpitBox és el cockpit com a caixa sobre el composer, per a terminals
// de menys de 120 columnes (Ctrl+T). Sense fila de pistes: les tecles són
// a ?.
func (m Model) cockpitBox(width int) string {
	var b strings.Builder
	done, total := tools.TodoStats()
	b.WriteString(headStyle.Render(fmt.Sprintf("✓ tasca  %d/%d", done, total)) + "\n")
	if total == 0 {
		b.WriteString(dimStyle.Render("  Cap pas estructurat encara.") + "\n")
	} else {
		for _, it := range tools.TodoList() {
			b.WriteString("  " + todoFila(it) + "\n")
		}
	}
	d := m.cockpitDades()
	if len(d.fitxers) > 0 {
		b.WriteString(projectStyle.Render(fmt.Sprintf("▣ canvis  %d", len(d.fitxers))) + "\n")
		for i, f := range d.fitxers {
			if i == 3 {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  … i %d més", len(d.fitxers)-3)) + "\n")
				break
			}
			b.WriteString("  " + m.rutaCurta(f, max(18, width-10)) + "\n")
		}
	}
	switch {
	case d.validacio == "pendent":
		b.WriteString(dimStyle.Render("◇ validació  pendent") + "\n")
	case d.fallada:
		b.WriteString(badStyle.Render("✗ validació  "+d.validacio) + "\n")
	default:
		b.WriteString(okStyle.Render("✓ validació  "+d.validacio) + "\n")
	}
	if len(m.queued) > 0 {
		b.WriteString(warnStyle.Render(fmt.Sprintf("↳ després  %d %s", len(m.queued), plural(len(m.queued), "instrucció", "instruccions"))) + "\n")
	}
	// El revisor automàtic: abans era una insignia a la barra d'estat, que
	// ja anava plena. Aquí diu el mode i prou.
	if m.autoVerify() {
		b.WriteString(okStyle.Render("◇ revisor  "+m.cfg.Verify.Mode) + "\n")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(green).Padding(0, 1).Width(max(width-2, 10)).
		Render(strings.TrimRight(b.String(), "\n"))
}

// todoFila pinta un pas de la checklist: marca segons l'estat i el títol
// apagat quan ja està fet.
func todoFila(it tools.TodoItem) string {
	mark, title := dimStyle.Render("☐"), it.Title
	switch it.Status {
	case "done":
		mark, title = okStyle.Render("✓"), dimStyle.Render(it.Title)
	case "working":
		mark = warnStyle.Render("◌")
	}
	return mark + " " + title
}

// cockpitColumna és el cockpit com a columna a la dreta de tot, quan el
// terminal fa 120 columnes o més: sense vora, una línia vertical a
// l'esquerra i títols de secció apagats. Fa exactament `alt` files,
// perquè s'enganxa a la vista sencera, i `ample` columnes.
func (m Model) cockpitColumna(ample, alt int) string {
	w := ample - 3 // un espai, la línia i un espai
	titol := func(s string) string { return faintStyle.Render(s) }
	var files []string

	done, total := tools.TodoStats()
	files = append(files, titol(fmt.Sprintf("tasca  %d/%d", done, total)))
	if total == 0 {
		files = append(files, dimStyle.Render("cap pas encara"))
	} else {
		for _, it := range tools.TodoList() {
			it.Title = timelineOneLine(it.Title, w-2)
			files = append(files, todoFila(it))
		}
	}

	d := m.cockpitDades()
	files = append(files, "", titol(fmt.Sprintf("canvis  %d", len(d.fitxers))))
	for i, f := range d.fitxers {
		if i == 5 {
			files = append(files, dimStyle.Render(fmt.Sprintf("… i %d més", len(d.fitxers)-5)))
			break
		}
		files = append(files, m.rutaCurta(f, w))
	}

	if len(m.queued) > 0 {
		files = append(files, "", titol(fmt.Sprintf("cua  %d", len(m.queued))))
		for i := max(0, len(m.queued)-3); i < len(m.queued); i++ {
			files = append(files, dimStyle.Render(timelineOneLine(m.queued[i], w)))
		}
	}

	// El context, aquí sí amb mesurador: a la barra només hi ha el tant
	// per cent. Sense animacions la marea és quieta.
	files = append(files, "", titol("context"))
	if fin := m.finestraAgent(); fin > 0 {
		pct := m.promptEst * 100 / fin
		frame := 0
		if m.animacions {
			frame = m.visualFrame()
		}
		st := okStyle
		switch {
		case pct >= 80:
			st = badStyle
		case pct >= 50:
			st = warnStyle
		}
		files = append(files, st.Render(fmt.Sprintf("%d%%", pct))+"  "+tideGauge(pct, min(16, w-6), frame))
	} else {
		files = append(files, dimStyle.Render("finestra desconeguda"))
	}

	files = append(files, "", titol("validació"))
	switch {
	case d.validacio == "pendent":
		files = append(files, dimStyle.Render("pendent"))
	case d.fallada:
		files = append(files, badStyle.Render("✗ "+d.validacio))
	default:
		files = append(files, okStyle.Render("✓ "+d.validacio))
	}

	if m.autoVerify() {
		files = append(files, "", titol("revisor"), okStyle.Render(m.cfg.Verify.Mode))
	}

	if len(files) > alt {
		files = files[:alt]
	}
	for len(files) < alt {
		files = append(files, "")
	}
	rail := lipgloss.NewStyle().Foreground(voraApagada).Render("│")
	for i, f := range files {
		if pad := w - lipgloss.Width(f); pad > 0 {
			f += strings.Repeat(" ", pad)
		}
		files[i] = " " + rail + " " + f
	}
	return strings.Join(files, "\n")
}

func (m Model) runSummaryLine() string {
	if len(m.timeline) == 0 {
		return ""
	}
	done, failed := 0, 0
	files := map[string]bool{}
	for _, e := range m.timeline {
		if e.done {
			done++
		}
		if e.failed {
			failed++
		}
		if e.done && !e.failed {
			switch e.name {
			case "write", "edit", "patch", "office_edit":
				if p := timelinePath(e.args); p != "" {
					files[p] = true
				}
			}
		}
	}
	parts := []string{fmt.Sprintf("%d %s", done, plural(done, "eina", "eines"))}
	if len(files) > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", len(files), plural(len(files), "fitxer", "fitxers")))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", failed, plural(failed, "error", "errors")))
	}
	if !m.turnStart.IsZero() {
		parts = append(parts, elapsedShort(time.Since(m.turnStart)))
	}
	r := m.roleRef()
	if usd, ok := agent.CostUSD(r.Provider, r.Model, m.tokUp, m.tokDown); ok {
		parts = append(parts, agent.FmtCost(usd))
	}
	return "resum del torn · " + strings.Join(parts, " · ")
}

// rutaCurta és una ruta per al cockpit: relativa al projecte i, si encara
// no hi cap, tallada per l'esquerra. «C:\\Users\\usera\\gregal-…» catorze
// vegades no deia res; «src/joc.js» sí.
func (m Model) rutaCurta(f string, w int) string {
	f = filepath.ToSlash(f)
	arrels := []string{filepath.ToSlash(m.cwd)}
	// La forma del Git Bash de Windows: C:/Users/x → /c/Users/x.
	if len(arrels[0]) > 2 && arrels[0][1] == ':' {
		arrels = append(arrels, "/"+strings.ToLower(arrels[0][:1])+arrels[0][2:])
	}
	for _, a := range arrels {
		a = strings.TrimRight(a, "/")
		if a != "" && strings.HasPrefix(f, a+"/") {
			f = f[len(a)+1:]
			break
		}
	}
	if r := []rune(f); len(r) > w && w > 1 {
		resta := string(r[len(r)-(w-1):])
		// Es talla a una barra: «…g/fitxer.go» amb mig segment no s'entén.
		if i := strings.Index(resta, "/"); i > 0 {
			resta = resta[i:]
		}
		f = "…" + resta
	}
	return f
}
