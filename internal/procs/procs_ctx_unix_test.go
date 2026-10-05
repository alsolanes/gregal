//go:build !windows

package procs

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestStartCtxKillsProcessGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p, err := New().StartCtx(ctx, "sess", t.TempDir(), "(sleep 0.4; echo child-survived) & wait")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if !p.Wait(3 * time.Second) {
		t.Fatal("el grup de processos hauria d'haver acabat")
	}
	if got := p.Tail(20); strings.Contains(got, "child-survived") {
		t.Fatalf("el fill ha sobreviscut a la cancel·lació: %q", got)
	}
}
