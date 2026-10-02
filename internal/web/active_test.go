package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/config"
)

func TestActiveMostraSessióIEventsEnCurs(t *testing.T) {
	s := goalTestServer(t)
	s.active = &activeAgent{
		ID: 7, Task: "revisa el projecte", StartedAt: time.Unix(100, 0),
		Role: "code", Mode: "code", Project: "tui-agent",
		Events: []liveEvent{{Kind: "activity", Text: "🔧 glob"}},
	}
	rec := httptest.NewRecorder()
	s.handleActive(rec, httptest.NewRequest(http.MethodGet, "/api/active", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("active: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Active *activeAgent `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Active == nil || out.Active.ID != 7 || out.Active.Task != "revisa el projecte" || len(out.Active.Events) != 1 {
		t.Fatalf("activa inesperada: %+v", out.Active)
	}
}

func TestAgentExposaActivaMentreExecuta(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	// El provider es pot cridar més d'un cop (la sonda de /models i el xat)
	// i l'ordre no és cosa del test: `once` evita el pànic de tancar un
	// canal dues vegades, i el select evita quedar-se penjat si la segona
	// crida arriba després que el test hagi deixat anar el release.
	var once sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Fet."}}]}`))
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	done := make(chan struct{})
	go func() {
		s.handleAgent(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"revisa ara"}`)))
		close(done)
	}()
	<-started
	rec := httptest.NewRecorder()
	s.handleActive(rec, httptest.NewRequest(http.MethodGet, "/api/active", nil))
	var out struct {
		Active *activeAgent `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Active == nil || out.Active.Task != "revisa ara" || out.Active.Role != "code" {
		t.Fatalf("active durant execució: %+v", out.Active)
	}
	close(release)
	<-done
	rec = httptest.NewRecorder()
	s.handleActive(rec, httptest.NewRequest(http.MethodGet, "/api/active", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Active != nil {
		t.Fatalf("active s'ha de netejar al final: %+v", out.Active)
	}
}

func TestActiveBuitRetornaNull(t *testing.T) {
	s := goalTestServer(t)
	rec := httptest.NewRecorder()
	s.handleActive(rec, httptest.NewRequest(http.MethodGet, "/api/active", nil))
	var out struct {
		Active *activeAgent `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Active != nil {
		t.Fatalf("no n'hi ha d'haver cap: %+v", out.Active)
	}
}
