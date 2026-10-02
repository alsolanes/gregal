package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"

	tea "github.com/charmbracelet/bubbletea"
)

// compactDoneMsg arriba quan el resum de compactació està llest.
type compactDoneMsg struct {
	summary string
	before  int // missatges que hi havia
	err     error
}

// compactCmd demana el resum al model (sense eines, amb failover).
func (m Model) compactCmd() (tea.Cmd, context.CancelFunc) {
	r := m.roleRef()
	p := m.cfg.Providers[r.Provider]
	convo := append([]llm.Message(nil), m.convo...)
	if m.compacted != "" {
		convo = append([]llm.Message{{Role: "user",
			Content: "[Resum de la conversa anterior]\n" + m.compacted}}, convo...)
	}
	before := len(m.convo)
	c := m.client
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	return func() tea.Msg {
		defer cancel()
		out, err := agent.Summarize(ctx, c,
			m.cfg.PrimTarget(p, r), m.cfg.FallbackTarget(r), convo)
		return compactDoneMsg{summary: out, before: before, err: err}
	}, cancel
}

// maybeCompactCmd compacta si l'estimació supera el 75% del pressupost de
// prompt (finestra menys la reserva de resposta). Es crida en acabar un
// torn (xat o agent). Força amb /compact.
func (m Model) maybeCompactCmd(force bool) (tea.Model, tea.Cmd) {
	if len(m.convo) <= 4 {
		return m, nil
	}
	r := m.roleRef()
	if m.agentRole != "" {
		if ar, ok := m.cfg.Roles[m.agentRole]; ok {
			r = ar
		}
	}
	budget := agent.Budget(m.cfg, r)
	hist := append([]llm.Message{{Role: "system", Content: m.sysPrompt()}}, m.convo...)
	if !force && !agent.NeedsCompact(llm.EstimateTokens(hist), budget) {
		return m, nil
	}
	m.push(dimStyle.Render(T("compact.fent") + fmt.Sprint(len(m.convo)) + " missatges)…"))
	m.busy = true
	m.status = "compactant…"
	cmd, cancel := m.compactCmd()
	m.cancel = cancel
	m.cancelRequested = false
	return m, cmd
}

// recalcCtx reestima el context real de la conversa (system + historial) i
// actualitza usedTokens i promptEst. El % de la barra surt de promptEst
// (View), no de usedTokens: cal tocar-los tots dos o el mesurador queda
// ancorat al valor previ a cada compactació. El resum ja viu dins de
// sysPrompt() (m.compacted): no s'ha de tornar a comptar com a missatge.
func (m *Model) recalcCtx() {
	hist := append([]llm.Message{{Role: "system", Content: m.sysPrompt()}}, m.convo...)
	est := llm.EstimateTokens(hist)
	m.usedTokens = est
	m.promptEst = est
}

func (m Model) handleCompactDone(msg compactDoneMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	m.cancel = nil
	if m.cancelRequested {
		m.cancelRequested = false
		m.status = "llest"
		m.push(systemLine(T("compact.cancel")))
		return m, nil
	}
	if msg.err != nil {
		m.status = "llest"
		// Si el resum falla (p.ex. timeout en model local), retallem igualment
		// per evitar bloquejar la conversa per excés de context.
		kept := agent.TrimKeep(m.convo, 4)
		m.convo = kept
		// El retall també canvia el context real: sense recalcular, la barra
		// es quedava amb el % vell (igual que al camí d'èxit).
		m.recalcCtx()
		m.push(systemLine(T("compact.fallida") + msg.err.Error()))
		return m, nil
	}
	kept := agent.TrimKeep(m.convo, 4)
	m.convo = kept
	// compactCmd ja envia el resum anterior al resumidor. El resultat nou el
	// substitueix: concatenar-los duplicava context a cada compactació i podia
	// fer créixer el "resum" indefinidament.
	m.compacted = msg.summary
	// La barra ctx es calculava amb usedTokens monòton: després de
	// compactar seguia en vermell (>=80%) per sempre encara que la
	// conversa real s'havia reduït. Es recomputa amb el que queda.
	m.recalcCtx()
	m.status = "llest"
	// La compactació també ha de ser visual. Abans només retallava m.convo:
	// el viewport conservava milers de línies antigues i semblava que /compact
	// no hagués fet res. Repintem el resum i els quatre missatges recents, que
	// són exactament el context que queda actiu.
	m.lines = nil
	m.vp.SetContent("")
	m.push(headStyle.Render("≋ " + T("compact.context")))
	m.push(assistantMD(msg.summary, m.vp.Width))
	for _, recent := range kept {
		switch recent.Role {
		case "user":
			m.push(userLine(agent.SenseMapa(recent.Content)))
		case "assistant":
			if strings.TrimSpace(recent.Content) != "" {
				m.push(assistantMD(recent.Content, m.vp.Width))
			}
		}
	}
	m.push(systemLine(fmt.Sprintf(T("compact.fet"),
		msg.before, len(kept), len(msg.summary))))
	return m, nil
}
