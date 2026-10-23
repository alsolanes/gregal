package telegram

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/agent"
)

// The live probe's deny-all policy must also block tools recovered from
// textual DSML after the HTTP proxy, not only structured tool_calls.
func TestProbePolicyBlocksRecoveredDSML(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	content := `<｜DSML｜calls><｜DSML｜invoke name="write"><｜DSML｜parameter name="path" string="true">` + marker + `</｜DSML｜parameter><｜DSML｜parameter name="content" string="true">unsafe</｜DSML｜parameter></｜DSML｜invoke></｜DSML｜calls>`
	model := newFakeLLM(t, replyText(content), replyText("Probe finished safely."))
	tg := newFakeTG(t)
	bot, _, _ := testBot(t, tg, model, map[string]string{"write": "allow"})
	deny := make(map[string]string)
	for _, spec := range agent.SpecsAll() {
		deny[spec.Name] = "deny"
	}
	bot.policy = &agent.Policy{Tools: deny}
	st := bot.state(1234567)
	st.mu.Lock()
	st.pol = bot.policy
	st.mode = agent.ModeAutonomous
	st.mu.Unlock()
	bot.handleMessage(context.Background(), msg(1234567, "Test probe tool guard"))
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("recovered DSML executed a write: %v", err)
	}
	model.mu.Lock()
	bodies := append([]string(nil), model.bodies...)
	model.mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("expected DSML handling then final answer, got %d calls", len(bodies))
	}
	var request struct {
		Messages []struct{ Role, Content string } `json:"messages"`
	}
	if err := json.Unmarshal([]byte(bodies[1]), &request); err != nil {
		t.Fatal(err)
	}
	blocked := false
	for _, message := range request.Messages {
		if message.Role == "tool" && strings.Contains(strings.ToLower(message.Content), "permissions") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatal("DSML was not recovered and rejected by explicit deny policy")
	}
}
