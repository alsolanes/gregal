package tui

// Estètica Claude Code per al TUI: preguntes seleccionables (AskUserQuestion),
// checklist de todos amb toggle, cua de missatges mentre l'agent treballa i
// síntesi final sense eines quan s'esgota el pressupost de passos.

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// questionPending és una pregunta del model pendent de resposta triable
// (fletxes + Enter, 1-4 o text lliure). call guarda la tool_call per
// respondre al loop. sel és l'opció ressaltada (0 = primera).
type questionPending struct {
	call    llm.ToolCall
	query   string
	options []tools.QuestionOption
	sel     int
}

// questionBox pinta la pregunta com a diàleg superposat, amb opcions
// numerades. L'opció ressaltada porta ▸ i és la que confirma l'Enter amb
// el composer buit; escriure-hi text continua sent resposta lliure.
// L'última fila del cos és el camp de text lliure: sense número (mai és
// triable amb fletxes) i mostra en directe el que escrius al composer
// (draft), perquè quedi clar on va la resposta pròpia.
func questionBox(q *questionPending, width int, draft string) string {
	w := ampleDialeg(width)
	var b strings.Builder
	for i, o := range q.options {
		n := fmt.Sprintf("%d", i+1)
		mark := "  "
		if i == q.sel {
			mark = okStyle.Render("▸") + " "
		}
		row := mark + n + " · " + o.Label
		if strings.TrimSpace(o.Description) != "" {
			row += dimStyle.Render(" — " + o.Description)
		}
		b.WriteString(row + "\n")
	}
	b.WriteString(campLliure(draft, w))
	return dialeg("? "+q.query, b.String(), strings.TrimSpace(fmt.Sprintf("1-%d%s", len(q.options), T("pregunta.pistes"))), width)
}

// campLliure és la fila de text lliure del seleccionable: mostra el que
// s'està escrivint al composer (una línia, retallada) o el suggeriment.
// No porta número perquè no és triable: l'Enter amb text l'envia tal qual.
func campLliure(draft string, width int) string {
	pla := strings.ReplaceAll(strings.TrimSpace(draft), "\n", " ")
	if pla == "" {
		return dimStyle.Render("✎ " + T("pregunta.lliure"))
	}
	ample := max(width-14, 10)
	if r := []rune(pla); len(r) > ample {
		pla = strings.TrimSpace(string(r[:ample])) + "…"
	}
	return dimStyle.Render("✎ ") + pla
}

// answerQuestion resol la pregunta: idx 0-based o text lliure ("").
// Buit + Esc = sense resposta: el model continua amb el seu criteri.
func (m *Model) answerQuestion(idx int, free string) (tea.Model, tea.Cmd) {
	q := m.pendingQ
	m.pendingQ = nil
	if q == nil {
		return m, nil
	}
	var answer string
	if strings.TrimSpace(free) != "" {
		answer = "Resposta lliure de l'usuari: " + strings.TrimSpace(free)
		m.push(okStyle.Render("✓ has respost (text): " + strings.TrimSpace(free)))
	} else if idx >= 0 && idx < len(q.options) {
		answer = "L'usuari ha triat: " + q.options[idx].Label
		if d := strings.TrimSpace(q.options[idx].Description); d != "" {
			answer += " (" + d + ")"
		}
		m.push(okStyle.Render("✓ has triat: " + q.options[idx].Label))
	} else {
		answer = "L'usuari no ha respost (Esc). Continua amb el teu millor criteri sense tornar a preguntar el mateix."
		m.push(systemLine(T("q.senseResposta")))
	}
	if m.torn != nil {
		m.pinta(m.torn.RepPregunta(answer))
	}
	return m.avanca()
}

// questionIndex retorna l'índex de la primera crida "question" (-1 si no n'hi ha).
func questionIndex(calls []llm.ToolCall) int {
	for i, c := range calls {
		switch c.Function.Name {
		case "question", "ask_question", "ask", "AskUserQuestion":
			return i
		}
	}
	return -1
}

