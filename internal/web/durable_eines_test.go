package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
	"gregal/internal/runs"
)

// La web fa servir la cua v2 (registre durable) quan el servidor l'anuncia.
// Si el registre desa les crides d'eina com a "activity" genèric, el client
// (app/runs.js) no les reconeix: cap targeta d'eina, la bombolla viva no es
// tanca entre passos i la resposta final acaba a la primera bombolla.
func TestV2RegistreDurableConservaLesEines(t *testing.T) {
	var n atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if n.Add(1) == 1 {
			w.Write([]byte(`{"choices":[{"message":{"content":"Miro el projecte.","tool_calls":[{"id":"c1","type":"function","function":{"name":"glob","arguments":"{\"pattern\":\"*.go\"}"}}]}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"Resposta final."}}]}`))
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	// El registre és del paquet (hi ha events de proves anteriors): només
	// mirem el que ve després d'ara.
	base := s.Hub().eventStore.Cursor()
	rec := v2Do(t, s, http.MethodPost, "/api/v2/runs", `{"task":"mira els fitxers go"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v2/runs: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Run runs.Run `json:"run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	waitRunState(t, s, out.Run.ID, runs.Completed)

	got, err := s.Hub().eventStore.After(base, 500)
	if err != nil {
		t.Fatal(err)
	}
	var ordre []string
	for _, e := range got {
		if e.RunID != int(out.Run.ID) {
			continue
		}
		switch e.Kind {
		case "thinking", "tool_call", "tool_result", "text", "done":
			ordre = append(ordre, e.Kind)
		}
		if e.Kind == "tool_call" {
			var p map[string]string
			if err := json.Unmarshal(e.Payload, &p); err != nil || p["name"] != "glob" || !strings.Contains(p["args"], "*.go") {
				t.Fatalf("tool_call sense nom/args al payload: %s (%v)", e.Payload, err)
			}
		}
		if e.Kind == "thinking" {
			var p map[string]string
			if err := json.Unmarshal(e.Payload, &p); err != nil || p["text"] != "Miro el projecte." {
				t.Fatalf("thinking sense text sencer al payload: %s (%v)", e.Payload, err)
			}
		}
	}
	want := "thinking tool_call tool_result text done"
	if strings.Join(ordre, " ") != want {
		t.Fatalf("ordre d'events durables = %q, volia %q", strings.Join(ordre, " "), want)
	}
}
