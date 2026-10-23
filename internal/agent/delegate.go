package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// delegateToolSpec és l'eina de subagents paral·lels (estil Task de Claude
// Code). v1: exploració read-only (mode inspect); els subagents no poden
// escriure ni delegar (sense recursió).
func delegateToolSpec() llm.ToolSpec {
	str := map[string]any{"type": "string"}
	return llm.ToolSpec{
		Name:        "delegate",
		Description: "Run 1-4 read-only subagents AT THE SAME TIME (read, grep, glob, web_*). When the work splits into independent parts (mapping several areas or modules, reviewing several files, web/docs research next to reading the code), put them all in ONE call: they run in parallel and each returns a summary. Make every task self-contained (paths, question, what to return). Not for a single lookup, a step that needs another's result, or work you are doing yourself. Use the summaries; do not redo the work. Optional `agent` picks a user-defined subagent (see SUBAGENTS in the system prompt) with its own tools, model and mode. Limits: 4 per call and 8 alive per process; the rest wait in a queue.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"tasks": map[string]any{"type": "array", "items": map[string]any{"type": "object",
				"properties": map[string]any{"prompt": str, "agent": str}, "required": []string{"prompt"}}},
		}, "required": []string{"tasks"}},
	}
}

// delegateRunner fa la feina real d'un subagent. Es configura a l'arrencada
// (mateix provider/model del rol code); nil = no disponible.
var delegateRunner func(ctx context.Context, prompt string) (string, error)
var delegateRunnerMu sync.RWMutex

type delegateScopeKey struct{}

type delegateScope struct {
	session string
	dir     string
}

type delegateRunnerContextKey struct{}

type delegateRunnerContext struct {
	run       func(context.Context, string) (string, error)
	semaphore chan struct{}
}

func withDelegateScope(ctx context.Context, session, dir string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, delegateScopeKey{}, delegateScope{session: session, dir: dir})
}

func delegateScopeFromContext(ctx context.Context) delegateScope {
	if ctx == nil {
		return delegateScope{}
	}
	scope, _ := ctx.Value(delegateScopeKey{}).(delegateScope)
	return scope
}

// SetDelegateRunner configura (o neteja amb nil) el runner de subagents.
func SetDelegateRunner(fn func(ctx context.Context, prompt string) (string, error)) {
	delegateRunnerMu.Lock()
	defer delegateRunnerMu.Unlock()
	delegateRunner = fn
}

func currentDelegateRunner() func(ctx context.Context, prompt string) (string, error) {
	delegateRunnerMu.RLock()
	defer delegateRunnerMu.RUnlock()
	return delegateRunner
}

// WithDelegateRunner returns a context that uses a runner built from cfg and
// client for delegate calls in that turn. An explicitly configured delegate
// role is validated and reported when invoked; otherwise the code role is used.
func WithDelegateRunner(ctx context.Context, cfg *config.Config, client *llm.Client) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	runner := buildDelegateRunner(cfg, client)
	return context.WithValue(ctx, delegateRunnerContextKey{}, delegateRunnerContext{run: runner, semaphore: make(chan struct{}, ParallelSubagents(cfg))})
}

func copyDelegatePermissions(src config.PermissionsCfg) config.PermissionsCfg {
	dst := config.PermissionsCfg{
		Tools:     make(map[string]string, len(src.Tools)),
		BashAllow: append([]string(nil), src.BashAllow...),
		BashDeny:  append([]string(nil), src.BashDeny...),
		Rules:     src.Rules,
		Subagents: src.Subagents,
	}
	for name, decision := range src.Tools {
		dst.Tools[name] = decision
	}
	return dst
}

// delegatePolicy és la política d'un fill; amb nom de subagent, la d'ell
// (permissions.subagents.<nom>).
func delegatePolicy(permissions config.PermissionsCfg, subagent string) *Policy {
	return subagentPolicy(permissions, subagent)
}

// MaxDelegateTasks topa els subagents per crida (cada un fa els seus passos:
// 4 ja són prou context de tornada).
const MaxDelegateTasks = 4

// maxResumFill topa el que cada fill torna al pare: 4 resums llargs
// entrarien sencers a l'historial a cada pas següent i la poda els
// retallaria dient "torna a cridar l'eina", quan re-delegar executa una
// altra cerca, no la mateixa. El text sencer no es re-obté enlloc: la
// marca ho diu perquè el pare treballi amb el resum.
const maxResumFill = 6000

