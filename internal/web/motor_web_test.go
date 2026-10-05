package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// La web ara condueix el motor compartit. El motor resol les crides d'un
// pas en un ordre propi (les executables abans de les aprovacions), però
// la UI aparella targetes i resultats per ordre d'arribada: cada tool_call
// ha d'anar seguit del seu resultat (o de la seva aprovació) abans del
// següent. Un pas amb una lectura permesa i una escriptura que demana
// permís és el cas que ho trencaria.
func TestMotorWebAparellaCridesIAprovacions(t *testing.T) {
	var n atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Tools []json.RawMessage `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if len(req.Tools) == 0 || n.Add(1) > 1 {
			w.Write([]byte(`{"choices":[{"message":{"content":"Fet i verificat."}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"Miro i escric.","tool_calls":[` +
			`{"id":"r1","type":"function","function":{"name":"read","arguments":"{\"path\":\"a.txt\"}"}},` +
			`{"id":"w1","type":"function","function":{"name":"write","arguments":"{\"path\":\"b.txt\",\"content\":\"hola\"}"}}]}}]}`))
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL + "/v1"}}
	s.cwd = t.TempDir()
	s.policy.ProjectDir = s.cwd
	// Un override del config fa que write demani permís encara que sigui
	// dins del projecte: és l'escriptura que cal aprovar.
	s.policy.Tools = map[string]string{"write": "ask"}
	if err := os.WriteFile(filepath.Join(s.cwd, "a.txt"), []byte("contingut"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Qui mira la pantalla aprova l'escriptura quan arriba.
	fet := make(chan struct{})
	go func() {
		defer close(fet)
		for i := 0; i < 400; i++ {
			s.mu.Lock()
			var ch chan bool
			for k, ar := range s.approvals {
				ch = ar.ch
				delete(s.approvals, k)
			}
			s.mu.Unlock()
			if ch != nil {
				ch <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"llegeix a.txt i escriu b.txt","mode":"code"}`)))
	<-fet

	body := rec.Body.String()
	var ordre []string
	for _, m := range regexp.MustCompile(`event: (\w+)`).FindAllStringSubmatch(body, -1) {
		switch m[1] {
		case "thinking", "tool_call", "tool_result", "approve_request", "blocked", "assistant", "done":
			ordre = append(ordre, m[1])
		}
	}
	want := "thinking tool_call tool_result tool_call approve_request tool_result assistant done"
	if got := strings.Join(ordre, " "); got != want {
		t.Fatalf("ordre d'events = %q\nvolia        %q\n%s", got, want, body)
	}
	if b, err := os.ReadFile(filepath.Join(s.cwd, "b.txt")); err != nil || string(b) != "hola" {
		t.Fatalf("l'escriptura aprovada s'ha d'haver fet: %q %v", b, err)
	}
}

func TestMaxTokensPerTorn(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		role   config.Role
		window int
		want   int
	}{
		{name: "legacy chat mode stays small", mode: agent.ModeChat, role: config.Role{MaxTokens: 1024}, window: 32768, want: 1024},
		{name: "legacy agent role gets useful budget", mode: agent.ModeCode, role: config.Role{MaxTokens: 1024}, window: 32768, want: 8192},
		{name: "legacy agent respects context ceiling", mode: agent.ModeCode, role: config.Role{MaxTokens: 1024}, window: 16384, want: 4096},
		{name: "explicit small budget is preserved", mode: agent.ModeCode, role: config.Role{MaxTokens: 2048}, window: 32768, want: 2048},
		{name: "provider default stays provider default", mode: agent.ModeCode, role: config.Role{}, window: 32768, want: 0},
		{name: "too small window does not lower legacy budget", mode: agent.ModeCode, role: config.Role{MaxTokens: 1024}, window: 4096, want: 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maxTokensPerTorn(tt.mode, tt.role, tt.window); got != tt.want {
				t.Fatalf("maxTokensPerTorn() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConduirTornEscalaPressupostIRetiraFragmentTallat(t *testing.T) {
	var budgets []int
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		budgets = append(budgets, req.MaxTokens)
		w.Header().Set("Content-Type", "text/event-stream")
		if len(budgets) == 1 {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"incomplet\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n")
		} else {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"complet\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL}}
	role := config.Role{Provider: "p", Model: "m", MaxTokens: 1024, ContextWindow: 32768}
	var tokens []string
	var errorsSeen []string
	reply, _, err := s.conduirTorn(context.Background(), "fes una resposta", agent.ModeCode, "chat", s.cfg.Providers["p"], role, 1, t.TempDir(), nil, func(kind string, data any) {
		if kind == "token" {
			tokens = append(tokens, data.(map[string]string)["text"])
		}
		if kind == "error" {
			errorsSeen = append(errorsSeen, data.(map[string]string)["message"])
		}
	})
	if err != nil {
		t.Fatalf("conduirTorn: %v", err)
	}
	if reply != "complet" || strings.Join(tokens, "") != "complet" {
		t.Fatalf("reply=%q emitted=%q, wanted only recovered answer", reply, strings.Join(tokens, ""))
	}
	if len(errorsSeen) != 0 {
		t.Fatalf("unexpected errors: %v", errorsSeen)
	}
	if len(budgets) != 2 || budgets[0] != 8192 || budgets[1] != 16384 {
		t.Fatalf("provider max_tokens=%v, wanted [8192 16384]", budgets)
	}
}
