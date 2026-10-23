package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Opt-in real-provider check. It only asks the model: returned tool calls are
// failures, never executed. No Telegram delivery or real session writes occur.
func TestLatestRequestLive(t *testing.T) {
	if os.Getenv("GREGAL_PRIORITY_LIVE") != "1" {
		t.Skip("set GREGAL_PRIORITY_LIVE=1 and GREGAL_PRIORITY_HISTORY to replay a saved history")
	}
	raw, err := os.ReadFile(os.Getenv("GREGAL_PRIORITY_HISTORY"))
	if err != nil {
		t.Fatal(err)
	}
	var history []llm.Message
	if err := json.Unmarshal(raw, &history); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("GREGAL_PRIORITY_CONFIG")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	sys := PromptFor(cfg.SystemPrompt(), ModeAutonomous)
	if info := ContextProjecte("/home/aleix"); info != "" {
		sys += "\n\n" + info
	}
	cases := []struct {
		name, latest string
		compact      bool
	}{
		{"greeting", "Ep", false},
		{"other-greeting", "Hola!", false},
		{"greeting-with-request", "Ep! Com es diu hello en català? Respon només la traducció.", false},
		{"new-topic", "Canvi de tema: com es diu hello en català? Respon només la traducció.", false},
		{"explicit-continuation", "Segueix amb el tema de les instruccions i tools: resumeix en una frase les opcions que havies proposat, sense implementar res.", false},
		{"cancel", "No facis res més del bench ni del dump. Ho deixem estar. Confirma-ho breument.", false},
		{"compacted-greeting", "Ep", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prior := history
			if tc.compact {
				prior = []llm.Message{{Role: "user", Content: "Resum de conversa anterior: el bench, --compare i /speed estan acabats. Es va parlar de posar instruccions i tools abans de la resposta i d'un possible dump, però no es va autoritzar implementar res. Continua amb aquest context."}}
			}
			messages := append([]llm.Message{{Role: "system", Content: sys}}, prior...)
			messages = append(messages, llm.Message{Role: "user", Content: tc.latest})
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			ctx = llm.WithThink(ctx, "no")
			text, calls, err := llm.New().ChatWithTools(ctx, "http://127.0.0.1:8731/v1", "", "halogen-qwen3.8-flash-next", messages, 0, 384, SpecsAll())
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("REAL HALOGEN: %q => %q (tool_calls=%d)", tc.latest, text, len(calls))
			if len(calls) != 0 {
				t.Fatalf("no tools needed; model requested %d calls (not executed)", len(calls))
			}
			lower := strings.ToLower(text)
			if strings.TrimSpace(text) == "" {
				t.Fatal("empty answer")
			}
			switch tc.name {
			case "greeting", "other-greeting", "compacted-greeting":
				if len([]rune(text)) > 100 || strings.Contains(text, "\n\n") {
					t.Errorf("greeting must stay brief, not become a history recap: %q", text)
				}
				for _, stale := range []string{"bench", "dump", "tools", "instruccions", "cache", "pendent"} {
					if strings.Contains(lower, stale) {
						t.Errorf("greeting reopened stale topic %q", stale)
					}
				}
			case "greeting-with-request", "new-topic":
				if strings.Trim(lower, " .!\n\t\"*") != "hola" {
					t.Errorf("wanted only hola, got %q", text)
				}
			case "cancel":
				if !strings.Contains(lower, "confirm") && !strings.Contains(lower, "deixem estar") && !strings.Contains(lower, "atur") && !strings.Contains(lower, "stop") {
					t.Errorf("response must acknowledge cancellation: %q", text)
				}
			case "explicit-continuation":
				if !strings.Contains(lower, "dump") && !strings.Contains(lower, "tools") && !strings.Contains(lower, "instruccions") {
					t.Errorf("explicit continuation lost the prior topic: %q", text)
				}
			}
		})
	}
}
