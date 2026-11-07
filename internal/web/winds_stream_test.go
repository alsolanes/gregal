package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
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
	windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
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
	windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
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

// TestWindChatStreamCommitGen: el commit d'un torn lent no trepitja un torn
// posterior publicat entremig; el GET no bloqueja mentre el model corre.
func TestWindChatStreamCommitGen(t *testing.T) {
	s := remindTestServer(t)
	old := windLoopStream
	defer func() { waitBackground(); windLoopStream = old }()
	hist0 := windHistory(s, "xaloc")
	hist0.mu.Lock()
	hist0.messages = nil
	hist0.gen = 0
	hist0.mu.Unlock()

	inModel := make(chan struct{})
	release := make(chan struct{})
	windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
		close(inModel)
		<-release
		return exploracio{text: "lent"}, nil
	}

	// Torn lent en marxa: ha de deixar el GET passar mentre corre.
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"lent"}`))
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { s.handleWindChatStream(rec, req); close(done) }()
	<-inModel

	getDone := make(chan struct{})
	var got map[string]any
	go func() {
		gr := httptest.NewRequest("GET", "/api/winds/chat?wind=xaloc", nil)
		grec := httptest.NewRecorder()
		s.handleWindChat(grec, gr)
		json.Unmarshal(grec.Body.Bytes(), &got)
		close(getDone)
	}()
	select {
	case <-getDone:
	case <-time.After(2 * time.Second):
		t.Fatal("el GET ha quedat bloquejat pel torn lent")
	}

	// Un segon torn ràpid publica entremig (disc + memòria, com un torn de
	// veritat que ha passat pel mateix commit).
	rapid := []llm.Message{{Role: "user", Content: "nou"}, {Role: "assistant", Content: "rapid"}}
	if err := saveWindHistory(s, companion.Wind{ID: "xaloc", Name: "Xaloc"}, rapid); err != nil {
		t.Fatal(err)
	}
	hist0.mu.Lock()
	hist0.messages = append([]llm.Message(nil), rapid...)
	hist0.gen = 99
	hist0.mu.Unlock()
	close(release)
	<-done
	waitBackground()

	hist0.mu.Lock()
	defer hist0.mu.Unlock()
	if hist0.gen != 99 {
		t.Fatalf("gen = %d, volia 99 (el torn vencellat no puja la generació)", hist0.gen)
	}
	if len(hist0.messages) != 2 || hist0.messages[1].Content != "rapid" {
		t.Fatalf("el torn lent ha trepitjat l'historial del ràpid: %v", hist0.messages)
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
	windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
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
