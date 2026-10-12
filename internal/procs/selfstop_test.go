package procs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStartCtxRejectsSelfStopBeforeAnyShellSideEffect(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake executable")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "spawned")
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte("#!/bin/sh\necho executed >> '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	store := New()
	p, err := store.StartCtx(context.Background(), "selfstop-test", dir, "echo before > spawned; systemctl --user restart gregal-telegram.service")
	if p != nil {
		p.Wait(time.Second)
	}
	if err == nil || !strings.Contains(err.Error(), "self-stop protection") {
		t.Errorf("execution guard missing: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("blocked command spawned shell/side effects: %v", err)
	}
	if len(store.List("selfstop-test")) != 0 {
		t.Error("blocked command was registered as a background process")
	}
}
