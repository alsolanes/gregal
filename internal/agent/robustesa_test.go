package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// validaProtocol comprova la regla dels proveïdors estrictes: després d'un
// assistant amb tool_calls venen, seguits, els tool results de TOTES les
// seves crides; cap altre missatge enmig.
func validaProtocol(t *testing.T, hist []llm.Message) {
	t.Helper()
	for i, m := range hist {
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		falten := map[string]bool{}
		for _, c := range m.ToolCalls {
			falten[c.ID] = true
		}
		for j := i + 1; len(falten) > 0; j++ {
			if j >= len(hist) {
				t.Fatalf("assistant #%d: calls without result %v", i, falten)
			}
			if hist[j].Role != "tool" {
				t.Fatalf("assistant #%d: %s message between tool results (missing %v): %q", i, hist[j].Role, falten, hist[j].Content)
			}
			delete(falten, hist[j].ToolCallID)
		}
	}
}

// B1: a doom guide raised while a call of the same batch is still waiting
// for approval must wait until the batch has all its results.
func TestDoomGuideEsperaElLotSencer(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	tn.Seguent()
	// Two greps that only differ in a number, same output: not yet a loop.
	for _, n := range []string{"1", "2"} {
		c := cridaProva("g"+n, "grep", `{"pattern":"needle`+n+`","dir":"."}`)
		tn.RepPas("", []llm.ToolCall{c}, nil)
		finsAExecuta(t, tn, []Execucio{{Call: c, Sortida: "a.go:1: needle"}})
	}
	// Third step: the third grep (doom) plus a write outside the project,
	// which waits for approval.
	g3 := cridaProva("g3", "grep", `{"pattern":"needle3","dir":"."}`)
	w := cridaProva("w1", "write", `{"path":"/fora/x.txt","content":"x"}`)
	tn.RepPas("", []llm.ToolCall{g3, w}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdreExecuta || len(p.Calls) != 1 || p.Calls[0].ID != "g3" {
		t.Fatalf("expected the grep to run first: %+v", p)
	}
	evs := tn.RepExecucions([]Execucio{{Call: g3, Sortida: "a.go:1: needle"}})
	vista := false
	for _, e := range evs {
		if e.Text == DoomGuide {
			vista = true
		}
	}
	if !vista {
		t.Fatal("the third identical call+output must raise the doom guide")
	}
	if p := tn.Seguent(); p.Ordre != OrdreAprova || p.Call.ID != "w1" {
		t.Fatalf("expected the approval of the write: %+v", p)
	}
	tn.RepAprovacio(false, "denied")
	p = tn.Seguent()
	if p.Ordre != OrdrePasModel {
		t.Fatalf("expected the next model step, got %d", p.Ordre)
	}
	validaProtocol(t, tn.Hist)
	if last := tn.Hist[len(tn.Hist)-1]; last.Role != "user" || last.Content != DoomGuide {
		t.Fatalf("the guide goes right after the batch: %+v", last)
	}
}

// B1: the guide is added at once when nothing in the batch is pending.
func TestDoomGuideLotCompletDeSeguida(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	tn.Seguent()
	var calls []llm.ToolCall
	var outs []Execucio
	for _, n := range []string{"1", "2", "3"} {
		c := cridaProva("g"+n, "grep", `{"pattern":"x`+n+`"}`)
		calls = append(calls, c)
		outs = append(outs, Execucio{Call: c, Sortida: "same"})
	}
	tn.RepPas("", calls, nil)
	finsAExecuta(t, tn, outs)
	validaProtocol(t, tn.Hist)
	if last := tn.Hist[len(tn.Hist)-1]; last.Content != DoomGuide {
		t.Fatalf("guide must follow the complete batch: %+v", last)
	}
}

// B1: different searches with no matches are progress, not a loop.
func TestDoomCerquesDiferentsSenseResultats(t *testing.T) {
	var d DoomTracker
	for _, p := range []string{"Ask", "auto", "approve", "policy"} {
		if d.NoteOutCall(DoomSig("grep", `{"pattern":"`+p+`"}`), "sense coincidències") {
			t.Fatalf("no-match output counted as a repeat (%s)", p)
		}
	}
	d = DoomTracker{}
	for _, p := range []string{"alpha", "beta", "gamma"} {
		if d.NoteOutCall(DoomSig("grep", `{"pattern":"`+p+`"}`), "x.go:1: hit") {
			t.Fatalf("different calls with the same output are not a loop (%s)", p)
		}
	}
	d = DoomTracker{}
	sig := DoomSig("bash", `{"command":"cat log.txt"}`)
	if d.NoteOutCall(sig, "boom") || d.NoteOutCall(sig, "boom") || !d.NoteOutCall(sig, "boom") {
		t.Fatal("the same call with the same output three times is a loop")
	}
}

// B2: the provider's reasoning is kept on tool-call messages only.
func TestRaonamentNomesAMissatgesAmbCrides(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	c := cridaProva("r1", "read", `{"path":"a.go"}`)
	tn.RepPasRaonat("", "I need a.go", []llm.ToolCall{c}, nil)
	if got := tn.Hist[len(tn.Hist)-1].ReasoningContent; got != "I need a.go" {
		t.Fatalf("reasoning lost: %q", got)
	}
	finsAExecuta(t, tn, []Execucio{{Call: c, Sortida: "package a"}})
	tn.Seguent()
	tn.RepPasRaonat("done", "thinking about the answer", nil, nil)
	for _, m := range tn.Hist {
		if m.Role == "assistant" && len(m.ToolCalls) == 0 && m.ReasoningContent != "" {
			t.Fatalf("final answer must not carry reasoning: %+v", m)
		}
	}
}

// B2: the «reasoning_content must be passed back» 400 is retried once.
func TestRaonamentRebutjatEsReintentaUnCop(t *testing.T) {
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	err := errors.New("provider x: HTTP 400: The `reasoning_content` in the thinking mode must be passed back to the API.")
	tn.RepPas("", nil, err)
	if tn.Fallit() || tn.Passos() != 0 {
		t.Fatalf("first round-trip error must be retried without spending a step (fallit=%v passos=%d)", tn.Fallit(), tn.Passos())
	}
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("expected a retry step, got %d", p.Ordre)
	}
	tn.RepPas("", nil, err)
	if !tn.Fallit() {
		t.Fatal("the second one kills the turn")
	}
}

