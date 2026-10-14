package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/runs"
)

// Un missatge amb «interromp» mentre el model encara pensa: la crida es
// talla, el pas es refà i la crida següent ja porta el missatge.
func TestSteerInterruptRedoesTheModelStepWithTheMessage(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	first := make(chan struct{})
	var once sync.Once
	done := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		n := len(bodies)
		mu.Unlock()
		if n == 1 {
			once.Do(func() { close(first) })
			// Model lent: només surt quan el client talla la crida.
			select {
			case <-r.Context().Done():
			case <-done:
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"fet en Python"}}]}`))
	}))
	t.Cleanup(func() { close(done); provider.Close() })

	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	if rec := v2Do(t, s, http.MethodPost, "/api/steer", `{"text":"hola"}`); rec.Code != http.StatusConflict {
		t.Fatalf("no turn running must answer 409, got %d", rec.Code)
	}
	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"escriu un script"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v2/runs: %d %s", rec.Code, rec.Body.String())
	}
	id := runIDFrom(t, rec.Body.String())
	select {
	case <-first:
	case <-time.After(10 * time.Second):
		t.Fatal("the model was never called")
	}
	if rec := v2Do(t, s, http.MethodPost, "/api/steer", `{"text":"fes-ho en Python","interrupt":true}`); rec.Code != http.StatusOK {
		t.Fatalf("steer while working: %d %s", rec.Code, rec.Body.String())
	}
	waitRunState(t, s, id, runs.Completed)
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) < 2 || !strings.Contains(bodies[len(bodies)-1], "fes-ho en Python") {
		t.Fatalf("the redone model call must carry the message (%d calls)", len(bodies))
	}
	kinds := storeKinds(t, s)
	if len(kinds["steer"]) == 0 || len(kinds["steer_restart"]) == 0 {
		t.Fatalf("expected steer and steer_restart events, got %v", keys(kinds))
	}
}

func runIDFrom(t *testing.T, body string) int64 {
	t.Helper()
	i := strings.Index(body, `"id":`)
	if i < 0 {
		t.Fatalf("no run id in %s", body)
	}
	var n int64
	for _, c := range body[i+5:] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// Un torn aturat deixa una nota a la conversa: el torn següent no l'ha de
// continuar com si fos una petició pendent.
func TestStoppedTurnIsClosedInTheConversation(t *testing.T) {
	provider, entrada, _ := v2Provider(t, "no hauria d'arribar")
	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"explica tots els fitxers"}`)
	id := runIDFrom(t, rec.Body.String())
	select {
	case <-entrada:
	case <-time.After(10 * time.Second):
		t.Fatal("the model was never called")
	}
	if rec := v2Do(t, s, http.MethodPost, "/api/v2/runs/"+strconv.FormatInt(id, 10)+"/cancel", ``); rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	waitRunState(t, s, id, runs.Cancelled)
	s.mu.Lock()
	defer s.mu.Unlock()
	last := s.convo[len(s.convo)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "Stopped by the user") {
		t.Fatalf("the stopped request must be closed in the conversation, last=%s %q", last.Role, last.Content)
	}
}
