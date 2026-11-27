package web

import (
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/session"
)

// Una conversa on el vent ha parlat primer (una rutina, un cor) no té cap
// missatge de la persona: el títol és el que el vent va dir, no el nom intern
// de la sessió («Company — Ponent»).
func TestWindThreadTitleWithoutUserMessage(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Ponent","color":"#7FB3E0","role":"El temps"}`)
	w, _ := s.winds.ByID(windIDByName(t, s, "Ponent"))
	postWindMessage(s, w, "routine", "Avui a Barcelona plou poc, només al vespre.\n\nDetalls…")
	t.Cleanup(waitBackground)
	var found bool
	for _, th := range s.recentWindThreads() {
		if th.Wind != w.ID {
			continue
		}
		found = true
		if strings.Contains(th.Title, "Company") || !strings.HasPrefix(th.Title, "Avui a Barcelona plou poc") {
			t.Fatalf("títol: %q", th.Title)
		}
	}
	if !found {
		t.Fatal("la conversa del vent ha de sortir a la llista")
	}
}

// Les converses d'encàrrec desades abans de c3b8a10f porten l'etiqueta per al
// model al primer missatge: en llegir-les es mostren com «Encàrrec de Gregal: …».
func TestWindThreadTitleOldHandoffPrefix(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.cfg.Language = "ca"
	postJSON(t, s.handleWinds, `{"name":"Ponent","color":"#7FB3E0","role":"El temps"}`)
	w, _ := s.winds.ByID(windIDByName(t, s, "Ponent"))
	name := s.windSessionName(w.ID)
	if _, err := session.SaveSession(s.sessDir(), session.Session{Name: name, Title: name, Convo: []llm.Message{
		{Role: "user", Content: "[Task handed over by the main agent] Fes un joc de la serp"},
		{Role: "assistant", Content: "Fet."},
	}}); err != nil {
		t.Fatal(err)
	}
	if got := threadTitle(s, name); got != "Encàrrec de Gregal: Fes un joc de la serp" {
		t.Fatalf("títol: %q", got)
	}
	h := windHistoryFor(s, name)
	h.mu.RLock()
	shown := h.messages[0].Content
	h.mu.RUnlock()
	if shown != "Encàrrec de Gregal: Fes un joc de la serp" {
		t.Fatalf("historial: %q", shown)
	}
}
