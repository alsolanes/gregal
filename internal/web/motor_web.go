package web

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

const legacyAgentMaxTokens = 1024
const agentOutputBudget = 8192

// maxTokensPerTorn evita que una configuració antiga amb el rol chat a
// 1024 tokens talli les respostes d'un torn d'agent. Només promociona el
// valor heretat exacte; qualsevol altre límit configurat explícitament es
// conserva. El màxim és un quart de la finestra per deixar espai al prompt.
func maxTokensPerTorn(mode string, role config.Role, window int) int {
	if mode == agent.ModeChat || role.MaxTokens != legacyAgentMaxTokens {
		return role.MaxTokens
	}
	ceiling := window / 4
	if ceiling <= role.MaxTokens {
		return role.MaxTokens
	}
	if ceiling < agentOutputBudget {
		return ceiling
	}
	return agentOutputBudget
}

// conduirTorn fa córrer el torn d'agent de la web sobre el motor compartit
// (agent.Torn), com el TUI, Telegram i el headless.
//
// Fins al 2026-09-28 la web tenia un bucle propi a runTurn, de 400 línies,
// que duplicava el del motor. Aquell dia es va veure el cost: les
// proteccions contra els bucles llargs (topall interactiu d'ampliacions,
// ratxa de comprovacions vermelles, síntesi honesta) van arribar al motor
// però no a la web, i la web i el desktop podien cremar fins a 25 trossos
// de passos amb la suite vermella. Ara la política del bucle viu en un sol
// lloc; aquí només hi ha l'E/S de la web: text en directe, aprovacions i
// preguntes que esperen la UI, checkpoints autònoms i els events SSE que
// el frontend espera (index.html onRunEvent, app/runs.js durableToAgent).
//
// Torna la resposta final i l'historial del torn amb el system (per al
// cost).
func (s *Server) conduirTorn(runCtx context.Context, task, mode, roleName string, p config.Provider, role config.Role, maxSteps int, workspace string, pol *agent.Policy, emit func(string, any)) (string, []llm.Message, error) {
	s.mu.Lock()
	sys := s.sysPromptAmb(mode, workspace)
	convo := append([]llm.Message(nil), s.convo...)
	s.mu.Unlock()
	window := agent.Window(s.cfg, role)
	originalMaxTokens := role.MaxTokens
	role.MaxTokens = maxTokensPerTorn(mode, role, window)
	if role.MaxTokens != originalMaxTokens {
		emit("status", map[string]string{"message": fmt.Sprintf("Pressupost de sortida de l'agent ampliat a %d tokens (el límit antic de 1024 era insuficient).", role.MaxTokens)})
	}
	if mode != agent.ModeChat {
		runCtx = llm.WithRecoverTruncation(runCtx)
	}

	// think: no del rol (fins ara la web l'ignorava) i comptador de tokens
	// reals: el topall de cost autònom en depèn.
	runCtx = llm.WithThink(runCtx, role.Think)
	uso := &llm.UsageMeter{}
	runCtx = llm.WithUsage(runCtx, uso)

	torn := agent.NouTorn(agent.OpcionsTorn{
		Cfg: s.cfg, Pol: pol, Mode: mode, Tasca: task, MaxSteps: maxSteps, Usage: uso,
		Hist:      convo,
		SysPrompt: func() string { return sys },
		Finestra: func() (int, int) {
			return agent.Window(s.cfg, role), agent.Budget(s.cfg, role)
		},
		Rol: func() (string, string) { return role.Provider, role.Model },
		Recordat: func(name, args string) bool {
			return s.remembers.Allowed(webRememberScope, agent.Sig(name, args))
		},
		AutoAprova: s.permissiveNow,
		// La checklist és de la sessió (execTool intercepta todowrite), no
		// la global del procés: amb dues pestanyes es trepitjaven.
		Todos: func() []tools.TodoItem {
			s.mu.Lock()
			defer s.mu.Unlock()
			return append([]tools.TodoItem(nil), s.todo...)
		},
		// La finestra real que diu el proveïdor queda al rol de la sessió,
		// com feia el bucle antic.
		AprenFinestra: func(w int) {
			role.ContextWindow = w
			s.mu.Lock()
			if rc, ok := s.cfg.Roles[roleName]; ok {
				rc.ContextWindow = w
				s.cfg.Roles[roleName] = rc
			}
			s.mu.Unlock()
		},
	})
	histAmbSys := func() []llm.Message {
		return append([]llm.Message{{Role: "system", Content: sys}}, torn.Hist...)
	}
	onFallback := func(model string) { emit("fallback", map[string]string{"model": model}) }

	// La UI aparella cada targeta d'eina amb el seu resultat per l'ordre
	// d'arribada (tool_call empeny, tool_result/blocked treu). El motor
	// anuncia totes les crides d'un pas de cop i en resol algunes més
	// tard (aprovacions), així que aquí cada tool_call s'emet just abans
	// del seu resultat o de la seva aprovació, amb l'id per no repetir-lo.
	type crida struct{ nom, args string }
	anunciades := map[string]crida{}
	emeses := map[string]bool{}
	emetCrida := func(id, nom, args string) {
		if emeses[id] {
			return
		}
		emeses[id] = true
		emit("tool_call", map[string]string{"id": id, "name": nom, "args": args})
	}
	pinta := func(evs []agent.Event) {
		for _, e := range evs {
			switch e.Tipus {
			case agent.EvCrida:
				anunciades[e.ID] = crida{e.Eina, e.EinaArgs}
			case agent.EvResultat:
				// Només els que el motor resol sense executar (bloquejada,
				// repetida, ajornada, pregunta malformada): els de les
				// execucions ja els emet aquest conductor amb la sortida.
				c, ok := anunciades[e.ID]
				if e.ID == "" || !ok {
					continue
				}
				emetCrida(e.ID, c.nom, c.args)
				switch {
				case e.Clau == "app.repetida":
					emit("tool_result", map[string]string{"id": e.ID, "name": c.nom, "output": agent.DoomGuide})
				case strings.HasPrefix(e.Text, "bloquejada: "):
					emit("blocked", map[string]string{"id": e.ID, "reason": strings.TrimPrefix(e.Text, "bloquejada: ")})
				default:
					emit("tool_result", map[string]string{"id": e.ID, "name": c.nom, "output": e.Text})
				}
			case agent.EvResposta:
				emit("assistant", map[string]string{"text": e.Text})
			case agent.EvNota, agent.EvAvis:
				text := strings.TrimSpace(e.Missatge())
				switch {
				case e.Clau == "est.cancellat":
					// La UI ja ha pintat el Denegat a la targeta.
				case e.Clau == "compact.fet" && len(e.Args) >= 2:
					emit("compact", map[string]any{"abans": e.Args[0], "despres": e.Args[1]})
					emit("status", map[string]string{"message": text})
				case e.Text == agent.AvisEinaText:
					emit("tool_text", map[string]string{"message": agent.AvisEinaText})
				case e.Clau == "app.ratxaVermella":
					emit("status", map[string]string{"message": "Comprovació vermella repetida: " + text})
				case text != "":
					emit("status", map[string]string{"message": text})
				}
			}
			// EvError: el torn mort surt per OrdreError; una síntesi
			// fallida acaba sense resposta i té el seu text a sota.
			// EvDiff: la web no el pinta.
		}
	}

	avisatEsgotat := false
	for {
		if runCtx.Err() != nil {
			torn.Cancella()
			emit("error", map[string]string{"message": "torn aturat per l'usuari"})
			return "", histAmbSys(), runCtx.Err()
		}
		pas := torn.Seguent()
		prim, fb := s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role)
		switch pas.Ordre {
		case agent.OrdrePasModel:
			ctx, cancel := context.WithTimeout(runCtx, 240*time.Second)
			ctx = llm.WithThink(ctx, agent.ThinkPerPas(role.Think, pas))
			ctx = llm.WithRetryHook(ctx, func(attempt, total int, wait time.Duration, err error) {
				emit("status", map[string]string{"message": llm.RetryNote(attempt, total, wait, err)})
			})
			content, calls, _, err := s.client.ChatStreamWithToolsFO(ctx, prim, fb, pas.Hist, role.Temperature, role.MaxTokens, agent.SpecsAll(),
				func(tok string) { emit("token", map[string]string{"text": tok}) }, nil, onFallback)
			cancel()
			// El text que acompanya crides d'eina és treball, no la resposta:
			// la UI el plega i hi tanca la bombolla del pas.
			if err == nil && len(calls) > 0 && strings.TrimSpace(content) != "" {
				emit("thinking", map[string]string{"text": content})
			}
			pinta(torn.RepPas(content, calls, err))

		case agent.OrdreCompacta:
			ctx, cancel := context.WithTimeout(runCtx, 240*time.Second)
			room := agent.MakeRoom(ctx, s.client, prim, fb, sys, torn.Hist, pas.Budget)
			cancel()
			if room.Changed() {
				emit("compact", map[string]int{"abans": room.Before, "despres": room.After})
			}
			pinta(torn.RepCompactacio(room))

		case agent.OrdreExecuta:
			for _, c := range pas.Calls {
				emetCrida(c.ID, c.Function.Name, c.Function.Arguments)
			}
			outs := agent.RunCalls(pas.Calls, nil, func(_ int, c llm.ToolCall) agent.Execucio {
				out, imgs := s.execToolCtx(runCtx, c, workspace)
				return agent.Execucio{Call: c, Sortida: out, Imatges: imgs}
			})
			for _, o := range outs {
				emit("tool_result", map[string]string{"id": o.Call.ID, "name": o.Call.Function.Name, "output": o.Sortida})
			}
			pinta(torn.RepExecucions(outs))

		case agent.OrdreAprova:
			c := pas.Call
			emetCrida(c.ID, c.Function.Name, c.Function.Arguments)
			ok, caducada := s.waitApproval(runCtx, emit, c.ID, c.Function.Name, c.Function.Arguments, agent.Sig(c.Function.Name, c.Function.Arguments))
			if runCtx.Err() != nil {
				continue
			}
			switch {
			case ok:
				pinta(torn.RepAprovacio(true))
			case caducada:
				emit("blocked", map[string]string{"id": c.ID, "reason": "sense resposta a temps (denegat)"})
				pinta(torn.RepAprovacio(false, "EINA DENEGADA per temps esgotat (sense resposta a temps). L'usuari NO ha premut Denega: torna a proposar l'acció si cal."))
			default:
				emit("blocked", map[string]string{"id": c.ID, "reason": "denegat per l'usuari"})
				pinta(torn.RepAprovacio(false, "EINA DENEGADA per l'usuari."))
			}

		case agent.OrdrePregunta:
			c := pas.Call
			emetCrida(c.ID, c.Function.Name, c.Function.Arguments)
			q, opts, _ := tools.ParseQuestion(c.Function.Arguments)
			ans, caducada := s.waitQuestion(runCtx, emit, c.ID, q, opts)
			if runCtx.Err() != nil {
				continue
			}
			if caducada || strings.TrimSpace(ans) == "" {
				emit("tool_result", map[string]string{"id": c.ID, "name": c.Function.Name, "output": "sense resposta"})
				pinta(torn.RepPregunta("L'usuari no ha respost (temps esgotat). Continua amb el teu millor criteri sense tornar a preguntar el mateix."))
				continue
			}
			if strings.HasPrefix(ans, "text:") {
				ans = "Resposta lliure de l'usuari: " + strings.TrimSpace(strings.TrimPrefix(ans, "text:"))
			} else {
				ans = "L'usuari ha triat: " + strings.TrimSpace(ans)
			}
			emit("tool_result", map[string]string{"id": c.ID, "name": c.Function.Name, "output": ans})
			pinta(torn.RepPregunta(ans))

		case agent.OrdreAmplia:
			ctx, cancel := context.WithTimeout(runCtx, agent.TimeoutDecisio)
			ctx = llm.WithThink(ctx, agent.ThinkPerPas(role.Think, pas))
			resp, _, err := s.client.ChatFO(ctx, prim, fb, pas.Hist, role.Temperature, role.MaxTokens, onFallback)
			cancel()
			pinta(torn.RepAmpliacio(resp, err))

		case agent.OrdreSintesi:
			// Dir-ho abans de la síntesi: el que ve és un resum del que s'ha
			// arribat a fer, no la feina acabada.
			if torn.Esgotat() && !avisatEsgotat {
				avisatEsgotat = true
				emit("steps_exhausted", map[string]any{"max": maxSteps})
			}
			ctx, cancel := context.WithTimeout(runCtx, agent.TimeoutDecisio)
			ctx = llm.WithThink(ctx, agent.ThinkPerPas(role.Think, pas))
			final, _, err := s.client.ChatFO(ctx, prim, fb, pas.Hist, role.Temperature, role.MaxTokens, onFallback)
			cancel()
			pinta(torn.RepSintesi(final, err))

		case agent.OrdreCheckpoint:
			nota, veredicte := s.checkpointAutonom(runCtx, torn, emit, workspace)
			pinta(torn.RepCheckpoint(nota, veredicte))

		case agent.OrdreError:
			emit("error", map[string]string{"message": pas.Text})
			return "", histAmbSys(), errors.New(pas.Text)

		default: // OrdreAcaba (o OrdreRes, que un conductor síncron no espera)
			reply := strings.TrimSpace(torn.Resposta())
			if reply == "" {
				// La síntesi ha fallat o ha sortit buida: mai un torn en
				// blanc, que l'app no tindria res a ensenyar.
				reply = "He acabat els passos disponibles, però el model no ha pogut generar la síntesi final. Pots obrir Activitat per veure què s'ha executat."
				emit("assistant", map[string]string{"text": reply})
			}
			return reply, histAmbSys(), nil
		}
	}
}

