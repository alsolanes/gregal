package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// primeraLiniaNoBuida és la primera línia amb text (per a la traça).
func primeraLiniaNoBuida(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// TraceCall és una crida d'eina del run, per diagnosticar-lo després.
type TraceCall struct {
	Step    int    `json:"step"`
	Tool    string `json:"tool"`
	Args    string `json:"args"`
	Failed  bool   `json:"failed,omitempty"`
	Summary string `json:"summary,omitempty"`
	// Round és la ronda de --verify (1, 2…); 0 sense --verify.
	Round int `json:"round,omitempty"`
	// id aparella la crida amb el seu resultat (tool_call id). Per nom
	// d'eina, una crida denegada es quedava el resultat de la següent.
	id string
}

// RunResult és el resum d'una execució no interactiva.
type RunResult struct {
	Answer string   `json:"answer"`
	Steps  int      `json:"steps"`
	Tools  []string `json:"tools_used"`
	// Trace és cada crida amb els seus arguments i com ha anat, retallat.
	// Només amb els noms (tools_used) una tirada de 18 passos al banc A/B
	// no deia res de per què havia trigat quatre vegades més que opencode.
	Trace []TraceCall `json:"trace,omitempty"`
	// Tokens de pujada i baixada de tot el run i cost si el model té preu al
	// config. TokensSource diu d'on surten: "api" = suma de l'usage que ha
	// declarat el proveïdor a cada crida (inclou que cada pas torna a enviar
	// tot el prompt); "mixed" = algunes crides no l'han declarat i la suma és
	// un mínim; "estimate" = cap crida l'ha declarat i és l'heurístic sobre
	// l'historial final (pot quedar-se curt en un o dos ordres de magnitud).
	UpTokens     int      `json:"up_tokens"`
	DownTokens   int      `json:"down_tokens"`
	TokensSource string   `json:"tokens_source"`
	CachedTokens int      `json:"cached_tokens,omitempty"`
	LLMCalls     int      `json:"llm_calls,omitempty"`
	CostUSD      *float64 `json:"cost_usd,omitempty"`
	// FallbackModel diu quin model ha respost si el primari ha caigut
	// (buit = primari). Transparència: la resposta no és del model triat.
	FallbackModel string `json:"fallback_model,omitempty"`
	// RoutedFrom/To/Why: el router ha canviat el rol (buit = sense canvi).
	RoutedFrom string `json:"routed_from,omitempty"`
	RoutedTo   string `json:"routed_to,omitempty"`
	RouteWhy   string `json:"route_why,omitempty"`
	// Verified diu si una comanda de verificació externa (--verify) ha
	// passat. Fals = sense verificar o verificació fallida; VerifSortida
	// en porta la sortida retallada.
	// És un punter perquè surti sempre (true o false) quan hi ha --verify
	// i mai sense: un CI ha de poder distingir «fallada» de «no demanada».
	Verified     *bool  `json:"verified,omitempty"`
	VerifSortida string `json:"verif_sortida,omitempty"`
	// VerifyRounds és quantes rondes de --verify s'han fet.
	VerifyRounds int `json:"verify_rounds,omitempty"`
	// Exhausted: el torn s'ha tancat perquè s'ha arribat al límit de passos
	// (o el motor ha forçat la síntesi: encallat, ratxa vermella, topall).
	// La resposta és un resum del que s'ha fet, no necessàriament feina
	// acabada (com l'event steps_exhausted de la web).
	Exhausted bool `json:"exhausted"`
}

// RunOpts són les opcions d'una execució no interactiva.
type RunOpts struct {
	Task     string
	Mode     string
	MaxSteps int
	// HardStepLimit fa de MaxSteps un topall dur: en arribar-hi no es
	// demana cap ampliació, es fa la síntesi. El CLI l'activa quan l'usuari
	// passa --max-steps; amb el límit del config es conserven les
	// ampliacions de sempre.
	HardStepLimit bool
	AutoApprove   bool
	// Dir és el workspace de les eines, del mapa i de la política (les
	// escriptures dins del projecte passen soles en mode code). Buit = el
	// directori del procés.
	Dir    string
	OnStep func(step int, tools []string)
	// Role fixa el rol del config (sense router). Buit = code/chat pel mode.
	Role string
	// Tools limita les eines natives que veu i pot fer servir el model
	// (nil = totes les no interactives). Les altres queden denegades.
	Tools []string
	// System s'afegeix al system prompt (instruccions del rol d'equip o
	// del subagent definit per l'usuari).
	System string
	// Approve decideix les accions que demanen permís (nil = denegades,
	// com sempre sense terminal). La web hi penja el seu approve_request.
	Approve func(ctx context.Context, call llm.ToolCall, reason string) bool
	// AutoReview activa el revisor automàtic (mode auto-review) encara que
	// el config no digui permissions.mode: auto-review.
	AutoReview bool
	// OnEvent rep els events del motor (crides, resultats, avisos).
	OnEvent func(Event)
}

// approvalNote substitueix headlessNote quan hi ha qui aprova: ningú
// respon preguntes, però les accions amb permís arriben a l'usuari.
const approvalNote = "\n\nNobody can answer questions during this run, but actions that need permission are sent to the user for approval. If one is denied, do not retry it: report it. Do only what the task asks and work only inside the working directory."

// restrictPolicy denega les eines natives que no són a la llista.
func restrictPolicy(pol *Policy, allowed []string) *Policy {
	if len(allowed) == 0 {
		return pol
	}
	cp := *pol
	cp.Tools = make(map[string]string, len(pol.Tools)+len(tools.Noms()))
	for k, v := range pol.Tools {
		cp.Tools[k] = v
	}
	keep := map[string]bool{}
	for _, t := range allowed {
		keep[t] = true
	}
	for _, n := range tools.Noms() {
		if !keep[n] {
			cp.Tools[n] = "deny"
		}
	}
	return &cp
}

// headlessNote diu al model que no hi ha ningú per respondre: sense això
// cridava question i, amb la resposta «continua amb el teu criteri», feia
// canvis que ningú no havia demanat.
const headlessNote = "\n\nNon-interactive run: nobody can answer questions or approve actions during this run. Do only what the task asks. If a decision you cannot deduce from the project would change the scope of the work, stop and state the open question in your final answer instead of making changes that were not requested. Work only inside the working directory."

// NoUIQuestionReply és el resultat de l'eina question quan no hi ha UI
// (el mateix text que fan servir Telegram i el bucle de subagents).
const NoUIQuestionReply = PreguntaSenseUI

// einesInteractives no tenen sentit sense ningú davant: question no la
// respon ningú, preview i office_open obren finestres que ningú no mira, i
// browser condueix un navegador que algú ha de poder vigilar (i les accions
// que actuen demanen permís).
var einesInteractives = map[string]bool{"question": true, "preview": true, "dashboard_save": true, "office_open": true, "browser": true}

// SpecsNoInteractiu és SpecsAll sense les eines que necessiten una persona.
func SpecsNoInteractiu() []llm.ToolSpec {
	all := SpecsAll()
	out := make([]llm.ToolSpec, 0, len(all))
	for _, s := range all {
		if !einesInteractives[s.Name] {
			out = append(out, s)
		}
	}
	return out
}

// ErrorAmbSortida és el resultat d'una eina que ha fallat. Si l'eina havia
// produït sortida que l'error no inclou (el timeout de bash torna el que la
// comanda havia escrit abans del tall), també hi va: sovint ja diu prou.
func ErrorAmbSortida(err error, out string) string {
	msg := "ERROR: " + err.Error()
	o := strings.TrimSpace(out)
	if o == "" || strings.Contains(err.Error(), o) {
		return msg
	}
	return msg + "\n--- output before the error ---\n" + out
}

// RunNonInteractive executa una tasca sense TUI: loop model→eines fins a
// resposta final o límit. Ask es denega sense UI, tret que ho permeti el
// config; autoApprove no salta aprovacions en mode autònom.
func RunNonInteractive(ctx context.Context, client *llm.Client, cfg *config.Config, task, mode string, maxSteps int, autoApprove bool) (RunResult, error) {
	return RunNonInteractiveIn(ctx, client, cfg, task, mode, maxSteps, autoApprove, "")
}

// RunNonInteractiveIn és la variant que fixa el workspace de totes les
// eines. Els executors que treballen sobre una conversa represa han de passar
// aquesta carpeta per no dependre del directori del procés.
func RunNonInteractiveIn(ctx context.Context, client *llm.Client, cfg *config.Config, task, mode string, maxSteps int, autoApprove bool, dir string) (RunResult, error) {
	return RunNonInteractiveExIn(ctx, client, cfg, task, mode, maxSteps, autoApprove, dir, nil)
}

// RunNonInteractiveEx és el mateix amb heartbeat: onStep rep cada pas amb
// els noms de les eines demanades (buit si el model respon directe).
// Serveix el CLI (stderr) i qualsevol executor sense streaming.
func RunNonInteractiveEx(ctx context.Context, client *llm.Client, cfg *config.Config, task, mode string, maxSteps int, autoApprove bool, onStep func(step int, tools []string)) (RunResult, error) {
	return RunNonInteractiveExIn(ctx, client, cfg, task, mode, maxSteps, autoApprove, "", onStep)
}

// RunNonInteractiveExIn és RunNonInteractiveEx amb workspace explícit.
func RunNonInteractiveExIn(ctx context.Context, client *llm.Client, cfg *config.Config, task, mode string, maxSteps int, autoApprove bool, dir string, onStep func(step int, tools []string)) (RunResult, error) {
	return RunNonInteractiveOpts(ctx, client, cfg, RunOpts{
		Task: task, Mode: mode, MaxSteps: maxSteps, AutoApprove: autoApprove, Dir: dir, OnStep: onStep,
	})
}

// RunNonInteractiveOpts és l'execució no interactiva amb totes les opcions.
func RunNonInteractiveOpts(ctx context.Context, client *llm.Client, cfg *config.Config, o RunOpts) (RunResult, error) {
	task, mode, maxSteps, autoApprove, dir, onStep := o.Task, o.Mode, o.MaxSteps, o.AutoApprove, o.Dir, o.OnStep
	if strings.TrimSpace(dir) == "" {
		// Sense workspace explícit, el del procés: la política l'ha de
		// conèixer, o cap escriptura dins del projecte no passava sola.
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		}
	}
	res := RunResult{Tools: []string{}}
	if mode != ModeChat && mode != ModeInspect && mode != ModeGoal && mode != ModeAutonomous && mode != ModeDebug {
		mode = "code"
	}
	if maxSteps < 1 || maxSteps > 200 {
		return res, fmt.Errorf("max_steps %d fora de rang (1-200)", maxSteps)
	}
	roleName := "code"
	if mode == "chat" {
		roleName = "chat"
	}
	if o.Role != "" {
		roleName = o.Role
	} else if routed, why := cfg.Route(task, false, roleName); routed != roleName {
		res.RoutedFrom, res.RoutedTo, res.RouteWhy = roleName, routed, why
		roleName = routed
	}
	r, ok := cfg.Roles[roleName]
	if !ok {
		return res, fmt.Errorf("el config no té rol %s", roleName)
	}
	pp, ok := cfg.Providers[r.Provider]
	if !ok {
		return res, fmt.Errorf("provider desconegut: %s", r.Provider)
	}
	// think: no al rol = el model no raona. El valor viatja al context i tots
	// els mètodes de xat el respecten; les crides de subagents hereden.
	ctx = llm.WithThink(ctx, r.Think)
	// Comptador del consum real: totes les crides que surten d'aquest
	// context (passos, compactació, ampliació, síntesi, revisió) hi sumen.
	meter := &llm.UsageMeter{}
	ctx = llm.WithUsage(ctx, meter)
	autoCfg := cfg.AutonomousConfig()
	pol := &Policy{Tools: cfg.Permissions.Tools, BashAllow: cfg.Permissions.BashAllow, BashDeny: cfg.Permissions.BashDeny, ProjectDir: dir}
	pol = restrictPolicy(pol, o.Tools)
	var hist []llm.Message

	// finish tanca el resultat: tokens + cost si hi ha preu. Mana l'usage
	// del proveïdor; l'estimació només queda per quan no n'ha declarat cap.
	finish := func() {
		u, calls, withUsage := meter.Total()
		res.LLMCalls = calls
		if withUsage > 0 {
			res.UpTokens, res.DownTokens = u.PromptTokens, u.CompletionTokens
			res.CachedTokens = u.PromptTokensDetails.CachedTokens
			res.TokensSource = "api"
			if withUsage < calls {
				res.TokensSource = "mixed"
			}
		} else {
			res.TokensSource = "estimate"
			for _, m := range hist {
				n := llm.EstimateTokens([]llm.Message{{Role: m.Role, Content: m.Content, ToolCalls: m.ToolCalls}})
				if m.Role == "assistant" {
					res.DownTokens += n
				} else {
					res.UpTokens += n
				}
			}
		}
		if usd, ok := CostUSD(r.Provider, r.Model, res.UpTokens, res.DownTokens); ok {
			res.CostUSD = &usd
		}
	}

	sys := cfg.SystemPrompt()
	note := headlessNote
	if o.Approve != nil {
		note = approvalNote
	}
	sys = AmbContext(PromptFor(sys+note, mode), o.System)
	hist = []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: task}}
	// Cortesia pura ("hola"): resposta directa, zero passos, zero cost.
	if reply, ok := SmallTalkFor(task, cfg.Lang(), false); ok {
		hist = append(hist, llm.Message{Role: "assistant", Content: reply})
		res.Answer = reply
		res.Steps = 0
		finish()
		return res, nil
	}
	// Reintents del client visibles com a pas «reintent: …».
	ctx = llm.WithRetryHook(ctx, func(attempt, total int, wait time.Duration, err error) {
		if onStep != nil {
			onStep(0, []string{"reintent: " + llm.RetryNote(attempt, total, wait, err)})
		}
	})
	// Finestra del model: la que declara el proveïdor si el rol no en fixa.
	dctx, dcancel := context.WithTimeout(ctx, 5*time.Second)
	DetectRoleWindows(dctx, cfg, r)
	dcancel()

	// El bucle: el motor decideix, aquí només es fa la feina. Abans això
	// era una màquina d'estats sencera duplicada de la del TUI.
	torn := NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: pol, Mode: mode, Tasca: task, MaxSteps: maxSteps, Usage: meter,
		Hist:      []llm.Message{{Role: "user", Content: TascaAmbMapa(mode, dir, task, nil)}},
		SysPrompt: func() string { return sys },
		Finestra: func() (int, int) {
			win := Window(cfg, r)
			return win, PromptBudget(win, r.MaxTokens)
		},
		Rol:              func() (string, string) { return r.Provider, r.Model },
		AutoAprova:       func() bool { return autoApprove },
		Revisor:          revisorRun(client, cfg, o.AutoReview),
		SenseAmpliacions: o.HardStepLimit,
		Ctx:              ctx,
		Workspace:        dir,
		Aposta:           ApostaLateral(client, cfg, dir),
		CriticaPla:       CriticaPlaHook(client, cfg, dir),
	})

	specs := SpecsNoInteractiu()
	if len(o.Tools) > 0 {
		keep := map[string]bool{}
		for _, t := range o.Tools {
			keep[t] = true
		}
		specs = filterSpecs(specs, func(name string) bool { return keep[name] })
	}
	// marca tanca l'entrada del trace d'una crida que no s'ha executat
	// (denegada, pregunta sense UI).
	marca := func(id, resum string) {
		for i := range res.Trace {
			if tc := &res.Trace[i]; tc.id == id && tc.Summary == "" {
				tc.Failed, tc.Summary = true, retallaText(resum, 140)
				return
			}
		}
	}
	// finish llegeix l'historial del motor.
	hist = torn.Hist
	avisa := func(pas int, evs []Event) {
		for _, e := range evs {
			if o.OnEvent != nil {
				o.OnEvent(e)
			}
			switch e.Tipus {
			case EvCrida:
				res.Tools = append(res.Tools, e.Eina)
				res.Trace = append(res.Trace, TraceCall{Step: pas, Tool: e.Eina, Args: retallaText(e.EinaArgs, 200), id: e.ID})
			case EvResultat:
				// La guia del DoomTracker no és un resultat.
				if e.ID == "" && e.Text == DoomGuide {
					continue
				}
				// El resultat va a la crida amb el mateix id; sense id (motors
				// antics), a la primera de la mateixa eina que encara no en té.
				for i := range res.Trace {
					tc := &res.Trace[i]
					if e.ID != "" && tc.id != e.ID {
						continue
					}
					if tc.Tool == e.Eina && tc.Summary == "" && !tc.Failed {
						tc.Failed = e.Fallada || strings.HasPrefix(e.Text, "ERROR:")
						tc.Summary = retallaText(primeraLiniaNoBuida(e.Missatge()), 140)
						if tc.Summary == "" {
							tc.Summary = "(sense sortida)"
						}
						break
					}
				}
			case EvError:
				if onStep != nil {
					onStep(pas, []string{e.Text})
				}
			}
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			hist = torn.Hist
			finish()
			return res, err
		}
		p := torn.Seguent()
		hist = torn.Hist
		res.Steps = torn.Passos()
		if len(p.Events) > 0 {
			avisa(p.Passos, p.Events)
			if onStep != nil {
				for _, e := range p.Events {
					if e.Revisio != nil {
						onStep(p.Passos, []string{ReviewNote(*e.Revisio)})
					}
				}
			}
		}
		switch p.Ordre {
		case OrdrePasModel:
			stepCtx, cancel := torn.ModelContext(ctx)
			pctx := llm.WithThink(stepCtx, ThinkPerPas(r.Think, p))
			raonament := &llm.ReasoningCapture{}
			pctx = llm.WithReasoningCapture(pctx, raonament)
			content, calls, _, err := perfilCrida(pctx, p.Passos, p.Hist, func() (string, []llm.ToolCall, error) {
				c, cs, _, e := client.ChatWithToolsFO(pctx, cfg.PrimTarget(pp, r), cfg.FallbackTarget(r), p.Hist,
					r.Temperature, r.MaxTokens, specs, func(model string) { res.FallbackModel = model })
				return c, cs, e
			})
			err = ModelError(stepCtx, err)
			cancel()
			if onStep != nil {
				noms := make([]string, 0, len(calls))
				for _, c := range calls {
					noms = append(noms, c.Function.Name)
				}
				onStep(p.Passos, noms)
			}
			avisa(p.Passos, torn.RepPasRaonat(content, raonament.Text(), calls, err))

		case OrdreCompacta:
			room := MakeRoom(ctx, client, cfg.PrimTarget(pp, r), cfg.FallbackTarget(r), sys, torn.Hist, p.Budget)
			torn.RepCompactacio(room)
			if onStep != nil && room.Changed() {
				onStep(p.Passos, []string{"compactacio: " + room.Note()})
			}

		case OrdreExecuta:
			outs := RunCalls(p.Calls, nil, func(_ int, c llm.ToolCall) Execucio {
				o, imgs, err := ExecCtx(ctx, "", dir, c.Function.Name, c.Function.Arguments)
				if err != nil {
					o = ErrorAmbSortida(err, o)
				}
				return Execucio{Call: c, Sortida: o, Imatges: imgs}
			})
			// Torn mort mentre corrien les eines: resultats falsos
			// ("ERROR: context canceled") que embrutarien historial i trace.
			// La capçalera del bucle avorta net a la iteració següent.
			if ctx.Err() == nil {
				avisa(p.Passos, torn.RepExecucions(outs))
			}

		case OrdreAprova:
			if o.Approve != nil {
				if o.Approve(ctx, p.Call, p.Text) {
					avisa(p.Passos, torn.RepAprovacio(true))
				} else {
					motiu := "TOOL DENIED by the user."
					marca(p.Call.ID, motiu)
					avisa(p.Passos, torn.RepAprovacio(false, motiu))
				}
				continue
			}
			// Sense terminal no hi ha ningú a qui preguntar: es denega i
			// es diu per què, que és el que el model ha de saber.
			motiu := "TOOL DENIED: this non-interactive run needs --auto-approve for this action."
			if mode == ModeAutonomous {
				motiu = "TOOL DENIED: autonomous mode needs interactive approval for this action."
			}
			marca(p.Call.ID, motiu)
			torn.RepAprovacio(false, motiu)
			if onStep != nil {
				onStep(p.Passos, []string{"denegada: " + p.Call.Function.Name})
			}

		case OrdrePregunta:
			marca(p.Call.ID, NoUIQuestionReply)
			torn.RepPregunta(NoUIQuestionReply)

		case OrdreAmplia:
			actx, cancel := context.WithTimeout(llm.WithThink(ctx, ThinkPerPas(r.Think, p)), TimeoutDecisio)
			resp, _, aerr := client.ChatFO(actx, cfg.PrimTarget(pp, r), cfg.FallbackTarget(r), p.Hist,
				r.Temperature, r.MaxTokens, func(model string) { res.FallbackModel = model })
			cancel()
			torn.RepAmpliacio(resp, aerr)
			if onStep != nil {
				onStep(p.Passos, []string{"ampliacio"})
			}

		case OrdreSintesi:
			fctx, cancel := context.WithTimeout(llm.WithThink(ctx, ThinkPerPas(r.Think, p)), TimeoutDecisio)
			final, _, ferr := client.ChatFO(fctx, cfg.PrimTarget(pp, r), cfg.FallbackTarget(r), p.Hist,
				r.Temperature, r.MaxTokens, func(model string) { res.FallbackModel = model })
			cancel()
			torn.RepSintesi(final, ferr)

		case OrdreCheckpoint:
			var checks []AutonomousCheck
			if len(autoCfg.Verify) > 0 {
				checkCtx, cancel := context.WithTimeout(ctx, verifyTimeout)
				checks = RunAutonomousChecks(checkCtx, dir, autoCfg.Verify)
				cancel()
			}
			veredicte := ""
			if autoCfg.ReviewEvery > 0 && (torn.CheckpointNum()+1)%autoCfg.ReviewEvery == 0 {
				reviewCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
				v, _, rerr := AutonomousReview(reviewCtx, client, cfg, AutonomousTranscript(torn.Hist), AutonomousDiff(dir))
				cancel()
				switch {
				case rerr != nil:
					veredicte = "error: " + rerr.Error()
				case v.Approved:
					veredicte = "APROVAT"
				default:
					veredicte = "CAL REVISAR: " + v.Summary
				}
			}
			torn.RepCheckpoint(RenderCheckpoint(checks, veredicte), veredicte)
			if onStep != nil {
				tag := fmt.Sprintf("autonom: checkpoint %d", torn.CheckpointNum())
				if veredicte != "" {
					tag += " · " + veredicte
				}
				onStep(p.Passos, []string{tag})
			}

		case OrdreError:
			hist = torn.Hist
			res.Exhausted = torn.Esgotat()
			finish()
			return res, fmt.Errorf("%s", p.Text)

		default: // OrdreAcaba, OrdreRes
			hist = torn.Hist
			res.Answer = torn.Resposta()
			res.Exhausted = torn.Esgotat()
			finish()
			return res, nil
		}
	}
}

// perfilCrida embolcalla una crida LLM del loop: amb GREGAL_PERFIL=1 pinta a
// stderr el cost del pas (tokens estimats del prompt, mida de la resposta i
// temps). Eina de bench, sense efecte en ús normal.
func perfilCrida(ctx context.Context, step int, hist []llm.Message, crida func() (string, []llm.ToolCall, error)) (string, []llm.ToolCall, bool, error) {
	if os.Getenv("GREGAL_PERFIL") == "" {
		c, cs, e := crida()
		return c, cs, false, e
	}
	t0 := time.Now()
	c, cs, e := crida()
	fmt.Fprintf(os.Stderr, "[perfil] pas %d: prompt~%dtok resposta~%dc+%deines %.1fs\n",
		step, llm.EstimateTokens(hist), len(c), len(cs), time.Since(t0).Seconds())
	return c, cs, false, e
}

// revisorRun és el revisor del headless: actiu amb permissions.mode:
// auto-review o amb RunOpts.AutoReview.
func revisorRun(client *llm.Client, cfg *config.Config, forçat bool) ReviewFunc {
	if cfg == nil || !(forçat || cfg.Permissions.AutoReview()) {
		return nil
	}
	return NewReviewer(client, cfg, nil)
}
