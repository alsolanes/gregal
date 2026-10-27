package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
)

// TestDelegateOwnedFilesWriteInParallelModes: en mode code, un fill amb
// `files` escriu els seus fitxers sense demanar res; un fitxer que no és
// seu queda denegat, i fora dels modes que escriuen `files` no s'accepta.
func TestDelegateOwnedFilesWriteInParallelModes(t *testing.T) {
	run := func(t *testing.T, mode, files string) (string, string, error) {
		bodies := make(chan string, 8)
		server := fakeChild(t, bodies)
		t.Cleanup(server.Close)
		previousParallel := delegateParallelDefault.Load()
		SetupDelegate(testCfg(server.URL), llm.New())
		t.Cleanup(func() {
			delegateParallelDefault.Store(previousParallel)
			SetDelegateRunner(nil)
		})
		ws := t.TempDir()
		ctx := WithTurnMode(context.Background(), mode)
		out, _, err := ExecCtx(ctx, "s", ws, "delegate", `{"tasks":[{"prompt":"make a file","files":`+files+`}]}`)
		return out, ws, err
	}

	t.Run("own file is written", func(t *testing.T) {
		out, ws, err := run(t, ModeCode, `["made.txt"]`)
		if err != nil || !strings.Contains(out, "done") {
			t.Fatalf("out=%q err=%v", out, err)
		}
		if b, err := os.ReadFile(filepath.Join(ws, "made.txt")); err != nil || string(b) != "hi" {
			t.Fatalf("owned write must run: %q %v", b, err)
		}
	})
	t.Run("other file is denied", func(t *testing.T) {
		_, ws, err := run(t, ModeCode, `["other.txt"]`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(ws, "made.txt")); err == nil {
			t.Fatal("a file the subagent does not own must not be written")
		}
	})
	t.Run("read-only mode rejects files", func(t *testing.T) {
		if _, _, err := run(t, ModeInspect, `["made.txt"]`); err == nil || !strings.Contains(err.Error(), "mode that writes") {
			t.Fatalf("inspect must refuse files: %v", err)
		}
	})
}

func TestResolveOwnedFiles(t *testing.T) {
	ws := t.TempDir()
	if _, err := resolveOwnedFiles(ModeCode, ws, [][]string{{"a.go"}, {"./a.go"}}); err == nil || !strings.Contains(err.Error(), "both own") {
		t.Fatalf("overlap must be refused: %v", err)
	}
	if _, err := resolveOwnedFiles(ModeCode, ws, [][]string{{"../fora.go"}}); err == nil || !strings.Contains(err.Error(), "outside the project") {
		t.Fatalf("outside must be refused: %v", err)
	}
	got, err := resolveOwnedFiles(ModeAutonomous, ws, [][]string{{"web/index.html", "web/app.js"}, nil, {"web/app.css"}})
	if err != nil || len(got[0]) != 2 || got[1] != nil || len(got[2]) != 1 {
		t.Fatalf("got %v err %v", got, err)
	}
	if out, err := resolveOwnedFiles(ModeInspect, ws, [][]string{nil, nil}); err != nil || len(out) != 2 {
		t.Fatalf("read-only tasks stay valid in any mode: %v %v", out, err)
	}
	many := make([]string, MaxOwnedFiles+1)
	for i := range many {
		many[i] = filepath.Join("f", string(rune('a'+i))+".go")
	}
	if _, err := resolveOwnedFiles(ModeCode, ws, [][]string{many}); err == nil {
		t.Fatal("too many files must be refused")
	}
}

func TestPlanOwnedCall(t *testing.T) {
	ws := t.TempDir()
	owned := []string{filepath.Join(ws, "a.go")}
	pol := &Policy{ProjectDir: ws}
	call := func(name, args string) llm.ToolCall {
		c := llm.ToolCall{}
		c.Function.Name, c.Function.Arguments = name, args
		return c
	}
	if p := planOwnedCall(pol, owned, ws, call("write", `{"path":"a.go","content":"x"}`), CallPlan{}); p.Fixed != "" {
		t.Fatalf("own file: %+v", p)
	}
	if p := planOwnedCall(pol, owned, ws, call("edit", `{"path":"b.go"}`), CallPlan{}); !strings.Contains(p.Fixed, "not your file") {
		t.Fatalf("other file: %+v", p)
	}
	if p := planOwnedCall(pol, owned, ws, call("bash", `{"command":"npm install left-pad"}`), CallPlan{}); !strings.Contains(p.Fixed, "cannot run") {
		t.Fatalf("commands: %+v", p)
	}
	if p := planOwnedCall(pol, owned, ws, call("read", `{"path":"b.go"}`), CallPlan{}); p.Fixed != "" || p.Ask {
		t.Fatalf("reading stays allowed: %+v", p)
	}
}

func TestMissingFilesError(t *testing.T) {
	writes := []string{
		"Create money/convert.go and money/convert_test.go (package money).",
		"Project root: /x (Go module). Write ONLY text/markdown.go with Plain(md string).",
		"Implementa dates/holidays.go amb els festius.",
	}
	for _, p := range writes {
		if err := missingFilesError(ModeCode, []string{p}, [][]string{nil}); err == nil || !strings.Contains(err.Error(), "`files`") {
			t.Errorf("%q must ask for files: %v", p, err)
		}
		if err := missingFilesError(ModeCode, []string{p}, [][]string{{"x.go"}}); err != nil {
			t.Errorf("with files it is fine: %v", err)
		}
		if err := missingFilesError(ModeInspect, []string{p}, [][]string{nil}); err != nil {
			t.Errorf("read-only modes never complain: %v", err)
		}
	}
	reads := []string{
		"Read money/convert.go and find where to add rounding; report the line.",
		"Map the handlers in internal/web/*.go and list which ones write files.",
		"Search the docs for how to create a release; summarise.",
	}
	for _, p := range reads {
		if err := missingFilesError(ModeCode, []string{p}, [][]string{nil}); err != nil {
			t.Errorf("read-only task %q must pass: %v", p, err)
		}
	}
}
