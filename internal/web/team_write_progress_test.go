package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/team"
)

func TestTeamWriteProgressPrivateAndPreview(t *testing.T) {
	ws := t.TempDir()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := calls.Add(1)
		names := []string{"main.go", "script.py", "index.html"}
		if int(n) <= len(names) {
			args, _ := json.Marshal(map[string]string{"path": names[n-1], "content": "hello"})
			fmt.Fprintf(w, `{"choices":[{"message":{"tool_calls":[{"id":"w%d","type":"function","function":{"name":"write","arguments":%q}}]},"finish_reason":"tool_calls"}]}`, n, args)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: server.URL}}
	s.permissive = true
	role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}
	var progress, public []string
	ctx := teamWithWriteProgress(context.Background(), func(path string) error { progress = append(progress, path); return nil })
	_, _, err := s.runTeamSeat(ctx, s.cfg, role, teamSeat{Agentic: true, Mode: agent.ModeCode, Tools: []string{"write"}, MaxSteps: 8}, ws, "English", "builder", "write files", false, func(_ string, v any) {
		if e, ok := v.(team.Event); ok && e.Type == "tool_result" && e.Path != "" {
			public = append(public, e.Path)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.go", "script.py", "index.html"} {
		if _, err := os.Stat(filepath.Join(ws, name)); err != nil {
			t.Fatal(err)
		}
	}
	if fmt.Sprint(progress) != "[main.go script.py index.html]" {
		t.Errorf("private progress = %v", progress)
	}
	if fmt.Sprint(public) != "[index.html]" {
		t.Errorf("public preview paths = %v", public)
	}
}

func TestTeamWriteProgressConcurrentSeats(t *testing.T) {
	ws := t.TempDir()
	var mu sync.Mutex
	var paths []string
	ctx := teamWithWriteProgress(context.Background(), func(path string) error {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, path)
		return nil
	})
	var wg sync.WaitGroup
	for _, file := range []string{"a.go", "b.py"} {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if calls.Add(1) == 1 {
				args, _ := json.Marshal(map[string]string{"path": file, "content": "ok"})
				fmt.Fprintf(w, `{"choices":[{"message":{"tool_calls":[{"id":"w","type":"function","function":{"name":"write","arguments":%q}}]},"finish_reason":"tool_calls"}]}`, args)
				return
			}
			fmt.Fprint(w, `{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}]}`)
		}))
		defer srv.Close()
		s := goalTestServer(t)
		s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: srv.URL}}
		s.permissive = true
		role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.runTeamSeat(ctx, s.cfg, role, teamSeat{Agentic: true, Mode: agent.ModeCode, Tools: []string{"write"}, MaxSteps: 4}, ws, "English", "builder", "write", false, func(string, any) {})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(paths) != 2 {
		t.Fatalf("concurrent writes: %v", paths)
	}
}

func TestTeamWritePathNeverReturnsEscapingRelativePath(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skip(err)
	}
	args, _ := json.Marshal(map[string]string{"path": filepath.Join(real, "secret.go")})
	if path := teamWritePath(alias, "write", string(args)); path != "" && !filepath.IsLocal(path) {
		t.Fatalf("escaping path %q", path)
	}
}

func TestTeamWriteProgressResults(t *testing.T) {
	for _, tc := range []struct {
		name, tool, target     string
		args                   map[string]any
		deny, outside, success bool
	}{
		{name: "edit go", tool: "edit", target: "a.go", args: map[string]any{"old_string": "old", "new_string": "new"}, success: true},
		{name: "patch python", tool: "patch", target: "a.py", args: map[string]any{"edits": []map[string]string{{"old": "old", "new": "new"}}}, success: true},
		{name: "failed edit", tool: "edit", target: "bad.html", args: map[string]any{"old_string": "missing", "new_string": "new"}},
		{name: "denied write", tool: "write", target: "denied.html", args: map[string]any{"content": "new"}, deny: true},
		{name: "outside edit", tool: "edit", target: "outside.go", args: map[string]any{"old_string": "old", "new_string": "new"}, outside: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			target := filepath.Join(ws, tc.target)
			if tc.outside {
				target = filepath.Join(t.TempDir(), tc.target)
			}
			if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			tc.args["path"] = target
			args, _ := json.Marshal(tc.args)
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if calls.Add(1) == 1 {
					fmt.Fprintf(w, `{"choices":[{"message":{"tool_calls":[{"id":"one","type":"function","function":{"name":%q,"arguments":%q}}]},"finish_reason":"tool_calls"}]}`, tc.tool, args)
					return
				}
				fmt.Fprint(w, `{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}]}`)
			}))
			defer srv.Close()
			s := goalTestServer(t)
			s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: srv.URL}}
			if tc.deny {
				s.cfg.Permissions.Tools = map[string]string{"write": "deny"}
			}
			s.permissive = true
			role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}
			var paths []string
			ctx := teamWithWriteProgress(context.Background(), func(path string) error { paths = append(paths, path); return nil })
			var public []string
			_, _, err := s.runTeamSeat(ctx, s.cfg, role, teamSeat{Agentic: true, Mode: agent.ModeCode, Tools: []string{tc.tool}, MaxSteps: 4}, ws, "English", "builder", "edit file", false, func(_ string, v any) {
				if e, ok := v.(team.Event); ok && e.Path != "" {
					public = append(public, e.Path)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.success && fmt.Sprint(paths) != "["+tc.target+"]" {
				t.Errorf("progress=%v", paths)
			}
			if !tc.success && len(paths) != 0 {
				t.Errorf("unexpected progress=%v", paths)
			}
			if len(public) != 0 {
				t.Errorf("unexpected preview=%v", public)
			}
			got, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if tc.success && string(got) != "new" {
				t.Errorf("file=%q", got)
			}
			if tc.deny && string(got) != "old" {
				t.Errorf("denied write changed file=%q", got)
			}
		})
	}
}