// B7: a transport error already went through the client's retries; the
// engine neither repeats them nor guides the model about it.
func TestErrorDeXarxaNoEsReintentaAlMotor(t *testing.T) {
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	n := len(tn.Hist)
	tn.RepPas("", nil, errors.New(`provider http://127.0.0.1:9/v1: Post "http://127.0.0.1:9/v1/chat/completions": dial tcp 127.0.0.1:9: connect: connection refused`))
	if !tn.Fallit() {
		t.Fatal("a transport error must end the turn")
	}
	if len(tn.Hist) != n {
		t.Fatalf("no guidance for a request that never reached the model: %+v", tn.Hist[n:])
	}
}

// B6: with a hard limit there is no CONTINUE/FINAL question.
func TestTopallDurSenseAmpliacio(t *testing.T) {
	tools.TodoClear()
	cfg := &config.Config{Mode: ModeCode, Agent: config.AgentCfg{MaxSteps: 1}}
	tn := NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeCode, MaxSteps: 1, SenseAmpliacions: true,
		Hist: []llm.Message{{Role: "user", Content: "x"}},
		Rol:  func() (string, string) { return "p", "m" },
	})
	tn.Seguent()
	c := cridaProva("r1", "read", `{"path":"a.go"}`)
	tn.RepPas("", []llm.ToolCall{c}, nil)
	finsAExecuta(t, tn, []Execucio{{Call: c, Sortida: "x"}})
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("hard limit must go straight to the summary, got %d", p.Ordre)
	}
	if !tn.Esgotat() {
		t.Fatal("the summary is forced: Esgotat")
	}
}

