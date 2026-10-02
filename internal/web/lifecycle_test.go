package web

import (
	"context"
	"testing"
	"time"
)

func TestStartBackgroundSignalsCompletionAfterCancellation(t *testing.T) {
	server := hubTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := server.StartBackground(ctx)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("background scheduler did not stop after context cancellation")
	}
}
