package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCrewClassifierKeepsQuotedAndNegatedRequestsAsChat(t *testing.T) {
	s, _ := rotllanaChatter(t, "chat")
	for _, msg := range []string{
		`Please explain this quote: “Build a Python app.”`,
		`The issue says "implement a web app"; what does that mean?`,
		`I am not asking you to create a website; explain the design instead.`,
		`Could you explain how to implement a login page?`,
	} {
		if s.crewChatAllowed(msg) {
			t.Errorf("quoted, negated, or explanatory text started autonomous work: %q", msg)
		}
	}
}

func TestCrewReadOnlyEntryRoutesRejectStartAndKeepChatInConversation(t *testing.T) {
	s, _ := rotllanaChatter(t, "chat")
	for _, mode := range []string{"plan", "chat", "inspect"} {
		s.mode = mode
		if s.crewMayResume() {
			t.Errorf("mode %q permits crew resume", mode)
		}
		if s.crewChatAllowed("Implementa una aplicació web") {
			t.Errorf("mode %q routes a request into crew work", mode)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/winds/crew/start", strings.NewReader(`{"task":"build a web app","reuse":["wind"]}`))
		s.handleCrewStart(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("mode %q: start route returned %d, want 403", mode, rec.Code)
		}
	}
}
