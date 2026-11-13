package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Les captures del torn són de la conversa desada, mai del cable al provider.
func TestShotsNeverReachTheProvider(t *testing.T) {
	m := Message{Role: "assistant", Content: "x", Shots: []MessageShot{{ID: "a", Title: "t"}}}
	saved, _ := json.Marshal(m)
	if !strings.Contains(string(saved), `"shots"`) {
		t.Fatal("la conversa desada les ha de conservar")
	}
	wire, err := marshalMessage(m, true)
	if err != nil || strings.Contains(string(wire), "shots") {
		t.Fatalf("han arribat al provider: %s %v", wire, err)
	}
}
