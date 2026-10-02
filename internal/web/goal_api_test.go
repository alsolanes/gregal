package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/goal"
)

func goalTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		Mode: "code",
		Roles: map[string]config.Role{
			"chat":     {Provider: "p", Model: "m", ContextWindow: 32768},
			"think":    {Provider: "p", Model: "m", ContextWindow: 32768},
			"code":     {Provider: "p", Model: "m", ContextWindow: 32768},
			"reviewer": {Provider: "p", Model: "m", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	return New(cfg, filepath.Join(t.TempDir(), "config.yaml"))
}

func postJSON(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestModeAcceptaObjectiu(t *testing.T) {
	s := goalTestServer(t)
	rec := postJSON(t, s.handleMode, `{"mode":"goal"}`)
	if rec.Code != 200 {
		t.Fatalf("mode goal rebutjat: %d %s", rec.Code, rec.Body.String())
	}
	if s.mode != "goal" {
		t.Fatalf("mode=%q", s.mode)
	}
	rec = postJSON(t, s.handleMode, `{"mode":"inspect"}`)
	if rec.Code != 200 || s.mode != "inspect" {
		t.Fatalf("mode consulta rebutjat: %d mode=%q %s", rec.Code, s.mode, rec.Body.String())
	}
	if s.sysPrompt() == s.cfg.SystemPrompt() {
		t.Fatal("el prompt del mode objectiu ha d'afegir instruccions")
	}
	if rec := postJSON(t, s.handleMode, `{"mode":"plan"}`); rec.Code == 200 {
		t.Fatal("plan no hauria de ser un mode vàlid")
	}
}

func TestPromptInclouContextActualISenseInvencions(t *testing.T) {
	s := goalTestServer(t)
	s.cwd = "/projectes/reals/tui-agent"
	p := s.sysPrompt()
	for _, want := range []string{
		"Directori de treball: \"/projectes/reals/tui-agent\"",
		"Projecte actual: \"tui-agent\"",
		"No inventis noms de projectes",
		"No tens cap llista implícita de fitxers",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt no conté %q:\n%s", want, p)
		}
	}
}

func TestGoalAPIllistaPreparaEsborra(t *testing.T) {
	s := goalTestServer(t)

	// GET inicial: buit.
	rec := httptest.NewRecorder()
	s.handleGoal(rec, httptest.NewRequest(http.MethodGet, "/api/goal", nil))
	var llista struct {
		Goals []goal.Goal `json:"goals"`
		Last  string      `json:"last"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &llista); err != nil {
		t.Fatal(err)
	}
	if len(llista.Goals) != 0 {
		t.Fatalf("esperava cap objectiu: %+v", llista.Goals)
	}

	// Un objectiu desat: ha d'aparèixer a la llista i poder-se preparar.
	if err := goal.Save(s.goalDir, goal.Goal{
		ID: "z1", Project: s.projectName(), Title: "Afegir scroll", Body: "tasca: Afegir scroll\ncriteris:\n- la roda va",
	}); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	s.handleGoal(rec, httptest.NewRequest(http.MethodGet, "/api/goal", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &llista); err != nil {
		t.Fatal(err)
	}
	if len(llista.Goals) != 1 || llista.Goals[0].ID != "z1" {
		t.Fatalf("llista=%+v", llista.Goals)
	}

	rec = postJSON(t, s.handleGoal, `{"action":"task","id":"z1"}`)
	if rec.Code != 200 {
		t.Fatalf("task: %d %s", rec.Code, rec.Body.String())
	}
	var task struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Task  string `json:"task"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(task.Task, "Afegir scroll") || task.ID != "z1" {
		t.Fatalf("tasca inesperada: %+v", task)
	}
	// En preparar-lo queda marcat com a fet.
	if g, err := goal.Get(s.goalDir, "z1"); err != nil || g.Status != goal.StatusFet {
		t.Fatalf("estat=%v err=%v", g.Status, err)
	}

	rec = postJSON(t, s.handleGoal, `{"action":"delete","id":"z1"}`)
	if rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := goal.Get(s.goalDir, "z1"); err == nil {
		t.Fatal("l'objectiu hauria d'estar esborrat")
	}
}

func TestGoalAPISenseObjectius(t *testing.T) {
	s := goalTestServer(t)
	if rec := postJSON(t, s.handleGoal, `{"action":"task"}`); rec.Code != 404 {
		t.Fatalf("esperava 404 sense objectius, tinc %d", rec.Code)
	}
}

func TestGoalDirAlCostatDelConfig(t *testing.T) {
	dir := t.TempDir()
	if got := goalDirFor(filepath.Join(dir, "config.yaml")); got != dir {
		t.Fatalf("goalDirFor=%q, volia %q", got, dir)
	}
}
