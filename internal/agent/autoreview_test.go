package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// revisorFals és un proveïdor OpenAI-compatible que respon content (o
// s'adorm delay) i compta les crides i el darrer prompt rebut.
func revisorFals(t *testing.T, content string, delay time.Duration) (*httptest.Server, *int32, *atomic.Value) {
	t.Helper()
	var n int32
	var prompt atomic.Value
	prompt.Store("")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&n, 1)
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) > 0 {
			prompt.Store(req.Messages[len(req.Messages)-1].Content)
		}
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		body, _ := json.Marshal(content)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &n, &prompt
}

func cfgRevisor(url string) *config.Config {
	return &config.Config{
		Mode:      ModeCode,
		Providers: map[string]config.Provider{"t": {BaseURL: url}},
		Roles: map[string]config.Role{
			"chat":     {Provider: "t", Model: "chat-m", MaxTokens: 100},
			"reviewer": {Provider: "t", Model: "rev-m", MaxTokens: 100},
		},
		Agent:       config.AgentCfg{MaxSteps: 10},
		Permissions: config.PermissionsCfg{Mode: "auto-review", ReviewTimeoutS: 1},
	}
}

func tornRevisat(cfg *config.Config, rev ReviewFunc) *Torn {
	return NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeCode, MaxSteps: 10,
		Tasca: "add a test", Hist: []llm.Message{{Role: "user", Content: "add a test"}},
		Revisor: rev,
	})
}

func TestAutoReviewAllowExecuta(t *testing.T) {
	tools.TodoClear()
	srv, n, prompt := revisorFals(t, `{"decision":"allow","reason":"runs the project tests"}`, 0)
	cfg := cfgRevisor(srv.URL)
	tn := tornRevisat(cfg, NewReviewer(llm.New(), cfg, nil))
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "bash", `{"command":"make deploy-test"}`)}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdreExecuta || p.Revisio == nil || p.Revisio.Decision != ReviewAllow {
		t.Fatalf("allow ha d'executar: %+v", p)
	}
	if len(p.Events) != 1 || !strings.Contains(p.Events[0].Text, "auto-approved by reviewer: runs the project tests") {
		t.Fatalf("falta la nota: %+v", p.Events)
	}
	if atomic.LoadInt32(n) != 1 {
		t.Fatalf("crides al revisor: %d", *n)
	}
	got := prompt.Load().(string)
	for _, w := range []string{"USER TASK:\nadd a test", "COMMAND:\nmake deploy-test", "PENDING CALL: bash"} {
		if !strings.Contains(got, w) {
			t.Errorf("el prompt no conté %q:\n%s", w, got)
		}
	}
}

func TestAutoReviewDenyBloqueja(t *testing.T) {
	tools.TodoClear()
	srv, _, _ := revisorFals(t, "Sure.\n```json\n{\"decision\":\"deny\",\"reason\":\"pushes to production\"}\n```", 0)
	cfg := cfgRevisor(srv.URL)
	tn := tornRevisat(cfg, NewReviewer(llm.New(), cfg, nil))
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "bash", `{"command":"make deploy-test"}`)}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdrePasModel {
		t.Fatalf("deny no executa ni pregunta, torna al model: %v", p.Ordre)
	}
	if len(p.Events) == 0 || p.Events[0].Revisio == nil || p.Events[0].Revisio.Decision != ReviewDeny || !p.Events[0].Fallada {
		t.Fatalf("falta l'event de bloqueig: %+v", p.Events)
	}
	var tool llm.Message
	for _, m := range tn.Hist {
		if m.Role == "tool" && m.ToolCallID == "1" {
			tool = m
		}
	}
	if !strings.Contains(tool.Content, "TOOL DENIED by the auto-reviewer: pushes to production") {
		t.Fatalf("el model ha de saber per què: %+v", tool)
	}
}

func TestAutoReviewAskTornaALaPersona(t *testing.T) {
	tools.TodoClear()
	srv, _, _ := revisorFals(t, `{"decision":"ask","reason":"irreversible"}`, 0)
	cfg := cfgRevisor(srv.URL)
	tn := tornRevisat(cfg, NewReviewer(llm.New(), cfg, nil))
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "bash", `{"command":"make deploy-test"}`)}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdreAprova || p.Revisio == nil || p.Revisio.Reason != "irreversible" {
		t.Fatalf("ask demana a la persona: %+v", p)
	}
}

