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
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func stubRunner(t *testing.T, fn func(ctx context.Context, prompt string) (string, error)) {
	t.Helper()
	SetDelegateRunner(fn)
	t.Cleanup(func() { SetDelegateRunner(nil) })
}

func TestDelegateFusiona(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		return "resum de " + prompt, nil
	})
	out, _, err := Exec("delegate", `{"tasks":[{"prompt":"pista A"},{"prompt":"pista B"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"== subagent 1:", "resum de pista A", "== subagent 2:", "resum de pista B"} {
		if !strings.Contains(out, want) {
			t.Fatalf("fusió sense %q:\n%s", want, out)
		}
	}
}

func TestDelegateValidacio(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) { return "x", nil })
	for _, args := range []string{
		`{"tasks":[]}`,
		`{"tasks":[{"prompt":"  "}]}`,
		`{"tasks":[{"prompt":"a"},{"prompt":"b"},{"prompt":"c"},{"prompt":"d"},{"prompt":"e"}]}`,
		`no-json`,
	} {
		if _, _, err := Exec("delegate", args); err == nil {
			t.Fatalf("hauria de rebutjar: %q", args)
		}
	}
}

func TestDelegateErrorNoTrenca(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		if prompt == "mala" {
			return "", fmt.Errorf("boom")
		}
		return "bé", nil
	})
	out, _, err := Exec("delegate", `{"tasks":[{"prompt":"bona"},{"prompt":"mala"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "bé") || !strings.Contains(out, "== subagent 2: ERROR: boom") {
		t.Fatalf("fusió amb error incorrecta:\n%s", out)
	}
}

func TestDelegateParalel(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		time.Sleep(300 * time.Millisecond)
		return "fet", nil
	})
	t0 := time.Now()
	_, _, err := Exec("delegate", `{"tasks":[{"prompt":"a"},{"prompt":"b"},{"prompt":"c"},{"prompt":"d"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if el := time.Since(t0); el > 900*time.Millisecond {
		t.Fatalf("4×300ms en sèrie serien 1.2s; han estat %v (no és paral·lel)", el)
	}
}

func TestDelegateSenseRunner(t *testing.T) {
	SetDelegateRunner(nil)
	if _, _, err := Exec("delegate", `{"tasks":[{"prompt":"a"}]}`); err == nil {
		t.Fatal("sense runner ha de fallar, no inventar")
	}
}

// Sense recursió: els subagents reben Specs() (natives) i delegate només
// existeix a SpecsAll() (loop pare).
func TestDelegateSenseRecursio(t *testing.T) {
	for _, s := range Specs() {
		if s.Name == "delegate" {
			t.Fatal("Specs() no pot incloure delegate (recursió)")
		}
	}
	if len(Specs()) != 24 {
		t.Fatalf("natives=%d, han de ser 24", len(Specs()))
	}
	trobat := false
	for _, s := range SpecsAll() {
		if s.Name == "delegate" {
			trobat = true
		}
	}
	if !trobat {
		t.Fatal("SpecsAll() ha d'exposar delegate al loop pare")
	}
}

func TestDelegatePermisPerDefecte(t *testing.T) {
	if d, _ := DefaultPolicy().Decide(ModeCode, "delegate", `{}`); d != "allow" {
		t.Fatalf("delegate=%s, ha de passar sol (subagents read-only)", d)
	}
	q := &Policy{Tools: map[string]string{"delegate": "deny"}}
	if d, _ := q.Decide(ModeCode, "delegate", `{}`); d != "deny" {
		t.Fatal("l'override del config ha de poder vetar delegate")
	}
}

func TestDelegateParentCancellation(t *testing.T) {
	started := make(chan struct{})
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := ExecCtx(ctx, "test", t.TempDir(), "delegate", `{"tasks":[{"prompt":"inspect"}]}`)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("delegate did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("got %v, want canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("delegate ignored parent cancellation")
	}
}

func TestDelegateExecCtxPassesWorkspaceAndSession(t *testing.T) {
	type gotScope struct {
		delegateScope
		prompt string
	}
	got := make(chan gotScope, 1)
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		got <- gotScope{delegateScope: delegateScopeFromContext(ctx), prompt: prompt}
		return "done", nil
	})

	workspace := filepath.Clean(t.TempDir())
	_, _, err := ExecCtx(context.Background(), "session-workspace", workspace, "delegate", `{"tasks":[{"prompt":"inspect this project"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	call := <-got
	if call.session != "session-workspace" || call.dir != workspace || call.prompt != "inspect this project" {
		t.Fatalf("delegate scope lost: %+v", call)
	}
}

func TestDelegateUsesCallingWorkspaceAndConfigDeny(t *testing.T) {
	var requests atomic.Int32
	bodies := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		bodies <- string(raw)
		w.Header().Set("Content-Type", "application/json")
		if requests.Add(1)%2 == 1 {
			inner, _ := json.Marshal(map[string]string{"path": filepath.Join("unused", "private.txt")})
			args, _ := json.Marshal(string(inner))
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, args)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	cfg := testCfg(server.URL)
	cfg.Permissions = config.PermissionsCfg{Tools: map[string]string{"read": "deny"}}
	SetupDelegate(cfg, llm.New())
	t.Cleanup(func() { SetDelegateRunner(nil) })

	for _, marker := range []string{"WORKSPACE_ONE_ONLY", "WORKSPACE_TWO_ONLY"} {
		workspace := t.TempDir()
		if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte(marker), 0o600); err != nil {
			t.Fatal(err)
		}
		out, _, err := ExecCtx(context.Background(), "session", workspace, "delegate", `{"tasks":[{"prompt":"inspect workspace"}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "done") {
			t.Fatalf("unexpected delegate output: %q", out)
		}
		first, second := <-bodies, <-bodies
		if !strings.Contains(first, marker) || strings.Contains(first, "WORKSPACE_ONE_ONLY") && marker == "WORKSPACE_TWO_ONLY" {
			t.Fatalf("system prompt did not use current workspace %q: %s", marker, first)
		}
		if !strings.Contains(second, "EINA NO DISPONIBLE") || !strings.Contains(second, "permissions del config") || !strings.Contains(second, marker) {
			t.Fatalf("child did not retain workspace context/configured read denial: %s", second)
		}
	}
}

func TestDelegateReadonlyPolicyOverrides(t *testing.T) {
	pol := delegatePolicy(config.PermissionsCfg{
		Tools:     map[string]string{"read": "deny", "write": "allow", "browser": "allow", "bash": "allow"},
		BashAllow: []string{"rm "},
	})
	for _, tc := range []struct {
		name string
		args string
	}{
		{"write", `{"path":"out.txt","content":"x"}`},
		{"browser", `{"action":"eval","js":"document.body.innerHTML='x'"}`},
		{"bash", `{"command":"rm file.txt"}`},
	} {
		if decision, _ := pol.Decide(ModeInspect, tc.name, tc.args); decision != "deny" {
			t.Errorf("read-only policy allowed %s despite explicit allow: %s", tc.name, decision)
		}
	}
	if decision, _ := pol.Decide(ModeInspect, "read", `{"path":"safe.txt"}`); decision != "deny" {
		t.Fatalf("parent-configured deny was not retained: %s", decision)
	}
}

func TestDelegateSnapshotsRunnerPerCall(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		started <- struct{}{}
		<-release
		return "runner A: " + prompt, nil
	})

	done := make(chan string, 1)
	go func() {
		out, _ := delegateExec(context.Background(), `{"tasks":[{"prompt":"one"},{"prompt":"two"}]}`)
		done <- out
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner A did not start")
	}
	SetDelegateRunner(func(context.Context, string) (string, error) { return "runner B", nil })
	close(release)
	select {
	case out := <-done:
		if strings.Contains(out, "runner B") || strings.Count(out, "runner A:") != 2 {
			t.Fatalf("in-flight delegate used inconsistent runners: %s", out)
		}
	case <-time.After(time.Second):
		t.Fatal("delegate call did not finish")
	}
}
