package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
)

func TestParallelAPIExecutaRolsConcurrentment(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\"resposta " + body.Model + "\"}}]}"))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{
		"chat":     {Provider: "local", Model: "chat"},
		"think":    {Provider: "local", Model: "think"},
		"reviewer": {Provider: "local", Model: "review"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/parallel", strings.NewReader(`{"message":"compara"}`))
	rec := httptest.NewRecorder()
	started := time.Now()
	s.handleParallel(rec, req)
	elapsed := time.Since(started)
	if rec.Code != http.StatusOK || elapsed > 500*time.Millisecond {
		t.Fatalf("parallel code=%d temps=%v body=%s", rec.Code, elapsed, rec.Body.String())
	}
	var out struct {
		Results []parallelWebResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Results) != 3 {
		t.Fatalf("resultats: %+v err=%v", out, err)
	}
}
