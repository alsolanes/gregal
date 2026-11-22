package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/team"
)

func TestCrewChatRoutingAndPlan(t *testing.T) {
	s, _ := rotllanaChatter(t, "chat")
	for _, tc := range []struct {
		msg  string
		want bool
	}{
		{"Feu un programa Python que gestioni notes", true},
		{"Fes una app que no demani registre", true},
		{"vull programar alguna cosa", false},
		{"Vull programar un joc", true},
		{"Podeu crear-me una aplicació?", true},
		{"Afegiu una funció al projecte", true},
		{"Haced un programa Python", true},
		{"Quiero programar una aplicación", true},
		{"Build a Python script", true},
		{"Could you create an application?", true},
		{"No modifiquis el codi", false},
		{"No creïs cap fitxer", false},
		{"No vull crear una web", false},
		{"No vull que facis una web", false},
		{"No vull que creïs una web", false},
		{"Do not ever create a web app", false},
		{"Com puc crear una aplicació?", false},
		{"Què cal per crear una web?", false},
		{"Can you explain how to build a web app?", false},
		{"No escribas código", false},
		{"Do not modify the code", false},
		{"¿Cómo puedo crear una aplicación?", false},
		{"How can I build a web app?", false},
		{"Implementa una aplicació web de notes", true},
		{"Vull que creïs una web espectacular", true},
		{"Ara explica com implementar una web", false},
		{"Build a Python script", true},
		{"Explica com implementar una aplicació web", false},
		{"Com puc implementar una aplicació web?", false},
		{"hola, com esteu?", false},
	} {
		if got := s.crewChatAllowed(tc.msg); got != tc.want {
			t.Errorf("%q: got %v", tc.msg, got)
		}
	}
	for _, tc := range []struct{ message, want string }{
		{"Revisa aquest projecte", "review"},
		{"Implementa una aplicació web", "work"},
		{"No modifiquis el codi", "chat"},
	} {
		rec := postJSON(t, s.handleWindIntent, `{"message":"`+tc.message+`"}`)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"intent":"`+tc.want+`"`) {
			t.Fatalf("intent %q: %d %s", tc.message, rec.Code, rec.Body.String())
		}
	}
}

// Codi's mode does not reach the winds: a crew starts and finishes with
// Codi in Pregunta or Planifica, resumes with the answer to its question,
// and Codi keeps its mode.
func TestCrewStartsWhateverTheCodiMode(t *testing.T) {
	for _, mode := range []string{"plan", "chat", "inspect"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := companionTestServer(t)
			a, _ := s.winds.Save(companion.Wind{Name: "Gregal", Role: "Organització"})
			b, _ := s.winds.Save(companion.Wind{Name: "Ratxa", Role: "Disseny"})
			savedWork, savedCoord := crewWork, crewCoordinator
			t.Cleanup(func() { crewWork, crewCoordinator = savedWork, savedCoord })
			crewWork = func(*Server, context.Context, Rotllana, string, team.FlowSpec, bool) (string, error) {
				return "Fet", nil
			}
			calls := 0
			crewCoordinator = func(*Server, context.Context, string, string) (string, error) {
				calls++
				if calls == 1 {
					return `{"action":"ask","question":"De quin color?","options":["Blava","Verda"]}`, nil
				}
				return `{"action":"finish","conclusion":"Web feta."}`, nil
			}
			s.mode = mode
			rec := postJSON(t, s.handleCrewStart, `{"task":"build a web app","lang":"ca","reuse":["`+a.ID+`","`+b.ID+`"]}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("got HTTP %d: %s", rec.Code, rec.Body.String())
			}
			var out struct {
				Rotllana Rotllana `json:"rotllana"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			waitCrew(t, s, out.Rotllana.ID, "waiting")
			rot, _ := s.rotllanaByID(out.Rotllana.ID)
			if got := s.crewTakeAnswer(rot, "Blava"); got != crewAnswerConsumed {
				t.Fatalf("the waiting crew must take the answer, got %v", got)
			}
			waitCrew(t, s, out.Rotllana.ID, "done")
			if s.mode != mode {
				t.Errorf("the crew changed Codi's mode to %q", s.mode)
			}
		})
	}
}

// A crew's builder seat writes with Codi in Planifica; Codi's own team keeps
// following Codi's mode.
func TestCrewSeatWritesWhateverTheCodiMode(t *testing.T) {
	for _, winds := range []bool{true, false} {
		t.Run(fmt.Sprintf("winds=%v", winds), func(t *testing.T) {
			ws := t.TempDir()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				body := `{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}]}`
				if calls.Add(1) == 1 {
					body = `{"choices":[{"message":{"tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":"{\"path\":\"index.html\",\"content\":\"hola\"}"}}]},"finish_reason":"tool_calls"}]}`
				}
				if _, err := io.WriteString(w, body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			s := goalTestServer(t)
			s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: server.URL}}
			s.permissive = true
			s.mode = agent.ModePlan
			role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}
			seat := teamSeat{Agentic: true, Mode: agent.ModeCode, Tools: []string{"write"}, MaxSteps: 4}
			if _, _, err := s.runTeamSeat(context.Background(), s.cfg, role, seat, ws, "English", "builder", "write the page", winds, func(string, any) {}, nil); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(filepath.Join(ws, "index.html"))
			if winds && err != nil {
				t.Fatalf("a crew seat could not write with Codi in plan: %v", err)
			}
			if !winds && err == nil {
				t.Fatal("Codi's own team wrote while Codi is in plan")
			}
			if s.mode != agent.ModePlan {
				t.Errorf("the crew changed Codi's mode to %q", s.mode)
			}
		})
	}
}

func TestCrewNodePublishesOnlyRealWinds(t *testing.T) {
	s, id := rotllanaChatter(t, "chat")
	rot, _ := s.rotllanaByID(id)
	members := s.rotMembersOf(rot)
	flow := team.FlowSpec{Nodes: []team.FlowNode{
		{ID: "build-1", Wind: members[0].ID, Role: "builder"},
		{ID: "review", Wind: members[1].ID, Role: "reviewer", Mode: "review"},
	}}
	s.crewPublishNode(rot, flow, team.Event{Type: "completed", Agent: "build-1", Output: "Wrote app.py"})
	s.crewPublishNode(rot, flow, team.Event{Type: "completed", Agent: "review", Output: "Wrote app.py", Message: "Read app.py"})
	s.crewPublishNode(rot, flow, team.Event{Type: "completed", Agent: "review", Output: "Wrote app.py"})
	s.crewPublishNode(rot, flow, team.Event{Type: "completed", Agent: "review", Output: "Wrote app.py", Message: `{"approved":true}`})
	s.crewPublishNode(rot, flow, team.Event{Type: "completed", Agent: "other", Output: "invented"})
	h := s.crewHistory(rot)
	if len(h) != 2 || h[0].SpeakerID != members[0].ID || h[1].SpeakerID != members[1].ID || !strings.Contains(h[1].Content, "Read app.py") || strings.Contains(h[1].Content, "approved") {
		t.Fatalf("node attribution: %+v", h)
	}
}

func TestCrewChatStreamProgressCancelAndHistory(t *testing.T) {
	s, id := rotllanaChatter(t, "ordinary chat")
	oldWork, oldCoord := crewWork, crewCoordinator
	t.Cleanup(func() { waitBackground(); crewWork, crewCoordinator = oldWork, oldCoord })
	started := make(chan struct{})

	crewCoordinator = func(*Server, context.Context, string, string) (string, error) { return `{"action":"work"}`, nil }
	crewWork = func(_ *Server, ctx context.Context, _ Rotllana, _ string, _ team.FlowSpec, _ bool) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}
	rec := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		s.handleRotllanaChatStream(rec, httptest.NewRequest("POST", "/api/winds/rotllana/stream", strings.NewReader(`{"rotllana":"`+id+`","message":"Implementa una aplicació web de notes"}`)))
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("crew work did not start")
	}
	invalid := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","wind":"not-a-member","message":"Implementa una aplicació web"}`)
	if invalid.Code != 400 {
		t.Fatalf("invalid recipient: %d", invalid.Code)
	}
	busy := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"hola"}`)
	if busy.Code != 409 {
		t.Fatalf("active crew accepted another message: %d", busy.Code)
	}
	cancel := postJSON(t, s.handleWindCancel, `{"rotllana":"`+id+`"}`)
	if cancel.Code != 200 {
		t.Fatalf("cancel: %d %s", cancel.Code, cancel.Body.String())
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not finish after cancel")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: phase") || !strings.Contains(body, "speaker:") || !strings.Contains(body, "event: token") || !strings.Contains(body, "round 1") || !strings.Contains(body, "event: done") {
		t.Fatalf("missing actual stream progress: %s", body)
	}
	rot, _ := s.rotllanaByID(id)
	hist := s.crewHistory(rot)
	n := 0
	for _, m := range hist {
		if m.Role == "user" && m.Content == "Implementa una aplicació web de notes" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("user message persisted %d times", n)
	}
	v, _ := crewJobs.Load(crewRegistryKey(s, id))
	if v.(*crewJob).get() != "cancelled" {
		t.Fatalf("step: %s", v.(*crewJob).get())
	}
}

func TestCrewBusyMessageIsLocalized(t *testing.T) {
	s, _ := rotllanaChatter(t, "chat")
	if !strings.Contains(s.crewBusyMessage(), "crew is already working") {
		t.Fatalf("English busy message: %q", s.crewBusyMessage())
	}
	s.cfg.Language = "ca"
	if strings.Contains(s.crewBusyMessage(), "crew is already working") {
		t.Fatalf("busy message not localized in ca: %q", s.crewBusyMessage())
	}
}
