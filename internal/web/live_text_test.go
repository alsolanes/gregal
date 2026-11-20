package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/runs"
)

// sseData escriu un event del flux del proveïdor de mentida.
func sseData(t *testing.T, w http.ResponseWriter, data string) {
	if _, err := fmt.Fprint(w, "data: "+data+"\n\n"); err != nil {
		t.Errorf("provider write: %v", err)
	}
}

// En un torn d'agent el text ha de sortir mentre el model l'escriu: abans
// es retenia fins al final de la crida i la resposta arribava de cop.
func TestAgentTurnEmitsTextWhileTheModelIsStillWriting(t *testing.T) {
	release := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseData(t, w, `{"choices":[{"delta":{"content":"Primer tros. "}}]}`)
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		sseData(t, w, `{"choices":[{"delta":{"content":"Segon tros."},"finish_reason":"stop"}]}`)
	}))
	released := false
	free := func() {
		if !released {
			released = true
			close(release)
		}
	}
	t.Cleanup(func() { free(); provider.Close() })

	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"explica"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v2/runs: %d %s", rec.Code, rec.Body.String())
	}
	id := runIDFrom(t, rec.Body.String())

	// El proveïdor encara té la crida oberta: el primer tros ja ha de ser al registre.
	live := func() string {
		for _, e := range storeKinds(t, s)["token"] {
			if int64(e.RunID) == id {
				return e.Text
			}
		}
		return ""
	}
	deadline := time.Now().Add(10 * time.Second)
	for live() == "" {
		if time.Now().After(deadline) {
			t.Fatalf("no token event while the model call is open; kinds: %v", keys(storeKinds(t, s)))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := live(); got != "Primer tros. " {
		t.Fatalf("first live token = %q", got)
	}
	free()
	waitRunState(t, s, id, runs.Completed)
}

// Un flux que es talla a mig text es torna a demanar: el torn avisa amb
// steer_restart perquè la pantalla retiri el text de l'intent descartat, i
// aquest text no queda ni a la resposta ni a la conversa.
func TestAgentTurnRestartsLiveTextWhenTheStreamIsInterrupted(t *testing.T) {
	var attempts atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if attempts.Add(1) == 1 {
			sseData(t, w, `{"choices":[{"delta":{"content":"TEXT DESCARTAT"}}]}`)
			return
		}
		sseData(t, w, `{"choices":[{"delta":{"content":"Resposta bona."},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(provider.Close)

	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"explica"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v2/runs: %d %s", rec.Code, rec.Body.String())
	}
	id := runIDFrom(t, rec.Body.String())
	waitRunState(t, s, id, runs.Completed)

	got, err := s.Hub().eventStore.After(0, 500)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, e := range got {
		if int64(e.RunID) != id {
			continue
		}
		switch e.Kind {
		case "token":
			order = append(order, "token:"+e.Text)
		case "steer_restart":
			order = append(order, "restart")
		case "text":
			order = append(order, "text:"+e.Text)
		}
	}
	if want := "token:TEXT DESCARTAT|restart|token:Resposta bona.|text:Resposta bona."; strings.Join(order, "|") != want {
		t.Fatalf("events:\n got %s\nwant %s", strings.Join(order, "|"), want)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.convo {
		if strings.Contains(m.Content, "DESCARTAT") {
			t.Fatalf("discarded attempt reached the conversation: %+v", m)
		}
	}
	if n := len(s.convo); n == 0 || s.convo[n-1].Content != "Resposta bona." {
		t.Fatalf("conversation does not end with the good answer: %+v", s.convo)
	}
}

// Recuperar un torn que va morir a mig fer no ha de ressuscitar el text
// d'un intent que el mateix torn ja havia descartat.
func TestRecoverRunMessagesDropsTextBeforeARestart(t *testing.T) {
	msgs := recoverRunMessages([]events.Event{
		{Kind: "token", Text: "TEXT DESCARTAT"},
		{Kind: "steer_restart", Text: "{}"},
		{Kind: "token", Text: "Text bo a mitges"},
	}, "failed")
	var all []string
	for _, m := range msgs {
		all = append(all, m.Content)
	}
	joined := strings.Join(all, "|")
	if strings.Contains(joined, "DESCARTAT") || !strings.Contains(joined, "Text bo a mitges") {
		t.Fatalf("recovered messages = %q", joined)
	}
}
