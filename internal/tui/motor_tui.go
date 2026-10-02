package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// El TUI com a conductor del motor (internal/agent/motor.go).
//
// Abans la màquina d'estats del torn vivia dins d'`Update`: sis casos de
// missatge, 1.100 línies i tretze comptadors al Model. Ara `Update` només
// alimenta el motor amb el que ha passat i executa l'ordre que en surt.
// Tot el que decideix (permisos, repeticions, reintents, pressupost,
// topalls) és al motor, que es prova sense xarxa ni terminal.

// agentSintesiMsg és la resposta a OrdreSintesi (final, sense eines).
type agentSintesiMsg struct {
	text string
	err  error
}

// pinta tradueix els events del motor a línies de la conversa.
func (m *Model) pinta(evs []agent.Event) {
	for _, e := range evs {
		text := e.Text
		if e.Clau != "" {
			text = fmt.Sprintf(T(e.Clau), e.Args...)
		}
		switch e.Tipus {
		case agent.EvCrida:
			if e.Eina == "todowrite" {
				m.showTodos = true
			}
			m.pushToolCall(e.Eina, e.EinaArgs)
		case agent.EvResultat:
			m.pushToolResult(e.Eina, text, e.Fallada)
		case agent.EvResposta:
			m.mostraResposta(text)
		case agent.EvDiff:
			// Dins del rail i dues columnes més endins: és la sortida de
			// l'edit, no una cosa a part de la feina.
			m.push(dinsRail(text))
		case agent.EvAvis:
			m.push(warnStyle.Render(text))
		case agent.EvError:
			m.push(systemLine(text))
		default:
			if strings.TrimSpace(text) != "" {
				m.push(dimStyle.Render(text))
			}
		}
	}
}

// mostraResposta pinta la resposta final a la línia en viu si encara hi
// és (el bloc de feina es converteix en la resposta) o a sota si no.
func (m *Model) mostraResposta(text string) {
	bloc := assistantMD(text, m.vp.Width)
	if m.streamLine >= 0 && m.streamLine < len(m.lines) {
		m.lines[m.streamLine] = bloc
		m.streamLine = -1
		m.refresh()
		return
	}
	m.push(bloc)
}

// histAgent és l'historial del torn viu (buit si no n'hi ha cap).
func (m Model) histAgent() []llm.Message {
	if m.torn == nil {
		return nil
	}
	return m.torn.Hist
}

// passosAgent i limitAgent són per a la barra d'estat.
func (m Model) passosAgent() int {
	if m.torn == nil {
		return 0
	}
	return m.torn.Passos()
}

func (m Model) limitAgent() int {
	if m.torn == nil {
		return m.cfg.Agent.MaxSteps
	}
	return m.torn.Limit()
}

// nouTorn construeix el motor per a una tasca.
func (m *Model) nouTorn(task string) *agent.Torn {
	hist := append(append([]llm.Message{}, m.convo...), llm.Message{Role: "user", Content: task})
	return agent.NouTorn(agent.OpcionsTorn{
		Cfg:       m.cfg,
		Pol:       m.policy,
		Mode:      m.mode,
		Tasca:     task,
		MaxSteps:  m.cfg.Agent.MaxSteps,
		Hist:      hist,
		SysPrompt: func() string { return m.sysPrompt() },
		Finestra: func() (int, int) {
			r := m.rolAgent()
			win := agent.Window(m.cfg, r)
			return win, agent.PromptBudget(win, r.MaxTokens)
		},
		Rol: func() (string, string) {
			r := m.rolAgent()
			return r.Provider, r.Model
		},
		Recordat: func(name, args string) bool {
			return m.remembers.Allowed("tui", agent.Sig(name, args))
		},
		AutoAprova: func() bool { return m.permissive },
	})
}

