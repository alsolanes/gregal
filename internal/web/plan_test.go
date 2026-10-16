package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
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
	for _, want := range []string{"PLAN MODE", "NEVER call write", "VERIFICATION", "no preamble"} {
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

// La tirada de 2026-10-16: en mode Pla, «revisa a fons el projecte» va
// tornar només «Plan · error: límit de 10 passos exhaurit». Amb un model
// que no para mai d'explorar, /api/plan ha de tornar un pla parcial (no un
// 502), sense executar res a la crida final, i s'ha de poder continuar.
func TestHandlePlanTossutTornaPlaParcialIContinua(t *testing.T) {
	work := t.TempDir()
	sentinella := filepath.Join(work, "NO_ESCRIT.txt")
	var ambEines, senseEines atomic.Int32
	var continuaVist atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Tools    []json.RawMessage `json:"tools"`
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("petició provider: %v", err)
			return
		}
		for _, m := range req.Messages {
			if strings.Contains(fmt.Sprint(m.Content), agent.PromptContinuaExploracio) {
				continuaVist.Store(true)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		write, _ := json.Marshal(map[string]string{"path": sentinella, "content": "NO"})
		writeArg, _ := json.Marshal(string(write))
		if len(req.Tools) == 0 {
			senseEines.Add(1)
			fmt.Fprintf(w, `{"choices":[{"message":{"content":"1. Revisar loop.go\nPENDENT: web/","tool_calls":[{"id":"w","type":"function","function":{"name":"write","arguments":%s}}]}}]}`, writeArg)
			return
		}
		n := ambEines.Add(1)
		args, _ := json.Marshal(map[string]string{"pattern": fmt.Sprintf("*%d.go", n), "dir": work})
		wire, _ := json.Marshal(string(args))
		fmt.Fprintf(w, `{"choices":[{"message":{"content":"","tool_calls":[{"id":"g%d","type":"function","function":{"name":"glob","arguments":%s}}]}}]}`, n, wire)
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Agent.MaxSteps = 10
	s.cfg.Plan.Critic = "off"
	s.cwd = work
	rec := postJSON(t, s.handlePlan, `{"task":"revisa a fons el projecte buscant punts febles"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s: quedar-se sense passos no és un error", rec.Code, rec.Body.String())
	}
	var out struct {
		Plan     string `json:"plan"`
		Partial  bool   `json:"partial"`
		Reason   string `json:"partial_reason"`
		MaxSteps int    `json:"max_steps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Partial || out.Reason != agent.MotiuPassos || !strings.Contains(out.Plan, "Revisar loop.go") {
		t.Fatalf("pla parcial esperat: %+v", out)
	}
	if got := ambEines.Load(); got != int32(agent.PassosMinimsPla) || out.MaxSteps != agent.PassosMinimsPla {
		t.Fatalf("passos=%d max=%d: el pla té com a mínim %d passos", got, out.MaxSteps, agent.PassosMinimsPla)
	}
	if senseEines.Load() == 0 {
		t.Fatal("cal la crida final sense eines")
	}
	if _, err := os.Stat(sentinella); err == nil {
		t.Fatal("la crida final ha executat una eina")
	}

	rec = postJSON(t, s.handlePlan, `{"continue":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("continuar: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !continuaVist.Load() {
		t.Fatal("continuar ha de reprendre la mateixa conversa amb la consigna de continuar")
	}
	if got := ambEines.Load(); got != int32(2*agent.PassosMinimsPla) {
		t.Fatalf("continuar dona un pressupost nou: passos=%d", got)
	}
}

func TestHandlePlanContinuaSenseExploracio(t *testing.T) {
	s := goalTestServer(t)
	if rec := postJSON(t, s.handlePlan, `{"continue":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("continuar sense res=%d, volia 409", rec.Code)
	}
}
