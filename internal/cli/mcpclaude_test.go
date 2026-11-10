package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"gregal/internal/mcp"
)

// fakeClaude fa de `claude -p`: eco dels arguments i del prompt en el JSON de
// resultat que escriu el CLI de debò.
func fakeClaude() {
	in, _ := io.ReadAll(os.Stdin)
	if strings.Contains(string(in), "FAIL") {
		os.Stdout.WriteString(`{"type":"result","is_error":true,"result":"Failed to authenticate","session_id":"s0"}`)
		return
	}
	res, _ := json.Marshal(map[string]any{
		"type": "result", "is_error": false, "session_id": "sess-1", "num_turns": 1, "duration_ms": 12,
		"total_cost_usd": 0.5, "modelUsage": map[string]any{"claude-test": map[string]any{}},
		"result": "prompt=" + string(in) + " args=" + strings.Join(os.Args[1:], "|") + " nested=" + os.Getenv("CLAUDECODE"),
	})
	os.Stdout.Write(res)
}

func TestClaudeArgs(t *testing.T) {
	c := &claudeCLI{defaultModel: "sonnet"}
	got, err := c.claudeArgs(map[string]any{"prompt": "x"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{"-p", "--output-format json", "--strict-mcp-config", "--model sonnet", "--no-session-persistence", "--setting-sources"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	got, _ = c.claudeArgs(map[string]any{"access": "edit", "keep_session": true, "effort": "high", "model": "opus"})
	joined = strings.Join(got, " ")
	if !strings.Contains(joined, "acceptEdits") || strings.Contains(joined, "--no-session-persistence") || !strings.Contains(joined, "--model opus") || strings.Contains(joined, "Bash") {
		t.Errorf("edit args: %q", joined)
	}
	got, _ = c.claudeArgs(map[string]any{"session": "abc"})
	if j := strings.Join(got, " "); !strings.Contains(j, "--resume abc") || strings.Contains(j, "--no-session-persistence") {
		t.Errorf("resume args: %q", j)
	}
	for _, bad := range []map[string]any{{"access": "root"}, {"effort": "max"}} {
		if _, err := c.claudeArgs(bad); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

func TestClaudeAskEndToEnd(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GREGAL_TEST_FAKE_CLAUDE", "1")
	t.Setenv("CLAUDECODE", "1")
	c := &claudeCLI{bin: exe, defaultWS: t.TempDir(), log: func(string, ...any) {}}
	tools := c.tools()
	if len(tools) != 1 || tools[0].Name != "claude_ask" {
		t.Fatalf("tools: %+v", tools)
	}
	out, err := tools[0].Handler(context.Background(), map[string]any{"prompt": "hola", "access": "read"})
	if err != nil {
		t.Fatal(err)
	}
	var a claudeAnswer
	if err := json.Unmarshal([]byte(out), &a); err != nil {
		t.Fatal(err)
	}
	if a.Session != "sess-1" || a.Model != "claude-test" || a.CostUSD != 0.5 || !strings.Contains(a.Answer, "prompt=hola") || !strings.Contains(a.Answer, "Read,Grep,Glob") {
		t.Errorf("answer: %+v", a)
	}
	if strings.HasSuffix(a.Answer, "nested=1") {
		t.Error("CLAUDECODE leaked to the child")
	}
	if _, err := tools[0].Handler(context.Background(), map[string]any{"prompt": "FAIL"}); err == nil || !strings.Contains(err.Error(), "authenticate") {
		t.Errorf("error not surfaced: %v", err)
	}
	if _, err := tools[0].Handler(context.Background(), map[string]any{}); err == nil {
		t.Error("empty prompt accepted")
	}
	if _, err := tools[0].Handler(context.Background(), map[string]any{"prompt": "x", "workspace": "/no/such/dir"}); err == nil {
		t.Error("bad workspace accepted")
	}
}

func TestMCPClaudeThroughManager(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GREGAL_TEST_MCP_CLAUDE", "1")
	m := mcp.Setup(map[string]mcp.Server{"claude": {Command: exe, Env: map[string]string{"GREGAL_TEST_MCP_CLAUDE": "1"}}})
	defer func() {
		m.Remove("claude")
		m.Close()
	}()
	if errs := m.ErrorsSnapshot(); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	found := false
	for _, s := range m.SpecsOf() {
		found = found || s.Name == "mcp_claude_claude_ask"
	}
	if !found {
		t.Fatalf("claude_ask not registered: %+v", m.SpecsOf())
	}
}
