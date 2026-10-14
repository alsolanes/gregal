package agent

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/shell"
	"gregal/internal/tools"
)

// TestHookHelperProcess no és un test: és el programa que fan servir els
// hooks dels tests (el binari de test es crida a si mateix amb `-- mode`).
// Així no depenem de python ni de cap eina del sistema.
func TestHookHelperProcess(t *testing.T) {
	args := os.Args
	i := 0
	for i = range args {
		if args[i] == "--" {
			break
		}
	}
	if i >= len(args)-1 || args[i] != "--" {
		return
	}
	mode := args[i+1]
	raw, _ := io.ReadAll(os.Stdin)
	var in HookInput
	_ = json.Unmarshal(raw, &in)
	switch {
	case mode == "deny":
		os.Stderr.WriteString("no destructive commands")
		os.Exit(2)
	case mode == "json-deny":
		os.Stdout.WriteString(`{"decision":"deny","reason":"policy says no"}`)
	case mode == "ask":
		os.Stdout.WriteString(`{"decision":"ask","reason":"check with a human"}`)
	case mode == "allow":
		os.Stdout.WriteString(`{"decision":"allow"}`)
	case strings.HasPrefix(mode, "context="):
		os.Stdout.WriteString(strings.TrimPrefix(mode, "context="))
	case mode == "continue-once":
		if in.StopCount == 0 {
			os.Stdout.WriteString(`{"continue":true,"reason":"tests are still red"}`)
		}
	case mode == "continue-always":
		os.Stdout.WriteString(`{"continue":true,"reason":"again"}`)
	case mode == "sleep":
		time.Sleep(30 * time.Second)
	case mode == "fail":
		os.Stderr.WriteString("boom")
		os.Exit(1)
	case mode == "env":
		wd, _ := os.Getwd()
		os.Stdout.WriteString(os.Getenv("GREGAL_EVENT") + "|" + os.Getenv("GREGAL_SESSION") + "|" + os.Getenv("GREGAL_WORKSPACE") + "|" + wd + "|" + in.Tool)
	case mode == "result":
		os.Stdout.WriteString("saw " + in.Tool + ": " + in.Result)
	case strings.HasPrefix(mode, "record="):
		f, _ := os.OpenFile(strings.TrimPrefix(mode, "record="), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		f.Write(append(raw, '\n'))
		f.Close()
	}
	os.Exit(0)
}

// hookCmd és la comanda que crida el helper amb un mode.
func hookCmd(t *testing.T, mode string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := "-test.run=^TestHookHelperProcess$"
	if shell.IsPOSIX() {
		return shQuote(filepath.ToSlash(exe)) + " " + shQuote(run) + " -- " + shQuote(mode)
	}
	return `"` + exe + `" ` + run + ` -- "` + mode + `"`
}

func ambHooks(t *testing.T, h config.HooksCfg) {
	t.Helper()
	SetLifecycleHooks(h)
	t.Cleanup(func() { SetLifecycleHooks(config.HooksCfg{}) })
}

func TestHookExit2Bloqueja(t *testing.T) {
	r := runHookCmds(context.Background(), HookPreTool, []config.HookCmd{{Command: hookCmd(t, "deny")}}, HookInput{Tool: "bash"})
	if !r.Bloqueja() || !strings.Contains(r.Reason, "no destructive") {
		t.Fatalf("exit 2 ha de bloquejar amb stderr: %+v", r)
	}
}

func TestHookDecisionsJSON(t *testing.T) {
	for mode, want := range map[string]string{"json-deny": "deny", "ask": "ask", "allow": "allow"} {
		r := runHookCmds(context.Background(), HookPreTool, []config.HookCmd{{Command: hookCmd(t, mode)}}, HookInput{Tool: "bash"})
		if r.Decision != want {
			t.Fatalf("%s: decision=%q (%+v)", mode, r.Decision, r)
		}
	}
	// deny mana sobre allow, encara que vingui després.
	r := runHookCmds(context.Background(), HookPreTool, []config.HookCmd{{Command: hookCmd(t, "allow")}, {Command: hookCmd(t, "json-deny")}}, HookInput{Tool: "bash"})
	if r.Decision != "deny" || r.Reason != "policy says no" {
		t.Fatalf("deny ha de manar: %+v", r)
	}
}

func TestHookMatcher(t *testing.T) {
	cmds := []config.HookCmd{{Command: hookCmd(t, "deny"), Matcher: "bash|write"}}
	if r := runHookCmds(context.Background(), HookPreTool, cmds, HookInput{Tool: "read"}); r.Decision != "" {
		t.Fatalf("read no casa amb bash|write: %+v", r)
	}
	if r := runHookCmds(context.Background(), HookPreTool, cmds, HookInput{Tool: "bash_background"}); r.Decision != "" {
		t.Fatalf("el matcher és ancorat: %+v", r)
	}
	if r := runHookCmds(context.Background(), HookPreTool, cmds, HookInput{Tool: "write"}); r.Decision != "deny" {
		t.Fatalf("write casa: %+v", r)
	}
}

func TestHookTimeoutNoBloqueja(t *testing.T) {
	t0 := time.Now()
	r := runHookCmds(context.Background(), HookPreTool, []config.HookCmd{{Command: hookCmd(t, "sleep"), TimeoutS: 1}}, HookInput{Tool: "bash"})
	if time.Since(t0) > 8*time.Second {
		t.Fatalf("el timeout no ha tallat: %s", time.Since(t0))
	}
	if r.Bloqueja() || len(r.Avisos) != 1 || !strings.Contains(r.Avisos[0], "timed out") {
		t.Fatalf("timeout = avís sense bloquejar: %+v", r)
	}
}

func TestHookCancelMataLaComanda(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	t0 := time.Now()
	r := runHookCmds(ctx, HookPreTool, []config.HookCmd{{Command: hookCmd(t, "sleep"), TimeoutS: 60}}, HookInput{Tool: "bash"})
	if time.Since(t0) > 8*time.Second {
		t.Fatalf("cancel·lar el torn ha de matar el hook: %s", time.Since(t0))
	}
	if !r.Cancellat || r.Bloqueja() || len(r.Avisos) != 0 {
		t.Fatalf("cancel·lat i en silenci: %+v", r)
	}
}

func TestHookFallaNoBloqueja(t *testing.T) {
	r := runHookCmds(context.Background(), HookPreTool, []config.HookCmd{{Command: hookCmd(t, "fail")}}, HookInput{Tool: "bash"})
	if r.Bloqueja() || len(r.Avisos) != 1 || !strings.Contains(r.Avisos[0], "boom") {
		t.Fatalf("exit 1 = avís: %+v", r)
	}
}

func TestHookEntornIDirectori(t *testing.T) {
	dir := t.TempDir()
	r := runHookCmds(context.Background(), HookPostTool, []config.HookCmd{{Command: hookCmd(t, "env")}}, HookInput{Session: "s1", Workspace: dir, Tool: "read"})
	parts := strings.Split(r.Context, "|")
	if len(parts) != 5 || parts[0] != "post_tool" || parts[1] != "s1" || parts[2] != dir || parts[4] != "read" {
		t.Fatalf("entorn: %q", r.Context)
	}
	got, _ := filepath.EvalSymlinks(parts[3])
	want, _ := filepath.EvalSymlinks(dir)
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
		t.Fatalf("cwd=%q, volia %q", parts[3], dir)
	}
}

