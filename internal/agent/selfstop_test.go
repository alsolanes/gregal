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
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestSelfStopHardDenySurvivesAllShellGrantsAndPlanning(t *testing.T) {
	pol := &Policy{Tools: map[string]string{"bash": "allow", "bash_background": "allow"}, BashAllow: []string{"systemctl ", "sh "}}
	for _, mode := range []string{ModeCode, ModeAutonomous, ModeInspect, ModeChat, ModeGoal} {
		for _, tool := range []string{"bash", "bash_background"} {
			for _, command := range []string{
				`systemctl --user stop gregal-telegram`,
				`echo before; systemctl restart --user gregal-telegram.service`,
				`systemctl disable --now gregal-web.service`,
				`sh -c 'systemctl kill --user gregal-telegram.service'`,
			} {
				args, _ := json.Marshal(map[string]string{"command": command})
				if decision, reason := pol.Decide(mode, tool, string(args)); decision != "deny" {
					t.Errorf("%s/%s permitted self-stop: %s (%s)", mode, tool, decision, reason)
				}
				call := llm.ToolCall{ID: "selfstop"}
				call.Function.Name, call.Function.Arguments = tool, string(args)
				plan := PlanCalls(pol, mode, nil, []llm.ToolCall{call})[0]
				if plan.Fixed == "" || plan.Ask {
					t.Errorf("%s/%s reached approval or execution: %+v", mode, tool, plan)
				}
			}
		}
	}
	for _, command := range []string{`systemctl status gregal-telegram`, `systemctl show gregal-telegram`, `systemctl restart nginx.service`} {
		args, _ := json.Marshal(map[string]string{"command": command})
		if decision, reason := pol.Decide(ModeCode, "bash", string(args)); decision != "allow" {
			t.Errorf("existing explicit grant changed for %q: %s (%s)", command, decision, reason)
		}
	}
}

func TestExecCtxShellAliasesCannotBypassSelfStopGuard(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake executable")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "spawned")
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte("#!/bin/sh\necho executed >> '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, tool := range []string{"bash", "bash_background"} {
		_, _, err := ExecCtx(context.Background(), "selfstop-test", dir, tool, `{"command":"echo before > spawned; systemctl --user stop gregal-telegram.service"}`)
		if err == nil || !strings.Contains(err.Error(), "self-stop protection") {
			t.Errorf("%s bypassed execution guard: %v", tool, err)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("blocked command spawned a shell: %v", err)
	}
}

func TestSetupDelegateRejectsSelfStopDespiteInheritedAllow(t *testing.T) {
	var requests atomic.Int32
	bodies := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies <- string(raw)
		w.Header().Set("Content-Type", "application/json")
		if requests.Add(1) == 1 {
			args, _ := json.Marshal(`{"command":"systemctl --user stop gregal-telegram.service"}`)
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"bash","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, args)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"guarded"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	cfg := testCfg(server.URL)
	cfg.Permissions = config.PermissionsCfg{Tools: map[string]string{"bash": "allow", "bash_background": "allow"}, BashAllow: []string{"systemctl "}}
	SetupDelegate(cfg, llm.New())
	t.Cleanup(func() { SetDelegateRunner(nil) })
	out, _, err := ExecCtx(context.Background(), "selfstop-delegate", t.TempDir(), "delegate", `{"tasks":[{"prompt":"inspect"}]}`)
	if err != nil || !strings.Contains(out, "guarded") {
		t.Fatalf("delegate failed: %q %v", out, err)
	}
	<-bodies
	second := <-bodies
	if !strings.Contains(second, "EINA NO DISPONIBLE") || !strings.Contains(second, "self-stop protection") {
		t.Fatalf("delegate did not hard-deny self-stop before execution: %s", second)
	}
}
