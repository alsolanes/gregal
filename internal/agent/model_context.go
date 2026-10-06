package agent

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type autonomousTimeLimitError struct{ minutes int }

func (e *autonomousTimeLimitError) Error() string {
	return fmt.Sprintf("autonomous run reached its %d-minute time budget", e.minutes)
}

func (e *autonomousTimeLimitError) Unwrap() error { return context.DeadlineExceeded }

// ModelContext shares the model-step deadline across frontends and never lets
// an autonomous request outlive the remaining run time budget.
func (t *Torn) ModelContext(parent context.Context) (context.Context, context.CancelFunc) {
	limit := t.o.Cfg.ModelTimeout(t.o.Mode)
	var cause error = fmt.Errorf("model step exceeded its %s time limit: %w", limit, context.DeadlineExceeded)
	deadline := time.Now().Add(limit)
	if t.o.Mode == ModeAutonomous && t.o.Cfg != nil {
		budget := t.o.Cfg.AutonomousConfig().MaxMinutes
		runDeadline := t.autoInici.Add(time.Duration(budget) * time.Minute)
		if runDeadline.Before(deadline) {
			deadline = runDeadline
			cause = &autonomousTimeLimitError{minutes: budget}
		}
	}
	return context.WithDeadlineCause(parent, deadline, cause)
}

// ModelError adds deadline diagnostics without making cancellation retryable.
func ModelError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
		return context.Cause(ctx)
	}
	return err
}