func TestMotorPreToolDeny(t *testing.T) {
	ambHooks(t, config.HooksCfg{PreTool: []config.HookCmd{{Command: hookCmd(t, "deny"), Matcher: "bash"}}})
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	evs := tn.RepPas("", []llm.ToolCall{cridaProva("1", "bash", `{"command":"ls"}`), cridaProva("2", "read", `{"path":"x.go"}`)}, nil)

	var bloc *Event
	for i := range evs {
		if evs[i].Tipus == EvResultat && evs[i].ID == "1" {
			bloc = &evs[i]
		}
	}
	if bloc == nil || !bloc.Fallada || !strings.Contains(bloc.Text, "hook: no destructive") {
		t.Fatalf("cal un resultat bloquejat amb el motiu: %+v", evs)
	}
	last := tn.Hist[len(tn.Hist)-1]
	if last.Role != "tool" || !strings.Contains(last.Content, "BLOCKED by pre_tool hook") {
		t.Fatalf("el model ha de veure el bloqueig: %+v", last)
	}
	p := tn.Seguent()
	if p.Ordre != OrdreExecuta || len(p.Calls) != 1 || p.Calls[0].ID != "2" {
		t.Fatalf("només s'executa la lectura: %+v", p)
	}
}

func TestMotorPreToolAskIAllow(t *testing.T) {
	ambHooks(t, config.HooksCfg{PreTool: []config.HookCmd{{Command: hookCmd(t, "ask"), Matcher: "read"}, {Command: hookCmd(t, "allow"), Matcher: "write"}}})
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	if p := tn.Seguent(); p.Ordre != OrdreAprova {
		t.Fatalf("ask ha de demanar permís encara que read sigui allow: %v", p.Ordre)
	}
	tn.RepAprovacio(false)
	tn.Seguent()
	// write fora del projecte demana permís per política; el hook allow el deixa passar.
	tn.RepPas("", []llm.ToolCall{cridaProva("2", "write", `{"path":"/fora/x.txt","content":"x"}`)}, nil)
	if p := tn.Seguent(); p.Ordre != OrdreExecuta {
		t.Fatalf("allow salta l'ask de la política: %v", p.Ordre)
	}
}

func TestMotorUserPromptBloqueja(t *testing.T) {
	ambHooks(t, config.HooksCfg{UserPrompt: []config.HookCmd{{Command: hookCmd(t, "deny")}}})
	tn := tornProva(ModeCode, 10)
	p := tn.Seguent()
	if p.Ordre != OrdreError || !strings.Contains(p.Text, "user_prompt hook") || tn.HookBloqueig() == "" {
		t.Fatalf("user_prompt bloqueja el torn: %+v", p)
	}
}

