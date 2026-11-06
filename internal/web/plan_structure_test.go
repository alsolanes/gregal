package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// El Pla envia les regles al system i la tasca a part, només amb eines de
// lectura, amb un pressupost de sortida que no talla el pla, i torna la tasca
// perquè «Executa el pla» la pugui enviar junt amb el pla.
func TestPlanSendsSystemTaskAndReadOnlyTools(t *testing.T) {
	var mu sync.Mutex
	var first struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
		MaxTokens int `json:"max_tokens"`
	}
	seen := false
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if !seen {
			seen = true
			_ = json.NewDecoder(r.Body).Decode(&first)
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"1. Canvia a.go\nVERIFICATION: go test"}}]}`)
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Plan.Critic = "off"
	s.cwd = t.TempDir()
	rec := postJSON(t, s.handlePlan, `{"task":"afegeix descomptes a la cistella"}`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Plan string `json:"plan"`
		Task string `json:"task"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Task != "afegeix descomptes a la cistella" || !strings.Contains(out.Plan, "VERIFICATION") {
		t.Fatalf("resposta: %+v", out)
	}
	if len(first.Messages) != 2 || first.Messages[0].Role != "system" || first.Messages[1].Role != "user" ||
		fmt.Sprint(first.Messages[1].Content) != "afegeix descomptes a la cistella" ||
		!strings.Contains(fmt.Sprint(first.Messages[0].Content), "PLAN MODE") {
		t.Fatalf("missatges: %+v", first.Messages)
	}
	if len(first.Tools) == 0 {
		t.Fatal("el pla explora amb eines")
	}
	for _, tool := range first.Tools {
		if !agent.IsReadOnlyTool(tool.Function.Name) {
			t.Fatalf("eina que no és de lectura al Pla: %s", tool.Function.Name)
		}
	}
	if first.MaxTokens != 0 && first.MaxTokens <= 1024 {
		t.Fatalf("pressupost de sortida massa curt: %d", first.MaxTokens)
	}
	if strings.Contains(planNotRun, "wind") || !strings.Contains(planNotRun, "PLAN MODE") {
		t.Fatalf("missatge de bloqueig: %s", planNotRun)
	}
}
