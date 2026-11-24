package web

import (
	"strings"
	"testing"
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
