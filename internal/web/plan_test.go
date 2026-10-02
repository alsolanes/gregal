package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlePlanValidacio(t *testing.T) {
	s := goalTestServer(t)
	// Tasca buida → 400.
	if rec := postJSON(t, s.handlePlan, `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("buida=%d, volia 400", rec.Code)
	}
	if rec := postJSON(t, s.handlePlan, `{"task":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blancs=%d, volia 400", rec.Code)
	}
	// Ocupat → 409 (agent, job o pla en marxa).
	for _, setup := range []func(){
		func() { s.agentBusy = true },
		func() { s.agentBusy = false; s.jobRunning = true },
		func() { s.jobRunning = false; s.planRunning = true },
	} {
		setup()
		if rec := postJSON(t, s.handlePlan, `{"task":"fes X"}`); rec.Code != http.StatusConflict {
			t.Fatalf("ocupat=%d, volia 409", rec.Code)
		}
	}
	s.planRunning = false
}

func TestHandlePlanMetode(t *testing.T) {
	s := goalTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rec := httptest.NewRecorder()
	s.handlePlan(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET=%d, volia 405", rec.Code)
	}
}

func TestPlanSystemEsReadOnly(t *testing.T) {
	for _, want := range []string{"mode PLA", "NO cridis mai write", "VERIFICACIÓ", "sense preàmbuls"} {
		if !strings.Contains(planSystem, want) {
			t.Fatalf("planSystem sense %q", want)
		}
	}
}

func TestPlanBriefAmbTascaIMemoria(t *testing.T) {
	s := goalTestServer(t)
	b := s.planBrief("arregla el login")
	if !strings.Contains(b, "arregla el login") {
		t.Fatalf("brief sense tasca:\n%s", b)
	}
}
