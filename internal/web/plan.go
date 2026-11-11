package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// planSystem obliga exploració read-only i tanca amb pla numerat + verificació
// (mateix contracte que el /plan del TUI).
const planSystem = agent.PlanSystem

// windAutoContinues és quantes rondes extra fa un torn interactiu de vent o
// colla quan el pressupost de passos s'esgota amb feina a mitges. Amb 2, una
// petició de codi té fins a tres rondes (3 × passos) abans de tancar amb un
// resum parcial. Els tests el posen a 0 per tenir un sol torn.
var windAutoContinues = 2

// motiuPassatger diu si un error de torn pot anar bé amb un reintent
// immediat: xarxa/estona, no clau dolenta ni context massa llarg.
func motiuPassatger(m string) bool {
	m = strings.ToLower(m)
	for _, s := range []string{"timeout", "deadline exceeded", "temporar", "http 5", "http 429", "rate limit", "connection reset", "eof", "broken pipe", "overloaded"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// exploracio és el resultat d'un loop read-only: el text, si és parcial
// (pressupost o error a mig camí: vegeu agent.Loop) i l'historial per
// poder-lo continuar.
type exploracio struct {
	text    string
	parcial bool
	motiu   string
	hist    []llm.Message
	// temps diu on ha anat el temps del torn (crides al model i eines).
	temps tempsTorn
}

// tempsTorn és el recompte final d'un loop: passos de model i temps de cada part.
type tempsTorn struct {
	passos int
	model  time.Duration
	eines  time.Duration
}

// loopTemps compta passos i temps d'un loop de lectura, per saber on s'espera.
type loopTemps struct {
	mu     sync.Mutex
	passos int
	model  time.Duration
	eines  time.Duration
}

func (t *loopTemps) addModel(d time.Duration) { t.mu.Lock(); t.passos++; t.model += d; t.mu.Unlock() }
func (t *loopTemps) addEina(d time.Duration)  { t.mu.Lock(); t.eines += d; t.mu.Unlock() }

// String resumeix el torn: «3 passos · model 21.4s · eines 9.8s».
func (t tempsTorn) String() string {
	return fmt.Sprintf("%d passos · model %.1fs · eines %.1fs", t.passos, t.model.Seconds(), t.eines.Seconds())
}

// runReadOnlyLoop fa un loop d'exploració (decisions inspect: escriure queda
// registrat com a no disponible, mai demana res). Nucli compartit de jobs i
// /api/plan. Amb prev, continua aquella exploració amb pressupost nou.
func (s *Server) runReadOnlyLoop(ctx context.Context, brief string, maxSteps int, prev ...[]llm.Message) (exploracio, error) {
	s.mu.Lock()
	p, role := s.roleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRole(ctx, brief, maxSteps, p, role, prev...)
}

// runReadOnlyLoopForRole és el mateix loop amb target explícit: els vents
// (xat, rotllana i cor) hi passen el seu (windRoleRef: rol base + model
// per defecte propi), la feina hi passa el del rol.
func (s *Server) runReadOnlyLoopForRole(ctx context.Context, brief string, maxSteps int, p config.Provider, role config.Role, prev ...[]llm.Message) (exploracio, error) {
	return s.runReadOnlyLoopForRoleStream(ctx, brief, maxSteps, p, role, nil, prev...)
}

// runReadOnlyLoopForRoleStream és el mateix loop amb emissió en directe:
// emit rep ("token", fragment) del text que genera el model i ("phase",
// noms d'eines separats per coma) quan un pas acaba en crides. Els tokens
// d'un pas que acaba en eines són especulatius (poden ser JSON d'eina):
// el client els descarta quan arriba la fase. Amb emit == nil les crides
// són les clàssiques (comportament idèntic al d'abans).
func (s *Server) runReadOnlyLoopForRoleStream(ctx context.Context, brief string, maxSteps int, p config.Provider, role config.Role, emit func(kind, text string), prev ...[]llm.Message) (exploracio, error) {
	scope, interactive := ctx.Value(interactiveWindKey{}).(windTurnScope)
	// El mode Pla fa servir aquest mateix bucle, però no és un xat de vent:
	// només eines de lectura i els seus propis missatges.
	planTurn := isPlanTurn(ctx)
	specs, notRun := windSpecs(), windNotRun
	if planTurn {
		specs, notRun = planSpecs(), planNotRun
	}
	if !interactive {
		scope = s.windLoopScope()
		scope.mode = agent.ModeInspect
	}
	ctx = s.withRemindServer(agent.WithTurnMode(ctx, scope.mode))
	ctx = llm.WithThink(ctx, role.Think)
	// Recuperar talls vol dir retenir tot el text fins al final: només per als
	// torns que ningú no mira en directe (cor, avisos, encàrrecs). En un xat
	// la persona ha de veure la resposta mentre s'escriu; des de 1.57 els
	// vents ho retenien tot i la resposta arribava d'un sol cop al final.
	if !agent.ReadOnlyMode(scope.mode) && emit == nil {
		ctx = llm.WithRecoverTruncation(ctx)
	}
	var requiresApproval bool // Decide is serial, before any tools run.
	var skipped []string
	stream := emit != nil
	if emit == nil {
		emit = func(_, _ string) {}
	}
	// Tools can run in parallel. Keep additive activity and legacy events safe
	// for callers other than the HTTP emitter as well.
	rawEmit := emit
	var emitMu sync.Mutex
	emit = func(kind, text string) {
		emitMu.Lock()
		defer emitMu.Unlock()
		rawEmit(kind, text)
	}
	var step atomic.Int32
	activity := func(phase, tool string) {
		if !stream {
			return
		}
		b, _ := json.Marshal(windActivity{Phase: phase, Tool: tool, Step: int(step.Load())})
		emit("activity", string(b))
	}
	modelContext := func(ctx context.Context) context.Context {
		step.Add(1)
		// A request is being attempted, not proof of an active inference slot.
		activity("model", "")
		last := ""
		ctx = llm.WithRetryHook(ctx, func(_, _ int, _ time.Duration, _ error) {
			last = "retry"
			activity("retry", "")
		})
		return llm.WithStreamProgress(ctx, func(kind string, _ int) {
			phase := ""
			switch kind {
			case llm.ProgressReasoning:
				phase = "thinking"
			case llm.ProgressText:
				phase = "responding"
			}
			if phase != "" && phase != last {
				last = phase
				activity(phase, "")
			}
		})
	}
	// Es pot preguntar a la persona? Un torn interactiu amb flux (xat o grup) la
	// té al davant: les aprovacions i preguntes s'hi publiquen i el torn
	// s'atura fins que respon (winds_ask.go). Sense flux (cor, plans, crides
	// soltes) no hi ha ningú: es continua com sempre, sense fer l'acció.
	canAsk := interactive && stream
	// Torn aparcable (xat d'un vent, o la represa d'un d'aparcat): si una
	// decisió no arriba en directe, el bucle s'atura al punt segur.
	spot := windParkOf(ctx)
	if !canAsk {
		spot = nil
	}
	// Respostes a les preguntes del lot, decidides abans d'executar-lo.
	var answersMu sync.Mutex
	answers := map[string]string{}
	var temps loopTemps
	loop := &agent.Loop{
		MaxSteps: maxSteps,
		Lang:     s.cfg.Lang(),
		// Poda preventiva de sortides velles abans de desbordar la finestra.
		Finestra: agent.PromptBudget(agent.Window(s.cfg, role), role.MaxTokens),
		Step: func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
			cctx, cancel := context.WithTimeout(ctx, 900*time.Second)
			defer cancel()
			defer func(t0 time.Time) { temps.addModel(time.Since(t0)) }(time.Now())
			if stream {
				cctx = modelContext(cctx)
			}
			// Steer: missatges escrits mentre el torn treballa. Es buiden
			// aquí —punt segur: cap eina del lot pendent— i entren com a
			// guia d'usuari al pas següent (winds_steer.go).
			if k, ok := windSteerKeyOf(ctx); ok {
				h = injectaSteer(k, h)
			}
			if !stream {
				content, calls, _, err := s.client.ChatWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, specs, nil)
				return content, calls, err
			}
			content, calls, _, err := s.client.ChatStreamWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, specs, func(tok string) { emit("token", tok) }, nil, nil)
			if err == nil && len(calls) > 0 {
				names := make([]string, 0, len(calls))
				for _, c := range calls {
					names = append(names, c.Function.Name)
				}
				emit("phase", strings.Join(names, ","))
			}
			return content, calls, err
		},
		// La resposta final va sense eines: cap crida no es pot executar.
		Finalize: func(ctx context.Context, h []llm.Message) (string, error) {
			cctx, cancel := context.WithTimeout(ctx, agent.TimeoutDecisio)
			defer cancel()
			defer func(t0 time.Time) { temps.addModel(time.Since(t0)) }(time.Now())
			if stream {
				cctx = modelContext(cctx)
			}
			if !stream {
				out, _, err := s.client.ChatFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, nil)
				return out, err
			}
			out, _, _, err := s.client.ChatStreamWithToolsFO(cctx, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), h, role.Temperature, role.MaxTokens, nil, func(tok string) { emit("token", tok) }, nil, nil)
			return out, err
		},
		RunTool: func(ctx context.Context, name, argsJSON string) (string, []string, error) {
			// Only known native tool NAMES are public. Never expose arguments,
			// results or arbitrary model-supplied/MCP names as a summary.
			tool := "tool"
			if tools.EsNativa(name) && len(name) <= 64 {
				tool = name
			}
			activity("tool", tool)
			defer activity("tool_done", tool) // returned, not necessarily succeeded
			// Una pregunta del vent, amb opcions que la persona toca (o text lliure).
			if name == "question" && canAsk {
				if _, _, err := tools.ParseQuestion(argsJSON); err != nil {
					return "", nil, err
				}
				answersMu.Lock()
				ans := answers[argsJSON]
				answersMu.Unlock()
				return ans, nil, nil
			}
			defer func(t0 time.Time) { temps.addEina(time.Since(t0)) }(time.Now())
			// Apps, publicar: les fa el servidor web, no el paquet agent.
			if name == "app" || name == "publish" || name == "tunnel" || name == "calendar" || name == "mail" || name == "notes" {
				out, err := s.runBuildTool(ctx, scope.workspace, name, argsJSON)
				return out, nil, err
			}
			dirs := append([]string{}, agent.SkillsDirsOf(ctx)...)
			dirs = append(dirs, s.userSkillsDirs()...)
			cwd := scope.workspace
			// Un vent crea els documents a la seva carpeta (winds_docs.go).
			if d := windDocsOf(ctx); d != "" && windDocTool(name) {
				cwd = d
			}
			return windToolExec(agent.WithSkillsDirs(ctx, dirs...), s.id, cwd, name, argsJSON)
		},
		Decide: func(name, argsJSON string) (string, string) {
			// Ja aparcat: la resta del lot no es pregunta ni s'executa.
			if spot.parkedRec() != nil {
				return "deny", "NOT RUN YET: waiting for the user's decision."
			}
			// La pregunta es fa aquí, abans d'executar res del lot: si s'ha
			// d'aparcar, el punt segur no té cap eina a mig fer.
			if name == "question" && canAsk {
				if q, opts, err := tools.ParseQuestion(argsJSON); err == nil {
					ans, parked := s.windQuestionCall(ctx, emit, q, opts, argsJSON)
					if !parked {
						answersMu.Lock()
						answers[argsJSON] = ans
						answersMu.Unlock()
					}
				}
				return "allow", "question"
			}
			// Els vents poden programar avisos (remind, eina validada); la
			// resta usa la política compartida i el mode del torn.
			if agent.ReadOnlyMode(scope.mode) && (name == "bash" || name == "bash_background") {
				return "deny", s.tr("mode de només lectura: no s'executa cap ordre de shell", "read-only mode: shell execution is disabled")
			}
			if name == "remind" {
				if agent.ReadOnlyMode(scope.mode) {
					return "deny", s.tr("mode de només lectura: no es poden programar avisos", "read-only mode: cannot schedule reminders")
				}
				if decision, reason := scope.policy.Decide(scope.mode, name, argsJSON); decision == "deny" {
					return decision, reason
				}
				return "allow", "eina del vent"
			}
			// El calaix del vent: només escriu a les dades del mateix vent (cap fitxer
			// de la persona), així que també el poden fer servir les vigilàncies.
			if name == "calaix" {
				return "allow", "eina del vent"
			}
			// remind_list només llegeix: cap problema ni en mode lectura.
			if name == "remind_list" {
				if decision, reason := scope.policy.Decide(scope.mode, name, argsJSON); decision == "deny" {
					return decision, reason
				}
				return "allow", "eina del vent"
			}
			// remind_edit toca els recordatoris existents de l'usuari:
			// mateix tractament que remind (res en mode lectura).
			if name == "remind_edit" {
				if agent.ReadOnlyMode(scope.mode) {
					return "deny", s.tr("mode de només lectura: no es poden editar els avisos", "read-only mode: cannot edit reminders")
				}
				if decision, reason := scope.policy.Decide(scope.mode, name, argsJSON); decision == "deny" {
					return decision, reason
				}
				return "allow", "eina del vent"
			}
			pol := scope.policy
			// Keep the existing documents scope, but interactive turns must
			// also respect the selected mode/access and explicit policy rules.
			if d := windDocsOf(ctx); d != "" && windDocTool(name) {
				if decision, reason := windDocDecide(d, name, argsJSON); decision != "allow" || !interactive {
					return decision, reason
				}
				cp := *pol
				cp.ProjectDir = d
				pol = &cp
			}
			decision, reason := pol.Decide(scope.mode, name, argsJSON)
			// La instrucció explícita del xat JA és l'aprovació per a la feina
			// d'aquí (escriure i executar dins del projecte, sense res
			// destructiu): si no, demanar codi al vent sempre acaba en «falta
			// permís». Tot «deny» continua denegat.
			if decision == "ask" && interactive && pol.WindAutoAllow(name, argsJSON) {
				decision, reason = "allow", "ordre del xat del vent ("+reason+")"
			}
			// Porta dura: instal·lar programari, baixar paquets, esborrar fora del
			// projecte o tocar serveis del sistema NO es fa sense el sí de la
			// persona, encara que el classificador ho deixi passar (amb poders de
			// servidor, per exemple). El torn s'atura fins que respon.
			kind, what := "other", ""
			if decision == "allow" && interactive && (name == "bash" || name == "bash_background") {
				var a struct {
					Command string `json:"command"`
				}
				if json.Unmarshal([]byte(argsJSON), &a) == nil {
					if r, risky := tools.Risky(a.Command, scope.workspace, serviceHome()); risky {
						decision, reason, kind, what = "ask", "needs your approval: "+r.Kind, r.Kind, r.What
					}
				}
			}
			if decision == "ask" {
				if what == "" {
					what = strings.TrimSpace(name + " " + windAskArgsHint(argsJSON))
				}
				if canAsk {
					if ok, msg := s.windApproveCall(ctx, emit, name, argsJSON, kind, what, reason); ok {
						return "allow", "approved by the user"
					} else {
						return "deny", msg
					}
				}
				requiresApproval = true
				skipped = append(skipped, reason)
				return "deny", notRun + " (" + reason + ")"
			}
			return decision, reason
		},
	}
	if spot != nil {
		loop.Lot = spot.setHist
		loop.Pausa = func() error {
			if spot.parkedRec() != nil {
				return errWindParked
			}
			return nil
		}
	}
	resuming := windResumeOf(ctx) != nil && len(prev) > 0 && len(prev[0]) > 0
	var out string
	var err error
	// Torn interactiu (xat del vent, colla): no hi ha cap UI d'aprovació i
	// la persona ha demanat una feina. Si el pressupost de passos s'esgota
	// a mig treball, no es tanca amb un resum: es reprèn sol amb pressupost
	// nou, com fan els agents que no pregunten (Claude Code, Codex). Només
	// l'última ronda sintetitza, perquè mai no quedi un torn sense resposta.
	rounds := windAutoContinues
	if !interactive {
		rounds = 0
	}
	first := true
	retried := false
	for round := 0; ; round++ {
		last := round >= rounds
		loop.SenseTancarPassos = interactive && !last
		switch {
		case first && resuming:
			out, _, err = loop.Repren(ctx, prev[0])
		case first && len(prev) > 0 && len(prev[0]) > 0:
			out, _, err = loop.Continua(ctx, prev[0])
		case first && len(windPromptOf(ctx)) > 0:
			out, _, err = loop.RunMessages(ctx, windPromptOf(ctx))
		case first:
			out, _, err = loop.Run(ctx, brief)
		default:
			out, _, err = loop.Continua(ctx, loop.Hist)
		}
		first = false
		if errors.Is(err, errWindParked) {
			break
		}
		if err != nil {
			// Caiguda passatgera del proveïdor abans de fer res: un reintent.
			if !retried && !errors.Is(ctx.Err(), context.Canceled) && ctx.Err() == nil && motiuPassatger(err.Error()) {
				retried = true
				continue
			}
			break
		}
		if !loop.Esgotat || last || ctx.Err() != nil {
			break
		}
	}
	// Do not depend on a model to relay a skipped action honestly: one short
	// line with what was not run (not a wall of text that reads as a crash).
	if requiresApproval {
		out = strings.TrimSpace(out) + "\n\n" + windSkippedNote(skipped)
	}
	res := exploracio{text: strings.TrimSpace(out), parcial: loop.Parcial, motiu: loop.Motiu, hist: loop.Hist}
	temps.mu.Lock()
	res.temps = tempsTorn{passos: temps.passos, model: temps.model, eines: temps.eines}
	temps.mu.Unlock()
	return res, err
}

