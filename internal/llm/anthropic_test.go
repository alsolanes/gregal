package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// nativeServer és un fake de proveïdor natiu: desa cada cos rebut i respon
// amb la funció del test.
type nativeServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
	heads  []http.Header
	paths  []string
}

func newNativeServer(t *testing.T, typ string, handle func(w http.ResponseWriter, n int, body map[string]any)) *nativeServer {
	t.Helper()
	ns := &nativeServer{}
	ns.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		ns.mu.Lock()
		ns.bodies = append(ns.bodies, body)
		ns.heads = append(ns.heads, r.Header.Clone())
		ns.paths = append(ns.paths, r.URL.Path)
		n := len(ns.bodies)
		ns.mu.Unlock()
		handle(w, n, body)
	}))
	t.Cleanup(ns.Close)
	RegisterProviderType(ns.URL+"/v1", typ)
	t.Cleanup(func() { RegisterProviderType(ns.URL+"/v1", "") })
	return ns
}

func (ns *nativeServer) body(i int) map[string]any {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	return ns.bodies[i]
}

func writeSSE(w http.ResponseWriter, events ...map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, ev := range events {
		b, _ := json.Marshal(ev)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev["type"], b)
	}
}

func antStreamToolTurn() []map[string]any {
	return []map[string]any{
		{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 10, "cache_creation_input_tokens": 100, "cache_read_input_tokens": 50, "output_tokens": 1}}},
		{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "thinking", "thinking": ""}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": "I should glob "}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": "the files."}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "signature_delta", "signature": "sig-abc"}},
		{"type": "content_block_stop", "index": 0},
		{"type": "ping"},
		{"type": "content_block_start", "index": 1, "content_block": map[string]any{"type": "text", "text": ""}},
		{"type": "content_block_delta", "index": 1, "delta": map[string]any{"type": "text_delta", "text": "Looking"}},
		{"type": "content_block_delta", "index": 1, "delta": map[string]any{"type": "text_delta", "text": " now."}},
		{"type": "content_block_stop", "index": 1},
		{"type": "content_block_start", "index": 2, "content_block": map[string]any{"type": "tool_use", "id": "toolu_01", "name": "glob", "input": map[string]any{}}},
		{"type": "content_block_delta", "index": 2, "delta": map[string]any{"type": "input_json_delta", "partial_json": `{"pattern":`}},
		{"type": "content_block_delta", "index": 2, "delta": map[string]any{"type": "input_json_delta", "partial_json": `"*.go"}`}},
		{"type": "content_block_stop", "index": 2},
		{"type": "message_delta", "delta": map[string]any{"stop_reason": "tool_use"}, "usage": map[string]any{"output_tokens": 42}},
		{"type": "message_stop"},
	}
}

