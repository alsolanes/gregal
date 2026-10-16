package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeOllama és un Ollama de mentida: /api/show, /api/ps i /api/chat.
type fakeOllama struct {
	mu       sync.Mutex
	caps     []string
	modelCtx int
	loaded   int // context_length del model carregat (0 = cap)
	bodies   []map[string]any
	reply    func(w http.ResponseWriter, body map[string]any)
}

func (f *fakeOllama) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/show":
			json.NewEncoder(w).Encode(map[string]any{
				"capabilities": f.caps,
				"model_info":   map[string]any{"general.architecture": "qwen3", "qwen3.context_length": f.modelCtx},
				"parameters":   "temperature 0.6",
			})
		case "/api/ps":
			models := []map[string]any{}
			if f.loaded > 0 {
				models = append(models, map[string]any{"name": "qwen3:latest", "model": "qwen3:latest", "context_length": f.loaded})
			}
			json.NewEncoder(w).Encode(map[string]any{"models": models})
		case "/api/chat":
			var body map[string]any
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("cos no JSON: %v", err)
			}
			f.mu.Lock()
			f.bodies = append(f.bodies, body)
			f.mu.Unlock()
			f.reply(w, body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func ndjson(w http.ResponseWriter, lines ...string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	for _, l := range lines {
		fmt.Fprintln(w, l)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func TestOllamaNatiuStreamAmbEinesINumCtxFix(t *testing.T) {
	resetOllama()
	f := &fakeOllama{caps: []string{"completion", "tools"}, modelCtx: 131072, loaded: 40960}
	f.reply = func(w http.ResponseWriter, body map[string]any) {
		ndjson(w,
			`{"message":{"role":"assistant","content":"","thinking":"hmm"},"done":false}`,
			`{"message":{"role":"assistant","content":"Llegeixo. "},"done":false}`,
			`{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"read","arguments":{"path":"a.go"}}}]},"done":false}`,
			`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":5000,"prompt_eval_duration":1000000000,"eval_count":20,"eval_duration":500000000,"load_duration":1000000}`,
		)
	}
	srv := f.server(t)
	base := srv.URL + "/v1"
	RegisterProviderType(base, ProviderOllama)
	t.Cleanup(func() { RegisterProviderType(base, "") })

	c := New()
	meter := &UsageMeter{}
	ctx := WithUsage(context.Background(), meter)
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "mira", Images: []string{"data:image/png;base64,QUJD"}},
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "c1", Type: "function", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "ls", Arguments: `{"dir":"."}`}}}},
		{Role: "tool", ToolCallID: "c1", Content: "a.go"},
	}
	specs := []ToolSpec{{Name: "read", Description: "read a file", Parameters: map[string]any{"type": "object"}}}
	var streamed strings.Builder
	text, calls, err := c.ChatStreamWithTools(ctx, base, "", "qwen3", msgs, 0.6, 1000, specs, func(s string) { streamed.WriteString(s) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if text != "Llegeixo. " || streamed.String() != "Llegeixo. " {
		t.Fatalf("text=%q streamed=%q", text, streamed.String())
	}
	if len(calls) != 1 || calls[0].Function.Name != "read" || calls[0].Function.Arguments != `{"path":"a.go"}` || calls[0].ID == "" {
		t.Fatalf("calls=%+v", calls)
	}
	// Segona crida: el num_ctx no pot canviar.
	if _, _, err := c.ChatStreamWithTools(ctx, base, "", "qwen3", msgs, 0.6, 1000, specs, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 2 {
		t.Fatalf("peticions = %d", len(f.bodies))
	}
	for i, b := range f.bodies {
		opts := b["options"].(map[string]any)
		if opts["num_ctx"].(float64) != 40960 {
			t.Fatalf("petició %d: num_ctx=%v, volia el del model carregat (40960)", i, opts["num_ctx"])
		}
		if opts["num_predict"].(float64) != 1000 || b["keep_alive"] != "30m" || b["stream"] != true {
			t.Fatalf("petició %d: %v", i, b)
		}
		if _, ok := b["think"]; ok {
			t.Fatalf("think enviat a un model sense capacitat thinking: %v", b["think"])
		}
	}
	wire, _ := json.Marshal(f.bodies[0]["messages"])
	for _, want := range []string{`"images":["QUJD"]`, `"tool_name":"ls"`, `"arguments":{"dir":"."}`} {
		if !strings.Contains(string(wire), want) {
			t.Fatalf("falta %s a %s", want, wire)
		}
	}
	if !strings.Contains(string(mustJSON(f.bodies[0]["tools"])), `"name":"read"`) {
		t.Fatalf("tools=%v", f.bodies[0]["tools"])
	}
	u, calls2, _ := meter.Total()
	if calls2 != 2 || u.CompletionTokens != 40 || u.PromptTokens != 10000 {
		t.Fatalf("usage=%+v calls=%d", u, calls2)
	}
	tm, ok := c.LastTimings()
	if !ok || tm.PromptN != 5000 || tm.PromptPerSecond != 5000 || tm.PredictedPerSecond != 40 || tm.LoadMs != 1 {
		t.Fatalf("timings=%+v ok=%v", tm, ok)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestOllamaNumCtxTria(t *testing.T) {
	cases := []struct {
		nom                  string
		modelCtx, loaded, cf int
		want                 int
	}{
		{"defecte 32k", 131072, 0, 0, OllamaDefaultNumCtx},
		{"carregat amb prou context", 131072, 65536, 0, 65536},
		{"carregat amb massa poc: no es reaprofita", 131072, 4096, 0, OllamaDefaultNumCtx},
		{"model petit", 8192, 0, 0, 8192},
		{"config mana", 131072, 65536, 49152, 49152},
	}
	for _, tc := range cases {
		t.Run(tc.nom, func(t *testing.T) {
			resetOllama()
			f := &fakeOllama{caps: []string{"completion", "tools", "thinking"}, modelCtx: tc.modelCtx, loaded: tc.loaded}
			f.reply = func(w http.ResponseWriter, body map[string]any) {
				ndjson(w, `{"message":{"content":"ok"},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":1}`)
			}
			srv := f.server(t)
			RegisterProviderType(srv.URL, ProviderOllama)
			RegisterLocalOptions(srv.URL, LocalOptions{NumCtx: tc.cf, KeepAlive: "-1"})
			t.Cleanup(func() { RegisterProviderType(srv.URL, ""); RegisterLocalOptions(srv.URL, LocalOptions{}) })
			if got := OllamaNumCtx(context.Background(), srv.URL, "", "qwen3"); got != tc.want {
				t.Fatalf("num_ctx=%d, volia %d", got, tc.want)
			}
			ctx := WithThink(context.Background(), "no")
			if _, err := New().Chat(ctx, srv.URL, "", "qwen3", []Message{{Role: "user", Content: "hola"}}, 0, 50); err != nil {
				t.Fatal(err)
			}
			b := f.bodies[0]
			if b["options"].(map[string]any)["num_ctx"].(float64) != float64(tc.want) {
				t.Fatalf("num_ctx al cable=%v", b["options"])
			}
			if b["keep_alive"] != float64(-1) || b["stream"] != false || b["think"] != false {
				t.Fatalf("keep_alive=%v stream=%v think=%v", b["keep_alive"], b["stream"], b["think"])
			}
		})
	}
}

func TestOllamaModelSenseEinesDonaErrorClar(t *testing.T) {
	resetOllama()
	f := &fakeOllama{caps: []string{"completion"}, modelCtx: 8192}
	f.reply = func(w http.ResponseWriter, body map[string]any) { t.Error("no hauria d'arribar a /api/chat") }
	srv := f.server(t)
	RegisterProviderType(srv.URL, ProviderOllama)
	t.Cleanup(func() { RegisterProviderType(srv.URL, "") })
	_, _, err := New().ChatWithTools(context.Background(), srv.URL, "", "gemma", []Message{{Role: "user", Content: "x"}}, 0, 50, []ToolSpec{{Name: "read"}})
	if err == nil || !strings.Contains(err.Error(), "does not support tools") {
		t.Fatalf("err=%v", err)
	}
}

func TestOllamaLengthIPensament(t *testing.T) {
	resetOllama()
	f := &fakeOllama{caps: []string{"completion", "tools", "thinking"}, modelCtx: 40960}
	f.reply = func(w http.ResponseWriter, body map[string]any) {
		ndjson(w, `{"message":{"content":"","thinking":"molt"},"done":false}`,
			`{"message":{"content":"a mitges"},"done":true,"done_reason":"length","prompt_eval_count":10,"eval_count":50}`)
	}
	srv := f.server(t)
	RegisterProviderType(srv.URL, ProviderOllama)
	t.Cleanup(func() { RegisterProviderType(srv.URL, "") })
	ctx := WithThink(context.Background(), "high")
	_, _, err := New().chatStreamWithToolsInner(ctx, srv.URL, "", "qwen3", []Message{{Role: "user", Content: "x"}}, 0, 50, nil, nil, nil)
	if _, ok := AsLengthError(err); !ok {
		t.Fatalf("err=%v, volia LengthError", err)
	}
	if f.bodies[0]["think"] != true {
		t.Fatalf("think=%v", f.bodies[0]["think"])
	}
}

// fakeLlama és un llama-server: /props i /v1/chat/completions.
type fakeLlama struct {
	mu     sync.Mutex
	props  string
	bodies []map[string]any
	hold   chan struct{} // si no és nil, la resposta espera
	stream bool
}

func (f *fakeLlama) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/props":
			fmt.Fprint(w, f.props)
		case "/v1/chat/completions":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.bodies = append(f.bodies, body)
			hold := f.hold
			f.mu.Unlock()
			if hold != nil {
				<-hold
			}
			if body["stream"] == true {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hola\"}}]}\n\n")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1200,\"completion_tokens\":3,\"total_tokens\":1203},\"timings\":{\"cache_n\":1100,\"prompt_n\":100,\"prompt_per_second\":850.5,\"predicted_n\":3,\"predicted_per_second\":40}}\n\n")
				fmt.Fprint(w, "data: [DONE]\n\n")
				return
			}
			fmt.Fprint(w, `{"choices":[{"message":{"content":"hola"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1200,"completion_tokens":3,"total_tokens":1203},"timings":{"cache_n":1000,"prompt_n":200,"prompt_per_second":900,"predicted_n":3,"predicted_per_second":41}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

const propsDosSlots = `{"default_generation_settings":{"n_ctx":32768,"params":{"n_predict":-1}},"total_slots":2,"chat_template":"{% for m in messages %}{{ m.content }}{% if tools %}tool{% endif %}{% endfor %}","model_path":"/m/qwen3.gguf","build_info":"b6000"}`

func TestLlamaCppCachePromptSlotsITimings(t *testing.T) {
	resetSlots()
	f := &fakeLlama{props: propsDosSlots}
	srv := f.server(t)
	base := srv.URL + "/v1"
	RegisterProviderType(base, ProviderLlamaCpp)
	t.Cleanup(func() { RegisterProviderType(base, "") })
	c := New()
	meter := &UsageMeter{}
	ctx := WithUsage(context.Background(), meter)
	msgs := []Message{{Role: "user", Content: "hola"}}
	if _, err := c.Chat(ctx, base, "", "qwen3", msgs, 0, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ChatStream(WithSideCall(ctx), base, "", "qwen3", msgs, 0, 100, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 2 {
		t.Fatalf("peticions=%d", len(f.bodies))
	}
	if f.bodies[0]["cache_prompt"] != true || f.bodies[0]["id_slot"] != float64(0) {
		t.Fatalf("sessió: %v", f.bodies[0])
	}
	if f.bodies[1]["cache_prompt"] != true || f.bodies[1]["id_slot"] != float64(1) {
		t.Fatalf("crida lateral: %v", f.bodies[1])
	}
	u, _, _ := meter.Total()
	if u.PromptTokensDetails.CachedTokens != 2100 || u.PromptTokens != 2400 {
		t.Fatalf("usage=%+v", u)
	}
	tm, ok := c.LastTimings()
	if !ok || tm.CacheN != 1100 || tm.PromptN != 100 || tm.PromptPerSecond != 850.5 {
		t.Fatalf("timings=%+v", tm)
	}
}

// Dues peticions alhora (subagents): la segona no pot fer cua darrere de la
// primera al mateix slot; va sense id_slot i el servidor en tria un de lliure.
func TestLlamaCppSlotOcupatNoEsFixa(t *testing.T) {
	resetSlots()
	f := &fakeLlama{props: propsDosSlots, hold: make(chan struct{})}
	srv := f.server(t)
	RegisterProviderType(srv.URL, ProviderLlamaCpp)
	t.Cleanup(func() { RegisterProviderType(srv.URL, "") })
	srvV1 := srv.URL + "/v1"
	RegisterProviderType(srvV1, ProviderLlamaCpp)
	t.Cleanup(func() { RegisterProviderType(srvV1, "") })
	c := New()
	errs := make(chan error, 2)
	go func() {
		_, err := c.Chat(context.Background(), srvV1, "", "m", []Message{{Role: "user", Content: "1"}}, 0, 10)
		errs <- err
	}()
	// Espera que la primera hagi arribat al servidor.
	for {
		f.mu.Lock()
		n := len(f.bodies)
		f.mu.Unlock()
		if n == 1 {
			break
		}
	}
	go func() {
		_, err := c.Chat(context.Background(), srvV1, "", "m", []Message{{Role: "user", Content: "2"}}, 0, 10)
		errs <- err
	}()
	for {
		f.mu.Lock()
		n := len(f.bodies)
		f.mu.Unlock()
		if n == 2 {
			break
		}
	}
	close(f.hold)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if f.bodies[0]["id_slot"] != float64(0) {
		t.Fatalf("primera: %v", f.bodies[0]["id_slot"])
	}
	if _, ok := f.bodies[1]["id_slot"]; ok {
		t.Fatalf("segona amb id_slot=%v mentre el slot 0 era ocupat", f.bodies[1]["id_slot"])
	}
	// Alliberat: la següent torna al slot 0.
	f.hold = nil
	if _, err := c.Chat(context.Background(), srvV1, "", "m", []Message{{Role: "user", Content: "3"}}, 0, 10); err != nil {
		t.Fatal(err)
	}
	if f.bodies[2]["id_slot"] != float64(0) {
		t.Fatalf("tercera: %v", f.bodies[2]["id_slot"])
	}
}

func TestLlamaCppUnSlotNoEnviaIdSlot(t *testing.T) {
	resetSlots()
	f := &fakeLlama{props: strings.Replace(propsDosSlots, `"total_slots":2`, `"total_slots":1`, 1)}
	srv := f.server(t)
	base := srv.URL + "/v1"
	RegisterProviderType(base, ProviderLlamaCpp)
	t.Cleanup(func() { RegisterProviderType(base, "") })
	if _, err := New().Chat(WithSideCall(context.Background()), base, "", "m", []Message{{Role: "user", Content: "x"}}, 0, 10); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.bodies[0]["id_slot"]; ok || f.bodies[0]["cache_prompt"] != true {
		t.Fatalf("cos=%v", f.bodies[0])
	}
}

// Un endpoint OpenAI-compatible normal no rep cap camp de llama.cpp.
func TestSenseTipusNoEnviaCampsLlama(t *testing.T) {
	resetSlots()
	f := &fakeLlama{props: propsDosSlots}
	srv := f.server(t)
	if _, err := New().Chat(context.Background(), srv.URL+"/v1", "", "m", []Message{{Role: "user", Content: "x"}}, 0, 10); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.bodies[0]["cache_prompt"]; ok {
		t.Fatalf("cos=%v", f.bodies[0])
	}
	if _, ok := f.bodies[0]["id_slot"]; ok {
		t.Fatalf("cos=%v", f.bodies[0])
	}
}

func TestProbeLlamaProps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"default_generation_settings":{"n_ctx":8192,"speculative":false,"params":{"context_shift":true}},"total_slots":4,"chat_template":"{{ messages }}"}`)
	}))
	defer srv.Close()
	p, err := ProbeLlamaProps(context.Background(), srv.URL+"/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.NCtx != 8192 || p.TotalSlots != 4 || p.ToolsTemplate || p.ContextShift == nil || !*p.ContextShift || p.Speculative == nil || *p.Speculative {
		t.Fatalf("props=%+v", p)
	}
}