func TestAutoReviewTimeoutIMalformatSonAsk(t *testing.T) {
	tools.TodoClear()
	srv, _, _ := revisorFals(t, `{"decision":"allow","reason":"late"}`, 3*time.Second)
	cfg := cfgRevisor(srv.URL)
	d := Review(context.Background(), llm.New(), cfg, ReviewRequest{Task: "x", Call: cridaProva("1", "bash", `{"command":"ls"}`)})
	if d.Decision != ReviewAsk || !strings.Contains(d.Reason, "timed out") {
		t.Fatalf("timeout → ask: %+v", d)
	}
	for _, raw := range []string{"I think it's fine", `{"decision":"maybe","reason":"x"}`, `{"decision":`, ""} {
		srv2, _, _ := revisorFals(t, raw, 0)
		cfg2 := cfgRevisor(srv2.URL)
		if d := Review(context.Background(), llm.New(), cfg2, ReviewRequest{Call: cridaProva("1", "bash", `{"command":"ls"}`)}); d.Decision != ReviewAsk {
			t.Errorf("%q → %+v, volia ask", raw, d)
		}
	}
}

func TestAutoReviewMaiVeuLesDenegades(t *testing.T) {
	tools.TodoClear()
	srv, n, _ := revisorFals(t, `{"decision":"allow","reason":"x"}`, 0)
	cfg := cfgRevisor(srv.URL)
	tn := tornRevisat(cfg, NewReviewer(llm.New(), cfg, nil))
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "bash", `{"command":"sudo rm -rf /"}`)}, nil)
	p := tn.Seguent()
	if p.Ordre == OrdreExecuta {
		t.Fatal("una crida denegada per les regles dures no es pot executar mai")
	}
	if atomic.LoadInt32(n) != 0 {
		t.Fatalf("el revisor no ha de veure crides denegades (%d crides)", *n)
	}
}

func TestAutoReviewInactiuIAutonom(t *testing.T) {
	tools.TodoClear()
	srv, n, _ := revisorFals(t, `{"decision":"allow","reason":"x"}`, 0)
	cfg := cfgRevisor(srv.URL)
	tn := tornRevisat(cfg, NewReviewer(llm.New(), cfg, func() bool { return false }))
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "bash", `{"command":"make x"}`)}, nil)
	if p := tn.Seguent(); p.Ordre != OrdreAprova || p.Revisio != nil {
		t.Fatalf("revisor apagat: pregunta com sempre: %+v", p)
	}
	auto := NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeAutonomous, MaxSteps: 10,
		Hist: []llm.Message{{Role: "user", Content: "x"}}, Revisor: NewReviewer(llm.New(), cfg, nil),
	})
	auto.Seguent()
	// En autònom, el que és normal passa sol; el perillós (git push) espera
	// la persona i no passa pel revisor.
	auto.RepPas("", []llm.ToolCall{cridaProva("1", "bash", `{"command":"git push origin main"}`)}, nil)
	if p := auto.Seguent(); p.Ordre != OrdreAprova {
		t.Fatalf("autònom continua demanant confirmació: %v", p.Ordre)
	}
	if atomic.LoadInt32(n) != 0 {
		t.Fatalf("crides inesperades al revisor: %d", *n)
	}
}

func TestReviewerRoleIVistaDiff(t *testing.T) {
	cfg := cfgRevisor("http://x")
	if ReviewerRole(cfg) != "reviewer" {
		t.Fatal("prefereix reviewer")
	}
	delete(cfg.Roles, "reviewer")
	if ReviewerRole(cfg) != "chat" {
		t.Fatal("si no, chat")
	}
	p := ReviewPrompt(ReviewRequest{Task: "t", Call: cridaProva("1", "edit", `{"path":"a.go","old":"x := 1","new":"x := 2"}`)})
	if !strings.Contains(p, "FILE: a.go") || !strings.Contains(p, "x := 1") || !strings.Contains(p, "x := 2") {
		t.Fatalf("l'edit es mostra com a diff:\n%s", p)
	}
}
