package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/tools"
)

// Un vent que demana permís o fa una pregunta s'ha d'aturar de debò: la petició
// surt pel flux, és a /api/winds/pending i el torn només continua quan la
// persona respon.

func waitForWindAsk(t *testing.T, s *Server) *windAskReq {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		for _, a := range s.windAsks {
			s.mu.Unlock()
			return a
		}
		s.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("el torn no ha publicat cap petició")
	return nil
}

func answerWindAsk(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/winds/answer", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleWindsAnswer(rec, req)
	return rec
}

// runAskStream engega un torn en un fil i torna una funció que n'espera el final.
func runAskStream(t *testing.T, s *Server, loop func(ctx context.Context, emit func(kind, text string)) string) func() string {
	t.Helper()
	old := windLoopStream
	windLoopStream = func(s *Server, ctx context.Context, _ string, emit func(kind, text string), _ ...*routedRole) (exploracio, error) {
		return exploracio{text: loop(ctx, emit)}, nil
	}
	t.Cleanup(func() { waitBackground(); windLoopStream = old })
	var wg sync.WaitGroup
	rec := httptest.NewRecorder()
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"instal·la l'SDK"}`))
		s.handleWindChatStream(rec, req)
	}()
	return func() string { wg.Wait(); return rec.Body.String() }
}

func TestWindApprovalBlocksUntilTheUserAnswers(t *testing.T) {
	s := remindTestServer(t)
	hist := windHistory(s, "xaloc")
	hist.mu.Lock()
	hist.messages = nil
	hist.mu.Unlock()

	released := make(chan struct{})
	wait := runAskStream(t, s, func(ctx context.Context, emit func(kind, text string)) string {
		ok, msg := s.windApprove(ctx, emit, "bash", "install", "sdkmanager --licenses", "needs your approval: install")
		close(released)
		if ok {
			return "Instal·lat."
		}
		return "No ho he fet: " + msg
	})
	ask := waitForWindAsk(t, s)
	// Mentre ningú no respon, el torn no avança.
	select {
	case <-released:
		t.Fatal("el torn ha continuat sense resposta")
	case <-time.After(150 * time.Millisecond):
	}
	// La petició es pot tornar a trobar (si l'app s'havia tancat).
	pend := httptest.NewRecorder()
	s.handleWindsPending(pend, httptest.NewRequest("GET", "/api/winds/pending", nil))
	var list struct {
		Pending []windAskReq `json:"pending"`
	}
	_ = json.Unmarshal(pend.Body.Bytes(), &list)
	if len(list.Pending) != 1 || list.Pending[0].Kind != "install" || list.Pending[0].What != "sdkmanager --licenses" || list.Pending[0].Type != "approve" {
		t.Fatalf("pendents: %+v", list.Pending)
	}
	if rec := answerWindAsk(t, s, `{"key":"`+ask.Key+`","approve":false}`); rec.Code != 200 {
		t.Fatalf("respondre: %d %s", rec.Code, rec.Body)
	}
	raw := wait()
	<-released
	if !strings.Contains(raw, "event: approve_request") || !strings.Contains(raw, `"kind":"install"`) {
		t.Fatalf("la petició no ha arribat al client:\n%s", raw)
	}
	if !strings.Contains(raw, "event: ask_resolved") || !strings.Contains(raw, `"answered":true`) {
		t.Fatalf("falta que el client sàpiga que s'ha resolt:\n%s", raw)
	}
	if !strings.Contains(raw, "tapped Decline") {
		t.Fatalf("el vent ha de saber que s'ha rebutjat:\n%s", raw)
	}
	// Una segona resposta a la mateixa petició ja no val.
	if rec := answerWindAsk(t, s, `{"key":"`+ask.Key+`","approve":true}`); rec.Code != http.StatusNotFound {
		t.Fatalf("resposta repetida: %d", rec.Code)
	}
}

func TestWindApprovalAllowedLetsTheActionRun(t *testing.T) {
	s := remindTestServer(t)
	wait := runAskStream(t, s, func(ctx context.Context, emit func(kind, text string)) string {
		ok, _ := s.windApprove(ctx, emit, "bash", "download", "curl -o x.zip https://e.com/x.zip", "")
		if ok {
			return "Baixat."
		}
		return "No."
	})
	ask := waitForWindAsk(t, s)
	answerWindAsk(t, s, `{"key":"`+ask.Key+`","approve":true}`)
	if raw := wait(); !strings.Contains(raw, "Baixat.") {
		t.Fatalf("amb el sí, l'acció s'havia de fer:\n%s", raw)
	}
}

func TestWindQuestionReturnsTheChoice(t *testing.T) {
	s := remindTestServer(t)
	wait := runAskStream(t, s, func(ctx context.Context, emit func(kind, text string)) string {
		return s.windQuestion(ctx, emit, "Quin color?", []tools.QuestionOption{{Label: "Blau"}, {Label: "Verd", Description: "més fosc"}})
	})
	ask := waitForWindAsk(t, s)
	if ask.Type != "question" || len(ask.Options) != 2 || ask.Options[1].Description != "més fosc" {
		t.Fatalf("pregunta: %+v", ask)
	}
	answerWindAsk(t, s, `{"key":"`+ask.Key+`","answer":"Verd"}`)
	raw := wait()
	if !strings.Contains(raw, "event: question_request") || !strings.Contains(raw, "The user answered: Verd") {
		t.Fatalf("la resposta no ha arribat al vent:\n%s", raw)
	}
}

func TestWindAskStopsWhenTheTurnIsCancelled(t *testing.T) {
	s := remindTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() {
		ok, msg := s.windApprove(ctx, nil, "bash", "delete", "rm -rf /srv/x", "")
		done <- !ok && strings.Contains(msg, "did not answer")
	}()
	waitForWindAsk(t, s)
	cancel()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("un torn aturat no ha de comptar com a «sí»")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("aturar el torn no allibera l'espera")
	}
	s.mu.Lock()
	n := len(s.windAsks)
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("la petició s'havia de netejar: %d", n)
	}
}

func TestWindAnswerNeedsAKnownRequest(t *testing.T) {
	s := remindTestServer(t)
	if rec := answerWindAsk(t, s, `{"key":"wa999","approve":true}`); rec.Code != http.StatusNotFound {
		t.Fatalf("clau desconeguda: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/winds/answer", nil)
	rec := httptest.NewRecorder()
	s.handleWindsAnswer(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", rec.Code)
	}
}
