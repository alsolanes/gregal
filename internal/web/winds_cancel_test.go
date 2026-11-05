package web

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWindTurnCanBeCanceledFromSiblingServer(t *testing.T) {
	owner, canceller := &Server{}, &Server{}
	id := "cross-tab-test-" + time.Now().Format("150405.000000000")
	ctx, done := owner.beginWindTurn("rotllana", id, context.Background())
	t.Cleanup(done)
	rec := postJSON(t, canceller.handleWindCancel, `{"rotllana":"`+id+`"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"aturat":true`) {
		t.Fatalf("same-user sibling endpoint could not cancel active turn: %d %s", rec.Code, rec.Body.String())
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cross-instance cancellation did not cancel the context")
	}
}

func TestWindTurnCancellationIsScopedToOwner(t *testing.T) {
	owner := &Server{}
	otherOwner := &Server{user: &User{Name: "wind-cancel-other-owner-test"}}
	id := "owner-scope-test-" + time.Now().Format("150405.000000000")
	ctx, done := owner.beginWindTurn("wind", id, context.Background())
	t.Cleanup(done)

	if otherOwner.cancelWindTurn("wind", id) {
		t.Fatal("a different user canceled this owner's turn")
	}
	select {
	case <-ctx.Done():
		t.Fatal("turn was canceled through another owner's registry scope")
	default:
	}
	if !owner.cancelWindTurn("wind", id) {
		t.Fatal("owner could not cancel its own turn")
	}
}

func TestReplacingWindTurnCancelsOldGenerationAndKeepsNewRegistered(t *testing.T) {
	s := &Server{}
	id := "replacement-test-" + time.Now().Format("150405.000000000")
	oldCtx, oldDone := s.beginWindTurn("rotllana", id, context.Background())
	newCtx, newDone := s.beginWindTurn("rotllana", id, context.Background())
	defer newDone()
	defer oldDone()

	select {
	case <-oldCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("replacement did not cancel the old generation")
	}
	oldDone() // A late cleanup from the previous generation must not remove the replacement.
	select {
	case <-newCtx.Done():
		t.Fatal("old generation cleanup canceled the replacement")
	default:
	}
	if !s.cancelWindTurn("rotllana", id) {
		t.Fatal("old generation cleanup removed the replacement from the registry")
	}
	select {
	case <-newCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancel did not reach the replacement")
	}
	newDone()
	if s.cancelWindTurn("rotllana", id) {
		t.Fatal("completed generation remained registered")
	}
}
