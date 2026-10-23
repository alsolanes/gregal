package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"gregal/internal/llm"
)

func TestParseSubagent(t *testing.T) {
	def, err := ParseSubagent("---\nname: Reviewer-1\ndescription: Reviews  diffs\nmodel: other/m2\ntools: [read, grep, delegate, nope]\nmode: code\nmax_steps: 99\n---\nBe strict.\n")
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "reviewer-1" || def.Description != "Reviews diffs" || def.Mode != ModeCode || def.MaxSteps != maxSubagentSteps || def.Prompt != "Be strict." {
		t.Fatalf("def=%+v", def)
	}
	if strings.Join(def.Tools, ",") != "read,grep" || len(def.Warnings) != 3 {
		t.Fatalf("tools=%v warnings=%v", def.Tools, def.Warnings)
	}
	scalar, err := ParseSubagent("---\nname: a\ndescription: d\ntools: read, glob\n---\nbody")
	if err != nil || strings.Join(scalar.Tools, ",") != "read,glob" || scalar.Mode != ModeInspect || scalar.MaxSteps != 12 {
		t.Fatalf("scalar=%+v err=%v", scalar, err)
	}
	for _, bad := range []string{
		"no front matter",
		"---\nname: x\n---\nbody",                     // sense descripció
		"---\nname: Bad Name\ndescription: d\n---\nb", // nom invàlid
		"---\nname: x\ndescription: d\ntools: [delegate]\n---\nb",
	} {
		if _, err := ParseSubagent(bad); err == nil {
			t.Errorf("should reject %q", bad)
		}
	}
}

func writeSubagent(t *testing.T, dir, name, body string) {
	t.Helper()
	d := filepath.Join(dir, ".gregal", "agents")
	if err := os.MkdirAll(d, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, name+".md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSubagentsAndIndex(t *testing.T) {
	ws := t.TempDir()
	writeSubagent(t, ws, "zz-scout", "---\nname: zz-scout\ndescription: Finds call sites\n---\nLook around.")
	writeSubagent(t, ws, "broken", "not an agent")
	defs := LoadSubagents(ws)
	def, ok := FindSubagent(defs, "ZZ-Scout")
	if !ok || def.Source != "project" {
		t.Fatalf("defs=%+v", defs)
	}
	if _, ok := FindSubagent(defs, "broken"); ok {
		t.Fatal("invalid file must be ignored")
	}
	idx := ContextProjecte(ws)
	if !strings.Contains(idx, "SUBAGENTS") || !strings.Contains(idx, "- zz-scout (inspect): Finds call sites") {
		t.Fatalf("index missing from project context:\n%s", idx)
	}
}

// fakeChild és un proveïdor que demana una escriptura i després acaba.
func fakeChild(t *testing.T, bodies chan<- string) *httptest.Server {
	var n atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		bodies <- string(raw)
		w.Header().Set("Content-Type", "application/json")
		if n.Add(1) == 1 {
			inner, _ := json.Marshal(map[string]string{"path": "made.txt", "content": "hi"})
			args, _ := json.Marshal(string(inner))
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, args)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`)
	}))
}

func TestDelegateDefinedSubagentWritesOnlyWithApproval(t *testing.T) {
	for _, approve := range []bool{false, true} {
		t.Run(fmt.Sprint("approver=", approve), func(t *testing.T) {
			bodies := make(chan string, 8)
			server := fakeChild(t, bodies)
			defer server.Close()
			previousParallel := delegateParallelDefault.Load()
			SetupDelegate(testCfg(server.URL), llm.New())
			t.Cleanup(func() {
				delegateParallelDefault.Store(previousParallel)
				SetDelegateRunner(nil)
			})
			ws := t.TempDir()
			writeSubagent(t, ws, "writer", "---\nname: writer\ndescription: Writes files\nmode: code\ntools: [read, write]\nmax_steps: 3\n---\nWrite what is asked.")
			var mu sync.Mutex
			var agents []string
			ctx := WithDelegateHook(context.Background(), func(e DelegateEvent) {
				mu.Lock()
				agents = append(agents, e.Agent)
				mu.Unlock()
			})
			asked := 0
			if approve {
				ctx = WithDelegateApprover(ctx, func(_ context.Context, call llm.ToolCall, _ string) bool {
					asked++
					return call.Function.Name == "write"
				})
			}
			out, _, err := ExecCtx(ctx, "s", ws, "delegate", `{"tasks":[{"prompt":"make a file","agent":"writer"}]}`)
			if err != nil || !strings.Contains(out, "done") {
				t.Fatalf("out=%q err=%v", out, err)
			}
			first, second := <-bodies, <-bodies
			if !strings.Contains(first, `"name":"write"`) || strings.Contains(first, `"name":"grep"`) || !strings.Contains(first, "Write what is asked.") {
				t.Fatalf("child request must carry only the allowlisted tools and the prompt: %s", first)
			}
			_, statErr := os.Stat(filepath.Join(ws, "made.txt"))
			if approve {
				if asked != 1 || statErr != nil {
					t.Fatalf("approved write must run (asked=%d, stat=%v)", asked, statErr)
				}
			} else {
				if statErr == nil || !strings.Contains(second, "read-only subagent") {
					t.Fatalf("without approver the subagent stays read-only: %s", second)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if len(agents) == 0 || agents[0] != "writer" {
				t.Fatalf("delegate events must name the subagent: %v", agents)
			}
		})
	}
}

func TestDelegateUnknownSubagent(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) { return "x", nil })
	ws := t.TempDir()
	writeSubagent(t, ws, "known", "---\nname: known\ndescription: d\n---\nb")
	_, _, err := ExecCtx(context.Background(), "s", ws, "delegate", `{"tasks":[{"prompt":"p","agent":"ghost"}]}`)
	if err == nil || !strings.Contains(err.Error(), "known") {
		t.Fatalf("unknown agent must list the defined ones: %v", err)
	}
}

func TestRunOptsToolsAndApprove(t *testing.T) {
	bodies := make(chan string, 8)
	server := fakeChild(t, bodies)
	defer server.Close()
	cfg := testCfg(server.URL)
	ws := t.TempDir()
	var asked []string
	res, err := RunNonInteractiveOpts(context.Background(), llm.New(), cfg, RunOpts{
		Task: "make a file please", Mode: ModeCode, MaxSteps: 4, Dir: ws,
		Tools: []string{"read", "write"}, System: "TEAM ROLE builder",
		Approve: func(_ context.Context, call llm.ToolCall, _ string) bool {
			asked = append(asked, call.Function.Name)
			return true
		},
	})
	if err != nil || res.Answer != "done" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	first := <-bodies
	if strings.Contains(first, `"name":"grep"`) || !strings.Contains(first, "TEAM ROLE builder") {
		t.Fatalf("tools/system not applied: %s", first)
	}
	if _, err := os.Stat(filepath.Join(ws, "made.txt")); err != nil {
		t.Fatalf("write inside the project should pass under the code policy: %v (asked=%v)", err, asked)
	}
}
