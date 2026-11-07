package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWindChatStreamEmitsPhaseTokensDone: l'SSE del xat emet fase, tokens
// i done en ordre, amb el mateix payload final que el xat clàssic, i desa
// l'historial.
func TestWindChatStreamEmitsPhaseTokensDone(t *testing.T) {
	s := remindTestServer(t)
	old := windLoopStream
	defer func() { waitBackground(); windLoopStream = old }()
	// Historial net: el registre és global al procés i un altre test hi
	// pot haver deixat el seu torn (contaminació preexistent).
	hist0 := windHistory(s, "xaloc")
	hist0.mu.Lock()
	hist0.messages = nil
	hist0.mu.Unlock()
	windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string), _ ...*routedRole) (exploracio, error) {
		emit("phase", "web_search,web_fetch")
		emit("token", "Hola")
		emit("token", "!")
		return exploracio{text: "Hola!"}, nil
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"hola"}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	raw := rec.Body.String()
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type = %q, volia event-stream", ct)
	}
	ph := strings.Index(raw, "event: phase")
	tk := strings.Index(raw, "event: token")
	dn := strings.Index(raw, "event: done")
	if ph < 0 || tk < 0 || dn < 0 {
		t.Fatalf("falten events (phase=%d token=%d done=%d):\n%s", ph, tk, dn, raw)
	}
	if !(ph < tk && tk < dn) {
		t.Fatalf("ordre malament (phase=%d token=%d done=%d)", ph, tk, dn)
	}
	if !strings.Contains(raw, `"phase":"searching"`) {
		t.Fatalf("la fase no diu searching:\n%s", raw)
	}
	if !strings.Contains(raw, `"token":"Hola"`) || !strings.Contains(raw, `"token":"!"`) {
		t.Fatalf("falten tokens:\n%s", raw)
	}
	if !strings.Contains(raw, `"reply":"Hola!"`) {
		t.Fatalf("el done no porta el reply:\n%s", raw)
	}
	hist := windHistory(s, "xaloc")
	hist.mu.Lock()
	defer hist.mu.Unlock()
	if len(hist.messages) != 2 {
		t.Fatalf("historial = %d missatges, volia 2", len(hist.messages))
	}
}

// TestWindChatStreamError: si el loop falla, l'SSE emet error i no desa res.
func TestWindChatStreamError(t *testing.T) {
	s := remindTestServer(t)
	old := windLoopStream
	defer func() { waitBackground(); windLoopStream = old }()
	windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string), _ ...*routedRole) (exploracio, error) {
		return exploracio{}, context.DeadlineExceeded
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"hola"}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	raw := rec.Body.String()
	if !strings.Contains(raw, "event: error") {
		t.Fatalf("volia event error:\n%s", raw)
	}
	if strings.Contains(raw, "event: done") {
		t.Fatalf("no hi ha d'haver done després d'un error:\n%s", raw)
	}
}

// TestWindChatStreamCommitFresh: sense tombades entremig, el mateix torn
// publica i puja la generació.
func TestWindChatStreamCommitFresh(t *testing.T) {
	s := remindTestServer(t)
	old := windLoopStream
	defer func() { waitBackground(); windLoopStream = old }()
	hist0 := windHistory(s, "xaloc")
	hist0.mu.Lock()
	hist0.messages = nil
	hist0.gen = 0
	hist0.mu.Unlock()
	windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string), _ ...*routedRole) (exploracio, error) {
		return exploracio{text: "bo"}, nil
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"hola"}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	waitBackground()
	hist0.mu.Lock()
	defer hist0.mu.Unlock()
	if hist0.gen != 1 || len(hist0.messages) != 2 || hist0.messages[1].Content != "bo" {
		t.Fatalf("commit fresc malament: gen=%d msgs=%v", hist0.gen, hist0.messages)
	}
}