// B6: «FINAL» followed by a real summary is the answer (no extra call).
func TestFinalAmbResumEsLaResposta(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.Seguent()
	c := cridaProva("r1", "read", `{"path":"a.go"}`)
	tn.RepPas("", []llm.ToolCall{c}, nil)
	finsAExecuta(t, tn, []Execucio{{Call: c, Sortida: "x"}})
	if p := tn.Seguent(); p.Ordre != OrdreAmplia {
		t.Fatalf("expected the extension question, got %d", p.Ordre)
	}
	resum := "Implemented all six functions in strutil.js and added 12 tests; `node --test` reports 12 passed, 0 failed."
	evs := tn.RepAmpliacio("FINAL\n\n"+resum, nil)
	if !tn.Acabat() || !tn.Esgotat() || tn.Resposta() != resum {
		t.Fatalf("acabat=%v esgotat=%v resposta=%q", tn.Acabat(), tn.Esgotat(), tn.Resposta())
	}
	var vista bool
	for _, e := range evs {
		vista = vista || (e.Tipus == EvResposta && e.Text == resum)
	}
	if !vista {
		t.Fatal("the summary must be emitted as the answer")
	}
	if p := tn.Seguent(); p.Ordre != OrdreAcaba {
		t.Fatalf("no synthesis call after a FINAL with summary, got %d", p.Ordre)
	}
}

func TestResumDeFinal(t *testing.T) {
	llarg := strings.Repeat("Changed calc.py and verified with python -m unittest. ", 3)
	for _, c := range []struct {
		in string
		ok bool
	}{
		{"FINAL", false},
		{"FINAL\n\nDone.", false},
		{"FINAL\n\n" + llarg, true},
		{"**FINAL**\n" + llarg, true},
		{llarg + "\nFINAL", true},
		{"FINAL: " + llarg, true},
		{"FINALLY " + llarg, false},
		{llarg, false},
		{"FINAL\n" + llarg + "\n<tool_call><function=glob><parameter=pattern>*</parameter></function></tool_call>", false},
	} {
		if _, ok := resumDeFinal(c.in); ok != c.ok {
			t.Errorf("resumDeFinal(%.40q) ok=%v, want %v", c.in, ok, c.ok)
		}
	}
}

// Cancelling with calls still queued must leave a valid history.
func TestCancellaTancaLesCrides(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	w := cridaProva("w1", "write", `{"path":"/fora/x.txt","content":"x"}`)
	r := cridaProva("r1", "read", `{"path":"a.go"}`)
	tn.RepPas("", []llm.ToolCall{w, r}, nil)
	tn.Cancella()
	validaProtocol(t, tn.Hist)
}

// B10: executed results carry the call id; the driver can tell them from
// the ones the engine resolves itself.
func TestResultatsExecutatsPortenID(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	c := cridaProva("r1", "read", `{"path":"a.go"}`)
	tn.RepPas("", []llm.ToolCall{c}, nil)
	evs := finsAExecuta(t, tn, []Execucio{{Call: c, Sortida: "x"}})
	for _, e := range evs {
		if e.Tipus == EvResultat && e.Text == "x" && (e.ID != "r1" || !e.Executada) {
			t.Fatalf("executed result without id/flag: %+v", e)
		}
	}
}

func TestErrorAmbSortida(t *testing.T) {
	if got := ErrorAmbSortida(errors.New("timeout (2m0s): still running"), "line 1\nline 2"); !strings.Contains(got, "line 2") || !strings.HasPrefix(got, "ERROR: timeout") {
		t.Fatalf("partial output lost: %q", got)
	}
	// bash exit errors already include the output: no duplicate.
	if got := ErrorAmbSortida(errors.New("exit: exit status 1\nFAIL x"), "FAIL x"); strings.Count(got, "FAIL x") != 1 {
		t.Fatalf("duplicated output: %q", got)
	}
	if got := ErrorAmbSortida(errors.New("boom"), ""); got != "ERROR: boom" {
		t.Fatalf("got %q", got)
	}
}

