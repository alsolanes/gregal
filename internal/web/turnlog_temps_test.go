package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// El registre d'un torn d'agent diu quant ha estat dins del model i quant
// executant eines; el que passa entre una cosa i l'altra (revisió, apostes)
// no és de cap dels dos. Els events van en l'ordre que els emet el conductor.
func TestAgentTurnLogSplitsModelAndTools(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	tl := startTurnLog("agent", "code", config.Role{Provider: "p", Model: "m"})
	emit := tl.emitter(func(string, any) {})

	emit("progress", map[string]string{"phase": "model", "state": "waiting"})
	time.Sleep(100 * time.Millisecond)
	emit("speed", map[string]any{"prompt": 10, "completion": 5}) // la crida ha tornat
	time.Sleep(300 * time.Millisecond)                           // revisió automàtica: de ningú
	emit("tool_call", map[string]string{"id": "c1", "name": "bash"})
	emit("progress", map[string]string{"phase": "tools", "state": "running", "tools": "bash"})
	time.Sleep(100 * time.Millisecond)
	emit("tool_result", map[string]string{"id": "c1", "name": "bash"})
	time.Sleep(300 * time.Millisecond) // feina entre passos: de ningú
	emit("progress", map[string]string{"phase": "model", "state": "waiting"})
	time.Sleep(100 * time.Millisecond)
	emit("speed", map[string]any{"prompt": 10, "completion": 5})
	emit("assistant", map[string]string{"text": "fet"})
	tl.finish(nil, false, false, nil)

	r := readTurnLog(t)[0]
	if r.ModelMs < 200 || r.ToolsMs < 100 || r.WaitMs != 0 {
		t.Fatalf("each part must hold its own time: %+v", r)
	}
	// 600 ms del torn no són ni model ni eines.
	if rest := r.TotalMs - r.ModelMs - r.ToolsMs; rest < 600 {
		t.Fatalf("time between the model and the tools leaked into them (rest %d ms): %+v", rest, r)
	}
}

// Una espera que arriba amb les eines en marxa (el permís d'un subagent) és
// espera mentre dura, i després el temps torna a ser de les eines.
func TestAgentTurnLogWaitInsideToolsGoesBackToTools(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	tl := startTurnLog("agent", "code", config.Role{})
	emit := tl.emitter(func(string, any) {})
	emit("progress", map[string]string{"phase": "tools", "state": "running", "tools": "delegate"})
	time.Sleep(100 * time.Millisecond)
	// Dos subagents demanen permís alhora.
	resumeA := turnLogFrom(tl.withUsage(context.Background())).waiting()
	resumeB := tl.waiting()
	time.Sleep(300 * time.Millisecond)
	resumeA()
	resumeB()
	time.Sleep(100 * time.Millisecond)
	emit("tool_result", map[string]string{"id": "d1", "name": "delegate"})
	tl.finish(nil, false, false, nil)

	r := readTurnLog(t)[0]
	if r.WaitMs < 300 || r.ToolsMs < 200 {
		t.Fatalf("wait and tools must each hold their own time: %+v", r)
	}
	if sum := r.ToolsMs + r.WaitMs; sum > r.TotalMs {
		t.Fatalf("the wait was counted twice (%d ms of %d): %+v", sum, r.TotalMs, r)
	}
}

// approveAfter fa de persona que mira la pantalla: quan arriba una petició
// de permís, triga d a aprovar-la. El canal es tanca quan ha contestat.
func approveAfter(s *Server, d time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			s.mu.Lock()
			var ch chan bool
			for _, ar := range s.approvals {
				ch = ar.ch
			}
			s.mu.Unlock()
			if ch != nil {
				time.Sleep(d)
				ch <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	return done
}

// De punta a punta amb el servidor: l'agent vol escriure un fitxer, la
// persona triga a donar el permís, i aquesta espera queda a wait_ms i no al
// model ni a les eines.
func TestAgentTurnLogCountsARealApprovalAsWait(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	var n atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tools []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		body := `{"choices":[{"message":{"content":"Fet."}}]}`
		if len(req.Tools) > 0 && n.Add(1) == 1 {
			body = `{"choices":[{"message":{"content":"Escric.","tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":"{\"path\":\"b.txt\",\"content\":\"hola\"}"}}]}}]}`
		}
		if _, err := fmt.Fprint(w, body); err != nil {
			t.Errorf("provider write: %v", err)
		}
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cwd = t.TempDir()
	s.policy.ProjectDir = s.cwd
	s.policy.AskEdits = true
	s.cfg.Permissions.Access = config.AccessAsk
	approved := approveAfter(s, 500*time.Millisecond)
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"escriu b.txt","mode":"code"}`)))
	<-approved
	if b, err := os.ReadFile(filepath.Join(s.cwd, "b.txt")); err != nil || string(b) != "hola" {
		t.Fatalf("the approved write must have happened: %q %v\n%s", b, err, rec.Body.String())
	}

	var r turnRecord
	for _, x := range readTurnLog(t) {
		if x.Kind == "agent" {
			r = x
		}
	}
	if r.WaitMs < 500 {
		t.Fatalf("the approval wait is missing from wait_ms: %+v", r)
	}
	if r.ModelMs >= r.WaitMs || r.ToolsMs >= r.WaitMs {
		t.Fatalf("the approval wait leaked into model or tools: %+v", r)
	}
	if sum := r.ModelMs + r.ToolsMs + r.WaitMs; sum > r.TotalMs {
		t.Fatalf("parts (%d ms) exceed the total (%d ms): %+v", sum, r.TotalMs, r)
	}
}

// De punta a punta: un torn de debò amb un proveïdor lent i una eina deixa
// el temps del model al registre.
func TestAgentTurnLogTimesARealTurn(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		body := `{"choices":[{"message":{"content":"fet"},"finish_reason":"stop"}]}`
		if calls.Add(1) == 1 {
			body = `{"choices":[{"message":{"tool_calls":[{"id":"g1","type":"function","function":{"name":"glob","arguments":"{\"pattern\":\"*.go\"}"}}]},"finish_reason":"tool_calls"}]}`
		}
		if _, err := fmt.Fprint(w, body); err != nil {
			t.Errorf("provider write: %v", err)
		}
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL}}
	role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}

	tl := startTurnLog("agent", agent.ModeCode, role)
	reply, _, err := s.conduirTorn(tl.withUsage(context.Background()), "inspect", agent.ModeCode, "code", s.cfg.Providers["p"], role, 3, t.TempDir(), nil, tl.emitter(func(string, any) {}))
	tl.finish(err, false, false, nil)
	if err != nil || reply != "fet" || calls.Load() != 2 {
		t.Fatalf("reply=%q calls=%d err=%v", reply, calls.Load(), err)
	}
	r := readTurnLog(t)[0]
	if r.ModelMs < 300 {
		t.Fatalf("two model calls of 150 ms must show in model_ms: %+v", r)
	}
	if sum := r.ModelMs + r.ToolsMs + r.WaitMs; sum > r.TotalMs {
		t.Fatalf("parts (%d ms) exceed the total (%d ms): %+v", sum, r.TotalMs, r)
	}
}