// checkpointAutonom corre les comprovacions i la revisió d'un checkpoint
// del mode autònom (com el bucle antic i el headless) i torna la nota per
// al model i el veredicte.
func (s *Server) checkpointAutonom(runCtx context.Context, torn *agent.Torn, emit func(string, any), workspace string) (string, string) {
	autoCfg := s.cfg.AutonomousConfig()
	num := torn.CheckpointNum() + 1
	var checks []agent.AutonomousCheck
	if len(autoCfg.Verify) > 0 {
		ctx, cancel := context.WithTimeout(runCtx, 120*time.Second)
		checks = agent.RunAutonomousChecks(ctx, workspace, autoCfg.Verify)
		cancel()
	}
	veredicte := ""
	if autoCfg.ReviewEvery > 0 && num%autoCfg.ReviewEvery == 0 {
		ctx, cancel := context.WithTimeout(runCtx, 60*time.Second)
		v, _, err := agent.AutonomousReview(ctx, s.client, s.cfg, agent.AutonomousTranscript(torn.Hist), agent.AutonomousDiff(workspace))
		cancel()
		switch {
		case err != nil:
			veredicte = "error: " + err.Error()
		case v.Approved:
			veredicte = "APROVAT"
		default:
			veredicte = "CAL REVISAR: " + v.Summary
		}
	}
	emit("autonomous_checkpoint", map[string]any{"number": num, "checks": checks, "review": veredicte})
	return agent.RenderCheckpoint(checks, veredicte), veredicte
}