// B8
func TestSmallTalkForConversa(t *testing.T) {
	if _, ok := SmallTalkFor("ok", "en", true, false); ok {
		t.Fatal("an acknowledgement mid-conversation must go to the model")
	}
	if r, ok := SmallTalkFor("ok", "en", false, false); !ok || r != smallTalkReplies[kindAck]["en"] {
		t.Fatalf("ok at the start: %q %v", r, ok)
	}
	if r, ok := SmallTalkFor("hola", "en", false, false); !ok || r != smallTalkReplies[kindGreet]["en"] {
		t.Fatalf("reply in the session language: %q", r)
	}
	if r, ok := SmallTalkFor("thanks", "ca", true, false); !ok || r != smallTalkReplies[kindThanks]["ca"] {
		t.Fatalf("thanks mid-conversation is still small talk: %q %v", r, ok)
	}
	if r, ok := SmallTalk("hola", false); !ok || r != smallTalkReplies[kindGreet]["ca"] {
		t.Fatalf("legacy SmallTalk keeps the phrase language: %q", r)
	}
}

// Q3
func TestSenseMapesAntics(t *testing.T) {
	hist := []llm.Message{
		{Role: "user", Content: "first task\n\n" + MarcaContext + " ab12] dir\nFiles (1):\na.go\n"},
		{Role: "assistant", Content: "done"},
		{Role: "user", Content: "second", Images: []string{"data:image/png;base64,AA"}},
	}
	out := SenseMapesAntics(hist)
	if out[0].Content != "first task" || len(out[2].Images) != 1 {
		t.Fatalf("got %+v", out)
	}
	if !strings.Contains(hist[0].Content, MarcaContext) {
		t.Fatal("input must not be modified")
	}
	if TeConversa(hist[:1]) || !TeConversa(hist) {
		t.Fatal("TeConversa")
	}
}

// Q5
func TestFinestraConeguda(t *testing.T) {
	for model, want := range map[string]int{
		"deepseek-v4.1-flash":                 128000,
		"deepseek-coder":                      64000,
		"anthropic/claude-sonnet-4-5":         200000,
		"gpt-4o-mini":                         128000,
		"some-unknown-model":                  0,
		"openrouter/google/gemini-2.5-pro":    1000000,
		"moonshotai/kimi-k2-instruct":         128000,
		"qwen/qwen3-coder-480b-a35b-instruct": 128000,
	} {
		if got := finestraConeguda(model); got != want {
			t.Errorf("finestraConeguda(%q)=%d, want %d", model, got, want)
		}
	}
	ResetWindows()
	cfg := &config.Config{Providers: map[string]config.Provider{
		"zen":   {BaseURL: "https://opencode.ai/zen/go/v1"},
		"local": {BaseURL: "http://localhost:8080/v1"},
	}}
	if n, src := WindowSource(cfg, config.Role{Provider: "zen", Model: "deepseek-v4.1-flash"}); n != 128000 || src != "coneguda" {
		t.Fatalf("remote known model: %d %s", n, src)
	}
	if n, _ := WindowSource(cfg, config.Role{Provider: "local", Model: "deepseek-v4.1-flash"}); n != 8192 {
		t.Fatalf("a local server keeps its conservative default: %d", n)
	}
	if n, src := WindowSource(cfg, config.Role{Provider: "zen", Model: "deepseek-v4.1-flash", ContextWindow: 50000}); n != 50000 || src != "config" {
		t.Fatalf("config wins: %d %s", n, src)
	}
}

// fakeLLM is an OpenAI-compatible stub: answer(n, body) returns the JSON
// reply for the n-th POST (1-based) and records every request body.
type fakeLLM struct {
	mu     sync.Mutex
	bodies []string
	answer func(n int, body string) string
}

