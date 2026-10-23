package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Opt-in: real Halogen responses go through the Telegram driver and motor.
// Telegram delivery is a local test sink; sessions use t.TempDir. Structured
// tool_calls abort at the proxy; recovered textual calls are denied by the
// motor policy before execution.
func TestTelegramLatestRequestLiveDriver(t *testing.T) {
	if os.Getenv("GREGAL_TELEGRAM_PRIORITY_LIVE") != "1" {
		t.Skip("opt-in real Halogen + Telegram driver probe")
	}
	raw, err := os.ReadFile(os.Getenv("GREGAL_PRIORITY_HISTORY"))
	if err != nil {
		t.Fatal(err)
	}
	var history []llm.Message
	if err = json.Unmarshal(raw, &history); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, request string }{
		{"greeting", "Ep"},
		{"cancel", "No facis res més del bench ni del dump. Ho deixem estar. Confirma-ho breument."},
		{"new-topic", "Canvi de tema: com es diu hello en català? Respon només la traducció."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tools.TodoSet([]tools.TodoItem{{Title: "old unrelated checklist", Status: "pending"}})
			t.Cleanup(tools.TodoClear)
			target, _ := url.Parse("http://127.0.0.1:8731")
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ModifyResponse = func(response *http.Response) error {
				if response.Request.Method != http.MethodPost || response.StatusCode < 200 || response.StatusCode >= 300 {
					return nil
				}
				// Decode while preserving the actual response bytes for the llm client.
				var payload struct {
					Choices []struct {
						Message struct {
							ToolCalls []json.RawMessage `json:"tool_calls"`
						} `json:"message"`
					} `json:"choices"`
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				response.Body = io.NopCloser(bytes.NewReader(body))
				if err != nil {
					return err
				}
				if err = json.Unmarshal(body, &payload); err != nil {
					return err
				}
				for _, choice := range payload.Choices {
					if len(choice.Message.ToolCalls) > 0 {
						return fmt.Errorf("probe refuses all returned tools (%d)", len(choice.Message.ToolCalls))
					}
				}
				return nil
			}
			model := &fakeLLM{}
			model.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					model.mu.Lock()
					model.calls++
					model.mu.Unlock()
				}
				proxy.ServeHTTP(w, r)
			}))
			t.Cleanup(model.srv.Close)
			tg := newFakeTG(t)
			bot, _, _ := testBot(t, tg, model, nil)
			cfg, _, err := config.Load(os.Getenv("GREGAL_PRIORITY_CONFIG"))
			if err != nil {
				t.Fatal(err)
			}
			cfg.Providers = map[string]config.Provider{"probe": {BaseURL: model.srv.URL + "/v1"}}
			cfg.Roles = map[string]config.Role{"chat": {Provider: "probe", Model: "halogen-qwen3.8-flash-next", MaxTokens: 384, Think: "no"}}
			cfg.Agent.MaxSteps = 4
			cfg.Verify.Mode = "off"
			bot.cfg = cfg
			// Defence in depth: deny every tool at the motor too, including
			// textual tool markup that might be decoded after the HTTP proxy.
			deny := make(map[string]string)
			for _, spec := range agent.SpecsAll() {
				deny[spec.Name] = "deny"
			}
			bot.policy = &agent.Policy{Tools: deny}
			st := bot.state(1234567)
			st.mu.Lock()
			st.pol = bot.policy
			st.mode = agent.ModeAutonomous
			st.role = "chat"
			st.rolePinned = true
			st.convo = append([]llm.Message(nil), history...)
			st.mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			bot.handleMessage(ctx, msg(1234567, tc.request))
			if ctx.Err() != nil {
				t.Fatal(ctx.Err())
			}
			model.mu.Lock()
			calls := model.calls
			model.mu.Unlock()
			if calls != 1 {
				t.Fatalf("correct answer must finish in one real model call, got %d", calls)
			}
			st.mu.Lock()
			last := st.convo[len(st.convo)-1]
			st.mu.Unlock()
			if last.Role != "assistant" || strings.TrimSpace(last.Content) == "" {
				t.Fatalf("missing final answer: %q", last.Content)
			}
			lower := strings.ToLower(last.Content)
			if tc.name == "greeting" {
				if len([]rune(last.Content)) > 100 || strings.Contains(lower, "bench") || strings.Contains(lower, "dump") || strings.Contains(lower, "pendent") {
					t.Errorf("greeting reopened stale topic: %q", last.Content)
				}
			}
			if tc.name == "new-topic" && strings.Trim(lower, " .!\n\t\"*") != "hola" {
				t.Errorf("wanted hola, got %q", last.Content)
			}
			if tc.name == "cancel" && !strings.Contains(lower, "confirm") && !strings.Contains(lower, "deixem estar") && !strings.Contains(lower, "atur") && !strings.Contains(lower, "stop") {
				t.Errorf("cancellation not acknowledged: %q", last.Content)
			}
			t.Logf("REAL HALOGEN + TELEGRAM MOTOR: %q => %q, model_calls=%d", tc.request, last.Content, calls)
		})
	}
}
