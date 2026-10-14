package web

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// El mode auto-review és exclusiu amb el permissiu i surt a /api/state
// juntament amb l'estat del sandbox.
func TestAutoReviewToggleIState(t *testing.T) {
	s := goalTestServer(t)
	post := func(body string) map[string]bool {
		req := httptest.NewRequest("POST", "/api/permissive", bytes.NewReader([]byte(body)))
		w := httptest.NewRecorder()
		s.handlePermissive(w, req)
		var out map[string]bool
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	post(`{"on":true}`)
	if r := post(`{"on":true,"review":true}`); !r["review"] || r["on"] {
		t.Fatalf("review ha d'apagar el permissiu: %v", r)
	}
	if !s.autoReviewNow() || s.permissiveNow() {
		t.Fatal("estat del servidor incorrecte")
	}
	req := httptest.NewRequest("GET", "/api/state", nil)
	w := httptest.NewRecorder()
	s.handleState(w, req)
	var st map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st["auto_review"] != true {
		t.Fatalf("auto_review: %v", st["auto_review"])
	}
	sb, ok := st["sandbox"].(map[string]any)
	if !ok || sb["mode"] != "off" || sb["backend"] != "none" {
		t.Fatalf("sandbox: %v", st["sandbox"])
	}
	if r := post(`{"on":false}`); r["review"] || r["on"] {
		t.Fatalf("manual apaga tots dos: %v", r)
	}
}
