package tui

import (
	"fmt"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

func (m Model) statsBlock() string {
	r := m.roleRef()
	conversation := llm.EstimateTokens(m.convo)
	working := llm.EstimateTokens(m.histAgent())
	compact := llm.EstimateTokens([]llm.Message{{Role: "system", Content: m.compacted}})
	route := T("stats.rutaAuto")
	if m.rolePinned {
		route = T("stats.rutaFixada")
	}
	if m.agentRole != "" && m.agentRole != m.role {
		route = T("stats.rutaEscalada") + m.agentRole
	}
	var b strings.Builder
	b.WriteString(headStyle.Render("≋ "+T("stats.titol")) + "\n")
	b.WriteString(fmt.Sprintf(T("stats.model")+"\n", r.Provider, r.Model, route))
	if w, src := agent.WindowSource(m.cfg, r); w > 0 {
		var proven string
		switch src {
		case "config":
			proven = T("stats.fontConfig")
		case "model":
			proven = T("stats.fontModel")
		case "coneguda":
			proven = T("stats.fontConeguda")
		default:
			proven = T("stats.fontDefecte")
		}
		b.WriteString(fmt.Sprintf(T("stats.finestra")+"\n",
			llm.FmtCount(w), proven, llm.FmtCount(agent.Budget(m.cfg, r))))
	}
	// El prompt actual és el que la barra resumeix en percentatge; aquí
	// surt en tokens, al costat del que s'ha acumulat en tota la sessió.
	b.WriteString(fmt.Sprintf(T("stats.context")+"\n",
		llm.FmtCount(m.promptEst), llm.FmtCount(m.usedTokens), llm.FmtCount(conversation), llm.FmtCount(working), llm.FmtCount(compact)))
	b.WriteString(fmt.Sprintf(T("stats.transit"), llm.FmtCount(m.tokUp), llm.FmtCount(m.tokDown)))
	if m.tokCached > 0 {
		b.WriteString(" · cache " + llm.FmtCount(m.tokCached))
	}
	if usd, ok := agent.CostUSD(r.Provider, r.Model, m.tokUp, m.tokDown); ok {
		b.WriteString(" · " + agent.FmtCost(usd))
	}
	done, failed := 0, 0
	for _, e := range m.timeline {
		if e.done {
			done++
		}
		if e.failed {
			failed++
		}
	}
	b.WriteString(fmt.Sprintf("\n"+T("stats.activitat"), done, failed))
	return b.String()
}

var asciiReplacer = strings.NewReplacer(
	"≋", "=", "≈", "~", "∼", "~", "·", ".", "◆", "*", "◇", "o", "◌", "o",
	"✓", "OK", "✗", "X", "▸", ">", "▌", "|", "┃", "|", "│", "|", "█", "#",
	"▓", "#", "▒", "=", "░", "-", "▏", "|", "▎", "|", "▍", "|", "▋", "|",
	"▊", "|", "▉", "#", "↑", "up", "↓", "down", "↳", "->", "↕", "<>",
	"╭", "+", "╮", "+", "╰", "+", "╯", "+", "─", "-", "└", "+", "☐", "[ ]",
	"🔒", "[locked]", "🔓", "[open]", "…", "...", "−", "-", "⚠", "!",
)

func asciiView(s string) string { return asciiReplacer.Replace(s) }
