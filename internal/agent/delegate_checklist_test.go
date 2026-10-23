package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

type delegateChecklistTranscript struct {
	prompt  string
	results map[string]string
	global  []tools.TodoItem
}

// Drive the real SetupDelegate loop; only the remote LLM is scripted.
func setupDelegateChecklistLLM(t *testing.T, permissions config.PermissionsCfg, script func(string, int) []llm.ToolCall, beforeStep func(context.Context, int)) <-chan delegateChecklistTranscript {
	t.Helper()
	transcripts := make(chan delegateChecklistTranscript, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var request struct {
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode LLM request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		transcript := delegateChecklistTranscript{results: make(map[string]string)}
		step := 0
		for _, message := range request.Messages {
			switch message.Role {
			case "user":
				transcript.prompt = message.Content
			case "assistant":
				step++
			case "tool":
				transcript.results[message.ToolCallID] = message.Content
			}
		}
		if beforeStep != nil {
			beforeStep(r.Context(), step)
		}
		calls := script(transcript.prompt, step)
		message := llm.Message{Role: "assistant", ToolCalls: calls}
		reason := "tool_calls"
		if len(calls) == 0 {
			message.Content = "done"
			reason = "stop"
			transcript.global = tools.TodoList()
			transcripts <- transcript
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": reason}}}); err != nil {
			t.Errorf("encode LLM response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	previousRunner := currentDelegateRunner()
	previousParallel := delegateParallelDefault.Load()
	cfg := testCfg(server.URL)
	cfg.Permissions = permissions
	SetupDelegate(cfg, llm.New())
	// El config de prova apunta a un servidor local (ParallelSubagents=1);
	// aquests tests necessiten les tasques alhora per provar l'aïllament.
	delegateParallelDefault.Store(int32(MaxDelegateTasks))
	t.Cleanup(func() {
		delegateParallelDefault.Store(previousParallel)
		SetDelegateRunner(previousRunner)
	})
	return transcripts
}

func delegateChecklistCall(id, name, args string) llm.ToolCall {
	call := llm.ToolCall{ID: id, Type: "function"}
	call.Function.Name = name
	call.Function.Arguments = args
	return call
}

func seedDelegateGlobalChecklist(t *testing.T) []tools.TodoItem {
	t.Helper()
	previous := tools.TodoList()
	t.Cleanup(func() { tools.TodoSet(previous) })
	tools.TodoSet([]tools.TodoItem{
		{Title: "GLOBAL_WORKING", Status: "working"},
		{Title: "GLOBAL_PENDING", Status: "pending"},
	})
	return tools.TodoList()
}

func delegateChecklistScript(prompt string, step int) []llm.ToolCall {
	switch step {
	case 0:
		return []llm.ToolCall{delegateChecklistCall("empty", "todoread", `{}`)}
	case 1:
		args, _ := json.Marshal(map[string]any{"items": []tools.TodoItem{
			{Title: "  " + prompt + " finished  ", Status: "completed"},
			{Title: prompt + " active", Status: "in_progress"},
			{Title: "  ", Status: "pending"},
		}})
		return []llm.ToolCall{delegateChecklistCall("write", "todowrite", string(args))}
	case 2:
		// Consecutive reads also exercise RunCalls' parallel-safe batch.
		return []llm.ToolCall{
			delegateChecklistCall("read1", "todoread", `{}`),
			delegateChecklistCall("read2", "todoread", `{}`),
			delegateChecklistCall("file", "read", `{"path":"safe.txt"}`),
		}
	default:
		return nil
	}
}

func checkDelegateChecklistTranscript(t *testing.T, transcript delegateChecklistTranscript, global []tools.TodoItem) {
	t.Helper()
	if got := transcript.results["empty"]; got != "cap todo (todowrite buit)" {
		t.Errorf("%s initial todoread = %q; want empty local checklist", transcript.prompt, got)
	}
	want := "todos 1/2:\n✓ " + transcript.prompt + " finished\n◌ " + transcript.prompt + " active"
	for _, id := range []string{"write", "read1", "read2"} {
		if got := transcript.results[id]; got != want {
			t.Errorf("%s %s = %q; want own normalized checklist %q", transcript.prompt, id, got, want)
		}
	}
	if got := transcript.results["file"]; !strings.Contains(got, "SAFE_FILE_CONTENT") || !strings.Contains(got, "safe.txt") {
		t.Errorf("non-todo read did not retain ExecCtx behavior: %q", got)
	}
	if !reflect.DeepEqual(transcript.global, global) {
		t.Errorf("%s changed process-global checklist: got %+v, want %+v", transcript.prompt, transcript.global, global)
	}
}

func delegateChecklistWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "safe.txt"), []byte("SAFE_FILE_CONTENT"), 0o600); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestDelegateChecklistFreshPerInvocation(t *testing.T) {
	global := seedDelegateGlobalChecklist(t)
	transcripts := setupDelegateChecklistLLM(t, config.PermissionsCfg{}, delegateChecklistScript, nil)
	workspace := delegateChecklistWorkspace(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Reuse the same configured runner and session: each invocation must reset
	// only its own checklist, never inherit a preceding job or the global list.
	for _, prompt := range []string{"FIRST_JOB", "SECOND_JOB"} {
		args, _ := json.Marshal(map[string]any{"tasks": []any{map[string]string{"prompt": prompt}}})
		out, _, err := ExecCtx(ctx, "same-session", workspace, "delegate", string(args))
		if err != nil || !strings.Contains(out, "done") {
			t.Fatalf("delegate output=%q err=%v", out, err)
		}
		checkDelegateChecklistTranscript(t, <-transcripts, global)
	}
	if got := tools.TodoList(); !reflect.DeepEqual(got, global) {
		t.Errorf("global checklist after separate jobs = %+v; want %+v", got, global)
	}
}

func TestDelegateChecklistSimultaneousJobsIsolated(t *testing.T) {
	global := seedDelegateGlobalChecklist(t)
	var writes atomic.Int32
	bothWritten := make(chan struct{})
	transcripts := setupDelegateChecklistLLM(t, config.PermissionsCfg{}, delegateChecklistScript, func(ctx context.Context, step int) {
		if step != 2 {
			return
		}
		// Both writes must finish before either job reads. A global (or runner-
		// scoped) store deterministically gives at least one job the wrong list.
		if writes.Add(1) == 2 {
			close(bothWritten)
		}
		select {
		case <-bothWritten:
		case <-ctx.Done():
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _, err := ExecCtx(ctx, "same-session", delegateChecklistWorkspace(t), "delegate", `{"tasks":[{"prompt":"JOB_A"},{"prompt":"JOB_B"}]}`)
	if err != nil || strings.Count(out, "done") != 2 {
		t.Fatalf("delegate output=%q err=%v", out, err)
	}
	if writes.Load() != 2 {
		t.Fatalf("did not synchronize two real delegate jobs: %d", writes.Load())
	}
	seen := make(map[string]bool)
	for i := 0; i < 2; i++ {
		transcript := <-transcripts
		seen[transcript.prompt] = true
		checkDelegateChecklistTranscript(t, transcript, global)
	}
	if !seen["JOB_A"] || !seen["JOB_B"] {
		t.Fatalf("missing delegate transcripts: %v", seen)
	}
	if got := tools.TodoList(); !reflect.DeepEqual(got, global) {
		t.Errorf("global checklist after simultaneous jobs = %+v; want %+v", got, global)
	}
}