func (f *fakeLLM) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.bodies = append(f.bodies, string(raw))
		n := len(f.bodies)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, f.answer(n, string(raw)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func toolReply(calls ...llm.ToolCall) string {
	type wireCall struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	var ws []wireCall
	for _, c := range calls {
		var w wireCall
		w.ID, w.Type = c.ID, "function"
		w.Function.Name, w.Function.Arguments = c.Function.Name, c.Function.Arguments
		ws = append(ws, w)
	}
	raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
		"message":       map[string]any{"role": "assistant", "content": "", "tool_calls": ws},
		"finish_reason": "tool_calls",
	}}})
	return string(raw)
}

func textReply(s string) string {
	raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
		"message": map[string]any{"role": "assistant", "content": s}, "finish_reason": "stop",
	}}})
	return string(raw)
}

// B9: the non-interactive run hides the tools that need a person, tells the
// model so, and a question still answers «not delivered».
func TestHeadlessSenseEinesInteractives(t *testing.T) {
	tools.TodoClear()
	f := &fakeLLM{answer: func(n int, body string) string {
		if n == 1 {
			return toolReply(cridaProva("q1", "question", `{"query":"Which file?","options":[{"label":"a"},{"label":"b"}]}`))
		}
		return textReply("I could not decide: which file should I change?")
	}}
	srv := f.server(t)
	res, err := RunNonInteractiveOpts(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "fix it", Mode: "code", MaxSteps: 5, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var first struct {
		Messages []llm.Message `json:"messages"`
		Tools    []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(f.bodies[0]), &first); err != nil {
		t.Fatal(err)
	}
	for _, tl := range first.Tools {
		if einesInteractives[tl.Function.Name] {
			t.Fatalf("interactive tool offered in a non-interactive run: %s", tl.Function.Name)
		}
	}
	if !strings.Contains(first.Messages[0].Content, "Non-interactive run") {
		t.Fatal("the system prompt must say nobody can answer")
	}
	if !strings.Contains(f.bodies[1], "QUESTION NOT DELIVERED") || !strings.Contains(f.bodies[1], "Do not make changes the user did not explicitly request") {
		t.Fatal("the question must be answered with the no-UI reply")
	}
	if len(res.Trace) != 1 || !res.Trace[0].Failed || !strings.Contains(res.Trace[0].Summary, "QUESTION NOT DELIVERED") {
		t.Fatalf("trace: %+v", res.Trace)
	}
	if res.Tools == nil {
		t.Fatal("tools_used must be [] not null")
	}
}

// B10: a denied call keeps its own trace entry; the next call's result is
// not attributed to it.
func TestHeadlessTraceAparellaPerID(t *testing.T) {
	tools.TodoClear()
	f := &fakeLLM{answer: func(n int, body string) string {
		if n == 1 {
			return toolReply(
				cridaProva("b1", "bash", `{"command":"python -c \"print(1)\""}`),
				cridaProva("b2", "bash", `{"command":"echo traced-ok"}`),
			)
		}
		return textReply("done")
	}}
	srv := f.server(t)
	res, err := RunNonInteractiveOpts(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "run", Mode: "code", MaxSteps: 5, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trace) != 2 {
		t.Fatalf("trace: %+v", res.Trace)
	}
	if !res.Trace[0].Failed || !strings.Contains(res.Trace[0].Summary, "TOOL DENIED") {
		t.Fatalf("denied call must be recorded as failed: %+v", res.Trace[0])
	}
	if res.Trace[1].Failed || !strings.Contains(res.Trace[1].Summary, "traced-ok") {
		t.Fatalf("echo result must stay with the echo call: %+v", res.Trace[1])
	}
}

// B4: in code mode a non-interactive run writes inside its workspace
// without --auto-approve (the policy now knows the project directory).
func TestHeadlessEscriuDinsDelWorkspace(t *testing.T) {
	tools.TodoClear()
	dir := t.TempDir()
	target := filepath.ToSlash(filepath.Join(dir, "out.txt"))
	f := &fakeLLM{answer: func(n int, body string) string {
		if n == 1 {
			args, _ := json.Marshal(map[string]string{"path": target, "content": "hello"})
			return toolReply(cridaProva("w1", "write", string(args)))
		}
		return textReply("written")
	}}
	srv := f.server(t)
	if _, err := RunNonInteractiveOpts(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "write", Mode: "code", MaxSteps: 5, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "hello" {
		t.Fatalf("write inside the workspace was not applied: %v %q", err, b)
	}
}

// B6: --max-steps is a hard cap and the JSON says the run was cut.
func TestHeadlessTopallDurIExhausted(t *testing.T) {
	tools.TodoClear()
	f := &fakeLLM{answer: func(n int, body string) string {
		if strings.Contains(body, `"tools"`) {
			args, _ := json.Marshal(map[string]string{"path": fmt.Sprintf("f%d.txt", n)})
			return toolReply(cridaProva(fmt.Sprintf("r%d", n), "read", string(args)))
		}
		return textReply("Summary: read one file, nothing else done.")
	}}
	srv := f.server(t)
	res, err := RunNonInteractiveOpts(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "read", Mode: "code", MaxSteps: 1, HardStepLimit: true, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 2 || res.Steps != 1 || !res.Exhausted {
		t.Fatalf("calls=%d steps=%d exhausted=%v", len(f.bodies), res.Steps, res.Exhausted)
	}
}

// B5: failed verification is reported (Verified=false) and the rounds add up.
func TestVerificacioFallidaSumaRondes(t *testing.T) {
	tools.TodoClear()
	f := &fakeLLM{answer: func(n int, body string) string { return textReply("done") }}
	srv := f.server(t)
	dir := t.TempDir()
	res, err := RunAmbVerificacio(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "do", Mode: "code", MaxSteps: 3, Dir: dir}, "exit 3", 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verified == nil || *res.Verified || res.VerifyRounds != 2 || len(f.bodies) != 2 {
		t.Fatalf("verified=%v rounds=%d calls=%d", res.Verified, res.VerifyRounds, len(f.bodies))
	}
	ok, err := RunAmbVerificacio(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "do", Mode: "code", MaxSteps: 3, Dir: dir}, "exit 0", 2)
	if err != nil || ok.Verified == nil || !*ok.Verified {
		t.Fatalf("passing verification: %+v %v", ok.Verified, err)
	}
	none, _ := RunAmbVerificacio(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "do", Mode: "code", MaxSteps: 3, Dir: dir}, "", 2)
	if none.Verified != nil {
		t.Fatal("without --verify there is no verified field")
	}
}

// A summary that is only a DSML tool call is not an answer: one more try,
// telling the model tools are off; then the plain-text answer is used.
func TestSintesiAmbCridaDSMLEsReintenta(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.Seguent()
	c := cridaProva("w1", "read", `{"path":"a.go"}`)
	tn.RepPas("", []llm.ToolCall{c}, nil)
	finsAExecuta(t, tn, []Execucio{{Call: c, Sortida: "x"}})
	tn.RepAmpliacio("FINAL", nil)
	p := tn.Seguent()
	if p.Ordre != OrdreSintesi {
		t.Fatalf("expected synthesis, got %d", p.Ordre)
	}
	dsml := "<｜｜DSML｜｜ calls>\n<｜｜DSML｜｜ invoke name=\"bash\">\n<｜｜DSML｜｜ parameter name=\"command\" string=\"true\">npm test</｜｜DSML｜｜ parameter>\n</｜｜DSML｜｜ invoke>\n</｜｜DSML｜｜ calls>"
	tn.RepSintesi(dsml, nil)
	if tn.Acabat() || tn.Resposta() != "" {
		t.Fatalf("a DSML-only summary must not end the turn (resposta=%q)", tn.Resposta())
	}
	p = tn.Seguent()
	if p.Ordre != OrdreSintesi || !strings.Contains(histUltim(p), sintesiSenseEines) {
		t.Fatalf("the retry must say tools are disabled: %d %q", p.Ordre, histUltim(p))
	}
	tn.RepSintesi("Wrote strutil.js; tests not run yet.", nil)
	if !tn.Acabat() || tn.Resposta() != "Wrote strutil.js; tests not run yet." {
		t.Fatalf("resposta=%q", tn.Resposta())
	}
}
