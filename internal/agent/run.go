package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
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
	Verified     bool   `json:"verified,omitempty"`
	VerifSortida string `json:"verif_sortida,omitempty"`
}

// RunNonInteractive executa una tasca sense TUI: loop model→eines fins a
// resposta final o límit. ask→ autoApprove? executa : denega (segur per defecte).
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
	var res RunResult
	if mode != ModeChat && mode != ModeInspect && mode != ModeGoal && mode != ModeAutonomous {
		mode = "code"
	}
	if maxSteps < 1 || maxSteps > 200 {
		return res, fmt.Errorf("max_steps %d fora de rang (1-200)", maxSteps)
	}
	roleName := "code"
	if mode == "chat" {
		roleName = "chat"
	}
	if routed, why := cfg.Route(task, false, roleName); routed != roleName {
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
	sys = PromptFor(sys, mode)
	hist = []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: task}}
	// Cortesia pura ("hola"): resposta directa, zero passos, zero cost.
	if reply, ok := SmallTalk(task); ok {
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
		Rol:        func() (string, string) { return r.Provider, r.Model },
		AutoAprova: func() bool { return autoApprove },
	})
	// finish llegeix l'historial del motor.
	hist = torn.Hist
	avisa := func(pas int, evs []Event) {
		for _, e := range evs {
			switch e.Tipus {
			case EvCrida:
				res.Tools = append(res.Tools, e.Eina)
				res.Trace = append(res.Trace, TraceCall{Step: pas, Tool: e.Eina, Args: retallaText(e.EinaArgs, 200)})
			case EvResultat:
				// El resultat va a la primera crida de la mateixa eina que
				// encara no en té (els resultats arriben en l'ordre de les
				// crides). La guia del DoomTracker no és un resultat.
				for i := range res.Trace {
					tc := &res.Trace[i]
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
		p := torn.Seguent()
		hist = torn.Hist
		res.Steps = torn.Passos()
		switch p.Ordre {
		case OrdrePasModel:
			pctx := llm.WithThink(ctx, ThinkPerPas(r.Think, p))
			content, calls, _, err := perfilCrida(pctx, p.Passos, p.Hist, func() (string, []llm.ToolCall, error) {
				c, cs, _, e := client.ChatWithToolsFO(pctx, cfg.PrimTarget(pp, r), cfg.FallbackTarget(r), p.Hist,
					r.Temperature, r.MaxTokens, SpecsAll(), func(model string) { res.FallbackModel = model })
				return c, cs, e
			})
			if onStep != nil {
				noms := make([]string, 0, len(calls))
				for _, c := range calls {
					noms = append(noms, c.Function.Name)
				}
				onStep(p.Passos, noms)
			}
			avisa(p.Passos, torn.RepPas(content, calls, err))

		case OrdreCompacta:
			room := MakeRoom(ctx, client, cfg.PrimTarget(pp, r), cfg.FallbackTarget(r), sys, torn.Hist, p.Budget)
			torn.RepCompactacio(room)
			if onStep != nil && room.Changed() {
				onStep(p.Passos, []string{"compactacio: " + room.Note()})
			}

		case OrdreExecuta:
			outs := RunCalls(p.Calls, nil, func(_ int, c llm.ToolCall) Execucio {
				o, imgs, err := ExecIn("", dir, c.Function.Name, c.Function.Arguments)
				if err != nil {
					o = "ERROR: " + err.Error()
				}
				return Execucio{Call: c, Sortida: o, Imatges: imgs}
			})
			avisa(p.Passos, torn.RepExecucions(outs))

		case OrdreAprova:
			// Sense terminal no hi ha ningú a qui preguntar: es denega i
			// es diu per què, que és el que el model ha de saber.
			torn.RepAprovacio(false, "EINA DENEGADA (cal --auto-approve)")
			if onStep != nil {
				onStep(p.Passos, []string{"denegada: " + p.Call.Function.Name})
			}

		case OrdrePregunta:
			torn.RepPregunta("PREGUNTA SENSE UI: no hi ha interfície per triar opcions aquí. Formula la pregunta en text a la resposta final i continua amb el teu millor criteri.")

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
			finish()
			return res, fmt.Errorf("%s", p.Text)

		default: // OrdreAcaba, OrdreRes
			hist = torn.Hist
			res.Answer = torn.Resposta()
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