func TestAnthropicStreamingToolCallAndThinkingRoundTrip(t *testing.T) {
	ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
		if n == 1 {
			writeSSE(w, antStreamToolTurn()...)
			return
		}
		writeSSE(w,
			map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 5}}},
			map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}},
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "Done."}},
			map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}, "usage": map[string]any{"output_tokens": 2}},
			map[string]any{"type": "message_stop"},
		)
	})
	c := New()
	meter := &UsageMeter{}
	ctx := WithUsage(WithThink(context.Background(), "high"), meter)
	var progress []string
	ctx = WithStreamProgress(ctx, func(kind string, n int) { progress = append(progress, kind) })
	msgs := []Message{
		{Role: "system", Content: "You are a coding agent."},
		{Role: "user", Content: "List the Go files."},
	}
	var toks, reason strings.Builder
	text, calls, err := c.ChatStreamWithTools(ctx, ns.URL+"/v1", "sk-ant-test", "claude-opus-5-5", msgs, 0.4, 4096, specsProva,
		func(s string) { toks.WriteString(s) }, func(s string) { reason.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if text != "Looking now." || toks.String() != "Looking now." {
		t.Fatalf("text %q / streamed %q", text, toks.String())
	}
	if reason.String() != "I should glob the files." {
		t.Fatalf("reasoning %q", reason.String())
	}
	if len(calls) != 1 || calls[0].ID != "toolu_01" || calls[0].Function.Name != "glob" || calls[0].Function.Arguments != `{"pattern":"*.go"}` {
		t.Fatalf("calls %+v", calls)
	}
	u, n, _ := meter.Total()
	if n != 1 || u.PromptTokens != 160 || u.PromptTokensDetails.CachedTokens != 50 || u.CacheWriteTokens != 100 || u.CompletionTokens != 42 {
		t.Fatalf("usage %+v calls %d", u, n)
	}
	if strings.Join(progress, ",") == "" || progress[0] != ProgressFirst {
		t.Fatalf("progress %v", progress)
	}

	// Petició 1: capçaleres, system amb cache, eines amb cache, pensament.
	h := ns.heads[0]
	if ns.paths[0] != "/v1/messages" || h.Get("x-api-key") != "sk-ant-test" || h.Get("anthropic-version") != AnthropicVersion || h.Get("Authorization") != "" {
		t.Fatalf("path %s headers %v", ns.paths[0], h)
	}
	b0 := ns.body(0)
	if _, ok := b0["temperature"]; ok {
		t.Fatal("temperature must not be sent")
	}
	sys := b0["system"].([]any)
	if sys[0].(map[string]any)["text"] != "You are a coding agent." || sys[0].(map[string]any)["cache_control"] == nil {
		t.Fatalf("system %v", sys)
	}
	tools := b0["tools"].([]any)
	tool := tools[len(tools)-1].(map[string]any)
	if tool["cache_control"] == nil || tool["input_schema"] == nil || tool["eager_input_streaming"] != true {
		t.Fatalf("tool %v", tool)
	}
	if th := b0["thinking"].(map[string]any); th["type"] != "adaptive" {
		t.Fatalf("thinking %v", th)
	}
	if oc := b0["output_config"].(map[string]any); oc["effort"] != "high" {
		t.Fatalf("output_config %v", oc)
	}
	for _, m := range b0["messages"].([]any) {
		if m.(map[string]any)["role"] == "system" {
			t.Fatal("system leaked into messages")
		}
	}

	// Petició 2: l'assistant torna amb el bloc de pensament signat davant
	// del tool_use, i el resultat va com a tool_result al missatge user.
	msgs = append(msgs, Message{Role: "assistant", Content: text, ToolCalls: calls},
		Message{Role: "tool", ToolCallID: "toolu_01", Content: "a.go\nb.go"})
	text2, _, err := c.ChatStreamWithTools(ctx, ns.URL+"/v1", "sk-ant-test", "claude-opus-5-5", msgs, 0.4, 4096, specsProva, nil, nil)
	if err != nil || text2 != "Done." {
		t.Fatalf("second turn %q %v", text2, err)
	}
	conv := ns.body(1)["messages"].([]any)
	if len(conv) != 3 {
		t.Fatalf("messages %v", conv)
	}
	asst := conv[1].(map[string]any)["content"].([]any)
	first := asst[0].(map[string]any)
	if first["type"] != "thinking" || first["signature"] != "sig-abc" || first["thinking"] != "I should glob the files." {
		t.Fatalf("thinking block not round-tripped: %v", asst)
	}
	if last := asst[len(asst)-1].(map[string]any); last["type"] != "tool_use" || last["id"] != "toolu_01" {
		t.Fatalf("tool_use %v", last)
	}
	res := conv[2].(map[string]any)
	rb := res["content"].([]any)[0].(map[string]any)
	if res["role"] != "user" || rb["type"] != "tool_result" || rb["tool_use_id"] != "toolu_01" || rb["cache_control"] == nil {
		t.Fatalf("tool_result %v", res)
	}
}

func TestAnthropicNonStreamingAndMessageShaping(t *testing.T) {
	ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"type": "message", "stop_reason": "tool_use",
			"content": []any{
				map[string]any{"type": "text", "text": "Reading."},
				map[string]any{"type": "tool_use", "id": "toolu_9", "name": "read", "input": map[string]any{"path": "x.go"}},
			},
			"usage": map[string]any{"input_tokens": 3, "output_tokens": 4},
		})
	})
	c := New()
	var orphan ToolCall
	orphan.ID, orphan.Function.Name, orphan.Function.Arguments = "call.1", "bash", `{"cmd": "ls`
	msgs := []Message{
		{Role: "system", Content: "S1"},
		{Role: "system", Content: "S2"},
		{Role: "assistant", Content: "hello"},
		{Role: "user", Content: "look", Images: []string{"data:image/png;base64,AAAA"}},
		{Role: "user", Content: "again"},
		{Role: "assistant", ToolCalls: []ToolCall{orphan}},
		{Role: "user", Content: "never mind"},
	}
	text, calls, err := c.ChatWithTools(WithThink(context.Background(), "no"), ns.URL+"/v1", "k", "claude-haiku-5-5", msgs, 0.2, 0, specsProva)
	if err != nil {
		t.Fatal(err)
	}
	if text != "Reading." || len(calls) != 1 || calls[0].Function.Arguments != `{"path":"x.go"}` {
		t.Fatalf("%q %+v", text, calls)
	}
	b := ns.body(0)
	if b["stream"] != nil || b["max_tokens"].(float64) != 8192 || b["thinking"] != nil {
		t.Fatalf("body %v", b)
	}
	if oc := b["output_config"].(map[string]any); oc["effort"] != "low" {
		t.Fatalf("think no → effort low: %v", oc)
	}
	if len(b["system"].([]any)) != 2 {
		t.Fatalf("system %v", b["system"])
	}
	conv := b["messages"].([]any)
	roles := []string{}
	for _, m := range conv {
		roles = append(roles, m.(map[string]any)["role"].(string))
	}
	// user (continue) · assistant · user (imatge + 2 textos) · assistant
	// (tool_use) · user (tool_result sintètic + text).
	if strings.Join(roles, ",") != "user,assistant,user,assistant,user" {
		t.Fatalf("roles %v", roles)
	}
	img := conv[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	if img["type"] != "image" || img["source"].(map[string]any)["media_type"] != "image/png" {
		t.Fatalf("image %v", img)
	}
	tu := conv[3].(map[string]any)["content"].([]any)[0].(map[string]any)
	if tu["id"] != "call_1" || fmt.Sprint(tu["input"]) != "map[]" {
		t.Fatalf("tool_use sanitised %v", tu)
	}
	last := conv[4].(map[string]any)["content"].([]any)
	if last[0].(map[string]any)["type"] != "tool_result" || last[0].(map[string]any)["is_error"] != true || last[1].(map[string]any)["text"] != "never mind" {
		t.Fatalf("synthetic result %v", last)
	}
}

