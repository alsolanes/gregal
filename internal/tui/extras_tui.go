package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// Extres del TUI: /agents (subagents definits), /btw (pregunta al marge
// mentre l'agent treballa, sense tocar el torn), /recap (resum de les
// últimes peticions) i la proposta de notes del projecte en acabar un torn
// de codi (/keep per desar-les). Res no s'escriu sense que l'usuari ho
// demani.

// asideMsg és la resposta d'una crida al marge (btw, recap) o la proposta
// de notes (notes != nil).
type asideMsg struct {
	title string
	text  string
	notes []string
	err   error
}

// sideRef és el rol de les crides al marge: primer el rol actiu (el model
// amb què es treballa i que respon), amb el rol chat de reserva. Abans era
// sempre chat, i si el proveïdor ja no en servia el model, /recap i /btw
// fallaven cada cop (com al web: vegeu sideRoleRef).
func (m Model) sideRef() (config.Provider, config.Role, bool) {
	side, sr, sideOK := agent.SideRole(m.cfg)
	if r, ok := m.cfg.Roles[m.role]; ok && strings.TrimSpace(r.Model) != "" {
		if p, known := m.cfg.Providers[r.Provider]; known {
			if sideOK && side != m.role && strings.TrimSpace(r.FallbackModel) == "" && (sr.Provider != r.Provider || sr.Model != r.Model) {
				r.FallbackProvider, r.FallbackModel = sr.Provider, sr.Model
			}
			return p, r, true
		}
	}
	if !sideOK {
		return config.Provider{}, config.Role{}, false
	}
	p, known := m.cfg.Providers[sr.Provider]
	return p, sr, known
}

// hist és una còpia de la conversa amb el torn en curs (si n'hi ha).
func (m Model) sideHist() []llm.Message {
	if m.torn != nil && !m.torn.Acabat() {
		return append([]llm.Message(nil), m.torn.Hist...)
	}
	return append([]llm.Message(nil), m.convo...)
}

func (m Model) btwCmd(question string) (tea.Model, tea.Cmd) {
	question = strings.TrimSpace(question)
	if question == "" {
		m.push(systemLine(T("btw.us")))
		return m, nil
	}
	p, r, ok := m.sideRef()
	if !ok {
		m.push(systemLine(T("btw.senseModel")))
		return m, nil
	}
	m.push(dimStyle.Render("↳ /btw " + question))
	transcript := agent.TranscriptDigest(m.sideHist(), 24000)
	cfg, client := m.cfg, m.client
	return m, func() tea.Msg {
		out, err := agent.AnswerSide(context.Background(), client, cfg, p, r, question, transcript)
		return asideMsg{title: T("btw.titol"), text: out, err: err}
	}
}

func (m Model) recapCmd(arg string) (tea.Model, tea.Cmd) {
	n := 3
	if a := strings.TrimSpace(arg); a != "" {
		v, err := strconv.Atoi(a)
		if err != nil || v < 1 || v > 50 {
			m.push(systemLine(T("recap.us")))
			return m, nil
		}
		n = v
	}
	part := agent.RecapTurns(m.sideHist(), n)
	if len(part) == 0 {
		m.push(systemLine(T("recap.buit")))
		return m, nil
	}
	p, r, ok := m.sideRef()
	if !ok {
		m.push(systemLine(T("btw.senseModel")))
		return m, nil
	}
	activity := agent.TranscriptDigest(part, 24000)
	cfg, client := m.cfg, m.client
	return m, func() tea.Msg {
		out, err := agent.Recap(context.Background(), client, cfg, p, r, activity)
		return asideMsg{title: T("recap.titol"), text: out, err: err}
	}
}

func (m Model) agentsList() string {
	defs := agent.LoadSubagents(m.cwd)
	if len(defs) == 0 {
		return T("agents.cap")
	}
	var b strings.Builder
	b.WriteString(T("agents.titol"))
	for _, d := range defs {
		access := T("agents.lectura")
		if d.Mode == agent.ModeCode {
			access = T("agents.escriu")
		}
		fmt.Fprintf(&b, "\n%s (%s, %s) — %s", d.Name, access, d.Source, d.Description)
		if d.Model != "" {
			b.WriteString(" · " + d.Model)
		}
		if len(d.Tools) > 0 {
			b.WriteString(" · " + strings.Join(d.Tools, ", "))
		}
	}
	return b.String()
}

// memoryCmd demana les notes duradores d'un torn de codi acabat (en segon
// pla; la resposta arriba com a asideMsg amb notes).
func (m Model) memoryCmd() tea.Cmd {
	if m.torn == nil || m.serviceClient != nil {
		return nil
	}
	hist := append([]llm.Message(nil), m.torn.Hist...)
	if !agent.ShouldSuggestMemory(m.mode, m.torn.Execucions(), m.torn.Esgotat() || m.torn.Fallit(), hist) {
		return nil
	}
	p, r, ok := m.sideRef()
	if !ok {
		return nil
	}
	cfg, client, cwd := m.cfg, m.client, m.cwd
	return func() tea.Msg {
		notes, err := agent.SuggestMemory(context.Background(), client, cfg, p, r, hist, config.Notes(cwd, ""))
		if err != nil || len(notes) == 0 {
			return nil
		}
		return asideMsg{notes: notes}
	}
}

// onAside pinta el que torna una crida al marge.
func (m Model) onAside(msg asideMsg) (tea.Model, tea.Cmd) {
	if msg.notes != nil {
		m.memNotes = msg.notes
		var b strings.Builder
		b.WriteString(T("mem.proposta"))
		for i, n := range msg.notes {
			fmt.Fprintf(&b, "\n  %d. %s", i+1, n)
		}
		b.WriteString("\n" + T("mem.pista"))
		m.push(systemLine(b.String()))
		return m, nil
	}
	if msg.err != nil {
		m.push(warnStyle.Render("⚠ " + msg.title + ": " + msg.err.Error()))
		return m, nil
	}
	m.push(dimStyle.Render("↳ " + msg.title))
	m.push(assistantMD(msg.text, m.vp.Width))
	return m, nil
}

// keepCmd desa (o descarta) les notes proposades: /keep [n|all|none].
func (m Model) keepCmd(arg string) (tea.Model, tea.Cmd) {
	if len(m.memNotes) == 0 {
		m.push(systemLine(T("mem.capProposta")))
		return m, nil
	}
	arg = strings.ToLower(strings.TrimSpace(arg))
	var pick []string
	switch arg {
	case "none", "cap":
		m.memNotes = nil
		m.push(systemLine(T("mem.descartades")))
		return m, nil
	case "", "all", "totes":
		pick = m.memNotes
	default:
		i, err := strconv.Atoi(arg)
		if err != nil || i < 1 || i > len(m.memNotes) {
			m.push(systemLine(T("mem.pista")))
			return m, nil
		}
		pick = []string{m.memNotes[i-1]}
	}
	saved := 0
	for _, n := range pick {
		if err := config.Note(m.cwd, n); err == nil || config.IsDuplicateNote(err) {
			saved++
		} else {
			m.push(warnStyle.Render("⚠ " + err.Error()))
		}
	}
	m.memNotes = nil
	m.push(systemLine(fmt.Sprintf(T("mem.desades"), saved, config.ProjectMemoryPath(m.cwd))))
	return m, nil
}