// windToolExec executa una eina d'un torn de vent. Els tests el substitueixen
// per comptar quantes vegades s'aplica un efecte.
var windToolExec = agent.ExecCtx

// planBrief munta la consigna d'exploració amb memòria del projecte.
func (s *Server) planBrief(task string) string {
	return agent.PlanBrief(s.cwd, task, s.cfg.Lang())
}

// handlePlan explora en read-only i torna el pla (sense actuar).
func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Task string `json:"task"`
		// Continue reprèn l'última exploració parcial amb pressupost nou i
		// la conversa intacta (el botó «Continua explorant»).
		Continue bool `json:"continue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (strings.TrimSpace(req.Task) == "" && !req.Continue) {
		http.Error(w, "tasca buida", 400)
		return
	}
	s.mu.Lock()
	if s.agentBusy || s.jobRunning || s.planRunning {
		s.mu.Unlock()
		http.Error(w, "agent ocupat: torna-ho a provar", 409)
		return
	}
	prevHist, prevTask := s.planHist, s.planTask
	if req.Continue && len(prevHist) == 0 {
		s.mu.Unlock()
		http.Error(w, "no hi ha cap exploració per continuar", 409)
		return
	}
	s.planRunning = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.planRunning = false
		s.mu.Unlock()
	}()
	defer func() {
		if rec := recover(); rec != nil {
			http.Error(w, fmt.Sprintf("exploració interrompuda: %v", rec), 500)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	task := strings.TrimSpace(req.Task)
	var res exploracio
	var err error
	// El pla és una feina de lectura llarga: el pressupost de sortida del
	// rol (sovint 1.024 tokens) tallava el pla a mitges, com passava al torn
	// de codi abans que el motor l'ampliés.
	s.mu.Lock()
	p, role := s.roleRef()
	s.mu.Unlock()
	role.MaxTokens = maxTokensPerTorn(agent.ModePlan, role, agent.Window(s.cfg, role))
	ctx = withPlanTurn(ctx)
	if req.Continue {
		task = prevTask
		res, err = s.runReadOnlyLoopForRole(ctx, "", agent.PassosPla(s.cfg.Agent.MaxSteps), p, role, prevHist)
	} else {
		// Regles al system i la tasca com a missatge d'usuari (abans, un sol bloc).
		msgs := []llm.Message{{Role: "system", Content: agent.PlanSystemPrompt(s.cwd, s.cfg.Lang())}, {Role: "user", Content: task}}
		res, err = s.runReadOnlyLoopForRole(withWindPrompt(ctx, msgs), s.planBrief(task), agent.PassosPla(s.cfg.Agent.MaxSteps), p, role)
	}
	s.mu.Lock()
	s.planHist, s.planTask = res.hist, task
	s.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	plan := res.text
	if plan == "" {
		http.Error(w, "exploració buida", 502)
		return
	}
	// La tasca torna amb el pla: «Executa el pla» l'envia a l'agent junt amb
	// el pla (abans només el pla, i l'agent no sabia què es volia aconseguir).
	out := map[string]any{"plan": plan, "task": task}
	if res.parcial {
		// Parcial: el pressupost (o el model) s'ha acabat abans que el pla.
		// La UI ho marca i ofereix continuar.
		out["partial"] = true
		out["partial_reason"] = res.motiu
		out["max_steps"] = agent.PassosPla(s.cfg.Agent.MaxSteps)
	}
	// Crític del pla (plan.critic, per defecte on): problemes, no un pla nou.
	if c := s.planCritic(ctx, task, plan); c != nil {
		out["critic"] = c
	}
	writeJSON(w, out)
}

type planTurnKey struct{}

func withPlanTurn(ctx context.Context) context.Context {
	return context.WithValue(ctx, planTurnKey{}, true)
}

func isPlanTurn(ctx context.Context) bool { v, _ := ctx.Value(planTurnKey{}).(bool); return v }

// planNotRun és el que rep el model quan prova una eina que no és de lectura
// durant el Pla (abans rebia el missatge dels vents, que parla de carpetes de
// projecte i de l'eina app).
const planNotRun = "NOT AVAILABLE IN PLAN MODE: exploring is read-only. Do not retry; use read, grep, glob or symbols, and put the change in the plan as a step instead."

// planSpecs: només les eines de lectura (el Pla no pot escriure ni executar).
func planSpecs() []llm.ToolSpec {
	var out []llm.ToolSpec
	for _, t := range agent.SpecsAll() {
		if agent.IsReadOnlyTool(t.Name) {
			out = append(out, t)
		}
	}
	return out
}