func TestAnthropicErrors(t *testing.T) {
	t.Run("400 is final and not retryable", func(t *testing.T) {
		ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long"}}`)
		})
		_, _, err := New().ChatWithTools(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100, nil)
		if err == nil || !strings.Contains(err.Error(), "HTTP 400") || IsRetryable(err) || len(ns.bodies) != 1 {
			t.Fatalf("err %v (%d requests)", err, len(ns.bodies))
		}
	})
	t.Run("529 overloaded is retried", func(t *testing.T) {
		ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
			if n < 3 {
				w.WriteHeader(529)
				fmt.Fprint(w, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"type":"message","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`)
		})
		retries := 0
		ctx2 := WithRetryHook(context.Background(), func(a, tot int, w time.Duration, err error) { retries++ })
		out, err := New().Chat(ctx2, ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100)
		if err != nil || out != "ok" || retries != 2 {
			t.Fatalf("%q %v retries %d", out, err, retries)
		}
	})
	t.Run("unknown optional field is negotiated away", func(t *testing.T) {
		ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
			if body["thinking"] != nil {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"thinking.type: adaptive is not supported on this model"}}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"type":"message","stop_reason":"end_turn","content":[{"type":"text","text":"plain"}]}`)
		})
		c := New()
		for i := 0; i < 2; i++ {
			out, err := c.Chat(WithThink(context.Background(), "medium"), ns.URL+"/v1", "k", "claude-haiku-4-5", []Message{{Role: "user", Content: "hi"}}, 0, 100)
			if err != nil || out != "plain" {
				t.Fatalf("%q %v", out, err)
			}
		}
		// 400 + reintent sense thinking; la segona crida ja no l'envia.
		if len(ns.bodies) != 3 || ns.body(2)["thinking"] != nil || ns.body(2)["output_config"] == nil {
			t.Fatalf("requests %d, last %v", len(ns.bodies), ns.body(len(ns.bodies)-1))
		}
	})
	t.Run("max_tokens stop is a LengthError", func(t *testing.T) {
		ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
			writeSSE(w,
				map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
				map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "thinking", "thinking": ""}},
				map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "thinking_delta", "thinking": strings.Repeat("x", 50)}},
				map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "max_tokens"}},
				map[string]any{"type": "message_stop"},
			)
		})
		_, err := New().ChatStream(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100, nil, nil)
		if le, ok := AsLengthError(err); !ok || le.Reasoned != 50 {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("truncated stream is interrupted", func(t *testing.T) {
		ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
			writeSSE(w,
				map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
				map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}},
				map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "partial"}},
			)
		})
		_, _, err := New().ChatStreamWithTools(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100, specsProva, nil, nil)
		var si *StreamInterruptedError
		if !errors.As(err, &si) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("stream error event", func(t *testing.T) {
		ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
			writeSSE(w, map[string]any{"type": "error", "error": map[string]any{"type": "invalid_request_error", "message": "bad tool"}})
		})
		_, _, err := New().ChatStreamWithTools(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100, specsProva, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "bad tool") {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("refusal", func(t *testing.T) {
		ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"type":"message","stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber"},"content":[]}`)
		})
		_, err := New().Chat(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100)
		if err == nil || !strings.Contains(err.Error(), "refusal") || !strings.Contains(err.Error(), "cyber") {
			t.Fatalf("err %v", err)
		}
	})
}

