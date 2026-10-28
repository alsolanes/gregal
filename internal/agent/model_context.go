package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

type autonomousTimeLimitError struct{ minutes int }

func (e *autonomousTimeLimitError) Error() string {
	return fmt.Sprintf("autonomous run reached its %d-minute time budget", e.minutes)
}

func (e *autonomousTimeLimitError) Unwrap() error { return context.DeadlineExceeded }

// ModelStallError is the cause of a model step cut by the inactivity limit:
// the provider sent nothing at all for Idle. It wraps DeadlineExceeded (it
// is a timeout, not the user pressing Stop) and the engine retries the step
// once, because a request stuck on a shared server usually goes through
// the second time.
type ModelStallError struct{ Idle time.Duration }

func (e *ModelStallError) Error() string {
	return fmt.Sprintf("model sent nothing for %s (inactivity limit)", e.Idle)
}

func (e *ModelStallError) Unwrap() error { return context.DeadlineExceeded }

// IsModelStall diu si l'error és un pas tallat per inactivitat.
func IsModelStall(err error) bool {
	var s *ModelStallError
	return errors.As(err, &s)
}

// ModelContext bounds one model step. The idle limit (Cfg.ModelIdleTimeout:
// 4 min interactive, 5 autonomous, or agent.model_timeout_s) is an
// INACTIVITY limit: it only expires when the provider sends nothing for that
// long. A slow model that keeps streaming reasoning, text or tool arguments
// is never cut mid-answer; a hard ceiling (30 min, 60 autonomous, never below
// the idle limit) still ends a runaway step, and an autonomous request never
// outlives the remaining run budget.
//
// Vist el 2026-10-13: un model lent escrivia una web dins una crida d'eina
// (16k caràcters, ~60/s) i el límit fix de 4 minuts el va tallar a mitges.
func (t *Torn) ModelContext(parent context.Context) (context.Context, context.CancelFunc) {
	idle := t.o.Cfg.ModelIdleTimeout(t.o.Mode)
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
	return stepContext(parent, idle, deadline, hardCause)
}

// StepContext és ModelContext per a qui no té Torn (els subagents de
// delegate): límit d'inactivitat del mode i sostre dur de 30 minuts.
func StepContext(parent context.Context, cfg *config.Config, mode string) (context.Context, context.CancelFunc) {
	idle := cfg.ModelIdleTimeout(mode)
	hard := 30 * time.Minute
	if hard < idle {
		hard = idle
	}
	return stepContext(parent, idle, time.Now().Add(hard),
		fmt.Errorf("model step exceeded its %s hard limit: %w", hard, context.DeadlineExceeded))
}

func stepContext(parent context.Context, idle time.Duration, deadline time.Time, hardCause error) (context.Context, context.CancelFunc) {
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
					cancelIdle(&ModelStallError{Idle: idle})
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
