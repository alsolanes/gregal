package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
)

// Reprodueix el cas real: el model usa totes les eines permeses i no deixa
// contingut final al darrer pas. L'agent ha de fer una síntesi sense eines.
func TestAgentSintetitzaRespostaDesprésDelLimitDines(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			// La sonda de finestres (GET /models) no és una petició de xat.
			http.NotFound(w, r)
			return
		}
		var req struct {
			Tools []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("petició provider: %v", err)
		}
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if len(req.Tools) == 0 {
			w.Write([]byte(`{"choices":[{"message":{"content":"Resum final verificat."}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"tool-` + string(rune('0'+n)) + `","type":"function","function":{"name":"glob","arguments":"{\"pattern\":\"*.go\",\"dir\":\"/tmp\"}"}}]}}]}`))
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Agent.MaxSteps = 2
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"inspecciona"}`)))

	body := rec.Body.String()
	if !strings.Contains(body, `event: assistant`) || !strings.Contains(body, `Resum final verificat.`) {
		t.Fatalf("falta la síntesi final SSE:\n%s", body)
	}
	if !strings.Contains(body, `"reply":"Resum final verificat."`) {
		t.Fatalf("done sense resposta final:\n%s", body)
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("calls=%d, esperava 2 passos d'eines + 1 pregunta d'ampliació (FINAL) + 1 síntesi", got)
	}
}
