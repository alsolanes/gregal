package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
)

// El complement d'Office en mode consulta envia mode:"chat": el torn ha
// d'anar amb el prompt de només lectura i les escriptures bloquejades,
// encara que la sessió estigui en code. Un mode que no restringeix
// (code) s'ignora.
func TestAgentModePerTornNomesRestringeix(t *testing.T) {
	var sistemes []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []json.RawMessage `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
			sistemes = append(sistemes, req.Messages[0].Content)
		}
		w.Header().Set("Content-Type", "application/json")
		if len(req.Tools) > 0 && len(sistemes) == 1 {
			// Primer torn: intenta escriure (ha de quedar bloquejat en xat).
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":"{\"path\":\"x.txt\",\"content\":\"hola\"}"}}]}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"Resposta al panell."}}]}`))
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Mode = "code"
	s.mode = "code"
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"resumeix això","mode":"chat"}`)))
	body := rec.Body.String()
	if !strings.Contains(body, "EINA BLOQUEJADA") && !strings.Contains(body, "blocked") {
		t.Fatalf("en mode consulta el write s'havia de bloquejar:\n%s", body)
	}
	if len(sistemes) == 0 || !strings.Contains(sistemes[0], "Mode xat") {
		t.Fatalf("el system prompt havia de ser el de xat: %.200q", sistemes)
	}
	if s.mode != "code" {
		t.Fatal("el mode de la sessió no ha de canviar")
	}

	// mode:"code" en una sessió de xat no amplia res.
	sistemes = nil
	s.mode = "chat"
	rec = httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"una altra cosa","mode":"code"}`)))
	if len(sistemes) == 0 || !strings.Contains(sistemes[0], "Mode xat") {
		t.Fatalf("mode:code no pot ampliar una sessió de xat: %.200q", sistemes)
	}
}
