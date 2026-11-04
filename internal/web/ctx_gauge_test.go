package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// «Explora tot el codi» llegeix desenes de milers de tokens d'eines, però convo
// només guarda els missatges: el mesurador de context s'ha de fixar en l'última
// crida real al model, no només en l'historial.
func TestContextGaugeUsesLastRealPrompt(t *testing.T) {
	s, _ := companionTestServer(t)
	used := func() int {
		w := httptest.NewRecorder()
		s.handleState(w, httptest.NewRequest("GET", "/api/state", nil))
		var st map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatal(err)
		}
		return int(st["used_tokens"].(float64))
	}
	if got := used(); got > 100 {
		t.Fatalf("sense conversa hauria de ser ~0, és %d", got)
	}
	s.lastPrompt.Store(48000)
	if got := used(); got != 48000 {
		t.Fatalf("used_tokens = %d, volia 48000 (l'última crida)", got)
	}
	s.lastPrompt.Store(0)
	if got := used(); got > 100 {
		t.Fatalf("després de netejar, %d", got)
	}
}
