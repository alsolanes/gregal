package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/tools"
)

func TestModeDebugAcceptat(t *testing.T) {
	s := goalTestServer(t)
	if rec := postJSON(t, s.handleMode, `{"mode":"debug"}`); rec.Code != 200 {
		t.Fatalf("mode debug rebutjat: %d %s", rec.Code, rec.Body.String())
	}
	if s.mode != agent.ModeDebug {
		t.Fatalf("mode: %q", s.mode)
	}
}

// proveidorCaps respon com a planificador, crític o agent segons la consigna.
func proveidorCaps(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		tot := ""
		teTool := false
		for _, m := range req.Messages {
			tot += fmt.Sprint(m.Content) + "\n"
			if m.Role == "tool" {
				teTool = true
			}
		}
		resposta := func(text string) {
			raw, _ := json.Marshal(text)
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, raw)
		}
		switch {
		case strings.Contains(tot, "critiquing an implementation plan"):
			resposta("MISSING: a test for the new flag\nRISKY: edits the shared config loader")
		case strings.Contains(tot, "PLAN MODE"):
			resposta("1. config.go: add the flag\nVERIFICATION: go test ./...")
		case strings.Contains(tot, "scoring ONE candidate"):
			resposta("SCORE: 7\n- fine")
		case teTool:
			resposta("fet")
		default:
			args, _ := json.Marshal(map[string]string{"path": "sol.txt", "content": "uno\n"})
			argsJSON, _ := json.Marshal(string(args))
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, argsJSON)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func capsServer(t *testing.T, url, cwd string) *Server {
	t.Helper()
	s := goalTestServer(t)
	s.mu.Lock()
	s.cfg.Providers["p"] = config.Provider{BaseURL: url}
	s.cwd = cwd
	s.mu.Unlock()
	return s
}

