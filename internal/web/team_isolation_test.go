package web

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/team"
)

func TestCrewAssignmentValidationAndIsolation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src/main.go"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src/run.sh"), []byte("#!/bin/sh\nprintf changed > src/main.go\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module isolation.test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build/build.go"), []byte("package build\nfunc Value() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build/build_test.go"), []byte("package build\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value()!=42 { t.Fatal(Value()) } }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := &team.FlowSpec{Nodes: []team.FlowNode{{ID: "build", Role: "builder", CrewAssigned: true, Files: []string{"src/main.go", "src/run.sh"}}}}
	workspaces, err := PrepareCrewAssignments(root, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer workspaces.Close()
	private, ok, err := workspaces.Workspace("build")
	if err != nil || !ok {
		t.Fatalf("private workspace=(%q,%v), err=%v", private, ok, err)
	}
	if private == root {
		t.Fatal("assigned builder received original project")
	}
	if st, err := os.Stat(filepath.Join(private, "src/run.sh")); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("script executable bit lost: mode=%v err=%v", st, err)
	}
	if runtime.GOOS == "linux" {
		boundary := crewToolBoundary{workspace: private, allowed: []string{"src/main.go", "src/run.sh"}}
		out, _, err := boundary.shell(context.Background(), "bash", "./src/run.sh")
		if err != nil {
			t.Fatalf("confined script failed: %v (%s)", err, out)
		}
		out, _, err = boundary.shell(context.Background(), "bash", "CGO_ENABLED=0 go test ./build")
		if err != nil {
			t.Fatalf("confined project build/test failed: %v (%s)", err, out)
		}
		got, _ := os.ReadFile(filepath.Join(root, "src/main.go"))
		if string(got) != "original" {
			t.Fatalf("shell changed original project: %q", got)
		}
		if err := workspaces.Integrate("build", nil, nil, func(string) error { return nil }); err != nil {
			t.Fatal(err)
		}
		got, _ = os.ReadFile(filepath.Join(root, "src/main.go"))
		if string(got) != "changed" {
			t.Fatalf("integrated file=%q", got)
		}
		if st, err := os.Stat(filepath.Join(root, "src/run.sh")); err != nil || st.Mode().Perm()&0o111 == 0 {
			t.Fatalf("integrated executable bit lost: mode=%v err=%v", st, err)
		}
		if got, _ := os.ReadFile(filepath.Join(root, "other.txt")); string(got) != "untouched" {
			t.Fatalf("unassigned file changed: %q", got)
		}
	}
}

func TestCrewAssignmentRejectsInvalidTargetsAndOverlaps(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "alias")); err != nil {
		t.Skip(err)
	}
	for _, files := range [][]string{{"../secret"}, {filepath.Join(root, "secret")}, {"alias/secret"}} {
		_, err := PrepareCrewAssignments(root, &team.FlowSpec{Nodes: []team.FlowNode{{ID: "b", Role: "builder", CrewAssigned: true, Files: files}}})
		if err == nil {
			t.Errorf("accepted invalid assigned path %q", files)
		}
	}
	for _, files := range [][]string{{"x", "x"}, {"x", "x/y"}} {
		_, err := PrepareCrewAssignments(root, &team.FlowSpec{Nodes: []team.FlowNode{{ID: "a", Role: "builder", CrewAssigned: true, Files: files}}})
		if err == nil {
			t.Errorf("accepted overlapping assignments %q", files)
		}
	}
	_, err := PrepareCrewAssignments(root, &team.FlowSpec{Nodes: []team.FlowNode{
		{ID: "a", Role: "builder", CrewAssigned: true, Files: []string{"x"}},
		{ID: "b", Role: "builder", CrewAssigned: true, Files: []string{"x/y"}},
	}})
	if err == nil {
		t.Fatal("accepted ownership overlap between builders")
	}
}

