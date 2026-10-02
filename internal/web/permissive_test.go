package web

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPermissiveToggle(t *testing.T) {
	s := goalTestServer(t)
	if s.permissiveNow() {
		t.Fatal("per defecte ha d'estar desactivat")
	}
	// GET.
	req := httptest.NewRequest("GET", "/api/permissive", nil)
	w := httptest.NewRecorder()
	s.handlePermissive(w, req)
	var g map[string]bool
	json.Unmarshal(w.Body.Bytes(), &g)
	if g["on"] {
		t.Fatal("GET diu on sense activar")
	}
	// POST on.
	req = httptest.NewRequest("POST", "/api/permissive", bytes.NewReader([]byte(`{"on":true}`)))
	w = httptest.NewRecorder()
	s.handlePermissive(w, req)
	if w.Code != 200 || !s.permissiveNow() {
		t.Fatalf("POST on: code=%d perm=%v", w.Code, s.permissiveNow())
	}
	// POST off.
	req = httptest.NewRequest("POST", "/api/permissive", bytes.NewReader([]byte(`{"on":false}`)))
	w = httptest.NewRecorder()
	s.handlePermissive(w, req)
	if s.permissiveNow() {
		t.Fatal("POST off no desactiva")
	}
	// POST il·legible.
	req = httptest.NewRequest("POST", "/api/permissive", bytes.NewReader([]byte(`{`)))
	w = httptest.NewRecorder()
	s.handlePermissive(w, req)
	if w.Code != 400 {
		t.Fatalf("json trencat hauria de ser 400: %d", w.Code)
	}
}

// /api/state no es pot penjar (regressió: llegir permissive sota s.mu).
func TestStateNoPenja(t *testing.T) {
	s := goalTestServer(t)
	s.permissive = true
	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	done := make(chan int, 1)
	go func() { s.handleState(w, req); done <- w.Code }()
	select {
	case code := <-done:
		if code != 200 {
			t.Fatalf("state=%d", code)
		}
		var st map[string]any
		json.Unmarshal(w.Body.Bytes(), &st)
		if st["permissive"] != true {
			t.Fatalf("state sense permissive: %v", st["permissive"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handleState penjat (deadlock?)")
	}
}
