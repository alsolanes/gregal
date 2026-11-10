package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// No routing request may compete with the actual reply on a busy provider.
func TestRotllanaSelectionIsLocalAndKeepsTheThread(t *testing.T) {
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	rot, _ := s.rotllanaByID(id)
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"parlen\":[\"tramuntana\"]}"}}]}`))
	}))
	defer provider.Close()
	for name, p := range s.cfg.Providers {
		p.BaseURL = provider.URL
		s.cfg.Providers[name] = p
	}
	tail := []llm.Message{{Role: "assistant", SpeakerID: "tramuntana", Content: "Tramuntana: hola"}}
	got := s.routeRotllanaSpeakers(context.Background(), rot, "", "hola a tothom", tail)
	if requests.Load() != 0 {
		t.Fatalf("speaker selection called provider %d times", requests.Load())
	}
	// Un missatge sense ofici ni nom el continua qui portava el fil.
	if len(got) != 1 || got[0].ID != "tramuntana" {
		t.Fatalf("expected the wind that was talking, got %+v", got)
	}
}

func TestRotllanaLocalSelectionSemantics(t *testing.T) {
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	rot, _ := s.rotllanaByID(id)
	tail := []llm.Message{{Role: "assistant", Content: "Tramuntana: old reply"}}
	for _, tc := range []struct{ name, wanted, msg, want string }{
		{"thread continuity with legacy identity", "", "hola", "tramuntana"},
		{"explicit member wins over mention", "llevant", "@Xaloc hola", "llevant"},
		{"unknown explicit member is not silently replaced", "absent", "hola", ""},
		{"accent insensitive case", "", "@XALOC hola", "xaloc"},
		{"word boundaries", "", "Xaloca", "tramuntana"},
		{"two named members", "", "@Tramuntana @Llevant", "tramuntana,llevant"},
		{"two voice cap", "", "Tramuntana Xaloc Llevant", "tramuntana,xaloc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ids []string
			for _, w := range s.routeRotllanaSpeakers(context.Background(), rot, tc.wanted, tc.msg, tail) {
				ids = append(ids, w.ID)
			}
			if got := strings.Join(ids, ","); got != tc.want {
				t.Fatalf("got=%s want=%s", got, tc.want)
			}
		})
	}
	members := []companion.Wind{{ID: "a", Name: "Ponent"}, {ID: "b", Name: "Ponent"}}
	if got := windsNamedIn(members, "@Ponent"); len(got) != 0 {
		t.Fatalf("ambiguous name chose %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := s.routeRotllanaSpeakers(ctx, rot, "", "hola", nil); len(got) != 0 {
		t.Fatal("cancelled selector chose a speaker")
	}
	// Una conversa no salta de vent en vent: qui l'ha començada la continua.
	var participation []llm.Message
	first := ""
	for range 3 {
		got := s.routeRotllanaSpeakers(context.Background(), rot, "", "hola", participation)
		if len(got) != 1 || (first != "" && got[0].ID != first) {
			t.Fatalf("the thread jumped to another wind: %+v", got)
		}
		first = got[0].ID
		participation = append(participation, llm.Message{Role: "user"}, llm.Message{Role: "assistant", SpeakerID: got[0].ID})
	}
}

// TestWindNamedInSkipsRouter: si el missatge nomena un sol vent, respon ell
// sense passar pel model que tria; un nom dins d'una altra paraula no compta.
func TestWindNamedInSkipsRouter(t *testing.T) {
	members := []companion.Wind{{ID: "ponent", Name: "Ponent"}, {ID: "garbi", Name: "Garbí"}, {ID: "gregal", Name: "Gregal"}}
	for msg, want := range map[string]string{
		"Ponent, i un de poesia?":       "ponent",
		"@garbí quan juga el Barça?":    "garbi",
		"Ponent i Garbí, què en dieu?":  "",
		"Ponentí no és ningú":           "",
		"un llibre curt per al viatge?": "",
	} {
		w, ok := windNamedIn(members, msg)
		if got := map[bool]string{true: w.ID, false: ""}[ok]; got != want {
			t.Errorf("%q → %q, volia %q", msg, got, want)
		}
	}
}
