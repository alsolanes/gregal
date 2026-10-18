package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/goal"
)

func TestResolGoal(t *testing.T) {
	dir := t.TempDir()
	saved := goal.Goal{ID: "abc123", Title: "t", Body: "task: t\ncriteria:\n- x", Project: "p"}
	if err := goal.Save(dir, saved); err != nil {
		t.Fatal(err)
	}
	if g, ok := resolGoal(dir, "p", "/w", "abc123"); !ok || g.Title != "t" {
		t.Fatalf("%+v %v", g, ok)
	}
	if g, ok := resolGoal(dir, "p", "/w", "last"); !ok || g.ID != "abc123" {
		t.Fatalf("last: %+v %v", g, ok)
	}
	g, ok := resolGoal(dir, "p", "/w", "add a --version flag")
	if ok || g.ID != "" || !strings.HasPrefix(g.Body, "task: add a --version flag") {
		t.Fatalf("inline: %+v %v", g, ok)
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
