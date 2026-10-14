package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/config"
	"gregal/internal/procs"

	"gregal/internal/shell"
)

// Hooks del cicle de vida (config `hooks:`): session_start, user_prompt,
// pre_tool, post_tool, stop i pre_compact. Són comandes de l'usuari, de
// confiança com el post_edit: reben un JSON per stdin, corren al workspace
// de la sessió i moren amb el torn.
//
// Protocol de sortida (comú a tots els events):
//   - exit 0: stdout pot ser un JSON {"decision","reason","continue",
//     "context"} o text pla, que val com a context/nota per al model;
//   - exit 2: bloqueja; el motiu és stderr (o stdout si stderr és buit);
//   - qualsevol altra sortida, o el temps exhaurit: avís que no bloqueja.
//
// Viuen aquí (i no a cada client) perquè el motor i ExecCtx són el camí
// comú del TUI, la web, el headless i Telegram.

// Noms dels events.
const (
	HookSessionStart = "session_start"
	HookUserPrompt   = "user_prompt"
	HookPreTool      = "pre_tool"
	HookPostTool     = "post_tool"
	HookStop         = "stop"
	HookPreCompact   = "pre_compact"
)

// hookTimeoutDefecte és el temps d'una comanda sense timeout_s.
const hookTimeoutDefecte = 10 * time.Second

// stopMaxDefecte és quantes vegades per torn un hook stop pot fer
// continuar l'agent si el config no ho diu.
const stopMaxDefecte = 3

var lifecycleHooks atomic.Pointer[config.HooksCfg]

// SetLifecycleHooks desa els hooks del cicle de vida del config. Es crida
// a cada punt d'arrencada, al costat de SetPostEditHook.
func SetLifecycleHooks(h config.HooksCfg) {
	cp := h
	lifecycleHooks.Store(&cp)
}

func hooksActius() config.HooksCfg {
	if h := lifecycleHooks.Load(); h != nil {
		return *h
	}
	return config.HooksCfg{}
}

// hooksDe torna les comandes d'un event.
func hooksDe(event string) []config.HookCmd {
	return hooksActius().HookEvents()[event]
}

// HiHaHooks diu si l'event té alguna comanda configurada.
func HiHaHooks(event string) bool { return len(hooksDe(event)) > 0 }

// HookInput és el JSON que la comanda rep per stdin.
type HookInput struct {
	Event     string          `json:"event"`
	Session   string          `json:"session,omitempty"`
	Workspace string          `json:"workspace,omitempty"`
	Mode      string          `json:"mode,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	Result    string          `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Prompt    string          `json:"prompt,omitempty"`
	Answer    string          `json:"answer,omitempty"`
	StopCount int             `json:"stop_count,omitempty"`
	Messages  int             `json:"messages,omitempty"`
	Tokens    int             `json:"tokens,omitempty"`
}

// HookResult és la decisió agregada de totes les comandes d'un event.
type HookResult struct {
	// Decision: "" (cap opinió), "allow", "ask" o "deny". Un exit 2 és
	// "deny". Entre diverses comandes mana deny, després ask, després allow.
	Decision string
	Reason   string
	// Continue: un hook stop demana que l'agent continuï (exit 2 a stop
	// també compta com a continue, igual que a Claude Code).
	Continue bool
	// Context és el text per al model (stdout pla o el camp context).
	Context string
	// Avisos són fallades que no bloquegen (temps exhaurit, exit ≠ 0/2).
	Avisos []string
	// Cancellat: el torn s'ha aturat a mitja execució; no s'ha decidit res.
	Cancellat bool
}

// Bloqueja diu si el resultat atura l'acció.
func (r HookResult) Bloqueja() bool { return r.Decision == "deny" }

type hookSortida struct {
	Decision          string `json:"decision"`
	Reason            string `json:"reason"`
	Continue          *bool  `json:"continue"`
	Context           string `json:"context"`
	AdditionalContext string `json:"additionalContext"`
	Note              string `json:"note"`
}

var (
	matcherMu    sync.Mutex
	matcherCache = map[string]*regexp.Regexp{}
)

// hookCasa diu si el matcher (regex ancorada; buida = tot) accepta l'eina.
func hookCasa(matcher, tool string) bool {
	matcher = strings.TrimSpace(matcher)
	if matcher == "" || matcher == "*" {
		return true
	}
	matcherMu.Lock()
	re, ok := matcherCache[matcher]
	if !ok {
		re, _ = regexp.Compile("^(?:" + matcher + ")$")
		matcherCache[matcher] = re
	}
	matcherMu.Unlock()
	return re != nil && re.MatchString(tool)
}

