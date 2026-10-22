package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/session"
)

// TestRotllanaRename: POST amb el mateix id canvia el nom sense tocar
// els membres ni crear-ne una altra.
func TestRotllanaRename(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Tramuntana","color":"#29ADFF","role":"sistema"}`)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)

	rec := postJSON(t, s.handleRotllanes, `{"name":"Vesprada","members":["tramuntana","xaloc"]}`)
	var saved struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}

	rec2 := postJSON(t, s.handleRotllanes, `{"id":"`+saved.Rotllana.ID+`","name":"Nit","members":["tramuntana","xaloc"]}`)
	var renamed struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &renamed); err != nil {
		t.Fatal(err)
	}
	if renamed.Rotllana.ID != saved.Rotllana.ID || renamed.Rotllana.Name != "Nit" {
		t.Fatalf("renom mal fet: %+v", renamed.Rotllana)
	}
	if len(renamed.Rotllana.Members) != 2 {
		t.Fatalf("els membres no s'havien de tocar: %+v", renamed.Rotllana.Members)
	}
	g := httptest.NewRequest(http.MethodGet, "/api/winds/rotllanes", nil)
	b := httptest.NewRecorder()
	s.handleRotllanes(b, g)
	if strings.Contains(b.Body.String(), "Vesprada") || !strings.Contains(b.Body.String(), "Nit") {
		t.Fatalf("llistat: %s", b.Body.String())
	}
}

// TestDeleteRotllanaCleansHistory: esborrar la rotllana també neteja la
// conversa del disc i de memòria (si no, tornaria amb el mateix id).
func TestDeleteRotllanaCleansHistory(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Tramuntana","color":"#29ADFF","role":"sistema"}`)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)
	rec := postJSON(t, s.handleRotllanes, `{"name":"Vesprada","members":["tramuntana","xaloc"]}`)
	var saved struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	id := saved.Rotllana.ID

	// Conversa prèvia.
	hist := rotllanaHistory(s, id)
	hist.mu.Lock()
	hist.messages = append(hist.messages, llm.Message{Role: "user", Content: "hola"})
	hist.mu.Unlock()
	if err := saveRotllanaHistory(s, id, hist.messages); err != nil {
		t.Fatal(err)
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/winds/rotllanes", strings.NewReader(`{"id":"`+id+`"}`))
	db := httptest.NewRecorder()
	s.handleRotllanes(db, del)
	if db.Code != 200 {
		t.Fatalf("delete: %d %s", db.Code, db.Body.String())
	}
	if _, ok := s.rotllanaByID(id); ok {
		t.Fatal("la rotllana encara existeix")
	}
	if got := len(rotllanaHistory(s, id).messages); got != 0 {
		t.Fatalf("memòria amb %d missatges", got)
	}
	if saved2, err := session.Load(s.sessDir(), "rotllana-"+id); err == nil && len(saved2.Convo) > 0 {
		t.Fatalf("l'historial orfe encara és al disc: %d missatges", len(saved2.Convo))
	}
}

// TestClearWindChat: DELETE /api/winds/chat buida la conversa (disc +
// memòria) però el vent continua existint.
func TestClearWindChat(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#29ADFF","role":"sistema"}`)
	wind, ok := s.winds.ByID("llevant")
	if !ok {
		t.Fatal("el vent no s'ha creat")
	}

	hist := windHistory(s, "llevant")
	hist.mu.Lock()
	hist.messages = append(hist.messages,
		llm.Message{Role: "user", Content: "hola"},
		llm.Message{Role: "assistant", Content: "què tal"},
	)
	hist.mu.Unlock()
	if err := saveWindHistory(s, wind, hist.messages); err != nil {
		t.Fatal(err)
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/winds/chat", strings.NewReader(`{"wind":"llevant"}`))
	db := httptest.NewRecorder()
	s.handleWindChat(db, del)
	if db.Code != 200 {
		t.Fatalf("clear: %d %s", db.Code, db.Body.String())
	}
	if got := len(windHistory(s, "llevant").messages); got != 0 {
		t.Fatalf("memòria amb %d missatges", got)
	}

	// El vent hi continua sent i el xat torna buit.
	get := httptest.NewRequest(http.MethodGet, "/api/winds/chat?wind=llevant", nil)
	gb := httptest.NewRecorder()
	s.handleWindChat(gb, get)
	if gb.Code != 200 || strings.Contains(gb.Body.String(), "què tal") {
		t.Fatalf("get: %d %s", gb.Code, gb.Body.String())
	}

	// Vent inexistent: 400, no 500.
	bad := httptest.NewRequest(http.MethodDelete, "/api/winds/chat", strings.NewReader(`{"wind":"fantasma"}`))
	bb := httptest.NewRecorder()
	s.handleWindChat(bb, bad)
	if bb.Code != 400 {
		t.Fatalf("fantasma: %d", bb.Code)
	}
}