func retallaResum(out string) string {
	r := []rune(strings.TrimSpace(out))
	if len(r) <= maxResumFill {
		return string(r)
	}
	return string(r[:maxResumFill]) + fmt.Sprintf("…[resum del subagent retallat a %d de %d caràcters: és tot el que ha tornat; continua amb aquest resum]", maxResumFill, len(r))
}

// MaxDelegateLiveTotal topa els subagents vius de tot el procés. El semàfor
// per torn (WithDelegateRunner) només existeix a la web; el TUI, Telegram i
// el headless fan servir el runner global sense cap. Sense aquest sostre,
// N crides delegate d'un mateix pas (delegate és ParallelSafe) engegaven
// N×4 fills sense límit. Els fills no deleguen, així que no hi ha
// reentrància: qui espera un slot no en té cap de retingut.
const MaxDelegateLiveTotal = 8

var delegateGlobalLive = make(chan struct{}, MaxDelegateLiveTotal)

// SetupDelegate configura el runner amb el rol code del config (mateix
// provider/model que l'agent pare). Els subagents exploren en mode inspect
// (12 passos, eines natives sense delegate ni MCP, ask→deny): no escriuen,
// no demanen res, només tornen text.
func SetupDelegate(cfg *config.Config, client *llm.Client) {
	SetupSmallRole(cfg)
	SetDelegateRunner(buildDelegateRunner(cfg, client))
	delegateParallelDefault.Store(int32(ParallelSubagents(cfg)))
}

