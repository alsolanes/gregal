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
	"testing"
)

func rtCall(id string) ToolCall {
	c := ToolCall{ID: id, Type: "function"}
	c.Function.Name = "grep"
	c.Function.Arguments = `{}`
	return c
}

// A nudge (user message) inserted between the tool results of one assistant
// message made strict providers answer 400 and killed the turn. The wire
// moves it after the last result of that batch.
func TestNormalitzaOrdreEinesMouAvisEntreResultats(t *testing.T) {
	in := []Message{
		{Role: "system", Content: "s"},
		{Role: "user", Content: "task"},
		{Role: "assistant", ToolCalls: []ToolCall{rtCall("a"), rtCall("b"), rtCall("c")}},
		{Role: "tool", ToolCallID: "a", Content: "ra"},
		{Role: "user", Content: "nudge"},
		{Role: "tool", ToolCallID: "b", Content: "rb"},
		{Role: "tool", ToolCallID: "c", Content: "rc"},
		{Role: "assistant", Content: "done"},
	}
	got := normalitzaOrdreEines(in)
	var seq []string
	for _, m := range got {
		seq = append(seq, m.Role+":"+m.ToolCallID+m.Content)
	}
	want := "system:s user:task assistant: tool:ara tool:brb tool:crc user:nudge assistant:done"
	if strings.Join(seq, " ") != want {
		t.Fatalf("order:\n got %s\nwant %s", strings.Join(seq, " "), want)
	}
	// The caller's slice is not modified.
	if in[4].Content != "nudge" {
		t.Fatal("input slice was mutated")
	}
}

func TestNormalitzaOrdreEinesNoTocaOrdreCorrecte(t *testing.T) {
	in := []Message{
		{Role: "user", Content: "task"},
		{Role: "assistant", ToolCalls: []ToolCall{rtCall("a")}},
		{Role: "tool", ToolCallID: "a", Content: "ra"},
		{Role: "user", Content: "nudge after the batch is fine"},
	}
	got := normalitzaOrdreEines(in)
	if &got[0] != &in[0] {
		t.Fatal("a correct history must not be copied")
	}
	// A result that never arrives leaves everything as it was.
	orfe := []Message{
		{Role: "assistant", ToolCalls: []ToolCall{rtCall("a"), rtCall("b")}},
		{Role: "tool", ToolCallID: "a", Content: "ra"},
		{Role: "user", Content: "u"},
	}
	if out := normalitzaOrdreEines(orfe); len(out) != 3 || out[2].Role != "user" {
		t.Fatalf("orphan batch changed: %+v", out)
	}
}

// The provider request carries the normalised order; persistence keeps the
// original messages.
func TestWireNormalitzaIRaonament(t *testing.T) {
	asst := Message{Role: "assistant", ToolCalls: []ToolCall{rtCall("a"), rtCall("b")}, ReasoningContent: "I should grep"}
	req := chatRequest{Model: "m", Messages: []Message{
		{Role: "user", Content: "task"},
		asst,
		{Role: "tool", ToolCallID: "a", Content: "ra"},
		{Role: "user", Content: "nudge"},
		{Role: "tool", ToolCallID: "b", Content: "rb"},
	}}
	raw, err := marshalChatRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Messages[3]["role"] != "tool" || wire.Messages[4]["role"] != "user" {
		t.Fatalf("wire order not normalised: %s", raw)
	}
	if wire.Messages[1]["reasoning_content"] != "I should grep" {
		t.Fatalf("reasoning_content must go back to the provider: %s", raw)
	}
	if _, ok := wire.Messages[0]["reasoning_content"]; ok {
		t.Fatalf("empty reasoning must be omitted: %s", raw)
	}
	// Session persistence (Message.MarshalJSON) does not store it.
	saved, _ := json.Marshal(asst)
	if strings.Contains(string(saved), "reasoning_content") {
		t.Fatalf("persisted message keeps reasoning: %s", saved)
	}
}

func TestCapturaRaonamentNoStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"reasoning_content":"think A","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer srv.Close()
	rc := &ReasoningCapture{}
	ctx := WithReasoningCapture(context.Background(), rc)
	_, calls, err := New().ChatWithTools(ctx, srv.URL, "", "m", []Message{{Role: "user", Content: "x"}}, 0, 100, nil)
	if err != nil || len(calls) != 1 {
		t.Fatalf("calls=%v err=%v", calls, err)
	}
	if rc.Text() != "think A" {
		t.Fatalf("captured %q", rc.Text())
	}
}

