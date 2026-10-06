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
		{"interactive", ModeChat, 0, 4 * time.Minute},
		{"autonomous", ModeAutonomous, 0, 20 * time.Minute},
		{"explicit", ModeAutonomous, 30, 30 * time.Second},
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

func TestModelStepTimeoutDoesNotRetryOrClaimBudgetExhaustion(t *testing.T) {
	turn := NouTorn(OpcionsTorn{Mode: ModeAutonomous, Cfg: &config.Config{Agent: config.AgentCfg{ModelTimeoutS: 1}}})
	ctx, cancel := turn.ModelContext(context.Background())
	defer cancel()
	<-ctx.Done()
	err := ModelError(ctx, ctx.Err())
	turn.repError(err)
	if !turn.fallit || turn.sintesiDemanada || turn.serverRetries != 0 || !strings.Contains(err.Error(), "1s") {
		t.Fatalf("incorrect timeout handling: %v", err)
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
