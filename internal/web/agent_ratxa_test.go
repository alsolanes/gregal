package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
)

// La web té el seu bucle propi i no heretava el topall contra el «corre el
// test, retoca, corre el test» del motor: amb la suite vermella, cada
// ampliació es concedia i el torn podia arribar a 25 trossos. Aquí el model
// només demana comprovacions que fallen i diria CONTINUA si se li
// preguntés: no se li ha de preguntar, i la síntesi ha de ser la llarga.
func TestAgentWebRatxaVermellaNoAmplia(t *testing.T) {
	var preguntes, llargues atomic.Int32
	var n atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Tools    []json.RawMessage `json:"tools"`
			Messages []struct {
				Content any `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("petició provider: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if len(req.Tools) == 0 {
			ultim := ""
			if len(req.Messages) > 0 {
				ultim = fmt.Sprint(req.Messages[len(req.Messages)-1].Content)
			}
			switch {
			case strings.Contains(ultim, "pressupost"):
				llargues.Add(1)
				w.Write([]byte(`{"choices":[{"message":{"content":"SINTESI-LLARGA"}}]}`))
			case strings.Contains(ultim, "Les accions ja han acabat"):
				w.Write([]byte(`{"choices":[{"message":{"content":"SINTESI-CURTA"}}]}`))
			default:
				preguntes.Add(1)
				w.Write([]byte(`{"choices":[{"message":{"content":"CONTINUA 20"}}]}`))
			}
			return
		}
		i := n.Add(1)
		args := fmt.Sprintf(`{\"command\":\"go vet ./no-existeix-%d\"}`, i)
		w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"t` + fmt.Sprint(i) + `","type":"function","function":{"name":"bash","arguments":"` + args + `"}}]}}]}`))
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Agent.MaxSteps = 3
	s.permissive = true
	s.cwd = t.TempDir()
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"afegeix una funció i verifica-ho","mode":"code"}`)))

	body := rec.Body.String()
	if got := preguntes.Load(); got != 0 {
		t.Fatalf("amb tres comprovacions vermelles seguides no es pregunta si ampliar (%d preguntes)", got)
	}
	if llargues.Load() != 1 || !strings.Contains(body, "SINTESI-LLARGA") {
		t.Fatalf("la síntesi d'una ratxa vermella és la llarga:\n%s", body)
	}
	if !strings.Contains(body, "Comprovació vermella repetida") {
		t.Fatalf("la guia de ratxa s'ha d'avisar a la pantalla:\n%s", body)
	}
}