// todosBox pinta la checklist actual (Ctrl+T la mostra/amaga).
func todosBox(width int) string {
	done, total := tools.TodoStats()
	if total == 0 {
		return faintStyle.Render(T("todos.buit"))
	}
	var b strings.Builder
	b.WriteString(headStyle.Render(fmt.Sprintf("✓ tasca %d/%d", done, total)) + "\n")
	for _, it := range tools.TodoList() {
		mark := dimStyle.Render("☐")
		row := it.Title
		switch it.Status {
		case "done":
			mark = okStyle.Render("✓")
			row = dimStyle.Render(it.Title)
		case "working":
			mark = warnStyle.Render("◌")
		}
		b.WriteString("  " + mark + " " + row + "\n")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(green).
		Padding(0, 1).
		// Aquesta caixa viu just sobre el composer i forma pila amb ell: el
		// menú i la confirmació ja fan tota l'amplada, i aquestes dues es
		// quedaven a 78 columnes. Dins d'un terminal ample, la pila quedava
		// escantonada per la dreta sense cap motiu.
		Width(max(width-2, 10)).
		Render(strings.TrimRight(b.String(), "\n"))
}

// thinkingBox pinta el raonament actiu dins d'un bloc ordenat i compacte sobre el composer.
func thinkingBox(think string, width int) string {
	t := strings.TrimSpace(think)
	if t == "" {
		return ""
	}
	count := len([]rune(t))
	head := warnStyle.Render(fmt.Sprintf("◌ raonant (%s caràcters)", llm.FmtCount(count)))
	availWidth := max(width-6, 20)
	tail := tailRunes(t, availWidth*2)
	lines := strings.Split(tail, "\n")
	if len(lines) > 2 {
		lines = lines[len(lines)-2:]
	}
	body := dimStyle.Render(strings.Join(lines, "\n"))
	content := head + "\n" + body
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(voraApagada).
		Padding(0, 1).
		Width(max(width-2, 10)).
		Render(content)
}

// extensioCmd demana al model si val la pena continuar en esgotar el
// pressupost: CONTINUA <n> o FINAL, sense eines (com la síntesi).
func (m Model) extensioCmd(hist []llm.Message) (tea.Cmd, context.CancelFunc) {
	r := m.rolAgent()
	p := m.cfg.Providers[r.Provider]
	c := m.client
	// CONTINUA o FINAL no demana raonar (think: auto el treu).
	ctx, cancel := context.WithTimeout(llm.WithThink(context.Background(), agent.ThinkPerPas(r.Think, agent.Pas{Ordre: agent.OrdreAmplia})), agent.TimeoutDecisio)
	return func() tea.Msg {
		defer cancel()
		out, _, err := c.ChatFO(ctx, m.cfg.PrimTarget(p, r), m.cfg.FallbackTarget(r), hist, r.Temperature, r.MaxTokens, nil)
		if err != nil {
			return agentExtensioMsg{err: err}
		}
		return agentExtensioMsg{resp: strings.TrimSpace(out)}
	}, cancel
}

// finalStepCmd demana la síntesi final sense eines quan s'esgota el
// pressupost (estil opencode/Claude: resumeix + què falta, mai buit).
func (m Model) sintesiCmd(hist []llm.Message) (tea.Cmd, context.CancelFunc) {
	r := m.rolAgent()
	p := m.cfg.Providers[r.Provider]
	c := m.client
	// Resumir la feina feta no demana raonar (think: auto el treu).
	ctx, cancel := context.WithTimeout(llm.WithThink(context.Background(), agent.ThinkPerPas(r.Think, agent.Pas{Ordre: agent.OrdreSintesi})), agent.TimeoutDecisio)
	return func() tea.Msg {
		defer cancel()
		out, _, err := c.ChatFO(ctx, m.cfg.PrimTarget(p, r), m.cfg.FallbackTarget(r), hist, r.Temperature, r.MaxTokens, nil)
		if err != nil {
			return agentSintesiMsg{err: err}
		}
		return agentSintesiMsg{text: strings.TrimSpace(out)}
	}, cancel
}

// boolStr adapta el flag de fallback al camp fb (nom del model o "").
// ChatFO retorna bool; el detall del model ja s'ha notificat via streamer,
// aquí només cal saber que hi ha hagut fallback per a la línia ↪.
func boolStr(fb bool) string {
	if fb {
		return "fallback"
	}
	return ""
}

// enqueue desa un missatge escrit mentre l'agent treballa (estil Claude:
// s'envia quan acaba el pas actual en comptes de perdre's).
func (m *Model) enqueue(text string) {
	// Doble Enter del mateix text: sense això s'encuava dos cops i la
	// tasca s'executava (i es desava a la conversa) duplicada.
	if n := len(m.queued); n > 0 && m.queued[n-1] == text {
		return
	}
	m.queued = append(m.queued, text)
	if len(m.queued) > 5 {
		m.queued = m.queued[len(m.queued)-5:]
	}
	m.push(systemLine(fmt.Sprintf(T("cua.encuat"), len(m.queued))))
}

// drainQueue envia el següent missatge encuat quan el TUI queda lliure.
// Es crida des del tick: un sol lloc, cap camí d'alliberament se'l deixa.
func (m Model) drainQueue() (tea.Model, tea.Cmd) {
	if len(m.queued) == 0 {
		return m, nil
	}
	// Amb menú obert (p. ex. accions de l'objectiu) no s'envia res sol:
	// el teclat és del menú i un submit pel darrere el trencaria.
	if m.busy || m.agentActive || m.planning || m.menu != nil || m.pending != nil || m.pendingQ != nil {
		return m, nil
	}
	text := m.queued[0]
	m.queued = m.queued[1:]
	m.push(systemLine("reprenent missatge encuat" + (map[bool]string{true: "", false: fmt.Sprintf(T("cua.mesEnCua"), len(m.queued))}[len(m.queued) == 0])))
	// Sense recordHist: l'Enter ja l'ha enregistrat en encuar-lo. Tornar-lo
	// a desar aquí duplicava entrades no consecutives ([A,B] → [A,B,A]) i
	// la ↑ passava per repetits: semblava que saltava o anava de dos en dos.
	if strings.HasPrefix(text, "/") {
		return m.runCommand(text)
	}
	return m.submitText(text)
}