// avanca demana la següent ordre al motor i la converteix en feina.
func (m Model) avanca() (tea.Model, tea.Cmd) {
	if m.torn == nil {
		return m, nil
	}
	p := m.torn.Seguent()
	switch p.Ordre {
	case agent.OrdrePasModel:
		m.busy = true
		m.status = fmt.Sprintf("agent pas %d/%d…", p.Passos, p.Limit)
		hist := p.Hist
		m.promptEst = llm.EstimateTokens(hist)
		s := nouStreamer()
		m.stream = s
		m.streamShown = ""
		m.push(workRail(dimStyle.Render("◌ " + T("est.escrivint"))))
		m.streamLine = len(m.lines) - 1
		m.pasMecanic = p.Mecanic
		cmd, cancel := m.agentStepCmd(hist, s)
		m.cancel = cancel
		m.cancelRequested = false
		return m, cmd

	case agent.OrdreCompacta:
		m.busy = true
		m.status = T("est.compactantTorn")
		win, _ := m.finestraIBudget()
		m.push(dimStyle.Render(fmt.Sprintf(T("compact.torn"),
			llm.FmtCount(llm.EstimateTokens(p.Hist)), llm.FmtCount(win))))
		cmd, cancel := m.agentCompactCmd(p.Budget)
		m.cancel = cancel
		m.cancelRequested = false
		return m, cmd

	case agent.OrdreExecuta:
		m.busy = true
		m.status = fmt.Sprintf("agent pas %d: executant %d…", p.Passos, len(p.Calls))
		// Amb cancel actiu, Esc i Ctrl+C aturen l'eina en marxa.
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		m.cancelRequested = false
		cwd, calls := m.cwd, p.Calls
		return m, func() tea.Msg {
			defer cancel()
			// Lectures alhora, escriptures en ordre (agent.RunCalls).
			res := agent.RunCalls(calls, nil, func(_ int, c llm.ToolCall) toolRes {
				out, diff, imgs, err := execToolWithDiffCtx(ctx, cwd, c.Function.Name, c.Function.Arguments)
				if err != nil {
					out = "ERROR: " + err.Error()
				}
				return toolRes{call: c, out: out, diff: diff, imgs: imgs}
			})
			return agentExecMsg{results: res}
		}

	case agent.OrdreAprova:
		m.busy = false
		m.confirmIdx = 0
		call := p.Call
		m.pending = &pendingOp{
			agent:   true,
			desc:    call.Function.Name + " " + humanArgs(call.Function.Name, call.Function.Arguments, 300),
			preview: previewCanvi(m.cwd, call.Function.Name, call.Function.Arguments),
		}
		m.status = T("est.confirmaLlarg")
		// La fila de la crida ja l'ha pintada el motor (EvCrida) quan ha
		// decidit que calia permís: tornar-la a pintar en feia dues i
		// desquadrava la vista d'activitat.
		if m.permissive {
			return m.answerPending("s")
		}
		return m, nil

	case agent.OrdrePregunta:
		q, opts, err := tools.ParseQuestion(p.Call.Function.Arguments)
		if err != nil {
			// Ja validada pel motor; si igualment falla, es continua.
			m.pinta(m.torn.RepPregunta("PREGUNTA IL·LEGIBLE: continua amb el teu criteri."))
			return m.avanca()
		}
		m.pendingQ = &questionPending{call: p.Call, query: q, options: opts}
		m.busy = false
		m.status = T("est.triaOpcio")
		return m, nil

	case agent.OrdreAmplia:
		m.busy = true
		m.status = T("est.sintesi")
		m.push(dimStyle.Render(fmt.Sprintf(T("app.ampliaDemana"), p.Limit)))
		cmd, cancel := m.extensioCmd(p.Hist)
		m.cancel = cancel
		m.cancelRequested = false
		return m, cmd

	case agent.OrdreSintesi:
		m.busy = true
		m.status = T("est.sintesi")
		m.push(workRail(dimStyle.Render("◌ " + T("est.escrivint"))))
		m.streamLine = len(m.lines) - 1
		cmd, cancel := m.sintesiCmd(p.Hist)
		m.cancel = cancel
		m.cancelRequested = false
		return m, cmd

	case agent.OrdreCheckpoint:
		m.busy = true
		m.status = "autònom: verificant…"
		cmd, cancel := m.autonomousCheckpointCmd()
		m.cancel = cancel
		m.cancelRequested = false
		return m, cmd

	default: // OrdreAcaba, OrdreError, OrdreRes
		return m.tancaTorn()
	}
}

