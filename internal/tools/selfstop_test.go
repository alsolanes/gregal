package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBashCtxRejectsSelfStopBeforeAnyShellSideEffect(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake executable")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "spawned")
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte("#!/bin/sh\necho executed >> '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := BashCtx(context.Background(), dir, "echo before > spawned; systemctl --user stop gregal-telegram", time.Second)
	if err == nil || !strings.Contains(err.Error(), "self-stop protection") {
		t.Errorf("execution guard missing: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("blocked command spawned shell/side effects: %v", err)
	}
}

func TestClassifyDeniesGregalSelfStop(t *testing.T) {
	if decision, reason := ClassifyWith("systemctl --user stop gregal-telegram", []string{"systemctl "}, nil); decision != "deny" {
		t.Fatalf("self-stop = %s (%s), want hard deny even with explicit prefix allow", decision, reason)
	}
}
