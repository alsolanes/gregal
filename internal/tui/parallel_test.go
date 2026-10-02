package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestParallelRoleNamesDedupeTargets(t *testing.T) {
	cfg := &config.Config{Language: "ca", Roles: map[string]config.Role{
		"chat":     {Provider: "local", Model: "small"},
		"think":    {Provider: "local", Model: "strong"},
		"reviewer": {Provider: "cloud", Model: "judge"},
		"code":     {Provider: "local", Model: "small"},
	}}
	got := parallelRoleNames(cfg, "code")
	if len(got) != 3 || got[0] != "code" || got[1] != "think" || got[2] != "reviewer" {
		t.Fatalf("rols paral·lels=%v", got)
	}
}

func TestStartParallelExecutaConcurrentment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(180 * time.Millisecond)
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\"resposta " + body.Model + "\"}}]}"))
	}))
	defer server.Close()
	cfg := &config.Config{Language: "ca",
		Providers: map[string]config.Provider{"local": {BaseURL: server.URL}},
		Roles: map[string]config.Role{
			"chat":     {Provider: "local", Model: "chat", MaxTokens: 20},
			"think":    {Provider: "local", Model: "think", MaxTokens: 20},
			"reviewer": {Provider: "local", Model: "review", MaxTokens: 20},
		},
	}
	m := Model{cfg: cfg, client: llm.New(), role: "chat", mode: "chat", lines: []string{}}
	_, cmd := m.startParallel("compara això")
	started := time.Now()
	msg := cmd()
	if time.Since(started) > 500*time.Millisecond {
		t.Fatalf("les crides semblen seqüencials: %v", time.Since(started))
	}
	got, ok := msg.(parallelDoneMsg)
	if !ok || len(got.results) != 3 {
		t.Fatalf("resultat paral·lel inesperat: %#v", msg)
	}
	for _, r := range got.results {
		if r.err != nil || !strings.Contains(r.reply, r.model) {
			t.Fatalf("resposta %s: %+v", r.role, r)
		}
	}
}

func TestParallelRoleNamesNeedsDifferentModels(t *testing.T) {
	cfg := &config.Config{Language: "ca", Roles: map[string]config.Role{
		"chat":     {Provider: "local", Model: "same"},
		"think":    {Provider: "local", Model: "same"},
		"reviewer": {Provider: "local", Model: "same"},
	}}
	if got := parallelRoleNames(cfg, "chat"); len(got) != 1 {
		t.Fatalf("no ha de repetir targets: %v", got)
	}
}
