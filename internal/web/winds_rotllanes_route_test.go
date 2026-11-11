package web

import (
	"context"
	"testing"

	"gregal/internal/llm"
)

// El router tria pel sentit: un tema d'un altre ofici no es queda amb qui
// portava el fil, i uns ids inventats o repetits es descarten.
func TestRotllanaModelRouteOverridesSticky(t *testing.T) {
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	rot, _ := s.rotllanaByID(id)
	old := rotllanaRouteModel
	defer func() { rotllanaRouteModel = old }()
	tail := []llm.Message{{Role: "assistant", SpeakerID: "tramuntana", Content: "hola"}}
	ids := func(raw string, msg string) string {
		rotllanaRouteModel = func(*Server, context.Context, []llm.Message) (string, error) { return raw, nil }
		out := ""
		for _, w := range s.routeRotllanaSpeakers(context.Background(), rot, "", msg, tail) {
			out += w.ID + ","
		}
		return out
	}
	if got := ids(`{"speakers":["xaloc"]}`, "quin llenguatge per a una API petita?"); got != "xaloc," {
		t.Fatalf("got %q", got)
	}
	if got := ids("```json\n{\"speakers\":[\"llevant\",\"nope\",\"llevant\",\"Xaloc\"]}\n```", "explica'm això amb detall"); got != "llevant,xaloc," {
		t.Fatalf("got %q", got)
	}
	// Resposta il·legible: cau a la regla local (qui portava el fil).
	if got := ids("no sé", "explica'm això amb detall"); got != "tramuntana," {
		t.Fatalf("fallback got %q", got)
	}
}
