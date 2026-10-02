package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/procs"
)

// Engegar, llegir i aturar un procés per l'API.
func TestExecCicle(t *testing.T) {
	s, _ := fileTestServer(t)
	w := httptest.NewRecorder()
	s.handleExec(w, httptest.NewRequest("POST", "/api/exec", strings.NewReader(`{"cmd":"echo hola-api"}`)))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("sense id: %v", out)
	}
	p := agent.Procs().Get(id)
	if p == nil || !p.Wait(5*time.Second) {
		t.Fatal("el procés no ha acabat")
	}

	w2 := httptest.NewRecorder()
	s.handleExecOutput(w2, httptest.NewRequest("GET", "/api/exec/output?id="+id, nil))
	var res struct {
		Lines []procs.Line `json:"lines"`
		Done  bool         `json:"done"`
	}
	json.Unmarshal(w2.Body.Bytes(), &res)
	if !res.Done || len(res.Lines) != 1 || res.Lines[0].Text != "hola-api" {
		t.Fatalf("%+v", res)
	}

	w3 := httptest.NewRecorder()
	s.handleExec(w3, httptest.NewRequest("GET", "/api/exec", nil))
	if !strings.Contains(w3.Body.String(), id) {
		t.Fatalf("la llista no té el procés: %s", w3.Body.String())
	}
}

// El terminal no és una porta del darrere: el que l'agent té denegat,
// tampoc s'engega des d'aquí.
func TestExecRespectaPolitica(t *testing.T) {
	s, _ := fileTestServer(t)
	w := httptest.NewRecorder()
	s.handleExec(w, httptest.NewRequest("POST", "/api/exec", strings.NewReader(`{"cmd":"sudo rm -rf /"}`)))
	if w.Code != 403 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

// Aturar un procés viu i un id inventat.
func TestExecKill(t *testing.T) {
	s, _ := fileTestServer(t)
	w := httptest.NewRecorder()
	s.handleExec(w, httptest.NewRequest("POST", "/api/exec", strings.NewReader(`{"cmd":"sleep 20"}`)))
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	id := out["id"].(string)
	w2 := httptest.NewRecorder()
	s.handleExecKill(w2, httptest.NewRequest("POST", "/api/exec/kill", strings.NewReader(`{"id":"`+id+`"}`)))
	if w2.Code != 200 {
		t.Fatalf("%d", w2.Code)
	}
	if agent.Procs().Get(id).IsRunning() {
		t.Fatal("hauria d'estar mort")
	}
	w3 := httptest.NewRecorder()
	s.handleExecKill(w3, httptest.NewRequest("POST", "/api/exec/kill", strings.NewReader(`{"id":"cap"}`)))
	if w3.Code != 404 {
		t.Fatalf("%d", w3.Code)
	}
}

// L'stream SSE dona les línies i tanca amb end.
func TestExecStream(t *testing.T) {
	s, _ := fileTestServer(t)
	p, err := agent.Procs().Start(s.id, t.TempDir(), "echo a; echo b")
	if err != nil {
		t.Fatal(err)
	}
	p.Wait(5 * time.Second)
	w := httptest.NewRecorder()
	s.handleExecStream(w, httptest.NewRequest("GET", "/api/exec/stream?id="+p.ID, nil))
	body := w.Body.String()
	if !strings.Contains(body, "event: line") || !strings.Contains(body, `"text":"a"`) || !strings.Contains(body, "event: end") {
		t.Fatalf("stream: %s", body)
	}
}

// Tancar la sessió s'endú els seus processos.
func TestTancarSessioMataProcessos(t *testing.T) {
	h := hubTestServer(t).Hub()
	s := h.Session("feina")
	p, _ := agent.Procs().Start(s.id, t.TempDir(), "sleep 20")
	if !h.Close("feina") {
		t.Fatal("no s'ha tancat")
	}
	p.Wait(3 * time.Second)
	if p.IsRunning() {
		t.Fatal("el procés hauria d'haver mort amb la sessió")
	}
}