func TestNativeFailoverToCompatFallback(t *testing.T) {
	prim := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
		w.WriteHeader(503)
		fmt.Fprint(w, "unavailable")
	})
	fb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"from fallback"},"finish_reason":"stop"}]}`)
	}))
	defer fb.Close()
	var notified string
	text, _, used, err := New().ChatWithToolsFO(context.Background(),
		Target{BaseURL: prim.URL + "/v1", APIKey: "k", Model: "claude-opus-5-5"},
		&Target{BaseURL: fb.URL, Model: "local"},
		[]Message{{Role: "user", Content: "hi"}}, 0, 100, specsProva, func(m string) { notified = m })
	if err != nil || !used || text != "from fallback" || notified != "local" {
		t.Fatalf("%q used=%v err=%v notified=%q", text, used, err, notified)
	}
	if len(prim.bodies) != RetryAttempts {
		t.Fatalf("primary attempts %d", len(prim.bodies))
	}
}

func TestProviderTypeRegistry(t *testing.T) {
	for in, want := range map[string]string{"": "", "OpenAI": "", "anthropic": ProviderAnthropic, "openai-responses": ProviderOpenAIResponses, "responses": ProviderOpenAIResponses} {
		if got, ok := NormalizeProviderType(in); !ok || got != want {
			t.Fatalf("%q → %q %v", in, got, ok)
		}
	}
	if _, ok := NormalizeProviderType("gemini"); ok {
		t.Fatal("unknown type accepted")
	}
	RegisterProviderType("https://example.test/v1/", "anthropic")
	defer RegisterProviderType("https://example.test/v1", "")
	if ProviderTypeFor("https://example.test/v1") != ProviderAnthropic || ProviderTypeFor("https://other.test/v1") != "" {
		t.Fatal("registry lookup")
	}
}

func TestProbeModelsUsesAnthropicHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" || r.Header.Get("Authorization") != "" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"claude-opus-5-5","max_input_tokens":1000000}]}`)
	}))
	defer srv.Close()
	RegisterProviderType(srv.URL+"/v1", ProviderAnthropic)
	defer RegisterProviderType(srv.URL+"/v1", "")
	ids, err := ProbeModels(srv.URL+"/v1", "k")
	if err != nil || len(ids) != 1 {
		t.Fatalf("%v %v", ids, err)
	}
	wins, err := ProbeModelWindows(srv.URL+"/v1", "k")
	if err != nil || wins["claude-opus-5-5"] != 1000000 {
		t.Fatalf("%v %v", wins, err)
	}
}

// Un 400 pels blocs de pensament reenviats (historial editat) es reintenta
// sense blocs, però la petició següent els torna a enviar.
func TestAnthropicRejectedThinkingBlocksAreDroppedOnce(t *testing.T) {
	ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
		if n == 1 {
			writeSSE(w, antStreamToolTurn()...)
			return
		}
		raw, _ := json.Marshal(body["messages"])
		if n == 2 && strings.Contains(string(raw), "sig-abc") {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"messages.1.content.0: invalid thinking block signature"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"type":"message","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`)
	})
	c := New()
	msgs := []Message{{Role: "user", Content: "go"}}
	text, calls, err := c.ChatStreamWithTools(context.Background(), ns.URL+"/v1", "k", "m", msgs, 0, 1000, specsProva, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	msgs = append(msgs, Message{Role: "assistant", Content: text, ToolCalls: calls}, Message{Role: "tool", ToolCallID: calls[0].ID, Content: "r"})
	for i := 0; i < 2; i++ {
		if out, _, err := c.ChatWithTools(context.Background(), ns.URL+"/v1", "k", "m", msgs, 0, 1000, specsProva); err != nil || out != "ok" {
			t.Fatalf("%q %v", out, err)
		}
	}
	has := func(i int) bool { raw, _ := json.Marshal(ns.body(i)["messages"]); return strings.Contains(string(raw), "sig-abc") }
	if len(ns.bodies) != 4 || !has(1) || has(2) || !has(3) {
		t.Fatalf("requests %d blocks: %v %v %v", len(ns.bodies), has(1), has(2), has(3))
	}
	if ns.body(2)["thinking"] != nil || ns.body(3)["thinking"] == nil {
		t.Fatal("thinking must be off only while its blocks are missing")
	}
}
