package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func writeRespSSE(w http.ResponseWriter, events ...map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, ev := range events {
		b, _ := json.Marshal(ev)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev["type"], b)
	}
}

func TestResponsesStreamingToolCallAndReasoningRoundTrip(t *testing.T) {
	reasoningItem := map[string]any{"type": "reasoning", "id": "rs_1", "summary": []any{map[string]any{"type": "summary_text", "text": "Need files."}}, "encrypted_content": "ENC=="}
	ns := newNativeServer(t, ProviderOpenAIResponses, func(w http.ResponseWriter, n int, body map[string]any) {
		if n == 1 {
			writeRespSSE(w,
				map[string]any{"type": "response.created", "response": map[string]any{"status": "in_progress"}},
				map[string]any{"type": "response.output_item.added", "output_index": 0, "item": map[string]any{"type": "reasoning", "id": "rs_1"}},
				map[string]any{"type": "response.reasoning_summary_text.delta", "output_index": 0, "delta": "Need "},
				map[string]any{"type": "response.reasoning_summary_text.delta", "output_index": 0, "delta": "files."},
				map[string]any{"type": "response.output_item.done", "output_index": 0, "item": reasoningItem},
				map[string]any{"type": "response.output_item.added", "output_index": 1, "item": map[string]any{"type": "message", "role": "assistant"}},
				map[string]any{"type": "response.output_text.delta", "output_index": 1, "delta": "Checking"},
				map[string]any{"type": "response.output_text.delta", "output_index": 1, "delta": "."},
				map[string]any{"type": "response.output_item.added", "output_index": 2, "item": map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_A", "name": "glob", "arguments": ""}},
				map[string]any{"type": "response.function_call_arguments.delta", "output_index": 2, "delta": `{"pattern":`},
				map[string]any{"type": "response.function_call_arguments.delta", "output_index": 2, "delta": `"*.go"}`},
				map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed", "usage": map[string]any{"input_tokens": 100, "output_tokens": 20, "total_tokens": 120, "input_tokens_details": map[string]any{"cached_tokens": 64}}}},
			)
			return
		}
		writeRespSSE(w,
			map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": "Two files."},
			map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed"}},
		)
	})
	c := New()
	meter := &UsageMeter{}
	ctx := WithUsage(WithThink(context.Background(), "max"), meter)
	msgs := []Message{{Role: "system", Content: "Be brief."}, {Role: "user", Content: "list"}}
	var toks, reason strings.Builder
	text, calls, err := c.ChatStreamWithTools(ctx, ns.URL+"/v1", "sk-test", "gpt-5", msgs, 0.3, 2048, specsProva,
		func(s string) { toks.WriteString(s) }, func(s string) { reason.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if text != "Checking." || toks.String() != "Checking." || reason.String() != "Need files." {
		t.Fatalf("text %q streamed %q reason %q", text, toks.String(), reason.String())
	}
	if len(calls) != 1 || calls[0].ID != "call_A" || calls[0].Function.Arguments != `{"pattern":"*.go"}` {
		t.Fatalf("calls %+v", calls)
	}
	u, _, _ := meter.Total()
	if u.PromptTokens != 100 || u.CompletionTokens != 20 || u.PromptTokensDetails.CachedTokens != 64 {
		t.Fatalf("usage %+v", u)
	}
	if ns.paths[0] != "/v1/responses" || ns.heads[0].Get("Authorization") != "Bearer sk-test" {
		t.Fatalf("path %s auth %q", ns.paths[0], ns.heads[0].Get("Authorization"))
	}
	b0 := ns.body(0)
	if b0["store"] != false || b0["max_output_tokens"].(float64) != 2048 || b0["stream"] != true {
		t.Fatalf("body %v", b0)
	}
	if r := b0["reasoning"].(map[string]any); r["effort"] != "high" || r["summary"] != "auto" {
		t.Fatalf("reasoning %v", r)
	}
	if inc := b0["include"].([]any); inc[0] != "reasoning.encrypted_content" {
		t.Fatalf("include %v", inc)
	}
	tool := b0["tools"].([]any)[0].(map[string]any)
	if tool["type"] != "function" || tool["name"] != "glob" {
		t.Fatalf("tool %v", tool)
	}

	msgs = append(msgs, Message{Role: "assistant", Content: text, ToolCalls: calls},
		Message{Role: "tool", ToolCallID: "call_A", Content: "a.go\nb.go", Images: []string{"data:image/png;base64,QQ=="}})
	text2, _, err := c.ChatStreamWithTools(ctx, ns.URL+"/v1", "sk-test", "gpt-5", msgs, 0.3, 2048, specsProva, nil, nil)
	if err != nil || text2 != "Two files." {
		t.Fatalf("%q %v", text2, err)
	}
	input := ns.body(1)["input"].([]any)
	kinds := []string{}
	for _, it := range input {
		m := it.(map[string]any)
		k, _ := m["type"].(string)
		if k == "" {
			k = "msg:" + m["role"].(string)
		}
		kinds = append(kinds, k)
	}
	if strings.Join(kinds, ",") != "msg:system,msg:user,reasoning,message,function_call,function_call_output,msg:user" {
		t.Fatalf("input kinds %v", kinds)
	}
	if r := input[2].(map[string]any); r["encrypted_content"] != "ENC==" || r["id"] != "rs_1" {
		t.Fatalf("reasoning item not round-tripped: %v", r)
	}
	if fc := input[4].(map[string]any); fc["call_id"] != "call_A" || fc["arguments"] != `{"pattern":"*.go"}` {
		t.Fatalf("function_call %v", fc)
	}
	if out := input[5].(map[string]any); out["call_id"] != "call_A" || out["output"] != "a.go\nb.go" {
		t.Fatalf("function_call_output %v", out)
	}
}

func TestResponsesNonStreamingAndErrors(t *testing.T) {
	t.Run("whole response", func(t *testing.T) {
		ns := newNativeServer(t, ProviderOpenAIResponses, func(w http.ResponseWriter, n int, body map[string]any) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi there"}]},{"type":"function_call","call_id":"c1","name":"read","arguments":"{\"path\":\"a\"}"}],"usage":{"input_tokens":5,"output_tokens":3}}`)
		})
		text, calls, err := New().ChatWithTools(context.Background(), ns.URL+"/v1", "k", "gpt-4.1", []Message{{Role: "user", Content: "x"}}, 0.5, 100, specsProva)
		if err != nil || text != "hi there" || len(calls) != 1 || calls[0].ID != "c1" {
			t.Fatalf("%q %+v %v", text, calls, err)
		}
		if ns.body(0)["temperature"].(float64) != 0.5 {
			t.Fatalf("temperature %v", ns.body(0))
		}
	})
	t.Run("temperature and reasoning negotiated", func(t *testing.T) {
		ns := newNativeServer(t, ProviderOpenAIResponses, func(w http.ResponseWriter, n int, body map[string]any) {
			if _, ok := body["temperature"]; ok {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"error":{"message":"Unsupported parameter: 'temperature' is not supported with this model.","type":"invalid_request_error","param":"temperature"}}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`)
		})
		out, err := New().Chat(context.Background(), ns.URL+"/v1", "k", "o4", []Message{{Role: "user", Content: "x"}}, 0.7, 100)
		if err != nil || out != "ok" || len(ns.bodies) != 2 || ns.body(1)["reasoning"] == nil {
			t.Fatalf("%q %v requests %d", out, err, len(ns.bodies))
		}
	})
	t.Run("incomplete is a LengthError", func(t *testing.T) {
		ns := newNativeServer(t, ProviderOpenAIResponses, func(w http.ResponseWriter, n int, body map[string]any) {
			writeRespSSE(w,
				map[string]any{"type": "response.reasoning_summary_text.delta", "delta": "thinking a lot"},
				map[string]any{"type": "response.incomplete", "response": map[string]any{"status": "incomplete", "incomplete_details": map[string]any{"reason": "max_output_tokens"}}},
			)
		})
		_, _, err := New().ChatStreamWithTools(context.Background(), ns.URL+"/v1", "k", "gpt-5", []Message{{Role: "user", Content: "x"}}, 0, 50, specsProva, nil, nil)
		if le, ok := AsLengthError(err); !ok || le.Reasoned == 0 {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("failed and truncated streams", func(t *testing.T) {
		ns := newNativeServer(t, ProviderOpenAIResponses, func(w http.ResponseWriter, n int, body map[string]any) {
			if n == 1 {
				writeRespSSE(w, map[string]any{"type": "response.failed", "response": map[string]any{"status": "failed", "error": map[string]any{"code": "invalid_prompt", "message": "nope"}}})
				return
			}
			writeRespSSE(w, map[string]any{"type": "response.output_text.delta", "delta": "half"})
		})
		c := New()
		_, _, err := c.ChatStreamWithTools(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "x"}}, 0, 50, specsProva, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "nope") {
			t.Fatalf("err %v", err)
		}
		_, _, err = c.ChatStreamWithTools(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "x"}}, 0, 50, specsProva, nil, nil)
		var si *StreamInterruptedError
		if !errors.As(err, &si) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("429 then success", func(t *testing.T) {
		ns := newNativeServer(t, ProviderOpenAIResponses, func(w http.ResponseWriter, n int, body map[string]any) {
			if n == 1 {
				w.WriteHeader(429)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`)
		})
		out, err := New().Chat(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "x"}}, 0, 50)
		if err != nil || out != "ok" || len(ns.bodies) != 2 {
			t.Fatalf("%q %v", out, err)
		}
	})
}
