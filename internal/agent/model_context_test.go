package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestModelContextBudgets(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		timeout    int
		want       time.Duration
	}{
		// The context deadline is the hard ceiling; the configured limit
		// (4 min / 20 min / model_timeout_s) is an inactivity limit tested
		// in model_idle_test.go.
		{"interactive", ModeChat, 0, 30 * time.Minute},
		{"autonomous", ModeAutonomous, 0, 60 * time.Minute},
		{"explicit", ModeChat, 30, 30 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turn := NouTorn(OpcionsTorn{Mode: tc.mode, Cfg: &config.Config{Agent: config.AgentCfg{ModelTimeoutS: tc.timeout}}})
			ctx, cancel := turn.ModelContext(context.Background())
			defer cancel()
			deadline, _ := ctx.Deadline()
			remaining := time.Until(deadline)
			if remaining > tc.want || remaining < tc.want-time.Second {
				t.Fatalf("remaining=%v want=%v", remaining, tc.want)
			}
		})
	}
}

func TestModelContextNeverOutlivesAutonomousBudget(t *testing.T) {
	turn := NouTorn(OpcionsTorn{Mode: ModeAutonomous, Cfg: &config.Config{Agent: config.AgentCfg{Autonomous: config.AutonomousCfg{MaxMinutes: 1}}}})
	turn.autoInici = time.Now().Add(-time.Minute + 20*time.Millisecond)
	ctx, cancel := turn.ModelContext(context.Background())
	defer cancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("request outlived autonomous budget")
	}
	err := ModelError(ctx, ctx.Err())
	if !errors.Is(err, context.DeadlineExceeded) || llm.IsRetryable(err) || !strings.Contains(err.Error(), "autonomous run") {
		t.Fatalf("err=%v", err)
	}
	turn.repError(err)
	if !turn.sintesiDemanada || !turn.esgotat || turn.fallit || len(turn.perExecutar) != 0 || len(turn.pendents) != 0 {
		t.Fatal("budget expiration must request a summary without retrying tools")
	}
	if next := turn.Seguent(); next.Ordre != OrdreSintesi {
		t.Fatalf("expected final summary, got %v", next.Ordre)
	}
}

// A stalled step (nothing received for the idle limit) is asked again once,
// without spending a step; a second stall ends the turn. Neither counts as
// a server retry or as an exhausted budget.
func TestModelStallRetriesOnceThenFails(t *testing.T) {
	turn := NouTorn(OpcionsTorn{Mode: ModeAutonomous, Cfg: &config.Config{Agent: config.AgentCfg{ModelTimeoutS: 1}}})
	stall := func() error {
		ctx, cancel := turn.ModelContext(context.Background())
		defer cancel()
		<-ctx.Done()
		return ModelError(ctx, ctx.Err())
	}
	err := stall()
	if !IsModelStall(err) || !strings.Contains(err.Error(), "1s") {
		t.Fatalf("not a stall: %v", err)
	}
	turn.passos = 1
	turn.repError(err)
	if turn.fallit || turn.sintesiDemanada || turn.passos != 0 || turn.stallRetries != 1 {
		t.Fatalf("first stall must retry the same step: fallit=%v passos=%d", turn.fallit, turn.passos)
	}
	turn.repError(stall())
	if !turn.fallit || turn.sintesiDemanada || turn.serverRetries != 0 {
		t.Fatalf("second stall must end the turn: fallit=%v", turn.fallit)
	}
}

func TestModelContextParentCancellationStopsImmediately(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	turn := NouTorn(OpcionsTorn{Mode: ModeAutonomous})
	ctx, cancel := turn.ModelContext(parent)
	defer cancel()
	stop()
	if !errors.Is(ModelError(ctx, ctx.Err()), context.Canceled) {
		t.Fatal("parent cancellation lost")
	}
}

func TestModelContextKeepsShorterParentDeadline(t *testing.T) {
	parent, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	ctx, cancel := NouTorn(OpcionsTorn{Mode: ModeAutonomous}).ModelContext(parent)
	defer cancel()
	want, _ := parent.Deadline()
	got, _ := ctx.Deadline()
	if !got.Equal(want) {
		t.Fatalf("parent deadline extended: got %v want %v", got, want)
	}
}
