package team

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHandoffsCarryDeliverables(t *testing.T) {
	var events []Event
	n := 0
	answer, err := Run(context.Background(), "prepare report", func(ctx context.Context, role, task string) (string, error) {
		if n > 0 && !strings.Contains(task, Roles[n-1]+" deliverable:\n"+Roles[n-1]+" result") {
			t.Fatal("missing handoff")
		}
		n++
		return role + " result", nil
	}, func(e Event) { events = append(events, e) })
	if err != nil || answer != "reviewer result" || n != 4 || len(events) != 11 {
		t.Fatalf("answer=%q n=%d events=%d err=%v", answer, n, len(events), err)
	}
}

func TestFailureStopsFollowingAgents(t *testing.T) {
	n := 0
	_, err := Run(context.Background(), "task", func(context.Context, string, string) (string, error) { n++; return "", errors.New("failed") }, func(Event) {})
	if err == nil || n != 1 {
		t.Fatal("failure did not stop run")
	}
}

func TestCancellationStopsFollowingAgents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	_, err := Run(ctx, "task", func(context.Context, string, string) (string, error) { n++; cancel(); return "result", nil }, func(Event) {})
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatal("cancellation did not stop run")
	}
}
