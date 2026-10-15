package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// planInstructions diuen al model com explorar: només lectura, resposta en
// forma de pla numerat. És el que Claude Code en diu plan mode.
const planInstructions = `PLAN MODE: do not modify anything, only explore.

Rules:
- Use read, glob, grep and web_* to understand the task.
- NEVER call write or edit (they are blocked in this mode).
- When you have enough information, reply with the plan only, in this format:
  1. First concrete step (files involved)
  2. Second step...
  VERIFICATION: how to check it (tests, build...)
- Be concise: actionable steps, no prose.`

// planSysPrompt compon el prompt de planificació (consulta + memòria). La
// llengua de la resposta la diu la directiva, que queda al final.
func (m Model) planSysPrompt() string {
	p := agent.PromptFor(m.cfg.SystemPrompt(), agent.ModeInspect)
	return agent.AmbContext(p, agent.ContextProjecte(m.cwd), planInstructions)
}

// startPlan engega l'exploració d'un pla per una tasca (sense escriure).
func (m *Model) startPlan(task string) (tea.Model, tea.Cmd) {
	if m.planning || m.agentActive || m.busy {
		m.push(systemLine(T("plan.feinaEnMarxa")))
		mm := *m
		return mm, nil
	}
	if strings.TrimSpace(task) == "" {
		m.push(systemLine(T("plan.us")))
		mm := *m
		return mm, nil
	}
	m.planning = true
	m.planSteps = 0
	m.planRetries = 0
	m.pendingPlan = ""
	m.planHist = []llm.Message{{Role: "user", Content: task}}
	m.push(userLine("◇ " + task))
	mm, cmd := m.dispatchPlanStep()
	return mm, tea.Batch(cmd, tick())
}

// dispatchPlanStep envia un pas d'exploració (mateix sostre que l'agent).
func (m Model) dispatchPlanStep() (tea.Model, tea.Cmd) {
	if m.planSteps >= m.cfg.Agent.MaxSteps {
		m.planning = false
		m.busy = false
		m.status = T("barra.llest")
		m.push(warnStyle.Render(fmt.Sprintf(T("plan.limit"), m.cfg.Agent.MaxSteps)))
		return m, nil
	}
	m.planSteps++
	m.busy = true
	m.status = fmt.Sprintf(T("plan.planificant"), m.planSteps, m.cfg.Agent.MaxSteps)
	hist := append([]llm.Message{{Role: "system", Content: m.planSysPrompt()}}, m.planHist...)
	m.promptEst = llm.EstimateTokens(hist)
	// El pla també ensenya el text en directe (H0), com l'agent.
	s := nouStreamer()
	m.stream = s
	m.streamShown = ""
	m.push(workRail(dimStyle.Render("◌ " + T("est.escrivint"))))
	m.streamLine = len(m.lines) - 1
	cmd, cancel := m.agentStepCmd(hist, s)
	m.cancel = cancel
	m.cancelRequested = false
	return m, cmd
}

// planStepMsg processa un pas de planificació: les lectures passen soles,
// les escriptures i aprovacions es registren com a no disponibles (mai
// demanen permís: planificar no fa popups).
func (m Model) planStepMsg(msg agentStepMsg) (tea.Model, tea.Cmd) {
	if len(msg.calls) == 0 {
		return m.finishPlan(msg.content), nil
	}
	var execs []llm.ToolCall
	for _, c := range msg.calls {
		m.push(toolCallLine(c.Function.Name, c.Function.Arguments))
		dec, reason := m.policy.Decide(agent.ModeInspect, c.Function.Name, c.Function.Arguments)
		switch dec {
		case "deny":
			m.push(toolResultLine(c.Function.Name, T("plan.noDisponible")+reason, true))
			m.planHist = append(m.planHist, llm.Message{
				Role: "tool", Content: "TOOL NOT AVAILABLE WHILE PLANNING: " + reason,
				ToolCallID: c.ID, Name: c.Function.Name,
			})
		default:
			execs = append(execs, c)
		}
	}
	if len(execs) > 0 {
		m.busy = true
		m.status = fmt.Sprintf(T("plan.explorant"), m.planSteps)
		return m, func() tea.Msg {
			res := agent.RunCalls(execs, nil, func(_ int, c llm.ToolCall) toolRes {
				out, imgs, err := agent.ExecIn("", m.cwd, c.Function.Name, c.Function.Arguments)
				if err != nil {
					out = "ERROR: " + err.Error()
				}
				return toolRes{call: c, out: out, imgs: imgs}
			})
			return agentExecMsg{results: res}
		}
	}
	return m.dispatchPlanStep()
}

// finishPlan tanca l'exploració i presenta el pla per aprovar.
func (m Model) finishPlan(reply string) tea.Model {
	m.planning = false
	m.busy = false
	m.planHist = append(m.planHist, llm.Message{Role: "assistant", Content: reply})
	if strings.TrimSpace(reply) == "" {
		m.status = T("barra.llest")
		m.push(systemLine(T("plan.buit")))
		return m
	}
	m.pendingPlan = reply
	m.status = T("plan.llest")
	m.push(planCard(reply, m.vp.Width))
	return m
}

// planCard pinta el pla amb les accions disponibles.
// planCard és la targeta del pla, a l'amplada de la conversa: els passos
// llargs s'embolcallen dins de la vora en comptes de sortir-ne.
func planCard(reply string, width int) string {
	cos := ""
	for _, linia := range strings.Split(strings.TrimSpace(reply), "\n") {
		cos += "  " + linia + "\n"
	}
	titol := goalTitleStyle.Render("◇ "+T("plan.titol")) +
		dimStyle.Render("  "+T("plan.revisa"))
	return goalBoxStyle.Width(max(width-2, 20)).Render(titol + "\n" + cos +
		dimStyle.Render(T("plan.opcions")))
}

// answerPlan resol el pla pendent: e = executa, d = descarta.
func (m *Model) answerPlan(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "e":
		pla := m.pendingPlan
		m.pendingPlan = ""
		m.push(okStyle.Render("◇ " + T("plan.executant")))
		// Els passos numerats del pla passen al checklist abans de
		// començar: el cockpit els mostra des del primer segon i el model
		// només els ha d'anar marcant.
		m.seedTodos = agent.TodosDelPla(pla)
		consigna := "Executa aquest pla pas a pas. Si un pas falla, adapta'l i continua"
		if len(m.seedTodos) > 0 {
			consigna += ". El checklist (todowrite) ja conté els passos numerats: quan n'acabis un, crida todowrite marcant-lo done i el següent working"
		}
		return m.startAgent(consigna + ":\n\n" + pla)
	default:
		m.pendingPlan = ""
		m.status = T("barra.llest")
		m.push(systemLine(T("plan.descartat")))
		mm := *m
		return mm, nil
	}
}
