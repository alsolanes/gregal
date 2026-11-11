package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
	"gregal/internal/llm"
	"gregal/internal/runs"
)

// Els missatges antics (sense id desat) en reben un de derivat que no canvia
// entre lectures; dos missatges diferents no comparteixen id.
func TestMessageIDDerivedIsStable(t *testing.T) {
	a := llm.Message{Role: "assistant", Content: "hola", Ts: 1700000000}
	b := llm.Message{Role: "assistant", Content: "adéu", Ts: 1700000000}
	if messageID(a) != messageID(a) || !strings.HasPrefix(messageID(a), "h-") {
		t.Fatalf("id derivat inestable: %q", messageID(a))
	}
	if messageID(a) == messageID(b) {
		t.Fatal("dos missatges diferents amb el mateix id")
	}
	if got := messageID(llm.Message{ID: "m-x", Content: "hola"}); got != "m-x" {
		t.Fatalf("no respecta l'id desat: %q", got)
	}
	if newMessageID() == newMessageID() {
		t.Fatal("ids nous repetits")
	}
	rows := petHistoryJSON([]llm.Message{a, {Role: "user", Content: "ei", ID: "m-1"}})
	if rows[0]["id"] != messageID(a) || rows[1]["id"] != "m-1" {
		t.Fatalf("l'historial no porta els ids: %+v", rows)
	}
}

// Una veu proactiva és UN missatge: la novetat (notificació, «Avui») hi apunta
// amb message_id, i és el mateix id que l'app rep a l'historial del fil.
func TestProactiveMessageNovetatPointsToThreadMessage(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"futbol"}`)
	if _, err := s.jobs.Save(jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc", Prompt: "vigila", Kind: "event", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	next := time.Now().Add(6 * time.Hour).UTC().Format(time.RFC3339)
	old := windHeartbeatLoop
	windHeartbeatLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
		return exploracio{text: "Dissabte juga el Barça a les 21:00.\nNEXT: " + next}, nil
	}
	t.Cleanup(func() { waitBackground(); windHeartbeatLoop = old })
	if _, err := s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc"}, runs.Run{},
		companion.Wind{ID: "xaloc", Name: "Xaloc", Role: "futbol", Persona: "ets en Xaloc"}); err != nil {
		t.Fatal(err)
	}
	hist := windHistory(s, "xaloc")
	hist.mu.Lock()
	rows := petHistoryJSON(hist.messages)
	hist.mu.Unlock()
	last := rows[len(rows)-1]
	id, _ := last["id"].(string)
	if !strings.HasPrefix(id, "m-") {
		t.Fatalf("la veu proactiva no té id propi: %+v", last)
	}
	nov := loadNovetats(s)
	if len(nov) == 0 || nov[len(nov)-1].MessageID != id {
		t.Fatalf("la novetat no apunta al missatge del fil (%q): %+v", id, nov)
	}
}
