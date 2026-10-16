package web

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
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
	generation, _ := runCtx.Value(workshopRunKey{}).(uint64)
	originalMaxTokens := role.MaxTokens
	role.MaxTokens = maxTokensPerTorn(mode, role, window)
	if role.MaxTokens != originalMaxTokens {
		emit("status", map[string]string{"message": fmt.Sprintf(s.tr("Pressupost de sortida de l'agent ampliat a %d tokens (el límit antic de 1024 era insuficient).",
			"Agent output budget raised to %d tokens (the old 1024 limit was too low)."), role.MaxTokens)})
	}
	if mode != agent.ModeChat {
		runCtx = llm.WithRecoverTruncation(runCtx)
	}

	// think: no del rol (fins ara la web l'ignorava) i comptador de tokens
	// reals: el topall de cost autònom en depèn.
	runCtx = llm.WithThink(runCtx, role.Think)
	uso := &llm.UsageMeter{}
	runCtx = llm.WithUsage(runCtx, uso)
	// Bind children to this session's model/config, not the last Server
	// that installed the process-global delegate runner.
	s.mu.Lock()
	delegateCfg := *s.cfg
	delegateCfg.Roles = maps.Clone(s.cfg.Roles)
	delegateCfg.Providers = maps.Clone(s.cfg.Providers)
	delegateCfg.Roles["code"] = role
	runCtx = agent.WithDelegateRunner(runCtx, &delegateCfg, s.client)
	if s.activity != nil && generation == s.workshopSeq {
		s.activity.Model = role.Model
	}
	s.mu.Unlock()
	var delegateEmitMu sync.Mutex
	runCtx = agent.WithDelegateHook(runCtx, func(event agent.DelegateEvent) {
		delegateEmitMu.Lock()
		defer delegateEmitMu.Unlock()
		s.noteDelegate(generation, event)
		emit("delegate", event)
	})

	// Subagents en mode code fora d'un worktree: cada canvi passa pel
	// mateix approve_request que el pare (recordat i permissiu inclosos).
	runCtx = agent.WithDelegateApprover(runCtx, func(ctx context.Context, call llm.ToolCall, reason string) bool {
		sig := agent.Sig(call.Function.Name, call.Function.Arguments)
		if s.permissiveNow() || s.remembers.Allowed(webRememberScope, sig) {
			return true
		}
		if s.autoReviewNow() && mode != agent.ModeAutonomous {
			d := agent.Review(ctx, s.client, s.cfg, agent.ReviewRequest{Task: task, Mode: mode, Dir: workspace, Call: call, Reason: reason})
			delegateEmitMu.Lock()
			emit("review", map[string]string{"id": call.ID, "name": call.Function.Name, "decision": d.Decision, "reason": d.Reason})
			delegateEmitMu.Unlock()
			switch d.Decision {
			case agent.ReviewAllow:
				return true
			case agent.ReviewDeny:
				return false
			}
		}
		ok, _ := s.waitApproval(ctx, func(ev string, v any) {
			delegateEmitMu.Lock()
			defer delegateEmitMu.Unlock()
			emit(ev, v)
		}, call.ID, call.Function.Name, call.Function.Arguments, sig, mode != agent.ModeAutonomous)
		return ok
	})

	runCtx = agent.WithHookScope(runCtx, s.id, workspace)
	// Investigador d'inici de torn (agent/scout.go): un fill real de només
	// lectura investiga abans del primer pas i el seu resum entra a la
	// tasca. Si falla, el torn continua igual.
	if agent.ScoutWanted(s.cfg, mode, task) {
		convo = s.runScout(runCtx, generation, task, workspace, convo, maxSteps, uso, role.Provider, emit)
	}
	torn := agent.NouTorn(agent.OpcionsTorn{
		Cfg: s.cfg, Pol: pol, Mode: mode, Tasca: task, MaxSteps: maxSteps, Usage: uso,
		Ctx: runCtx, Sessio: s.id, Workspace: workspace,

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
		// Mode auto-review: el revisor decideix abans de molestar ningú.
		Revisor: agent.NewReviewer(s.client, s.cfg, s.autoReviewNow),
		// Apostes laterals (agent.bets: side) i crític del contracte autònom.
		Aposta:     agent.ApostaLateral(s.client, s.cfg, workspace),
		CriticaPla: agent.CriticaPlaHook(s.client, s.cfg, workspace),
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
	onFallback := func(model string) {
		s.mu.Lock()
		if s.activity != nil && generation == s.workshopSeq {
			s.activity.Model = model
		}
		s.mu.Unlock()
		emit("fallback", map[string]string{"model": model})
	}

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
			// Aposta resolta: xip a la fila de l'eina (index.html, ev bet).
			if e.Aposta != nil {
				emit("bet", betPayload(e.ID, *e.Aposta))
				continue
			}
			// Decisió del revisor automàtic: la targeta de l'eina surt abans
			// i la decisió hi queda com una fila compacta (event review).
			if e.Revisio != nil {
				if c, ok := anunciades[e.ID]; ok {
					emetCrida(e.ID, c.nom, c.args)
				}
				emit("review", map[string]string{"id": e.ID, "name": e.Eina, "decision": e.Revisio.Decision, "reason": e.Revisio.Reason})
				if e.Revisio.Decision == agent.ReviewDeny {
					emit("blocked", map[string]string{"id": e.ID, "reason": agent.ReviewNote(*e.Revisio)})
				}
				continue
			}
			switch e.Tipus {
			case agent.EvCrida:
				anunciades[e.ID] = crida{e.Eina, e.EinaArgs}
			case agent.EvResultat:
				// Només els que el motor resol sense executar (bloquejada,
				// repetida, ajornada, pregunta malformada): els de les
				// execucions ja els emet aquest conductor amb la sortida.
				c, ok := anunciades[e.ID]
				if e.ID == "" || !ok || e.Executada {
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

	// Senyal de vida del torn (event `progress`): fase, estat i comptadors
	// de la crida al model en curs.
	progres := nouProgresTorn(emit)

	avisatEsgotat := false
	// La traça viva és la que llegeix /btw mentre el torn corre.
	defer s.clearLiveTrace()
	// Els missatges escrits mentre el torn treballa (steer.go) arriben per
	// la bústia que obre el torn sencer (turnExecutor): aquí es lliuren.
	for {
		s.setLiveTrace(torn.Hist)
		if runCtx.Err() != nil {
			torn.Cancella()
			emit("error", map[string]string{"message": s.tr("torn aturat per l'usuari", "Turn stopped by the user")})
			return "", histAmbSys(), runCtx.Err()
		}
		for _, m := range s.drainSteer() {
			if torn.Interjeccio(m) {
				emit("steer", map[string]string{"text": m})
			}
		}
		pas := torn.Seguent()
		pinta(pas.Events)
		state, tool := "thinking", ""
		switch pas.Ordre {
		case agent.OrdreExecuta:
			state = "tools"
			names := make([]string, 0, len(pas.Calls))
			for _, call := range pas.Calls {
				names = append(names, call.Function.Name)
			}
			tool = strings.Join(names, ", ")
		case agent.OrdreAprova, agent.OrdrePregunta:
			state, tool = "waiting", pas.Call.Function.Name
		case agent.OrdreCompacta:
			state = "compacting"
		case agent.OrdreCheckpoint, agent.OrdreSintesi:
			state = "reviewing"
		}
		s.updateWorkshop(generation, state, pas.Passos, pas.Limit, tool, "")
		s.noteUsage(generation, uso, role.Provider)
		prim, fb := s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role)
		switch pas.Ordre {
		case agent.OrdrePasModel:
			ctx, cancel := torn.ModelContext(runCtx)
			ctx = llm.WithThink(ctx, agent.ThinkPerPas(role.Think, pas))
			ctx = llm.WithRetryHook(ctx, func(attempt, total int, wait time.Duration, err error) {
				s.updateWorkshop(generation, "thinking", pas.Passos, pas.Limit, "", llm.RetryNote(attempt, total, wait, err))
				emit("status", map[string]string{"message": llm.RetryNote(attempt, total, wait, err)})
				progres.esperaModel()
			})
			// Cada delta (raonament inclòs, i també en mode recuperació, on
			// el text no surt fins al final) alimenta l'event progress.
			progres.iniciModel()
			ctx = llm.WithStreamProgress(ctx, progres.delta)
			// El reasoning_content de les crides d'eina torna al proveïdor
			// dins del torn (llm/reasoning.go).
			raonament := &llm.ReasoningCapture{}
			ctx = llm.WithReasoningCapture(ctx, raonament)
			// Un missatge amb «interromp» talla aquesta crida (i només
			// aquesta): el pas es refà amb el missatge ja a l'historial.
			ctx, tallaPas := context.WithCancel(ctx)
			fiCrida := s.steerModelCall(tallaPas)
			content, calls, _, err := s.client.ChatStreamWithToolsFO(ctx, prim, fb, pas.Hist, role.Temperature, role.MaxTokens, agent.SpecsForHist(mode, s.cfg.CompactFor(role), pas.Hist),
				func(tok string) { emit("token", map[string]string{"text": tok}) }, nil, onFallback)
			interromput := fiCrida()
			tallaPas()
			if interromput && runCtx.Err() == nil {
				cancel()
				progres.fiModel()
				emit("steer_restart", map[string]string{})
				continue
			}
			err = agent.ModelError(ctx, err)
			cancel()
			progres.fiModel()
			// El text que acompanya crides d'eina és treball, no la resposta:
			// la UI el plega i hi tanca la bombolla del pas.
			if err == nil && len(calls) > 0 && strings.TrimSpace(content) != "" {
				emit("thinking", map[string]string{"text": content})
			}
			pinta(torn.RepPasRaonat(content, raonament.Text(), calls, err))

		case agent.OrdreCompacta:
			progres.faseNova(faseCompacta, nil)
			ctx, cancel := context.WithTimeout(runCtx, 240*time.Second)
			room := agent.MakeRoom(ctx, s.client, prim, fb, sys, torn.Hist, pas.Budget)
			cancel()
			if room.Changed() {
				emit("compact", map[string]int{"abans": room.Before, "despres": room.After})
			}
			pinta(torn.RepCompactacio(room))

		case agent.OrdreExecuta:
			noms := make([]string, 0, len(pas.Calls))
			for _, c := range pas.Calls {
				emetCrida(c.ID, c.Function.Name, c.Function.Arguments)
				noms = append(noms, c.Function.Name)
			}
			progres.faseNova(faseEines, noms)
			laneInfo := classifyCalls(pas.Calls)
			lane, ho := s.beginLane(generation, pas.Passos, laneInfo, "")
			if ho != nil {
				emit("handoff", handoffPayload(*ho))
			}
			if lane.Seq > 0 {
				emit("lane", lanePayload(lane))
			}
			outs := agent.RunCalls(pas.Calls, nil, func(_ int, c llm.ToolCall) agent.Execucio {
				out, imgs := s.execToolCtx(runCtx, c, workspace)
				return agent.Execucio{Call: c, Sortida: out, Imatges: imgs}
			})
			// El torn ha mort mentre corrien les eines: els outs són soroll
			// ("ERROR: context canceled"), no troballes. Ni a l'historial ni
			// a l'SSE; la capçalera del bucle avorta net. Les targetes
			// penjades les recull stopToolAnimation en tancar visuals.
			if runCtx.Err() != nil {
				continue
			}
			for _, o := range outs {
				ev := map[string]string{"id": o.Call.ID, "name": o.Call.Function.Name, "output": o.Sortida}
				if img := toolResultImage(o.Call.Function.Name, o.Imatges); img != "" {
					ev["image"] = img
				}
				emit("tool_result", ev)
			}
			if done, ok := s.endLane(generation, lane.Seq, laneResult(laneInfo, outs)); ok {
				emit("lane", lanePayload(done))
			}
			pinta(torn.RepExecucions(outs))

		case agent.OrdreAprova:
			c := pas.Call
			emetCrida(c.ID, c.Function.Name, c.Function.Arguments)
			ok, caducada := s.waitApproval(runCtx, emit, c.ID, c.Function.Name, c.Function.Arguments, agent.Sig(c.Function.Name, c.Function.Arguments), mode != agent.ModeAutonomous)
			if runCtx.Err() != nil {
				continue
			}
			switch {
			case ok:
				pinta(torn.RepAprovacio(true))
			case caducada:
				emit("blocked", map[string]string{"id": c.ID, "reason": s.tr("sense resposta a temps (denegat)", "no answer in time (denied)")})
				pinta(torn.RepAprovacio(false, "TOOL DENIED because the approval timed out (no answer in time). The user did NOT press Deny: propose the action again if it is still needed."))
			default:
				emit("blocked", map[string]string{"id": c.ID, "reason": s.tr("denegat per l'usuari", "denied by the user")})
				pinta(torn.RepAprovacio(false, "TOOL DENIED by the user."))
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
			// Decidir si val la pena continuar és una revisió del progrés.
			progres.faseNova(faseRevisio, nil)
			ctx, cancel := context.WithTimeout(runCtx, agent.TimeoutDecisio)
			ctx = llm.WithThink(ctx, agent.ThinkPerPas(role.Think, pas))
			resp, _, err := s.client.ChatFO(ctx, prim, fb, pas.Hist, role.Temperature, role.MaxTokens, onFallback)
			cancel()
			evs := torn.RepAmpliacio(resp, err)
			// FINAL amb el resum ja escrit: el resum és la resposta (no hi
			// haurà OrdreSintesi). L'avís va abans, com a la síntesi.
			if torn.Acabat() && torn.Esgotat() && !avisatEsgotat {
				avisatEsgotat = true
				emit("steps_exhausted", map[string]any{"max": maxSteps})
			}
			pinta(evs)

		case agent.OrdreSintesi:
			// Dir-ho abans de la síntesi: el que ve és un resum del que s'ha
			// arribat a fer, no la feina acabada.
			if torn.Esgotat() && !avisatEsgotat {
				avisatEsgotat = true
				emit("steps_exhausted", map[string]any{"max": maxSteps})
			}
			progres.faseNova(faseRevisio, nil)
			ctx, cancel := context.WithTimeout(runCtx, agent.TimeoutDecisio)
			ctx = llm.WithThink(ctx, agent.ThinkPerPas(role.Think, pas))
			final, _, err := s.client.ChatFO(ctx, prim, fb, pas.Hist, role.Temperature, role.MaxTokens, onFallback)
			cancel()
			pinta(torn.RepSintesi(final, err))

		case agent.OrdreCheckpoint:
			progres.faseNova(faseRevisio, nil)
			rev, ho := s.beginLane(generation, pas.Passos, laneInfo{Role: roleReviewer, Kind: "review"}, "")
			if ho != nil {
				emit("handoff", handoffPayload(*ho))
			}
			if rev.Seq > 0 {
				emit("lane", lanePayload(rev))
			}
			nota, veredicte := s.checkpointAutonom(runCtx, torn, emit, workspace)
			revResult := "ok"
			if veredicte != "" && !strings.HasPrefix(veredicte, "APROVAT") {
				revResult = "fail"
			}
			if done, ok := s.endLane(generation, rev.Seq, revResult); ok {
				emit("lane", lanePayload(done))
			}
			// La línia d'estat del checkpoint ja l'ha emesa checkpointAutonom
			// (traduïda i només si hi ha res a dir): el motor la repetiria en
			// català ("autònom · checkpoint N", "✓ APROVAT").
			pinta(senseAvisosCheckpoint(torn.RepCheckpoint(nota, veredicte), veredicte))

		case agent.OrdreError:
			emit("error", map[string]string{"message": pas.Text})
			return "", histAmbSys(), errors.New(pas.Text)

		default: // OrdreAcaba (o OrdreRes, que un conductor síncron no espera)
			reply := strings.TrimSpace(torn.Resposta())
			if reply == "" {
				// La síntesi ha fallat o ha sortit buida: mai un torn en
				// blanc, que l'app no tindria res a ensenyar.
				reply = s.tr("He acabat els passos disponibles, però el model no ha pogut generar la síntesi final. Pots obrir Activitat per veure què s'ha executat.",
					"I used up the available steps, but the model could not write the final summary. Open Activity to see what was run.")
				emit("assistant", map[string]string{"text": reply})
			}
			// Proposta de notes duradores en segon pla: mai no retarda el
			// done ni s'escriu sola (vegeu memory_extras.go).
			s.queueMemorySuggestion(mode, workspace, torn.Execucions(), torn.Esgotat() || torn.Fallit(), histAmbSys())
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
	// checks sempre és una llista (buida si no n'hi ha de configurades): els
	// clients la recorren i un null els obligava a un cas especial.
	if checks == nil {
		checks = []agent.AutonomousCheck{}
	}
	emit("autonomous_checkpoint", map[string]any{"number": num, "checks": checks, "review": veredicte})
	if msg := s.resumCheckpoint(num, checks, veredicte); msg != "" {
		emit("status", map[string]string{"message": msg})
	}
	return agent.RenderCheckpoint(checks, veredicte), veredicte
}

// resumCheckpoint és la línia d'estat d'un checkpoint autònom, en l'idioma
// de la sessió. Buida quan no hi ha res a dir (cap comprovació configurada
// i cap revisió en aquest checkpoint): abans sortia igualment un
// "autònom · checkpoint 1" que no aportava res.
func (s *Server) resumCheckpoint(num int, checks []agent.AutonomousCheck, veredicte string) string {
	var parts []string
	if len(checks) > 0 {
		var fallen []string
		for _, c := range checks {
			if c.Code != 0 {
				fallen = append(fallen, c.Command)
			}
		}
		part := fmt.Sprintf(s.tr("%d/%d comprovacions superades", "%d/%d checks passed"), len(checks)-len(fallen), len(checks))
		if len(fallen) > 0 {
			part += fmt.Sprintf(s.tr(" (fallen: %s)", " (failing: %s)"), strings.Join(fallen, ", "))
		}
		parts = append(parts, part)
	}
	switch {
	case veredicte == "":
	case strings.HasPrefix(veredicte, "APROVAT"):
		parts = append(parts, s.tr("revisió aprovada", "review approved"))
	case strings.HasPrefix(veredicte, "CAL REVISAR:"):
		resum := retallaRunes(strings.TrimSpace(strings.TrimPrefix(veredicte, "CAL REVISAR:")), 400)
		parts = append(parts, s.tr("la revisió demana canvis: ", "review asks for changes: ")+resum)
	case strings.HasPrefix(veredicte, "error: "):
		resum := retallaRunes(strings.TrimPrefix(veredicte, "error: "), 400)
		parts = append(parts, s.tr("la revisió ha fallat: ", "review failed: ")+resum)
	default:
		parts = append(parts, retallaRunes(veredicte, 400))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf(s.tr("Checkpoint autònom %d: ", "Autonomous checkpoint %d: "), num) + strings.Join(parts, "; ")
}

// senseAvisosCheckpoint treu dels events de RepCheckpoint la nota
// "autònom · checkpoint N" i la del veredicte: la web ja n'ha emès el resum
// (resumCheckpoint). La resta d'events, si n'hi hagués, passen.
func senseAvisosCheckpoint(evs []agent.Event, veredicte string) []agent.Event {
	out := evs[:0:0]
	for _, e := range evs {
		if e.Clau == "app.checkpoint" {
			continue
		}
		if veredicte != "" && e.Clau == "" && (e.Text == "✓ "+veredicte || e.Text == "⚠ "+veredicte) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func retallaRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// maxToolResultImage topa la imatge que viatja a la targeta de l'eina (data
// URL): una captura del navegador en JPEG en fa uns centenars de kB.
const maxToolResultImage = 1500 << 10

// toolResultImage és la imatge que la targeta de l'eina pot ensenyar: la
// captura de browser screenshot. Les altres eines (read_image) ja mostren
// una imatge que l'usuari té.
func toolResultImage(name string, imgs []string) string {
	if name != "browser" || len(imgs) == 0 {
		return ""
	}
	img := imgs[0]
	if len(img) > maxToolResultImage || !strings.HasPrefix(img, "data:image/") {
		return ""
	}
	return img
}
