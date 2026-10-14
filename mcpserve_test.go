package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
)

func TestGregalMCPRunTornaCanvis(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sense git")
	}
	ws := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", ws}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(ws, "a.txt"), []byte("a"), 0o600)
	os.WriteFile(filepath.Join(ws, "brut.txt"), []byte("1"), 0o600)
	git("add", ".")
	git("commit", "-qm", "init")
	os.WriteFile(filepath.Join(ws, "brut.txt"), []byte("2"), 0o600) // ja brut abans del torn

	cfg := &config.Config{Mode: "code", Agent: config.AgentCfg{MaxSteps: 10}}
	var got agent.RunOpts
	g := &gregalMCP{cfg: cfg, defaultWS: ws, sessionsDir: t.TempDir(), log: func(string, ...any) {},
		run: func(_ context.Context, o agent.RunOpts, verify string) (agent.RunResult, error) {
			got = o
			os.WriteFile(filepath.Join(ws, "a.txt"), []byte("b"), 0o600)
			os.WriteFile(filepath.Join(ws, "nou.go"), []byte("package x"), 0o600)
			return agent.RunResult{Answer: "fet", Steps: 2, Tools: []string{"write"}}, nil
		}}
	out, err := g.handleRun(context.Background(), map[string]any{"task": "canvia a", "max_steps": float64(5)})
	if err != nil {
		t.Fatal(err)
	}
	var res mcpRunResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if res.Answer != "fet" || strings.Join(res.ChangedFiles, ",") != "a.txt,nou.go" || res.Session == "" {
		t.Fatalf("resultat: %+v", res)
	}
	if got.Dir != ws || got.MaxSteps != 5 || !got.HardStepLimit || got.AutoApprove {
		t.Fatalf("opcions del torn: %+v", got)
	}
	// La conversa desada es pot llistar i llegir.
	list, err := g.handleSessionsList(context.Background(), map[string]any{})
	if err != nil || !strings.Contains(list, res.Session) {
		t.Fatalf("list=%s err=%v", list, err)
	}
	one, err := g.handleSessionGet(context.Background(), map[string]any{"name": res.Session})
	if err != nil || !strings.Contains(one, "canvia a") || !strings.Contains(one, `"total_messages": 2`) {
		t.Fatalf("get=%s err=%v", one, err)
	}
}

func TestGregalMCPRunValida(t *testing.T) {
	cfg := &config.Config{Mode: "code", Agent: config.AgentCfg{MaxSteps: 10}}
	g := &gregalMCP{cfg: cfg, defaultWS: t.TempDir(), sessionsDir: t.TempDir(), log: func(string, ...any) {},
		run: func(context.Context, agent.RunOpts, string) (agent.RunResult, error) {
			t.Fatal("no s'hauria d'executar")
			return agent.RunResult{}, nil
		}}
	for _, args := range []map[string]any{
		{},
		{"task": "x", "mode": "nope"},
		{"task": "x", "mode": "autonomous", "auto_approve": true},
		{"task": "x", "workspace": filepath.Join(t.TempDir(), "no-existeix")},
	} {
		if _, err := g.handleRun(context.Background(), args); err == nil {
			t.Fatalf("hauria de fallar: %v", args)
		}
	}
}

func TestChangedFilesSenseGit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x", "y.go")
	tr := []agent.TraceCall{
		{Tool: "write", Args: `{"path":` + jsonStr(p) + `,"content":"..."}`},
		{Tool: "read", Args: `{"path":"z.go"}`},
		{Tool: "edit", Args: `{"path":"rel.go"}`, Failed: true},
	}
	got := changedFiles(dir, nil, false, tr)
	if len(got) != 1 || got[0] != "x/y.go" {
		t.Fatalf("%v", got)
	}
}

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }
