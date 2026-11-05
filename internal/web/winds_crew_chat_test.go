package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	for _, mode := range []string{"plan", "chat", "inspect"} {
		s.mode = mode
		if s.crewChatAllowed("Implementa una aplicació web") || s.crewMayResume() {
			t.Errorf("read-only mode %q must not start or resume crew", mode)
		}
	}
}

func TestCrewStartEndpointHonorsReadOnlyMode(t *testing.T) {
	s, _ := rotllanaChatter(t, "chat")
	for _, mode := range []string{"plan", "chat", "inspect"} {
		s.mode = mode
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/winds/crew/start", strings.NewReader(`{"task":"build a web app","reuse":["wind"]}`))
		s.handleCrewStart(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("mode %q: got HTTP %d, want 403", mode, rec.Code)
		}
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
	oldDiscuss, oldWork, oldCoord := crewDiscussLoop, crewWork, crewCoordinator
	t.Cleanup(func() { waitBackground(); crewDiscussLoop, crewWork, crewCoordinator = oldDiscuss, oldWork, oldCoord })
	started := make(chan struct{})
	crewDiscussLoop = func(*Server, context.Context, string) (string, error) { return "Real design", nil }
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
	if !strings.Contains(body, "event: phase") || !strings.Contains(body, "speaker:") || !strings.Contains(body, "event: token") || !strings.Contains(body, "Real design") || !strings.Contains(body, "event: done") {
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
