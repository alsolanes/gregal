//go:build windows

package procs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/shell"
)

func TestStartCtxCancellationKillsWindowsDescendants(t *testing.T) {
	dir := t.TempDir()
	command := "sleep 30 & echo descendant-ready; wait"
	if !shell.IsPOSIX() {
		script := "@echo off\r\nstart \"\" /B ping -n 30 127.0.0.1\r\necho descendant-ready\r\nping -n 30 127.0.0.1\r\n"
		if err := os.WriteFile(filepath.Join(dir, "descendants.cmd"), []byte(script), 0600); err != nil {
			t.Fatal(err)
		}
		command = "descendants.cmd"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := New().StartCtx(ctx, "sess", dir, command)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(p.Tail(20), "descendant-ready") && time.Now().Before(deadline) {
		if !p.IsRunning() {
			t.Fatalf("process ended before spawning its child: %q", p.Tail(20))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(p.Tail(20), "descendant-ready") {
		t.Fatalf("child was not running before cancellation: %q", p.Tail(20))
	}
	cancel()
	if !p.Wait(3 * time.Second) {
		t.Fatalf("process tree did not stop on cancellation: %q", p.Tail(20))
	}
}