// RunHooks executa les comandes de l'event en ordre i n'agrega el
// resultat. Una denegació atura les comandes següents.
func RunHooks(ctx context.Context, event string, in HookInput) HookResult {
	return runHookCmds(ctx, event, hooksDe(event), in)
}

func runHookCmds(ctx context.Context, event string, cmds []config.HookCmd, in HookInput) HookResult {
	var res HookResult
	if len(cmds) == 0 {
		return res
	}
	if ctx == nil {
		ctx = context.Background()
	}
	in.Event = event
	if in.Workspace == "" {
		in.Workspace, _ = os.Getwd()
	}
	var contexts, reasons []string
	for _, h := range cmds {
		if strings.TrimSpace(h.Command) == "" {
			continue
		}
		if (event == HookPreTool || event == HookPostTool) && !hookCasa(h.Matcher, in.Tool) {
			continue
		}
		if ctx.Err() != nil {
			res.Cancellat = true
			return res
		}
		one := runOneHook(ctx, event, h, in)
		if one.Cancellat {
			res.Cancellat = true
			return res
		}
		res.Avisos = append(res.Avisos, one.Avisos...)
		if one.Context != "" {
			contexts = append(contexts, one.Context)
		}
		if one.Reason != "" {
			reasons = append(reasons, one.Reason)
		}
		if one.Continue {
			res.Continue = true
		}
		switch one.Decision {
		case "deny":
			res.Decision = "deny"
		case "ask":
			if res.Decision != "deny" {
				res.Decision = "ask"
			}
		case "allow":
			if res.Decision == "" {
				res.Decision = "allow"
			}
		}
		if res.Decision == "deny" {
			break
		}
	}
	res.Context = strings.Join(contexts, "\n")
	res.Reason = strings.Join(reasons, "; ")
	return res
}

// runOneHook executa una comanda. El timeout penja del context del torn:
// cancel·lar el torn mata la comanda (com el post_edit).
func runOneHook(parent context.Context, event string, h config.HookCmd, in HookInput) HookResult {
	var res HookResult
	timeout := hookTimeoutDefecte
	if h.TimeoutS > 0 {
		timeout = time.Duration(h.TimeoutS) * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	payload, _ := json.Marshal(in)
	prog, args := shell.Argv(h.Command)
	c := exec.Command(prog, args...)
	if st, err := os.Stat(in.Workspace); err == nil && st.IsDir() {
		c.Dir = in.Workspace
	}
	c.Env = append(os.Environ(),
		"GREGAL_EVENT="+event,
		"GREGAL_SESSION="+in.Session,
		"GREGAL_WORKSPACE="+in.Workspace,
		"GREGAL_TOOL="+in.Tool,
	)
	c.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	c.Stdout, c.Stderr = &stdout, &stderr
	// Un fill que hereta les canonades (un sleep dins de sh -c) faria que
	// Wait esperés el seu final encara que el shell ja fos mort.
	c.WaitDelay = 2 * time.Second
	// procs.Run mata l'arbre sencer (grup de processos / job de Windows)
	// quan el context acaba: matar només el shell deixava el fill viu.
	err := procs.Run(ctx, c)
	out := strings.TrimSpace(truncateHook(stdout.String()))
	errOut := strings.TrimSpace(truncateHook(stderr.String()))
	if parent.Err() != nil {
		res.Cancellat = true
		return res
	}
	if ctx.Err() == context.DeadlineExceeded {
		res.Avisos = append(res.Avisos, fmt.Sprintf("hook %s: timed out after %s (%s)", event, timeout, hookNom(h.Command)))
		return res
	}
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			res.Avisos = append(res.Avisos, fmt.Sprintf("hook %s: %v", event, err))
			return res
		}
	}
	switch code {
	case 0:
		aplicaSortida(&res, out)
	case 2:
		motiu := errOut
		if motiu == "" {
			motiu = out
		}
		if motiu == "" {
			motiu = "blocked by " + event + " hook"
		}
		res.Reason = motiu
		if event == HookStop {
			res.Continue = true
		} else {
			res.Decision = "deny"
		}
	default:
		msg := errOut
		if msg == "" {
			msg = out
		}
		res.Avisos = append(res.Avisos, strings.TrimSpace(fmt.Sprintf("hook %s failed (exit %d): %s", event, code, msg)))
	}
	return res
}

// aplicaSortida interpreta el stdout d'un exit 0.
func aplicaSortida(res *HookResult, out string) {
	if out == "" {
		return
	}
	if strings.HasPrefix(out, "{") {
		var s hookSortida
		if json.Unmarshal([]byte(out), &s) == nil {
			switch d := strings.ToLower(strings.TrimSpace(s.Decision)); d {
			case "allow", "ask", "deny":
				res.Decision = d
			case "block":
				res.Decision = "deny"
			}
			res.Reason = strings.TrimSpace(s.Reason)
			if s.Continue != nil && *s.Continue {
				res.Continue = true
			}
			ctx := firstOf(s.Context, s.AdditionalContext, s.Note)
			res.Context = strings.TrimSpace(ctx)
			return
		}
	}
	res.Context = out
}

