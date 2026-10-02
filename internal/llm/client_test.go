package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sseServer(t *testing.T, chunks []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func TestChatStreamContent(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"content":"ho"}}]}`,
		`{"choices":[{"delta":{"content":"la"}}]}`,
	})
	defer srv.Close()
	var tokens []string
	got, err := New().ChatStream(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100, func(s string) { tokens = append(tokens, s) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hola" || len(tokens) != 2 {
		t.Fatalf("got=%q tokens=%v", got, tokens)
	}
}

func TestChatStreamReasoning(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"reasoning_content":"pensant "}}]}`,
		`{"choices":[{"delta":{"reasoning":"fort "}}]}`,
		`{"choices":[{"delta":{"thought":"i "}}]}`,
		`{"choices":[{"delta":{"thinking":"profund "}}]}`,
		`{"choices":[{"delta":{"content":"fet"}}]}`,
	})
	defer srv.Close()
	var think []string
	got, err := New().ChatStream(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100, nil, func(s string) { think = append(think, s) })
	if err != nil {
		t.Fatal(err)
	}
	if got != "fet" {
		t.Fatalf("got=%q", got)
	}
	if len(think) != 4 || strings.Join(think, "") != "pensant fort i profund " {
		t.Fatalf("think=%v", think)
	}
}

func TestChatStreamThinkTags(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"content":"<think>analitzant el problema</think>la solució"}}]}`,
	})
	defer srv.Close()
	var think []string
	got, err := New().ChatStream(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100, nil, func(s string) { think = append(think, s) })
	if err != nil {
		t.Fatal(err)
	}
	if got != "la solució" {
		t.Fatalf("got=%q", got)
	}
	if len(think) != 1 || think[0] != "analitzant el problema" {
		t.Fatalf("think=%v", think)
	}
}

func TestChatWithToolsParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer srv.Close()
	content, calls, err := New().ChatWithTools(context.Background(), srv.URL, "", "m",
		[]Message{{Role: "user", Content: "h"}}, 0.5, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "" || len(calls) != 1 || calls[0].Function.Name != "read" || calls[0].ID != "c1" {
		t.Fatalf("content=%q calls=%+v", content, calls)
	}
}

func TestPromptCacheOptionsOnlyAppearWhenConfigured(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		chatOK(w)
	}))
	defer srv.Close()
	c := New()
	c.SetPromptCache(srv.URL, "m", "gregal-test", "24h")
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100); err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["prompt_cache_key"] != "gregal-test" || wire["prompt_cache_retention"] != "24h" {
		t.Fatalf("opcions de cache=%v", wire)
	}
}

func TestEstimateTokens(t *testing.T) {
	got := EstimateTokens([]Message{
		{Role: "user", Content: "abcd"},          // 1+1+8 = 10
		{Role: "assistant", Content: "abcdefgh"}, // 2+2+8 = 12
	})
	if got != 22 {
		t.Fatalf("EstimateTokens=%d", got)
	}
}

func TestFmtCount(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1k", 1500: "1.5k", 2048: "2k", 150000: "150k"} {
		if got := FmtCount(in); got != want {
			t.Errorf("FmtCount(%d)=%q, volia %q", in, got, want)
		}
	}
}

func TestToolMessageMarshalNoName(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    "resultat",
		ToolCallID: "call_123",
		Name:       "edit",
	}
	bytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(bytes)
	if strings.Contains(raw, `"name"`) {
		t.Fatalf("un missatge role:tool no ha d'incloure 'name' al wire JSON (rebutjat per Console Go/Zen): %s", raw)
	}
	if !strings.Contains(raw, `"tool_call_id":"call_123"`) {
		t.Fatalf("ha d'incloure tool_call_id: %s", raw)
	}

	// Comprova també missatges d'altres rols
	userMsg := Message{Role: "user", Content: "hola", Name: "Joan"}
	uBytes, err := json.Marshal(userMsg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(uBytes), `"name"`) {
		t.Fatalf("cap missatge ha d'emetre 'name' al wire JSON: %s", string(uBytes))
	}
}

func TestMessageMarshalSanitizesMalformedToolArguments(t *testing.T) {
	msg := Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "c1", Type: "function"}}}
	msg.ToolCalls[0].Function.Name = "exec"
	msg.ToolCalls[0].Function.Arguments = "{\"command\":\"python - <<'PY'\nprint(\"oops)"
	original := msg.ToolCalls[0].Function.Arguments
	b, err := marshalChatRequest(chatRequest{Model: "m", Messages: []Message{msg}})
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Messages []struct {
			ToolCalls []struct {
				Function struct {
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Messages) != 1 || len(wire.Messages[0].ToolCalls) != 1 || wire.Messages[0].ToolCalls[0].Function.Arguments != "{}" {
		t.Fatalf("wire arguments=%q, volia {}", wire.Messages[0].ToolCalls[0].Function.Arguments)
	}
	if !json.Valid([]byte(wire.Messages[0].ToolCalls[0].Function.Arguments)) {
		t.Fatal("els arguments wire han de ser JSON vàlid")
	}
	if msg.ToolCalls[0].Function.Arguments != original {
		t.Fatal("la sanitització no ha de mutar la crida local")
	}
}

func TestMessageMarshalPreservesMalformedToolArgumentsForPersistence(t *testing.T) {
	msg := Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "c1", Type: "function"}}}
	msg.ToolCalls[0].Function.Arguments = `{"command":"python - <<'PY`
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		ToolCalls []ToolCall `json:"tool_calls"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.ToolCalls) != 1 || wire.ToolCalls[0].Function.Arguments != msg.ToolCalls[0].Function.Arguments {
		t.Fatalf("la serialització de la sessió ha de conservar els arguments originals: %s", b)
	}
}
