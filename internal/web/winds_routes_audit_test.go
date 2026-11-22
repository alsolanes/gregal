package web

import (
	"net/http"
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

// The route the clients ask before sending to a rotllana sends work to the
// crew with Codi in any mode.
func TestCrewEntryRouteIgnoresTheCodiMode(t *testing.T) {
	s, _ := rotllanaChatter(t, "chat")
	for _, mode := range []string{"plan", "chat", "inspect"} {
		s.mode = mode
		rec := postJSON(t, s.handleWindIntent, `{"message":"Implementa una aplicació web"}`)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"intent":"work"`) {
			t.Errorf("mode %q: intent route returned %d %s, want work", mode, rec.Code, rec.Body.String())
		}
	}
}
