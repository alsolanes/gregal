package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/verify"
)

func TestResolGoal(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	cwd := t.TempDir()
	if _, _, err := goal.Record("```goal\ntitle: t\nwhy: w\ncriteria:\n- x\n```", cwd); err != nil {
		t.Fatal(err)
	}
	if g, item := resolGoal(cwd, "#1"); item != 1 || g.Title != "t" || !strings.Contains(g.Body, "1. [ ] x") {
		t.Fatalf("%+v %v", g, item)
	}
	if g, item := resolGoal(cwd, "last"); item != 1 || g.ID != "#1" {
		t.Fatalf("last: %+v %v", g, item)
	}
	g, item := resolGoal(cwd, "add a --version flag")
	if item != 0 || g.ID != "" || !strings.HasPrefix(g.Body, "task: add a --version flag") {
		t.Fatalf("inline: %+v %v", g, item)
	}
}

// Un objectiu complert deixa la tasca a revisió amb el veredicte del bucle;
// un que no, la deixa en curs amb el que falta al log.
func TestFinishItem(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	cwd := t.TempDir()
	if _, _, err := goal.Record("```goal\ntitle: t\nwhy: w\ncriteria:\n- x\n```", cwd); err != nil {
		t.Fatal(err)
	}
	if err := startItem(cwd, 1); err != nil {
		t.Fatal(err)
	}
	finishItem(cwd, 1, agent.GoalResult{Stop: agent.GoalStopRounds, Rounds: []agent.GoalRound{{Verdict: verify.GoalVerdict{Missing: "x is not done"}}}})
	it, _ := goal.Resolve(cwd, "1")
	if it.Status != "in_progress" || !strings.Contains(it.Log[len(it.Log)-1].Text, "x is not done") {
		t.Fatalf("%+v", it)
	}
	finishItem(cwd, 1, agent.GoalResult{Met: true, Stop: agent.GoalMet, Rounds: []agent.GoalRound{{Verdict: verify.GoalVerdict{Met: true,
		Criteria: []verify.GoalCriterion{{Text: "x", Status: "MET", Evidence: "x.go:1"}}}}}})
	it, _ = goal.Resolve(cwd, "1")
	if it.Status != "in_review" || it.Review == nil || it.Review.Status != "met" || it.Review.Criteria[0].Evidence != "x.go:1" {
		t.Fatalf("%+v %+v", it.Status, it.Review)
	}
}

// The whole loop through a fake provider: the agent says "done" every time;
// the independent verifier (a read-only run whose prompt carries the goal
// rules) rejects once and then confirms with evidence.
func TestRunGoalHeadlessUntilVerifierConfirms(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
	verifierCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reply := "done"
		if strings.Contains(string(body), "independent verifier of a goal") {
			verifierCalls++
			reply = "VERDICT: NOT MET\nCRITERIA:\n- [NOT MET] c :: nothing\nMISSING: do it"
			if verifierCalls > 1 {
				reply = "VERDICT: MET\nCRITERIA:\n- [MET] c :: read x.txt"
			}
		}
		raw, _ := json.Marshal(reply)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, raw)
	}))
	defer srv.Close()
	role := config.Role{Provider: "t", Model: "m", MaxTokens: 100}
	cfg := &config.Config{
		Mode:      "code",
		Providers: map[string]config.Provider{"t": {BaseURL: srv.URL}},
		Roles:     map[string]config.Role{"code": role, "chat": role},
		Verify:    config.VerifyCfg{Mode: "off"},
		Router:    config.RouterCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 5},
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if code := runGoalHeadless(cfg, "", goalFlags{Spec: "make c true", Output: "text", MaxRounds: 4}); code != exitOK {
		t.Fatalf("exit %d, verifier asked %d times", code, verifierCalls)
	}
	if verifierCalls != 2 {
		t.Fatalf("the goal must stop only after the verifier confirms: %d calls", verifierCalls)
	}
	// A verifier that never confirms ends with exit 3 and a partial summary.
	verifierCalls = -100
	if code := runGoalHeadless(cfg, "", goalFlags{Spec: "make c true", Output: "json", MaxRounds: 1}); code != exitVerifyFailed {
		t.Fatalf("not met must exit %d, got %d", exitVerifyFailed, code)
	}
}
