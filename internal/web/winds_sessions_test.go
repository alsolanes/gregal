package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// makeTestWind crea un vent de prova i el torna.
func makeTestWind(t *testing.T, s *Server, name string) companion.Wind {
	t.Helper()
	rec := postJSON(t, s.handleWinds, `{"name":"`+name+`","color":"#29ADFF","role":"sistema"}`)
	var saved struct {
		Wind companion.Wind `json:"wind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Wind.ID == "" {
		t.Fatalf("vent no creat: %s", rec.Body.String())
	}
	return saved.Wind
}

// TestWindSessionsLifecycle: crear, llistar, obrir i esborrar sessions
// d'un vent sense perdre la conversa històrica.
func TestWindSessionsLifecycle(t *testing.T) {
	s, _ := companionTestServer(t)
	// Un vent de prova amb conversa històrica.
	wind := makeTestWind(t, s, "Marinada")
	send := func(method, path, body string) map[string]any {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.handleWindSessions(rec, req)
		if rec.Code/100 != 2 {
			t.Fatalf("%s %s: codi %d: %s", method, path, rec.Code, rec.Body.String())
		}
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	// La històrica existeix perquè el vent ha xerrat abans (desem directament).
	if err := saveWindHistory(s, wind, []llm.Message{{Role: "user", Content: "hola"}}); err != nil {
		t.Fatal(err)
	}
	// POST crea una sessió nova.
	created := send("POST", "/api/winds/sessions", `{"wind":"`+wind.ID+`"}`)
	name, _ := created["name"].(string)
	if name == "" || !strings.Contains(name, "-s") {
		t.Fatalf("sessió nova: nom inesperat %q", name)
	}
	// La nova sessió és l'activa i surt a la llista com a Current.
	if got := s.windSessionName(wind.ID); got != name {
		t.Fatalf("windSessionName = %q, want %q", got, name)
	}
	list := send("GET", "/api/winds/sessions?wind="+wind.ID, "")
	sessions, _ := list["sessions"].([]any)
	if len(sessions) != 2 {
		t.Fatalf("esperava 2 sessions (històrica + nova), got %d", len(sessions))
	}
	first := sessions[0].(map[string]any)
	if first["name"] != name || first["current"] != true {
		t.Fatalf("la sessió activa no és la primera: %+v", first)
	}
	// La històrica no existeix encara: crear-la desant una conversa.
	// (Es desa via el xat del vent; aquí desem directament.)
	if err := saveWindHistory(s, wind, []llm.Message{{Role: "user", Content: "hola"}}); err != nil {
		t.Fatal(err)
	}
	list = send("GET", "/api/winds/sessions?wind="+wind.ID, "")
	sessions, _ = list["sessions"].([]any)
	if len(sessions) != 2 {
		t.Fatalf("esperava 2 sessions (nova + històrica), got %d", len(sessions))
	}
	// DELETE de la nova: no es pot (és l'activa).
	req := httptest.NewRequest("DELETE", "/api/winds/sessions", strings.NewReader(`{"wind":"`+wind.ID+`","name":"`+name+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleWindSessions(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("esborrar l'activa: codi %d, want 409", rec.Code)
	}
	// Obrir la històrica: passa a ser l'activa.
	reqOpen := httptest.NewRequest("POST", "/api/winds/session/open", strings.NewReader(`{"wind":"`+wind.ID+`","name":"company-`+wind.ID+`"}`))
	reqOpen.Header.Set("Content-Type", "application/json")
	recOpen := httptest.NewRecorder()
	s.handleWindSessionOpen(recOpen, reqOpen)
	if recOpen.Code != 200 {
		t.Fatalf("obrir històrica: codi %d: %s", recOpen.Code, recOpen.Body.String())
	}
	var opened map[string]any
	json.Unmarshal(recOpen.Body.Bytes(), &opened)
	if opened["name"] != "company-"+wind.ID {
		t.Fatalf("obrir històrica: %+v", opened)
	}
	if got := s.windSessionName(wind.ID); got != "company-"+wind.ID {
		t.Fatalf("windSessionName després d'obrir = %q", got)
	}
	// Ara la nova es pot esborrar.
	send("DELETE", "/api/winds/sessions", `{"wind":"`+wind.ID+`","name":"`+name+`"}`)
	list = send("GET", "/api/winds/sessions?wind="+wind.ID, "")
	sessions, _ = list["sessions"].([]any)
	if len(sessions) != 1 {
		t.Fatalf("després d'esborrar: esperava 1, got %d", len(sessions))
	}
}

// TestWindSessionsIsolation: dues sessions d'un vent no comparteixen
// historial; el xat sempre escriu a l'activa.
func TestWindSessionsIsolation(t *testing.T) {
	s, _ := companionTestServer(t)
	wind := makeTestWind(t, s, "Ponent")
	// La històrica es desa amb un missatge.
	if err := saveWindHistory(s, wind, []llm.Message{{Role: "user", Content: "conversa vella"}}); err != nil {
		t.Fatal(err)
	}
	// Carregada en memòria: és la històrica.
	hist := windHistory(s, wind.ID)
	if len(hist.messages) != 1 || hist.messages[0].Content != "conversa vella" {
		t.Fatalf("històrica carregada: %+v", hist.messages)
	}
	// Nova sessió: la memòria s'invalida i la carregada és la nova (buida).
	s.mu.Lock()
	s.windActiveSession[wind.ID] = "company-" + wind.ID + "-s1000"
	s.mu.Unlock()
	windHistories.Delete(historyKey(s, "wind", wind.ID))
	hist2 := windHistory(s, wind.ID)
	if len(hist2.messages) != 0 {
		t.Fatalf("sessió nova ha de començar buida, got %d missatges", len(hist2.messages))
	}
	// Desar escriu a la sessió activa, no a la històrica.
	if err := saveWindHistory(s, wind, []llm.Message{{Role: "user", Content: "conversa nova"}}); err != nil {
		t.Fatal(err)
	}
	list := s.windSessionList(wind.ID)
	if len(list) != 2 {
		t.Fatalf("esperava 2 sessions, got %d", len(list))
	}
	for _, row := range list {
		if row.Name == "company-"+wind.ID && row.Msgs != 1 {
			t.Fatalf("la històrica ha de tenir 1 missatge, got %d", row.Msgs)
		}
		if row.Name == "company-"+wind.ID+"-s1000" && row.Msgs != 1 {
			t.Fatalf("la nova ha de tenir 1 missatge, got %d", row.Msgs)
		}
	}
}
