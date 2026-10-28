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
	m.planFinal, m.planParcial, m.planMotiu = false, false, ""
	m.pendingPlan = ""
	m.planHist = []llm.Message{{Role: "user", Content: task}}
	m.push(userLine("◇ " + task))
	mm, cmd := m.dispatchPlanStep()
	return mm, tea.Batch(cmd, tick())
}

// planLimit és el pressupost d'una exploració (agent.PassosPla: el de
// l'agent, amb un mínim per a revisions que llegeixen molts fitxers).
func (m Model) planLimit() int { return agent.PassosPla(m.cfg.Agent.MaxSteps) }

// dispatchPlanStep envia un pas d'exploració. En arribar al límit no
// s'atura en sec: demana la millor resposta amb el que s'ha vist.
func (m Model) dispatchPlanStep() (tea.Model, tea.Cmd) {
	if m.planSteps >= m.planLimit() {
		return m.dispatchPlanFinal(agent.MotiuPassos)
	}
	m.planSteps++
	m.planHist = agent.AmbAvisPassos(m.planHist, m.planSteps, m.planLimit())
	m.busy = true
	m.status = fmt.Sprintf(T("plan.planificant"), m.planSteps, m.planLimit())
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

// dispatchPlanFinal demana la resposta final de l'exploració SENSE eines:
// el pla (o les troballes) amb el que s'ha reunit fins ara. Ve del
// pressupost exhaurit (motiu = agent.MotiuPassos) o d'un error a mig camí.
func (m Model) dispatchPlanFinal(motiu string) (tea.Model, tea.Cmd) {
	m.planFinal = true
	m.planMotiu = motiu
	m.busy = true
	m.status = T("est.sintesi")
	if motiu == agent.MotiuPassos {
		m.push(warnStyle.Render("⚠ " + fmt.Sprintf(T("plan.passosEsgotats"), m.planLimit())))
	}
	hist := append([]llm.Message{{Role: "system", Content: m.planSysPrompt()}}, m.planHist...)
	hist = append(hist, llm.Message{Role: "user", Content: agent.PromptFinalExploracio(m.cfg.Lang())})
	m.push(workRail(dimStyle.Render("◌ " + T("est.escrivint"))))
	m.streamLine = len(m.lines) - 1
	cmd, cancel := m.sintesiCmd(hist)
	m.cancel = cancel
	m.cancelRequested = false
	return m, cmd
}

// passPlaFinal tanca l'exploració amb la resposta final. Si el model
// tampoc respon, el resum mecànic del que s'ha mirat: mai un torn buit.
func (m Model) passPlaFinal(msg agentSintesiMsg) tea.Model {
	m.comptaTokens(0, 0, msg.text, nil)
	if m.streamLine >= 0 && m.streamLine < len(m.lines) {
		m.lines[m.streamLine] = workRail(dimStyle.Render("◌ " + tailRunes(msg.text, 160)))
	}
	text := agent.NetejaFinal(msg.text)
	if msg.err != nil || text == "" {
		motiu := m.planMotiu
		if msg.err != nil {
			motiu += "; " + msg.err.Error()
		}
		text = agent.ResumDeFeina(m.planHist, motiu, m.cfg.Lang())
	}
	m.planParcial = true
	return m.finishPlan(text)
}

// continuePlan reprèn una exploració parcial amb pressupost nou i la
// conversa intacta (tecla [c] a la targeta del pla parcial).
func (m Model) continuePlan() (tea.Model, tea.Cmd) {
	m.pendingPlan = ""
	m.planning = true
	m.planSteps = 0
	m.planRetries = 0
	m.planFinal, m.planParcial, m.planMotiu = false, false, ""
	m.planHist = agent.AfegeixUsuari(m.planHist, agent.PromptContinuaExploracio)
	m.push(systemLine(fmt.Sprintf(T("plan.continuant"), m.planLimit())))
	mm, cmd := m.dispatchPlanStep()
	return mm, tea.Batch(cmd, tick())
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
	if m.planParcial {
		m.status = T("plan.llestParcial")
		m.push(planCardParcial(reply, m.vp.Width, m.planMotiu, m.planLimit()))
		return m
	}
	m.status = T("plan.llest")
	m.push(planCard(reply, m.vp.Width))
	return m
}

// planCardParcial és la targeta d'un pla que el pressupost (o un error) ha
// tallat: ho diu al títol i ofereix continuar.
func planCardParcial(reply string, width int, motiu string, limit int) string {
	nota := fmt.Sprintf(T("plan.parcialPassos"), limit)
	if motiu != agent.MotiuPassos {
		nota = fmt.Sprintf(T("plan.parcialError"), motiu)
	}
	return targetaPla(reply, width, T("plan.titolParcial"), nota, T("plan.opcionsParcial"))
}

// planCard pinta el pla amb les accions disponibles.
// planCard és la targeta del pla, a l'amplada de la conversa: els passos
// llargs s'embolcallen dins de la vora en comptes de sortir-ne.
func planCard(reply string, width int) string {
	return targetaPla(reply, width, T("plan.titol"), T("plan.revisa"), T("plan.opcions"))
}

// targetaPla pinta la targeta d'un pla amb títol, nota i opcions.
func targetaPla(reply string, width int, titolText, nota, opcions string) string {
	cos := ""
	for _, linia := range strings.Split(strings.TrimSpace(reply), "\n") {
		cos += "  " + linia + "\n"
	}
	titol := goalTitleStyle.Render("◇ "+titolText) +
		dimStyle.Render("  "+nota)
	return goalBoxStyle.Width(max(width-2, 20)).Render(titol + "\n" + cos +
		dimStyle.Render(opcions))
}

// answerPlan resol el pla pendent: e = executa, d = descarta.
func (m *Model) answerPlan(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "e":
		pla := m.pendingPlan
		m.pendingPlan = ""
		m.planParcial = false
		m.push(okStyle.Render("◇ " + T("plan.executant")))
		// Els passos numerats del pla passen al checklist abans de
		// començar: el cockpit els mostra des del primer segon i el model
		// només els ha d'anar marcant.
		m.seedTodos = agent.TodosDelPla(pla)
		// Executar és cosa d'Actua: des de Planifica hi passa, amb el
		// permís que hi havia.
		if agent.UIMode(m.mode) != agent.UIModeAct {
			m.setMode(m.currentActMode())
		}
		consigna := "Executa aquest pla pas a pas. Si un pas falla, adapta'l i continua"
		if len(m.seedTodos) > 0 {
			consigna += ". El checklist (todowrite) ja conté els passos numerats: quan n'acabis un, crida todowrite marcant-lo done i el següent working"
		}
		return m.startAgent(consigna + ":\n\n" + pla)
	default:
		m.pendingPlan = ""
		m.planParcial = false
		m.status = T("barra.llest")
		m.push(systemLine(T("plan.descartat")))
		mm := *m
		return mm, nil
	}
}