// Streaming: only reasoning_content is captured for the round trip; the
// generic "reasoning" field of other providers is shown but not sent back.
func TestCapturaRaonamentStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range []string{
			`{"choices":[{"delta":{"reasoning_content":"step "}}]}`,
			`{"choices":[{"delta":{"reasoning_content":"one","reasoning":"X"}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"read","arguments":"{}"}}]}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		} {
			fmt.Fprintf(w, "data: %s\n\n", ev)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	rc := &ReasoningCapture{}
	ctx := WithReasoningCapture(context.Background(), rc)
	var shown strings.Builder
	_, calls, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", []Message{{Role: "user", Content: "x"}}, 0, 100, nil, nil, func(s string) { shown.WriteString(s) })
	if err != nil || len(calls) != 1 {
		t.Fatalf("calls=%v err=%v", calls, err)
	}
	if rc.Text() != "step one" {
		t.Fatalf("captured %q", rc.Text())
	}
	if !strings.Contains(shown.String(), "X") {
		t.Fatalf("display reasoning lost: %q", shown.String())
	}
}

// Providers that never send reasoning_content see no new field.
func TestSenseRaonamentCapCampNou(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	msgs := []Message{{Role: "user", Content: "x"}, {Role: "assistant", ToolCalls: []ToolCall{rtCall("a")}}, {Role: "tool", ToolCallID: "a", Content: "r"}}
	if _, _, err := New().ChatWithTools(context.Background(), srv.URL, "", "m", msgs, 0, 100, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "reasoning_content") {
		t.Fatalf("unexpected field on the wire: %s", body)
	}
}

func TestIsReasoningRoundTripError(t *testing.T) {
	yes := errors.New(`provider x: HTTP 400: {"error":{"message":"Upstream request failed: [invalid_request_error] The ` + "`reasoning_content`" + ` in the thinking mode must be passed back to the API."}}`)
	if !IsReasoningRoundTripError(yes) {
		t.Fatal("must recognise the DeepSeek round-trip error")
	}
	if IsReasoningRoundTripError(nil) {
		t.Fatal("nil is not an error")
	}
	if IsReasoningRoundTripError(errors.New("provider x: HTTP 400: bad request")) {
		t.Fatal("false positive on a generic 400")
	}
}

// Loops that build the assistant message themselves (subagents, plan, TUI,
// Telegram) do not store the reasoning; the client puts it back on the wire
// from the call id it saw in the response.
func TestClientTornaElRaonamentPerIdDeCrida(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		if len(bodies) == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"reasoning_content":"look at go.mod","tool_calls":[{"id":"call_77","type":"function","function":{"name":"read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	c := New()
	hist := []Message{{Role: "user", Content: "x"}}
	_, calls, err := c.ChatWithTools(context.Background(), srv.URL, "", "m", hist, 0, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The caller stores the assistant message WITHOUT the reasoning.
	hist = append(hist, Message{Role: "assistant", ToolCalls: calls}, Message{Role: "tool", ToolCallID: calls[0].ID, Content: "module x"})
	if _, _, err := c.ChatWithTools(context.Background(), srv.URL, "", "m", hist, 0, 100, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bodies[1], `"reasoning_content":"look at go.mod"`) {
		t.Fatalf("reasoning not sent back:\n%s", bodies[1])
	}
	if hist[1].ReasoningContent != "" {
		t.Fatal("the caller's history must not be modified")
	}
}

func TestRaonamentCacheAcotada(t *testing.T) {
	var rc raonamentCache
	for i := 0; i < maxRaonaments+10; i++ {
		rc.recorda([]ToolCall{{ID: fmt.Sprintf("c%d", i)}}, "r")
	}
	if len(rc.m) != maxRaonaments || len(rc.ordre) != maxRaonaments {
		t.Fatalf("cache size %d/%d", len(rc.m), len(rc.ordre))
	}
	rc.recorda([]ToolCall{{ID: "dsml-1"}}, "r")
	if _, ok := rc.m["dsml-1"]; ok {
		t.Fatal("DSML ids repeat across responses and must not be cached")
	}
}