func buildDelegateRunner(cfg *config.Config, client *llm.Client) func(context.Context, string) (string, error) {
	if cfg == nil || client == nil {
		return nil
	}
	r, ok := cfg.Roles["delegate"]
	roleName := "delegate"
	if !ok {
		r, ok = cfg.Roles["code"]
		roleName = "code"
	}
	if !ok {
		return func(context.Context, string) (string, error) {
			return "", fmt.Errorf("rol %q no configurat per als subagents", roleName)
		}
	}
	p, ok := cfg.Providers[r.Provider]
	if !ok {
		return func(context.Context, string) (string, error) {
			return "", fmt.Errorf("provider %q del rol %q no configurat", r.Provider, roleName)
		}
	}
	if strings.TrimSpace(r.Model) == "" {
		return func(context.Context, string) (string, error) {
			return "", fmt.Errorf("model del rol %q buit", roleName)
		}
	}
	primary := cfg.PrimTarget(p, r)
	fallback := cfg.FallbackTarget(r)
	budget := Budget(cfg, r)
	baseSystem := cfg.SystemPrompt()
	permissions := copyDelegatePermissions(cfg.Permissions)
	return func(ctx context.Context, prompt string) (string, error) {
		scope := delegateScopeFromContext(ctx)
		workspace := scope.dir
		if workspace == "" {
			var err error
			workspace, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("no s'ha pogut resoldre el directori de treball: %w", err)
			}
		}
		session := scope.session
		if session == "" {
			session = procSession
		}
		// delegateExec already attached a chained meter; direct runners
		// (global, tests) get one so usage events are never blind.
		if llm.UsageFrom(ctx) == nil {
			ctx = llm.WithUsage(ctx, &llm.UsageMeter{})
		}
		// Subagent definit per l'usuari: el seu model, les seves eines, el
		// seu mode i els seus passos. Sense definició, el fill genèric de
		// lectura de sempre (12 passos, mode inspect, totes les natives).
		def := SubagentFromContext(ctx)
		mode, maxSteps, extra := ModeInspect, 12, ""
		rr, primary, fallback, budget := r, primary, fallback, budget
		if def != nil {
			mode, maxSteps = def.Mode, def.MaxSteps
			extra = "SUBAGENT " + def.Name + ": " + def.Description
			if def.Prompt != "" {
				extra += "\n\n" + def.Prompt
			}
			dr, err := SubagentRole(cfg, r, def)
			if err != nil {
				return "", err
			}
			dp, ok := cfg.Providers[dr.Provider]
			if !ok {
				return "", fmt.Errorf("subagent %q: provider %q is not configured", def.Name, dr.Provider)
			}
			rr, primary, fallback, budget = dr, cfg.PrimTarget(dp, dr), cfg.FallbackTarget(dr), Budget(cfg, dr)
		}
		writable := mode == ModeCode
		isolated := writable && IsolatedWorktree(workspace)
		approver := delegateApproverFrom(ctx)
		if writable {
			if isolated {
				extra += "\n\nYou work in an isolated git worktree: you may edit files there; the user reviews the changes before applying them."
			} else if approver != nil {
				extra += "\n\nEvery change you make (writes, edits, non-read-only commands) needs the user's approval. A denial means: do not retry it, report it."
			} else {
				extra += "\n\nNobody can approve changes right now: you are read-only. Describe the changes you would make instead of attempting them."
			}
		}
		specs := filterSpecs(Specs(), func(name string) bool { return subagentAllowed(def, name) })
		// El context del projecte va abans de la directiva d'idioma, que
		// tanca el prompt també als subagents.
		promptMode := ModeInspect
		if writable {
			promptMode = ModeCode
		}
		sys := AmbContext(PromptFor(baseSystem, promptMode), ContextProjecte(workspace), extra)
		subName := ""
		if def != nil {
			subName = def.Name
		}
		pol := delegatePolicy(permissions, subName)
		if isolated {
			pol.ProjectDir = workspace
		}
		// Cada invocació té el seu checklist. RunCalls serialitza todowrite
		// respecte als lots de todoread, que només llegeixen aquesta llista.
		var todos []tools.TodoItem
		hist := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: prompt}}
		ctx = llm.WithThink(ctx, rr.Think)
		// tanca és la sortida sense passos (o amb el model caigut a mig
		// camí): una crida final sense eines amb el que el fill ha reunit,
		// marcada com a parcial perquè el pare no la prengui per completa.
		// Abans tornava «límit de N passos exhaurit» i el pare perdia tota
		// la feina del fill.
		tanca := func(motiu string) (string, error) {
			emitDelegateEvent(ctx, DelegateEvent{Status: DelegateWorking, Step: maxSteps, MaxSteps: maxSteps, Model: primary.Model, Message: "wrap-up: " + motiu})
			fctx := ctx
			if ctx.Err() != nil {
				var cancel context.CancelFunc
				fctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), TimeoutDecisio)
				defer cancel()
			}
			fh := append(append([]llm.Message(nil), hist...), llm.Message{Role: "user", Content: PromptFinalExploracio(cfg.Lang())})
			out, _, ferr := client.ChatFO(fctx, primary, fallback, fh, rr.Temperature, rr.MaxTokens, nil)
			out = NetejaFinal(out)
			if ferr != nil || out == "" {
				m := motiu
				if ferr != nil {
					m += "; " + ferr.Error()
				}
				out = ResumDeFeina(hist, m, cfg.Lang())
			}
			return MarcaParcialSubagent(motiu, maxSteps) + "\n\n" + out, nil
		}
		for step := 1; step <= maxSteps; step++ {
			if err := ctx.Err(); err != nil {
				if step > 1 && !errors.Is(err, context.Canceled) {
					return tanca(err.Error())
				}
				return "", err
			}
			setDelegateModel(ctx, primary.Model)
			emitDelegateEvent(ctx, DelegateEvent{Status: DelegateWorking, Step: step, MaxSteps: maxSteps, Model: primary.Model, Message: fmt.Sprintf("step %d", step)})
			if room := MakeRoom(ctx, client, primary, fallback, "", hist, budget); room.Changed() {
				hist = room.Hist
			}
			model := primary.Model
			hist = AmbAvisPassos(hist, step, maxSteps)
			t0 := time.Now()
			content, calls, usedFallback, err := client.ChatWithToolsFO(ctx, primary, fallback, hist, rr.Temperature, rr.MaxTokens, specs, nil)
			if os.Getenv("GREGAL_PERFIL") != "" {
				// Com perfilCrida (run.go), però per als fills.
				fmt.Fprintf(os.Stderr, "[perfil] fill %s pas %d: prompt~%dtok resposta~%dc+%deines %.1fs\n",
					fillID(ctx), step, llm.EstimateTokens(hist), len(content), len(calls), time.Since(t0).Seconds())
			}
			if usedFallback && fallback != nil {
				model = fallback.Model
			}
			setDelegateModel(ctx, model)
			emitDelegateEvent(ctx, DelegateEvent{Status: DelegateWorking, Step: step, MaxSteps: maxSteps, Model: model, Message: "model response received"})
			if err != nil {
				if step > 1 && !errors.Is(ctx.Err(), context.Canceled) {
					return tanca(fmt.Sprintf("pas %d: %v", step, err))
				}
				return "", fmt.Errorf("pas %d: %w", step, err)
			}
			hist = append(hist, llm.Message{Role: "assistant", Content: content, ToolCalls: calls})
			if len(calls) == 0 {
				setDelegateModel(ctx, model)
				return content, nil
			}
			type resultat struct {
				out  string
				imgs []string
			}
			plans := PlanCalls(pol, ModeInspect, nil, calls)
			if writable {
				// Mode code: la política normal; el que no passa sol es
				// demana a qui aprova (o es denega sense ningú).
				plans = PlanCalls(pol, ModeCode, nil, calls)
			}
			for i, c := range calls {
				// El checklist del subagent és propi de la invocació: el
				// runner l'executa aquí mateix (switch de sota) i no toca
				// mai el del pare, així que ni es denega ni demana permís.
				local := c.Function.Name == "todowrite" || c.Function.Name == "todoread"
				switch {
				case local:
					plans[i] = CallPlan{}
				case c.Function.Name == "question" || c.Function.Name == "preview":
					plans[i] = CallPlan{Fixed: "TOOL NOT AVAILABLE (read-only subagent): this tool changes the parent's state", Reason: "eina d'estat global desactivada per a subagents"}
				case !subagentAllowed(def, c.Function.Name):
					plans[i] = CallPlan{Fixed: "TOOL NOT AVAILABLE: not in this subagent's tools", Reason: "not in this subagent's tools: " + c.Function.Name}
				}
				// Fora d'un worktree, tota acció que no sigui lectura passa
				// per l'usuari encara que la política la deixés passar sola.
				if writable && !isolated && !local && plans[i].Fixed == "" && !plans[i].Ask {
					if d, _ := pol.Decide(ModeInspect, c.Function.Name, c.Function.Arguments); d != "allow" {
						plans[i] = CallPlan{Ask: true, Reason: "subagent change outside an isolated worktree"}
					}
				}
				if writable && !local && plans[i].Ask {
					if approver == nil {
						plans[i] = CallPlan{Fixed: "TOOL DENIED: read-only subagent (no isolated worktree and nobody to approve)", Reason: "read-only subagent: no isolated worktree and nobody to approve"}
					} else if blocked, why := PreToolHook(ctx, session, workspace, c.Function.Name, c.Function.Arguments); blocked {
						plans[i] = CallPlan{Fixed: "BLOCKED by pre_tool hook: " + why, Reason: "blocked by pre_tool hook: " + why}
					} else if approver(ctx, c, plans[i].Reason) {
						plans[i] = CallPlan{}
						continue
					} else {
						plans[i] = CallPlan{Fixed: "TOOL DENIED by the user", Reason: "denied by the user"}
					}
				}
				// Hooks pre_tool: també manen dins dels subagents.
				if plans[i].Fixed == "" && !plans[i].Ask {
					if blocked, why := PreToolHook(ctx, session, workspace, c.Function.Name, c.Function.Arguments); blocked {
						plans[i] = CallPlan{Fixed: "BLOCKED by pre_tool hook: " + why, Reason: "blocked by pre_tool hook: " + why}
					}
				}
			}
			res := RunCalls(calls, func(i int) bool { return plans[i].Fixed != "" || plans[i].Ask }, func(_ int, c llm.ToolCall) resultat {
				emitDelegateEvent(ctx, DelegateEvent{Status: DelegateWorking, Step: step, MaxSteps: maxSteps, Model: model, Tool: c.Function.Name, Message: "tool started"})
				var o string
				var oi []string
				var err error
				switch c.Function.Name {
				case "todowrite":
					var input struct {
						Items []tools.TodoItem `json:"items"`
					}
					if err = json.Unmarshal([]byte(c.Function.Arguments), &input); err == nil {
						todos = tools.NetejaTodos(input.Items)
						o = tools.RenderTodos(todos)
					}
				case "todoread":
					o = tools.RenderTodos(todos)
				default:
					o, oi, err = ExecCtx(ctx, session, workspace, c.Function.Name, c.Function.Arguments)
				}
				if err != nil {
					return resultat{out: "ERROR: " + err.Error()}
				}
				return resultat{out: o, imgs: oi}
			})
			// Fill mort mentre corrien les eines: no s'afegeix soroll a
			// l'historial. Cancel·lat mana; si és el temps, es resumeix.
			if err := ctx.Err(); err != nil {
				if errors.Is(err, context.Canceled) {
					return "", err
				}
				return tanca(err.Error())
			}
			for i, c := range calls {
				if plans[i].Fixed != "" || plans[i].Ask {
					prefix := "TOOL NOT AVAILABLE (read-only subagent): "
					if writable {
						prefix = "TOOL NOT RUN: "
					}
					hist = append(hist, ToolMsg(c, prefix+plans[i].Reason))
					emitDelegateEvent(ctx, DelegateEvent{Status: DelegateWorking, Step: step, MaxSteps: maxSteps, Model: model, Tool: c.Function.Name, Message: plans[i].Reason})
					continue
				}
				hist = append(hist, ToolMsg(c, res[i].out, res[i].imgs...))
				emitDelegateEvent(ctx, DelegateEvent{Status: DelegateWorking, Step: step, MaxSteps: maxSteps, Model: model, Tool: c.Function.Name, Output: delegatePreview(res[i].out)})
			}
		}
		return tanca(MotiuPassos)
	}
}