func TestPlanPortaLaCritica(t *testing.T) {
	srv := proveidorCaps(t)
	s := capsServer(t, srv.URL, t.TempDir())
	rec := postJSON(t, s.handlePlan, `{"task":"add a --flag option"}`)
	if rec.Code != 200 {
		t.Fatalf("pla: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Plan   string             `json:"plan"`
		Critic agent.PlanCritique `json:"critic"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if !strings.Contains(out.Plan, "VERIFICATION") || len(out.Critic.Issues) != 2 || out.Critic.Issues[0].Kind != "missing" {
		t.Fatalf("resposta: %s", rec.Body.String())
	}
	s.cfg.Plan.Critic = "off"
	rec = postJSON(t, s.handlePlan, `{"task":"add a --flag option"}`)
	if strings.Contains(rec.Body.String(), `"critic"`) {
		t.Fatalf("plan.critic: off no porta crítica: %s", rec.Body.String())
	}
}

func TestCalibracioAPI(t *testing.T) {
	agent.BetsPathOverride = filepath.Join(t.TempDir(), "bets.jsonl")
	t.Cleanup(func() { agent.BetsPathOverride = "" })
	s := goalTestServer(t)
	s.Hub() // fixa l'id de la sessió per defecte
	cwd := s.sessionCwd()
	agent.RegistraAposta(agent.Aposta{Session: s.id, Project: cwd, Command: "go test ./...", P: 0.8, Passed: true})
	agent.RegistraAposta(agent.Aposta{Session: "altra", Project: cwd, Command: "go vet ./...", P: 0.6, Passed: false})
	agent.RegistraAposta(agent.Aposta{Session: "altra", Project: filepath.Join(cwd, "altre"), Command: "go test", P: 0.5, Passed: true})
	get := func(q string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodGet, "/api/calibration"+q, nil)
		rec := httptest.NewRecorder()
		s.handleCalibration(rec, req)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	count := func(out map[string]any) float64 {
		c, _ := out["calibration"].(map[string]any)
		n, _ := c["count"].(float64)
		return n
	}
	if code, out := get(""); code != 200 || count(out) != 2 {
		t.Fatalf("projecte: %d %v", code, out)
	}
	if _, out := get("?scope=session"); count(out) != 1 {
		t.Fatalf("sessió: %v", out)
	}
	if _, out := get("?scope=all"); count(out) != 3 {
		t.Fatalf("tot: %v", out)
	}
	if code, _ := get("?scope=marte"); code != 400 {
		t.Fatalf("scope invàlid: %d", code)
	}
}

func TestBetPayloadIDurable(t *testing.T) {
	p := betPayload("c1", agent.Aposta{P: 0.8, Passed: true, Command: "go test ./...", Source: "model"})
	if p["p"] != "80" || p["passed"] != "true" || !strings.Contains(p["text"], "bet 80% ✓") {
		t.Fatalf("payload: %v", p)
	}
	raw := interactiveEventPayload("bet", p)
	if !strings.Contains(string(raw), `"command":"go test ./..."`) || strings.Contains(string(raw), "text") {
		t.Fatalf("payload durable: %s", raw)
	}
}

func gitRepoWeb(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible")
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"commit", "-q", "-m", "init"}} {
		if args[0] == "add" {
			os.WriteFile(filepath.Join(dir, "README.md"), []byte("hola\n"), 0o644)
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestBestOfAPI(t *testing.T) {
	repo := gitRepoWeb(t)
	// Les escriptures dels intents van al journal global del procés: es
	// buida en acabar perquè no embruti els tests de rewind.
	defer tools.Active.Rewind()
	srv := proveidorCaps(t)
	s := capsServer(t, srv.URL, repo)
	for _, body := range []string{`{}`, `{"task":"x","n":5}`, `{"task":"x","n":2,"max_steps":500}`} {
		if rec := postJSON(t, s.handleBestOf, body); rec.Code != 400 {
			t.Fatalf("%s → %d, volia 400", body, rec.Code)
		}
	}
	get := func(q string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/bestof"+q, nil)
		rec := httptest.NewRecorder()
		s.handleBestOf(rec, req)
		return rec
	}
	if rec := get("?id=res"); rec.Code != 404 {
		t.Fatalf("id desconegut: %d", rec.Code)
	}
	rec := postJSON(t, s.handleBestOf, `{"task":"write the answer to sol.txt","n":2,"verify":["git grep -q --untracked uno"]}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("arrencada: %d %s", rec.Code, rec.Body.String())
	}
	var snap struct {
		ID          string                `json:"id"`
		Running     bool                  `json:"running"`
		Recommended int                   `json:"recommended"`
		Attempts    []agent.BestOfAttempt `json:"attempts"`
	}
	json.Unmarshal(rec.Body.Bytes(), &snap)
	id := snap.ID
	deadline := time.Now().Add(60 * time.Second)
	for {
		json.Unmarshal(get("?id="+id).Body.Bytes(), &snap)
		if !snap.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("el best-of no acaba: %+v %s", snap, get("?id="+id).Body.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(snap.Attempts) != 2 || snap.Recommended < 1 {
		t.Fatalf("comparació: %+v", snap)
	}
	for _, a := range snap.Attempts {
		if a.State != agent.BestOfDone || a.ChecksPassed == nil || !*a.ChecksPassed || a.Score == nil || *a.Score != 7 {
			t.Fatalf("intent %d: %+v", a.Index, a)
		}
	}
	if list := get(""); !strings.Contains(list.Body.String(), id) {
		t.Fatalf("la llista ha de portar el best-of: %s", list.Body.String())
	}
	rec = postJSON(t, s.handleBestOfApply, fmt.Sprintf(`{"id":%q,"attempt":2}`, id))
	if rec.Code != 200 {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
	if raw, err := os.ReadFile(filepath.Join(repo, "sol.txt")); err != nil || strings.TrimSpace(string(raw)) != "uno" {
		t.Fatalf("l'intent aplicat havia d'arribar al repo: %q %v", raw, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(repo, ".gregal", "worktrees")); len(entries) != 0 {
		t.Fatalf("no han de quedar worktrees: %v", entries)
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/bestof?id="+id, nil)
	drec := httptest.NewRecorder()
	s.handleBestOf(drec, req)
	if drec.Code != 200 {
		t.Fatalf("delete: %d %s", drec.Code, drec.Body.String())
	}
}

// La sortida de preview és el JSON que el mòbil obre: retallada ja no es llegeix.
func TestPreviewPayloadDurableSencer(t *testing.T) {
	out := `{"kind":"html","content":"` + strings.Repeat("x", 5000) + `"}`
	raw := interactiveEventPayload("tool_result", map[string]string{"id": "c1", "name": "preview", "output": out})
	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil || got["output"] != out {
		t.Fatalf("preview retallada: %d bytes, err %v", len(got["output"]), err)
	}
	raw = interactiveEventPayload("tool_result", map[string]string{"id": "c2", "name": "read", "output": strings.Repeat("y", 5000)})
	if err := json.Unmarshal(raw, &got); err != nil || len([]rune(got["output"])) != 3001 {
		t.Fatalf("la resta es retalla: %d", len([]rune(got["output"])))
	}
}
