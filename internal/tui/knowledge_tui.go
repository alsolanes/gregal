package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/knowledge"
)

// /knowledge: les col·leccions de coneixement des del terminal. Sense
// arguments, la llista (globals, del projecte i de la sessió) amb quants
// documents té cadascuna; `/knowledge search <consulta>` prova la cerca.
// Crear col·leccions i enllaçar carpetes es fa a la pàgina Knowledge de la
// finestra (o via /api/knowledge).
//
// Al TUI, les eines s'executen amb l'etiqueta de sessió "agent" (vegeu
// agent.procSession): les col·leccions «de sessió» del terminal són
// aquestes.
const tuiKnowledgeSession = "agent"

func (m Model) knowledgeWhere() knowledge.Where {
	return knowledge.Where{Project: m.cwd, Session: tuiKnowledgeSession}
}

func (m Model) knowledgeCmd(arg string) (tea.Model, tea.Cmd) {
	arg = strings.TrimSpace(arg)
	w := m.knowledgeWhere()
	sub, rest, _ := strings.Cut(arg, " ")
	switch strings.ToLower(sub) {
	case "", "list", "llista":
		m.push(systemLine(knowledgeListText(w)))
	case "search", "cerca":
		q := strings.TrimSpace(rest)
		if q == "" {
			m.push(systemLine(T("kb.usage")))
			return m, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		hits, err := knowledge.Search(ctx, w, q, knowledge.SearchOpts{K: 8})
		if err != nil {
			m.push(warnStyle.Render("⚠ " + err.Error()))
			return m, nil
		}
		m.push(systemLine(knowledgeHitsText(q, hits)))
	default:
		m.push(systemLine(T("kb.usage")))
	}
	return m, nil
}

func knowledgeListText(w knowledge.Where) string {
	list, err := knowledge.List(w)
	if err != nil {
		return "⚠ " + err.Error()
	}
	if len(list) == 0 {
		return T("kb.empty")
	}
	var b strings.Builder
	b.WriteString(T("kb.title"))
	for _, c := range list {
		state := ""
		if !c.Enabled {
			state = " · " + T("kb.disabled")
		}
		docs := T("kb.notIndexed")
		if ix, _ := knowledge.LoadIndex(w, c); ix != nil {
			st := ix.Stats()
			docs = fmt.Sprintf(T("kb.docs"), st.Docs, st.Chunks)
		}
		if p := knowledge.ProgressOf(c.ID); p.Running {
			docs = fmt.Sprintf(T("kb.indexing"), p.Done, p.Total)
		}
		fmt.Fprintf(&b, "\n  %s [%s] · %s%s", c.Name, c.Scope, docs, state)
		if c.Description != "" {
			fmt.Fprintf(&b, "\n    %s", c.Description)
		}
	}
	return b.String()
}

func knowledgeHitsText(q string, hits []knowledge.Hit) string {
	if len(hits) == 0 {
		return fmt.Sprintf(T("kb.noHits"), q)
	}
	var b strings.Builder
	fmt.Fprintf(&b, T("kb.hits"), len(hits), q)
	for i, h := range hits {
		loc := h.Path
		if h.Loc != "" {
			loc += " · " + h.Loc
		}
		snip := []rune(h.Snippet)
		if len(snip) > 240 {
			snip = append(snip[:240], '…')
		}
		fmt.Fprintf(&b, "\n%d. %s — %s\n   %s", i+1, loc, h.Collection, string(snip))
	}
	return b.String()
}