// MarcaParcialSubagent encapçala la resposta d'un subagent que s'ha quedat
// sense passos (o temps): el pare ha de saber que no és la feina completa.
func MarcaParcialSubagent(motiu string, maxSteps int) string {
	if motiu == MotiuPassos {
		return fmt.Sprintf("[PARTIAL: the subagent used its %d steps before finishing; this is its best answer from what it gathered]", maxSteps)
	}
	return "[PARTIAL: the subagent stopped early (" + motiu + "); this is its best answer from what it gathered]"
}

func delegateTerminalStatus(err, parentErr error) string {
	if errors.Is(err, context.Canceled) || errors.Is(parentErr, context.Canceled) {
		return DelegateCancelled
	}
	return DelegateFailed
}

// delegateExec valida, llança en paral·lel i fusiona els resums.
func delegateExec(ctx context.Context, argsJSON string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var a struct {
		Tasks []struct {
			Prompt string `json:"prompt"`
			Agent  string `json:"agent"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("arguments il·legibles: %w", err)
	}
	if len(a.Tasks) == 0 {
		return "", fmt.Errorf("cal almenys 1 tasca")
	}
	if len(a.Tasks) > MaxDelegateTasks {
		return "", fmt.Errorf("màxim %d subagents per crida (demanats %d)", MaxDelegateTasks, len(a.Tasks))
	}
	for i, t := range a.Tasks {
		if strings.TrimSpace(t.Prompt) == "" {
			return "", fmt.Errorf("tasca %d sense prompt", i+1)
		}
		if len(t.Prompt) > 2000 {
			return "", fmt.Errorf("tasca %d massa llarga (màx 2000 caràcters)", i+1)
		}
	}
	// Subagents definits per l'usuari: un nom desconegut és un error que
	// el model pot corregir (es diuen els que hi ha), no un fill genèric.
	defs := make([]*SubagentDef, len(a.Tasks))
	var catalog []SubagentDef
	for i, t := range a.Tasks {
		name := strings.TrimSpace(t.Agent)
		if name == "" {
			continue
		}
		if catalog == nil {
			catalog = LoadSubagents(delegateWorkspace(ctx))
		}
		def, ok := FindSubagent(catalog, name)
		if !ok {
			if len(catalog) == 0 {
				return "", fmt.Errorf("tasca %d: no hi ha cap subagent definit (.gregal/agents/*.md)", i+1)
			}
			return "", fmt.Errorf("tasca %d: subagent %q desconegut; els definits: %s", i+1, name, strings.Join(SubagentNames(catalog), ", "))
		}
		defs[i] = &def
	}
	runner := currentDelegateRunner()
	var semaphore chan struct{}
	if scoped, ok := ctx.Value(delegateRunnerContextKey{}).(delegateRunnerContext); ok {
		runner = scoped.run
		semaphore = scoped.semaphore
	}
	if runner == nil {
		return "", fmt.Errorf("subagents no disponibles en aquest loop")
	}
	// Sense semàfor de torn (TUI, headless, Telegram) el sostre és per
	// crida: el mateix que a la web (ParallelSubagents, 1 en local).
	if semaphore == nil {
		n := int(delegateParallelDefault.Load())
		if n <= 0 {
			n = MaxDelegateTasks
		}
		semaphore = make(chan struct{}, n)
	}
	outs := make([]string, len(a.Tasks))
	var wg sync.WaitGroup
	// Tots els fills surten alhora: els que caben prenen el lloc aquí, en
	// l'ordre de la crida; els altres surten «en cua» a la sala i l'esperen
	// dins de la seva goroutine (abans, els de després del sostre ni tan
	// sols apareixien fins que un altre acabava). L'ordre de presa és
	// sempre semàfor del torn i després el global; els fills no deleguen,
	// així que no hi ha interbloqueig.
	for i, t := range a.Tasks {
		id := nextDelegateID()
		started := time.Now()
		baseCtx := withDelegateIdentity(ctx, id, t.Prompt, started)
		baseCtx = withSubagentDef(baseCtx, defs[i])
		// Each child gets its own meter, chained to the parent's: the room
		// shows child-only totals while the parent turn still sums everything.
		baseCtx = llm.WithUsage(baseCtx, &llm.UsageMeter{})
		fail := func(index int, baseCtx context.Context, err error) {
			emitDelegateEvent(baseCtx, DelegateEvent{Status: delegateTerminalStatus(err, ctx.Err()), Message: err.Error(), MaxSteps: 12})
			outs[index] = fmt.Sprintf("== subagent %d: ERROR: %s", index+1, strings.TrimSpace(err.Error()))
		}
		if err := baseCtx.Err(); err != nil {
			fail(i, baseCtx, err)
			continue
		}
		gotTurn := tryAcquire(semaphore)
		gotGlobal := gotTurn && tryAcquire(delegateGlobalLive)
		if !gotGlobal {
			emitDelegateEvent(baseCtx, DelegateEvent{Status: DelegateQueued, Message: queuedMessage, MaxSteps: 12})
		}
		wg.Add(1)
		go func(index int, prompt string, baseCtx context.Context) {
			defer wg.Done()
			if !gotTurn && !waitSlot(baseCtx, semaphore) {
				fail(index, baseCtx, baseCtx.Err())
				return
			}
			defer func() { <-semaphore }()
			if !gotGlobal && !waitSlot(baseCtx, delegateGlobalLive) {
				fail(index, baseCtx, baseCtx.Err())
				return
			}
			defer func() { <-delegateGlobalLive }()
			// El rellotge del fill comença quan treballa, no mentre fa cua.
			childCtx, cancel := context.WithTimeout(baseCtx, 8*time.Minute)
			defer cancel()
			emitDelegateEvent(childCtx, DelegateEvent{Status: DelegateWorking, Message: "started", MaxSteps: 12})
			out, err := runDelegateSafely(runner, childCtx, prompt)
			if err != nil {
				status := delegateTerminalStatus(err, ctx.Err())
				emitDelegateEvent(childCtx, DelegateEvent{Status: status, Message: err.Error(), Output: ""})
				outs[index] = fmt.Sprintf("== subagent %d: ERROR: %s", index+1, strings.TrimSpace(err.Error()))
				return
			}
			if strings.TrimSpace(out) == "" {
				out = "(sense resposta)"
			}
			emitDelegateEvent(childCtx, DelegateEvent{Status: DelegateCompleted, Message: "completed", Output: delegatePreview(out)})
			outs[index] = fmt.Sprintf("== subagent %d:\n%s", index+1, retallaResum(out))
		}(i, t.Prompt, baseCtx)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return strings.Join(outs, "\n\n"), nil
}

func runDelegateSafely(runner func(context.Context, string) (string, error), ctx context.Context, prompt string) (out string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			out = ""
			err = fmt.Errorf("subagent panic: %v", recovered)
		}
	}()
	return runner(ctx, prompt)
}