// finestraIBudget és la finestra del rol actiu i el pressupost de prompt.
func (m Model) finestraIBudget() (int, int) {
	r := m.rolAgent()
	win := agent.Window(m.cfg, r)
	return win, agent.PromptBudget(win, r.MaxTokens)
}

// tancaTorn fa el que va després del torn: resum, memòria de l'objectiu,
// verificació i compactació de la conversa.
func (m Model) tancaTorn() (tea.Model, tea.Cmd) {
	resposta := ""
	if m.torn != nil {
		resposta = m.torn.Resposta()
	}
	m.agentActive = false
	m.busy = false
	m.cancel = nil
	m.stream = nil
	m.streamShown = ""
	if strings.TrimSpace(resposta) != "" {
		m.convo = append(m.convo, llm.Message{Role: "assistant", Content: resposta})
		if m.mode == agent.ModeGoal {
			// En objectiu, la resposta final pot portar el bloc ```goal.
			m.recordGoal(resposta)
		}
	}
	tools.Active.MarkConvo(len(m.convo))
	if summary := m.runSummaryLine(); summary != "" {
		// Apagat i sense el «·» de sistema: és un peu de torn, no un avís.
		m.push(faintStyle.Render(summary))
	}
	if resposta != "" && m.autoVerify() && m.turnDidRealWork() {
		m.busy = true
		m.status = T("est.verificant")
		cmd, cancel := m.sendVerify()
		m.cancel = cancel
		m.cancelRequested = false
		return m, cmd
	}
	// La conversa es desa aquí, no només en sortir: tancar la finestra
	// (o una caiguda) feia perdre hores de feina, perquè l'autodesat
	// penjava de /quit i de Ctrl+C.
	m.autoSave()
	if m.torn != nil && m.torn.Fallit() {
		m.status = "error"
		return m, nil
	}
	m.status = T("barra.llest")
	return m.maybeCompactCmd(false)
}

// cancellaTorn atura el torn en marxa (Esc, Ctrl+C).
func (m Model) cancellaTorn() (tea.Model, tea.Cmd) {
	m.cancelRequested = false
	m.agentActive = false
	m.planning = false
	m.busy = false
	m.cancel = nil
	m.stream = nil
	m.streamShown = ""
	m.pending = nil
	m.pendingQ = nil
	if m.torn != nil {
		evs := m.torn.Cancella()
		if m.streamLine >= 0 && m.streamLine < len(m.lines) {
			m.lines[m.streamLine] = systemLine(T("est.agentCancel"))
			m.streamLine = -1
			m.refresh()
		} else {
			m.pinta(evs)
		}
	} else {
		m.push(systemLine(T("est.agentCancel")))
	}
	m.status = T("barra.llest")
	return m, nil
}

// comptaTokens actualitza el mesurador amb el que ha costat un pas.
func (m *Model) comptaTokens(promptTokens, cachedTokens int, content string, calls []llm.ToolCall) {
	if promptTokens > 0 {
		m.promptEst = promptTokens
	}
	m.tokCached += cachedTokens
	baixada := llm.EstimateTokens([]llm.Message{{Role: "assistant", Content: content, ToolCalls: calls}})
	m.usedTokens += m.promptEst + baixada
	m.tokUp += m.promptEst
	m.tokDown += baixada
}

