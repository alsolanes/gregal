package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/config"
)

func TestParallelAPIExecutaRolsConcurrentment(t *testing.T) {
	arrived := make(chan string, 3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		arrived <- body.Model
		select {
		case <-release:
		case <-time.After(3 * time.Second):
			http.Error(w, "test concurrency barrier timed out", http.StatusGatewayTimeout)
			return
		}
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
	done := make(chan struct{})
	go func() {
		s.handleParallel(rec, req)
		close(done)
	}()

	models := make(map[string]bool, 3)
	var overlapErr string
	for len(models) < 3 {
		select {
		case model := <-arrived:
			models[model] = true
		case <-time.After(3 * time.Second):
			overlapErr = "parallel roles did not overlap at provider"
			releaseOnce.Do(func() { close(release) })
		}
		if overlapErr != "" {
			break
		}
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("parallel handler did not finish after releasing provider barrier")
	}
	if overlapErr != "" {
		t.Fatalf("%s; arrived models=%v", overlapErr, models)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("parallel code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Results []parallelWebResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Results) != 3 {
		t.Fatalf("resultats: %+v err=%v", out, err)
	}
	wantRoles := []string{"chat", "think", "reviewer"}
	for i, want := range wantRoles {
		if out.Results[i].Role != want {
			t.Fatalf("ordre de rols: got=%v want=%v", []string{out.Results[0].Role, out.Results[1].Role, out.Results[2].Role}, wantRoles)
		}
	}
}
