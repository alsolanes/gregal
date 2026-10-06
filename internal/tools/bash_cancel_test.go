package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/shell"
)

func TestBashCtxKillsDescendantOnCancelAndTimeout(t *testing.T) {
	cases := []struct {
		name    string
		cancel  bool
		timeout time.Duration
	}{
		{name: "cancel", cancel: true, timeout: 10 * time.Second},
		{name: "timeout", timeout: 8 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ready := filepath.Join(dir, "child.ready")
			release := filepath.Join(dir, "child.release")
			completed := filepath.Join(dir, "child.completed")
			command := bashMarkerCommand(t, dir, ready, release, completed)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type result struct {
				out string
				err error
			}
			done := make(chan result, 1)
			go func() {
				out, err := BashCtx(ctx, dir, command, tc.timeout)
				done <- result{out: out, err: err}
			}()

			readyBy := time.Now().Add(tc.timeout - 2*time.Second)
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				select {
				case res := <-done:
					t.Fatalf("bash ended before the child was ready: output=%q error=%v", res.out, res.err)
				default:
				}
				if time.Now().After(readyBy) {
					t.Fatal("child did not signal readiness before cancellation/timeout")
				}
				time.Sleep(10 * time.Millisecond)
			}

			if tc.cancel {
				cancel()
			}
			select {
			case res := <-done:
				if res.err == nil {
					t.Fatal("bash should report cancellation or timeout")
				}
				if tc.cancel && !strings.Contains(res.err.Error(), "aturada") {
					t.Fatalf("cancellation error = %v", res.err)
				}
				if !tc.cancel && !strings.Contains(res.err.Error(), "timeout (") {
					t.Fatalf("timeout error = %v", res.err)
				}
			case <-time.After(tc.timeout + 2*time.Second):
				t.Fatal("bash did not return after cancellation/timeout")
			}

			if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
				t.Fatalf("release descendant: %v", err)
			}
			time.Sleep(time.Second)
			if _, err := os.Stat(completed); err == nil {
				t.Fatal("descendant wrote its completion marker after cancellation/timeout")
			} else if !os.IsNotExist(err) {
				t.Fatalf("check completion marker: %v", err)
			}
		})
	}
}

// The test binary acts as a real Go child process launched by the shell.
func TestBashCtxDescendantMarkerHelper(t *testing.T) {
	if os.Getenv("GREGAL_BASH_TREE_CHILD") != "1" {
		return
	}
	ready := os.Getenv("GREGAL_BASH_TREE_READY")
	release := os.Getenv("GREGAL_BASH_TREE_RELEASE")
	completed := os.Getenv("GREGAL_BASH_TREE_COMPLETED")
	if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
		t.Fatalf("write ready marker: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			if err := os.WriteFile(completed, []byte("completed"), 0o600); err != nil {
				t.Fatalf("write completion marker: %v", err)
			}
			return
		} else if !os.IsNotExist(err) {
			t.Fatalf("check release marker: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func bashMarkerCommand(t *testing.T, dir, ready, release, completed string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if shell.IsPOSIX() {
		return fmt.Sprintf("GREGAL_BASH_TREE_CHILD=1 GREGAL_BASH_TREE_READY=%s GREGAL_BASH_TREE_RELEASE=%s GREGAL_BASH_TREE_COMPLETED=%s %s -test.run=TestBashCtxDescendantMarkerHelper & wait",
			quoteShell(ready), quoteShell(release), quoteShell(completed), quoteShell(executable))
	}
	const scriptName = "run-marker.cmd"
	script := fmt.Sprintf("@echo off\r\nset \"GREGAL_BASH_TREE_CHILD=1\"\r\nset \"GREGAL_BASH_TREE_READY=%s\"\r\nset \"GREGAL_BASH_TREE_RELEASE=%s\"\r\nset \"GREGAL_BASH_TREE_COMPLETED=%s\"\r\nstart \"\" /B \"%s\" -test.run=TestBashCtxDescendantMarkerHelper\r\nping -n 30 127.0.0.1 >nul\r\n",
		ready, release, completed, executable)
	if err := os.WriteFile(filepath.Join(dir, scriptName), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	return scriptName
}

func quoteShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
