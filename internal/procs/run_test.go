package procs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunKillsDescendantsOnNormalExit(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "child.ready")
	release := filepath.Join(dir, "child.release")
	completed := filepath.Join(dir, "child.completed")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDescendantHelper$")
	cmd.Env = runHelperEnv("parent", ready, release, completed)
	if err := Run(ctx, cmd); err != nil {
		t.Fatalf("run parent helper: %v", err)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("child did not signal readiness: %v", err)
	}
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatalf("release child: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(completed); err == nil {
			t.Fatal("descendant survived normal parent exit")
		} else if !os.IsNotExist(err) {
			t.Fatalf("check completion marker: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunDescendantHelper(t *testing.T) {
	mode := os.Getenv("GREGAL_PROCS_TREE_MODE")
	if mode == "" {
		return
	}
	ready := os.Getenv("GREGAL_PROCS_TREE_READY")
	release := os.Getenv("GREGAL_PROCS_TREE_RELEASE")
	completed := os.Getenv("GREGAL_PROCS_TREE_COMPLETED")
	if mode == "child" {
		if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
			t.Fatalf("write ready marker: %v", err)
		}
		deadline := time.Now().Add(20 * time.Second)
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
		return
	}
	if mode != "parent" {
		t.Fatalf("unexpected helper mode %q", mode)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestRunDescendantHelper$")
	child.Env = runHelperEnv("child", ready, release, completed)
	if err := child.Start(); err != nil {
		t.Fatalf("start child helper: %v", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatalf("check ready marker: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child helper did not signal readiness")
}

func runHelperEnv(mode, ready, release, completed string) []string {
	const prefix = "GREGAL_PROCS_TREE_"
	env := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, prefix) {
			env = append(env, entry)
		}
	}
	return append(env,
		"GREGAL_PROCS_TREE_MODE="+mode,
		"GREGAL_PROCS_TREE_READY="+ready,
		"GREGAL_PROCS_TREE_RELEASE="+release,
		"GREGAL_PROCS_TREE_COMPLETED="+completed,
	)
}