// passPla processa un pas de l'exploració de pla. El pla no passa pel
// motor: és un bucle curt de només lectura, sense permisos ni
// pressupost ampliable, i barrejar-lo hi afegiria estats que no fa
// servir. Comparteix el missatge del pas i prou.
func (m Model) passPla(msg agentStepMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if llm.IsRetryable(msg.err) && m.planRetries < 2 {
			m.planRetries++
			m.planHist = append(m.planHist, llm.Message{Role: "user", Content: agent.GuiaErrorServidor(msg.err)})
			m.push(dimStyle.Render("↻ " + T("app.serverReintentCurt")))
			return m.dispatchPlanStep()
		}
		m.planning = false
		m.busy = false
		m.status = "error"
		m.push(systemLine("agent: error: " + msg.err.Error()))
		return m, nil
	}
	m.comptaTokens(msg.promptTokens, msg.cachedTokens, msg.content, msg.calls)
	if m.streamLine >= 0 && m.streamLine < len(m.lines) {
		if strings.TrimSpace(msg.content) == "" {
			m.lines[m.streamLine] = workRail(dimStyle.Render("◌ " + T("est.escrivint")))
		} else {
			m.lines[m.streamLine] = workRail(dimStyle.Render("◌ " + tailRunes(msg.content, 160)))
		}
	}
	m.planHist = append(m.planHist, llm.Message{Role: "assistant", Content: msg.content, ToolCalls: msg.calls})
	return m.planStepMsg(msg)
}

// rolAgent és el rol que executa l'agent ara mateix: el rol actiu o, si
// el router n'ha escalat un altre, el de l'escalada.
func (m Model) rolAgent() config.Role {
	r := m.roleRef()
	if m.agentActive && m.agentRole != "" {
		if ar, ok := m.cfg.Roles[m.agentRole]; ok {
			r = ar
		}
	}
	return r
}

// checkpointNum és el checkpoint autònom en curs (0 sense torn).
func (m Model) checkpointNum() int {
	if m.torn == nil {
		return 0
	}
	return m.torn.CheckpointNum()
}

// clausDelMotor són les claus de traducció que emet internal/agent (el
// motor treballa amb claus perquè no depèn de la i18n de cap client, i
// aquí es resolen). La llista també fa que el test de claus mortes les
// vegi: si no, semblarien sense ús.
var clausDelMotor = []string{
	"est.agentCancel", "est.cancellat",
	"compact.fet", "compact.fallida",
	"app.serverReintentCurt", "app.repetida", "app.todosPendents",
	"app.tornBuit", "app.ampliaSegueix", "app.ampliaPerChecklist", "app.passosEsgotats",
	"app.verifReintent", "app.senseAccionsNoves", "app.checkpoint", "app.ratxaVermella",
}

// intervalViu és cada quant es repinta el text que va arribant. A cada
// tick (60 ms) seria refer el markdown disset vegades per segon; glamour
// costa uns quants mil·lisegons i es notava.
const intervalViu = 150 * time.Millisecond

// textEnViu pinta com a markdown el text que el model va escrivint.
// Abans era una cua de 160 caràcters retallada per l'esquerra: amb
// paràgrafs la línia ballava i no es podia llegir res.
func (m *Model) textEnViu(text string) string {
	if m.streamShown != "" && time.Since(m.streamRender) < intervalViu {
		return m.streamShown // encara val el d'abans
	}
	m.streamRender = time.Now()
	return assistantMD(tancaBlocs(text), m.vp.Width)
}

// tancaBlocs tanca el bloc de codi que hagi quedat obert. Mentre el text
// arriba, el ``` d'obertura pot haver arribat i el de tancament no, i
// aleshores glamour pinta la resta de la resposta com si fos codi.
func tancaBlocs(text string) string {
	if strings.Count(text, "```")%2 == 1 {
		return text + "\n```"
	}
	return text
}

// esborraLinia treu una fila de la conversa i quadra els índexs que hi
// apunten (la línia en viu i la fila de cada crida d'eina). Sense això,
// el resultat d'una eina acabaria escrivint sobre una altra fila.
func (m *Model) esborraLinia(i int) {
	if i < 0 || i >= len(m.lines) {
		return
	}
	m.lines = append(m.lines[:i], m.lines[i+1:]...)
	if m.streamLine == i {
		m.streamLine = -1
	} else if m.streamLine > i {
		m.streamLine--
	}
	for k := range m.timeline {
		if m.timeline[k].linia > i {
			m.timeline[k].linia--
		} else if m.timeline[k].linia == i {
			m.timeline[k].linia = -1
		}
	}
	m.refresh()
}