func TestProbeLocalServerILocalWindow(t *testing.T) {
	llama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/props" {
			fmt.Fprint(w, propsDosSlots)
			return
		}
		http.NotFound(w, r)
	}))
	defer llama.Close()
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ps" {
			fmt.Fprint(w, `{"models":[{"name":"qwen3:latest","context_length":4096}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer ollama.Close()
	lms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/models" {
			fmt.Fprint(w, `{"object":"list","data":[{"id":"qwen3-8b","type":"llm","state":"loaded","max_context_length":40960,"loaded_context_length":16384},{"id":"nomic","type":"embeddings","state":"not-loaded","max_context_length":2048}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer lms.Close()
	ctx := context.Background()
	cases := []struct {
		base, kind, model string
		window            int
	}{
		{llama.URL + "/v1", ServerLlamaCpp, "qualsevol", 32768},
		{ollama.URL + "/v1", ServerOllama, "qwen3", 4096},
		{lms.URL + "/v1", ServerLMStudio, "qwen3-8b", 16384},
	}
	for _, tc := range cases {
		srv := ProbeLocalServer(ctx, tc.base, "")
		if srv.Kind != tc.kind {
			t.Fatalf("%s: kind=%q, volia %q", tc.base, srv.Kind, tc.kind)
		}
		if n, ok := LocalWindow(ctx, srv, tc.base, "", tc.model); !ok || n != tc.window {
			t.Fatalf("%s: window=%d ok=%v, volia %d", tc.kind, n, ok, tc.window)
		}
	}
	if len(ProbeLocalServer(ctx, lms.URL, "").LMStudio) != 1 {
		t.Fatal("els models d'embeddings no compten")
	}
	none := httptest.NewServer(http.NotFoundHandler())
	defer none.Close()
	if k := ProbeLocalServer(ctx, none.URL, "").Kind; k != "" {
		t.Fatalf("kind=%q", k)
	}
}

func TestNormalizeProviderTypeLocals(t *testing.T) {
	for in, want := range map[string]string{"ollama": ProviderOllama, "llama.cpp": ProviderLlamaCpp, "llamacpp": ProviderLlamaCpp, "llama-server": ProviderLlamaCpp} {
		if got, ok := NormalizeProviderType(in); !ok || got != want {
			t.Fatalf("%s → %q %v", in, got, ok)
		}
	}
}