func hookNom(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if len(cmd) > 60 {
		return cmd[:60] + "…"
	}
	return cmd
}

// --- abast (sessió i workspace) per als hooks que corren lluny del motor ---

type hookScopeKey struct{}

type hookScope struct{ session, workspace string }

// WithHookScope desa la sessió i el workspace al context perquè els hooks
// que corren lluny del motor (pre_compact, dins de Summarize) els sàpiguen.
func WithHookScope(ctx context.Context, session, workspace string) context.Context {
	return context.WithValue(ctx, hookScopeKey{}, hookScope{session, workspace})
}

func hookScopeDe(ctx context.Context) (string, string) {
	if ctx == nil {
		return "", ""
	}
	sc, _ := ctx.Value(hookScopeKey{}).(hookScope)
	return sc.session, sc.workspace
}

// --- post_tool i pre_compact (fora del motor) ---

// ExecCtx és ExecIn amb el context del torn. Cancel·lar-lo atura les
// eines que poden trigar (bash i el navegador); la resta són prou
// ràpides perquè no calgui. És el que fa que Esc talli de debò una
// comanda llarga en comptes de deixar-la corrent en segon pla.
// Cap eina nova comença si el torn ja està cancel·lat.
//
// Després de l'eina hi passen els hooks post_tool: el que diguin
// s'enganxa al resultat perquè el model ho llegeixi.
func ExecCtx(ctx context.Context, session, dir, name, argsJSON string) (out string, images []string, err error) {
	out, images, err = execCtxEina(ctx, session, dir, name, argsJSON)
	if !HiHaHooks(HookPostTool) || ctx.Err() != nil {
		return out, images, err
	}
	in := HookInput{Session: session, Workspace: dir, Tool: name, Args: argsRaw(argsJSON), Result: truncateHook(out)}
	if err != nil {
		in.Error = err.Error()
	}
	if nota := notaPostTool(RunHooks(ctx, HookPostTool, in)); nota != "" {
		if err != nil {
			return out, images, fmt.Errorf("%w\n%s", err, nota)
		}
		out += "\n" + nota
	}
	return out, images, err
}

// notaPostTool converteix el resultat d'un post_tool en la nota que
// s'enganxa a la sortida de l'eina.
func notaPostTool(r HookResult) string {
	if r.Cancellat {
		return ""
	}
	var parts []string
	if r.Context != "" {
		parts = append(parts, "[hook] "+r.Context)
	}
	if r.Bloqueja() && r.Reason != "" {
		parts = append(parts, "[hook] "+r.Reason)
	} else if r.Reason != "" && r.Context == "" {
		parts = append(parts, "[hook] "+r.Reason)
	}
	for _, a := range r.Avisos {
		parts = append(parts, "[hook] warning: "+a)
	}
	return strings.Join(parts, "\n")
}

// hookPreCompact avisa abans de resumir l'historial (Summarize). No pot
// aturar la compactació: és per desar còpies o registrar-ho.
func hookPreCompact(ctx context.Context, msgs, tokens int) {
	if !HiHaHooks(HookPreCompact) {
		return
	}
	session, ws := hookScopeDe(ctx)
	RunHooks(ctx, HookPreCompact, HookInput{Session: session, Workspace: ws, Messages: msgs, Tokens: tokens})
}

// PreToolHook aplica els hooks pre_tool fora del motor (subagents). Torna
// el motiu si la crida queda bloquejada; un "ask" també bloqueja, perquè
// aquí no hi ha ningú per respondre.
func PreToolHook(ctx context.Context, session, dir, name, argsJSON string) (blocked bool, reason string) {
	if !HiHaHooks(HookPreTool) {
		return false, ""
	}
	r := RunHooks(ctx, HookPreTool, HookInput{Session: session, Workspace: dir, Tool: name, Args: argsRaw(argsJSON)})
	switch r.Decision {
	case "deny":
		return true, firstOf(r.Reason, "blocked by pre_tool hook")
	case "ask":
		return true, "pre_tool hook asked for confirmation, which is not available here: " + firstOf(r.Reason, name)
	}
	return false, ""
}

// argsRaw passa els arguments com a JSON si ho són (si no, com a string).
func argsRaw(argsJSON string) json.RawMessage {
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" {
		return nil
	}
	if json.Valid([]byte(argsJSON)) {
		return json.RawMessage(argsJSON)
	}
	b, _ := json.Marshal(argsJSON)
	return b
}
