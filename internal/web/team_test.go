package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
)

func TestTeamAPIRealHandoffs(t *testing.T) {
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
			Tools []any `json:"tools"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid model request")
		}
		if len(body.Tools) != 0 {
			t.Error("team must not expose tools")
		}
		if calls > 0 && !strings.Contains(body.Messages[len(body.Messages)-1].Content, "deliverable:\nresult") {
			t.Error("missing previous result")
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"result"}}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report","lang":"ca"}`)))
	if calls != 4 || !strings.Contains(w.Body.String(), `"type":"done"`) || strings.Count(w.Body.String(), `"type":"handoff"`) != 3 {
		t.Fatalf("calls=%d body=%s", calls, w.Body.String())
	}
	if s.flowRunning {
		t.Fatal("busy guard leaked")
	}
}

func TestTeamAPIValidation(t *testing.T) {
	s := goalTestServer(t)
	for _, body := range []string{`{}`, `{"task":"x","lang":"es"}`, `{"task":"x","token":"private"}`} {
		w := httptest.NewRecorder()
		s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("status=%d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("GET", "/api/v2/team/run", nil))
	if w.Code != 405 {
		t.Fatal("GET should not run")
	}
	s.agentBusy = true
	w = httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"x"}`)))
	if w.Code != 409 {
		t.Fatal("busy guard")
	}
}

func TestTeamRequiresAuthentication(t *testing.T) {
	s := goalTestServer(t)
	s.SetToken("synthetic-team-auth")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"x"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
}
