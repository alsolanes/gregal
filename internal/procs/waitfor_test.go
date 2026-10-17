package procs

import (
	"context"
	"testing"
	"time"
)

// bash_output wait/until: torna en veure el patró, en acabar o en
// esgotar el temps, sense esperar de més.
func TestWaitFor(t *testing.T) {
	s := New()
	p, err := s.Start("sess", t.TempDir(), "echo arrencant; sleep 1; echo READY on 8080; sleep 20")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Kill()
	t0 := time.Now()
	if got := p.WaitFor(context.Background(), 15*time.Second, "READY"); got != "matched" {
		t.Fatalf("until: %s", got)
	}
	if d := time.Since(t0); d > 8*time.Second {
		t.Fatalf("ha trigat %s a veure el patró", d)
	}
	if got := p.WaitFor(context.Background(), 300*time.Millisecond, "no hi és"); got != "timeout" {
		t.Fatalf("timeout: %s", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := p.WaitFor(ctx, 5*time.Second, ""); got != "cancelled" {
		t.Fatalf("cancel: %s", got)
	}
	q, _ := s.Start("sess", t.TempDir(), "echo fet")
	if got := q.WaitFor(context.Background(), 15*time.Second, ""); got != "exited" {
		t.Fatalf("exit: %s", got)
	}
	// El patró que ja hi era abans d'acabar també compta.
	if got := q.WaitFor(context.Background(), time.Second, "fet"); got != "matched" {
		t.Fatalf("patró d'un procés acabat: %s", got)
	}
}
