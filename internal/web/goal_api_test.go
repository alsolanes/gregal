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
	"gregal/internal/llm"
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
	if rec := postJSON(t, s.handleMode, `{"mode":"specs"}`); rec.Code == 200 {
		t.Fatal("specs no hauria de ser un mode vàlid")
	}
}

func TestPromptInclouContextActualISenseInvencions(t *testing.T) {
	s := goalTestServer(t)
	s.cwd = "/projectes/reals/tui-agent"
	p := s.sysPrompt()
	for _, want := range []string{
		"Working directory: \"/projectes/reals/tui-agent\"",
		"Current project: \"tui-agent\"",
		"Do not invent project names",
		"You have no implicit list of files",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt no conté %q:\n%s", want, p)
		}
	}
	// Els fets del workspace van abans: la directiva d'idioma tanca el
	// prompt també a la web. El resum de compactació ja no hi va (és el
	// primer missatge de la conversa): el prefix no canvia en compactar.
	s.compacted = "resum previ"
	p = s.sysPrompt()
	if !strings.HasSuffix(p, "\n\n"+config.LanguageDirective(s.cfg.Lang())) || strings.Contains(p, "resum previ") {
		t.Fatalf("la directiva d'idioma ha de tancar el prompt web, sense el resum:\n%s", p)
	}
}

// /api/goal és ara una vista del backlog: els objectius desats abans hi
// passen sols la primera vegada, i «task» i «delete» els comencen o els
// descarten com a tasques.
func TestGoalAPIllistaPreparaEsborra(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	if err := goal.Save(s.goalDir, goal.Goal{
		ID: "z1", CWD: s.cwd, Project: s.projectName(), Title: "Afegir scroll", Body: "tasca: Afegir scroll\ncontext: la roda no mou l'historial\ncriteris:\n- la roda va",
	}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleGoal(rec, httptest.NewRequest(http.MethodGet, "/api/goal", nil))
	var llista struct {
		Goals []struct{ ID, Title, Status string } `json:"goals"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &llista); err != nil {
		t.Fatal(err)
	}
	if len(llista.Goals) != 1 || llista.Goals[0].ID != "1" || llista.Goals[0].Title != "Afegir scroll" || llista.Goals[0].Status != "proposed" {
		t.Fatalf("l'objectiu vell ha de ser la tasca #1: %+v", llista.Goals)
	}

	rec = postJSON(t, s.handleGoal, `{"action":"task","id":"1"}`)
	if rec.Code != 200 {
		t.Fatalf("task: %d %s", rec.Code, rec.Body.String())
	}
	var task struct{ ID, Title, Task string }
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(task.Task, "backlog item #1") || !strings.Contains(task.Task, "la roda va") {
		t.Fatalf("tasca inesperada: %+v", task)
	}
	it, _ := goal.Resolve(s.cwd, "1")
	if it.Status != "in_progress" {
		t.Fatalf("en preparar-la queda en curs: %s", it.Status)
	}

	rec = postJSON(t, s.handleGoal, `{"action":"delete","id":"1"}`)
	if rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if it, _ := goal.Resolve(s.cwd, "1"); it.Status != "dropped" {
		t.Fatalf("esborrar-la la descarta: %s", it.Status)
	}
}

func TestModeObjectiuDesaAlBacklog(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	reply := "```goal\ntype: task\ntitle: Rename the config key\nwhy: confusing\ncriteria:\n- old key still read\n```"
	s.convo = []llm.Message{{Role: "user", Content: "x"}, {Role: "assistant", Content: reply}}
	var ev string
	var data any
	s.emitGoal(func(e string, d any) { ev, data = e, d }, reply)
	if ev != "backlog_item" {
		t.Fatalf("event %q", ev)
	}
	if !strings.Contains(s.convo[1].Content, "Saved to the backlog as #1 (proposed)") {
		t.Fatalf("el model ha de veure el número: %q", s.convo[1].Content)
	}
	_ = data
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