func TestCrewIntegrationConflictAndDeny(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "owned.txt")
	if err := os.WriteFile(path, []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := &team.FlowSpec{Nodes: []team.FlowNode{{ID: "b", Role: "builder", CrewAssigned: true, Files: []string{"owned.txt"}}}}
	ws, err := PrepareCrewAssignments(root, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	private, _, _ := ws.Workspace("b")
	if err := os.WriteFile(filepath.Join(private, "owned.txt"), []byte("candidate"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ws.Integrate("b", func(string) bool { return true }, nil, nil); err == nil {
		t.Fatal("user deny did not block integration")
	}
	if err := os.WriteFile(path, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ws.Integrate("b", nil, nil, nil); err == nil {
		t.Fatal("baseline conflict was not detected")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "external" {
		t.Fatalf("conflict overwrote external data: %q", got)
	}
}

func TestCrewDirectToolBoundaryRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "owned"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(root, "link")); err != nil {
		t.Skip(err)
	}
	b := crewToolBoundary{workspace: root, allowed: []string{"owned"}}
	args := func(v any) string { x, _ := json.Marshal(v); return string(x) }
	for _, tc := range []struct{ name, args string }{
		{"write", args(map[string]any{"path": "unowned", "content": "bad"})},
		{"write", args(map[string]any{"path": "../outside", "content": "bad"})},
		{"grep", args(map[string]any{"pattern": "x", "dir": "../../etc"})},
		{"grep", args(map[string]any{"pattern": "x", "dir": "/etc"})},
		{"glob", args(map[string]any{"pattern": "../../etc/passwd"})},
		{"read", args(map[string]any{"path": "link/passwd"})},
	} {
		if _, _, err := b.execute(context.Background(), tc.name, tc.args); err == nil {
			t.Errorf("%s accepted escaping args %s", tc.name, tc.args)
		}
	}
	got, _ := os.ReadFile(filepath.Join(root, "owned"))
	if string(got) != "old" {
		t.Fatalf("rejected write changed target: %q", got)
	}
}

func TestCrewReviewerShellIsReadOnlyAndNetworkless(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("bubblewrap isolation is Linux-only")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap unavailable")
	}
	root := t.TempDir()
	marker := filepath.Join(root, "host-marker")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := crewToolBoundary{workspace: root, readOnly: true}
	if _, _, err := b.shell(context.Background(), "bash", "printf changed > /work/host-marker"); err == nil {
		t.Fatal("review shell wrote to its read-only snapshot")
	}
	got, _ := os.ReadFile(marker)
	if string(got) != "unchanged" {
		t.Fatalf("host file changed: %q", got)
	}
	home, _ := os.UserHomeDir()
	out, _, err := b.shell(context.Background(), "bash", "if test -e "+shellQuote(home)+"; then echo visible; else echo hidden; fi")
	if err != nil || strings.Contains(out, "visible") { t.Fatalf("host home was visible to reviewer shell: %q err=%v", out, err) }
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr := listener.Addr().String()
	host, port, _ := strings.Cut(addr, ":")
	cmd := "python3 -c 'import socket; s=socket.socket(); s.settimeout(1); s.connect((\"" + host + "\", " + port + ")); print(\"connected\")'"
	out, _, err = b.shell(context.Background(), "bash", cmd)
	if err == nil && strings.Contains(out, "connected") {
		t.Fatal("review shell reached host network")
	}
}

func TestAssignedTeamFlowCannotWriteOriginalProject(t *testing.T) {
	root := t.TempDir()
	owned := filepath.Join(root, "owned.txt")
	hostFile := filepath.Join(root, "must-stay.txt")
	if err := os.WriteFile(owned, []byte("base"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(hostFile, []byte("host"), 0o644); err != nil { t.Fatal(err) }
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			args, _ := json.Marshal(map[string]string{"command": "printf compromised > " + shellQuote(hostFile)})
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"tool_calls": []any{map[string]any{"id":"shell","type":"function","function":map[string]any{"name":"bash","arguments":string(args)}}}}, "finish_reason":"tool_calls"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content":"finished"}, "finish_reason":"stop"}}})
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL}}
	role := config.Role{Provider:"p", Model:"m", MaxTokens:1024, ContextWindow:8192}
	s.mu.Lock(); s.permissive = true; s.mu.Unlock()
	flow := team.FlowSpec{Nodes: []team.FlowNode{{ID:"build", Role:"builder", Instruction:"build", CrewAssigned:true, Files:[]string{"owned.txt"}}}, Output:"build"}
	var shellFailure bool
	answer := s.runTeamFlow(context.Background(), teamFlowRun{task:"try a shell write", flow:&flow, withTools:true, workspace:root, subagents:map[string]*agent.SubagentDef{}, cfg:s.cfg, client:llm.New(), provider:s.cfg.Providers["p"], role:role, started:time.Now()}, func(kind string, value any) {
		if e, ok := value.(team.Event); ok && e.Type == "tool_result" && strings.Contains(strings.ToLower(e.Message), "confined shell failed") { shellFailure = true }
	})
	if answer == "" { t.Fatal("isolated team flow failed before delivering") }
	if !shellFailure { t.Fatal("runTeamFlow did not route the actual shell call through confinement") }
	got, _ := os.ReadFile(hostFile)
	if string(got) != "host" { t.Fatalf("model shell changed original project: %q", got) }
}

func TestCrewWorkspaceSetupFailureDoesNotFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := PrepareCrewAssignments(root, &team.FlowSpec{Nodes: []team.FlowNode{{ID: "b", Role: "builder", CrewAssigned: true, Files: []string{"ok"}}, {ID: "r", Role: "reviewer"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.seats["r"].dir = filepath.Join(root, "ok")
	// The lazy reviewer snapshot sees filesystem errors as errors, not a missing
	// workspace that could silently fall back to the original project.
	_, _, err = w.Workspace("r")
	if err == nil {
		t.Fatal("expected snapshot failure")
	}
}