func TestMotorUserPromptISessionStartAfegeixenContext(t *testing.T) {
	ambHooks(t, config.HooksCfg{
		SessionStart: []config.HookCmd{{Command: hookCmd(t, "context=branch main")}},
		UserPrompt:   []config.HookCmd{{Command: hookCmd(t, "context=ticket 42")}},
	})
	tn := tornProva(ModeCode, 10)
	p := tn.Seguent()
	if p.Ordre != OrdrePasModel {
		t.Fatalf("el torn continua: %v", p.Ordre)
	}
	user := tn.Hist[len(tn.Hist)-1].Content
	if !strings.Contains(user, "branch main") || !strings.Contains(user, "ticket 42") || !strings.HasPrefix(user, "fes una cosa") {
		t.Fatalf("context enganxat a la tasca: %q", user)
	}
	// Conversa que ja té respostes: session_start no torna a córrer.
	cfg := &config.Config{Mode: ModeCode, Agent: config.AgentCfg{MaxSteps: 10}}
	tn2 := NouTorn(OpcionsTorn{Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeCode, MaxSteps: 10, Tasca: "i ara",
		Hist: []llm.Message{{Role: "user", Content: "hola"}, {Role: "assistant", Content: "hola!"}, {Role: "user", Content: "i ara"}}})
	tn2.Seguent()
	if u := tn2.Hist[len(tn2.Hist)-1].Content; strings.Contains(u, "branch main") || !strings.Contains(u, "ticket 42") {
		t.Fatalf("session_start només en conversa nova: %q", u)
	}
}

func TestMotorStopContinua(t *testing.T) {
	ambHooks(t, config.HooksCfg{Stop: []config.HookCmd{{Command: hookCmd(t, "continue-once")}}})
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	evs := tn.RepPas("fet", nil, nil)
	if tn.Acabat() {
		t.Fatalf("el hook stop demana continuar: %+v", evs)
	}
	if last := tn.Hist[len(tn.Hist)-1]; last.Role != "user" || !strings.Contains(last.Content, "tests are still red") {
		t.Fatalf("el motiu va al model: %+v", last)
	}
	if !strings.Contains(claus(evs), "stop hook") {
		t.Fatalf("la UI ho veu: %s", claus(evs))
	}
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("un altre pas: %v", p.Ordre)
	}
	tn.RepPas("ara sí", nil, nil)
	if !tn.Acabat() || tn.Resposta() != "ara sí" {
		t.Fatalf("la segona vegada acaba: %q", tn.Resposta())
	}
}

func TestMotorStopTopall(t *testing.T) {
	ambHooks(t, config.HooksCfg{StopMaxContinues: 2, Stop: []config.HookCmd{{Command: hookCmd(t, "continue-always")}}})
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	for i := 0; i < 5 && !tn.Acabat(); i++ {
		tn.Seguent()
		tn.RepPas("fet", nil, nil)
	}
	if !tn.Acabat() || tn.stopContinues != 2 {
		t.Fatalf("com a molt 2 continuacions: acabat=%v n=%d", tn.Acabat(), tn.stopContinues)
	}
}

func TestPostToolAnotaElResultat(t *testing.T) {
	ambHooks(t, config.HooksCfg{PostTool: []config.HookCmd{{Command: hookCmd(t, "result"), Matcher: "read"}}})
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("hola"), 0o600)
	out, _, err := ExecCtx(context.Background(), "", dir, "read", `{"path":`+quoteJSON(p)+`}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[hook] saw read:") {
		t.Fatalf("post_tool hauria d'anotar: %q", out)
	}
	// Una eina que no casa no s'anota.
	out, _, _ = ExecCtx(context.Background(), "", dir, "glob", `{"pattern":"*.txt"}`)
	if strings.Contains(out, "[hook]") {
		t.Fatalf("glob no casa: %q", out)
	}
}

func TestPreCompactHook(t *testing.T) {
	rec := filepath.Join(t.TempDir(), "rec.jsonl")
	ambHooks(t, config.HooksCfg{PreCompact: []config.HookCmd{{Command: hookCmd(t, "record="+rec)}}})
	ctx := WithHookScope(context.Background(), "web-1", t.TempDir())
	hookPreCompact(ctx, 12, 3400)
	raw, err := os.ReadFile(rec)
	if err != nil {
		t.Fatal(err)
	}
	var in HookInput
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &in); err != nil {
		t.Fatal(err)
	}
	if in.Event != "pre_compact" || in.Session != "web-1" || in.Messages != 12 || in.Tokens != 3400 {
		t.Fatalf("payload: %+v", in)
	}
}

func TestPreToolHookSubagents(t *testing.T) {
	ambHooks(t, config.HooksCfg{PreTool: []config.HookCmd{{Command: hookCmd(t, "ask"), Matcher: "grep"}}})
	if blocked, why := PreToolHook(context.Background(), "", t.TempDir(), "grep", `{}`); !blocked || !strings.Contains(why, "confirmation") {
		t.Fatalf("ask sense ningú = bloqueig: %v %q", blocked, why)
	}
	if blocked, _ := PreToolHook(context.Background(), "", t.TempDir(), "read", `{}`); blocked {
		t.Fatal("read no casa")
	}
}
