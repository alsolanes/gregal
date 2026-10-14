package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/llm"
)

type autonomousTimeLimitError struct{ minutes int }

func (e *autonomousTimeLimitError) Error() string {
	return fmt.Sprintf("autonomous run reached its %d-minute time budget", e.minutes)
}

func (e *autonomousTimeLimitError) Unwrap() error { return context.DeadlineExceeded }

// ModelContext bounds one model step. The configured limit (4 min
// interactive, 20 min autonomous, or agent.model_timeout_s) is an
// INACTIVITY limit: it only expires when the provider sends nothing for that
// long. A slow model that keeps streaming reasoning, text or tool arguments
// is never cut mid-answer; a hard ceiling (30 min, 60 autonomous, never below
// the idle limit) still ends a runaway step, and an autonomous request never
// outlives the remaining run budget.
//
// Vist el 2026-10-13: un model lent escrivia una web dins una crida d'eina
// (16k caràcters, ~60/s) i el límit fix de 4 minuts el va tallar a mitges.
func (t *Torn) ModelContext(parent context.Context) (context.Context, context.CancelFunc) {
	idle := t.o.Cfg.ModelTimeout(t.o.Mode)
	hard := 30 * time.Minute
	if t.o.Mode == ModeAutonomous {
		hard = 60 * time.Minute
	}
	if hard < idle {
		hard = idle
	}
	var hardCause error = fmt.Errorf("model step exceeded its %s hard limit: %w", hard, context.DeadlineExceeded)
	deadline := time.Now().Add(hard)
	if t.o.Mode == ModeAutonomous && t.o.Cfg != nil {
		budget := t.o.Cfg.AutonomousConfig().MaxMinutes
		runDeadline := t.autoInici.Add(time.Duration(budget) * time.Minute)
		if runDeadline.Before(deadline) {
			deadline = runDeadline
			hardCause = &autonomousTimeLimitError{minutes: budget}
		}
	}
	ctx, cancelHard := context.WithDeadlineCause(parent, deadline, hardCause)
	ctx, cancelIdle := context.WithCancelCause(ctx)
	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	ctx = llm.WithStreamProgress(ctx, func(string, int) { last.Store(time.Now().UnixNano()) })
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(idleCheckEvery(idle))
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case now := <-tick.C:
				if now.Sub(time.Unix(0, last.Load())) >= idle {
					cancelIdle(fmt.Errorf("model sent nothing for %s (inactivity limit): %w", idle, context.DeadlineExceeded))
					return
				}
			}
		}
	}()
	var once sync.Once
	return ctx, func() {
		once.Do(func() { close(done) })
		cancelIdle(context.Canceled)
		cancelHard()
	}
}

// idleCheckEvery: prou fi per no allargar gaire el límit, sense despertar
// el goroutine a cada mil·lisegon en els tests amb límits curts.
func idleCheckEvery(idle time.Duration) time.Duration {
	d := idle / 20
	if d < 10*time.Millisecond {
		d = 10 * time.Millisecond
	}
	if d > time.Second {
		d = time.Second
	}
	return d
}

// ModelError adds deadline diagnostics without making cancellation retryable.
// The inactivity limit cancels the step with a cause that wraps
// DeadlineExceeded; the client then reports a plain cancellation, which must
// not read as the user pressing Stop.
func ModelError(ctx context.Context, err error) error {
	if err == nil || ctx.Err() == nil {
		return err
	}
	cause := context.Cause(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return cause
	}
	if errors.Is(err, context.Canceled) && errors.Is(cause, context.DeadlineExceeded) {
		return cause
	}
	return err
}
