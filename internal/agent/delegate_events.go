package agent

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/llm"
)

// DelegateEvent describes a child agent lifecycle or progress update. The
// hook supplied to WithDelegateHook may be called concurrently by children.
type DelegateEvent struct {
	ID   string `json:"id"`
	Task string `json:"task"`
	// Agent és el nom del subagent definit per l'usuari (buit = genèric).
	Agent      string `json:"agent,omitempty"`
	Status     string `json:"status"`
	Model      string `json:"model,omitempty"`
	Step       int    `json:"step,omitempty"`
	MaxSteps   int    `json:"max_steps,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Message    string `json:"message,omitempty"`
	Output     string `json:"output,omitempty"`
	UpTokens   int    `json:"up_tokens,omitempty"`
	DownTokens int    `json:"down_tokens,omitempty"`
	Calls      int    `json:"calls,omitempty"`
	// UsageKnown tells real provider-declared consumption apart from
	// "no calls yet" and "calls but the provider said nothing".
	UsageKnown bool      `json:"usage_known,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

const (
	// DelegateQueued: launched but waiting for a free slot (per-turn or
	// process cap, or a single-slot local server). Not working yet.
	DelegateQueued    = "queued"
	DelegateWorking   = "working"
	DelegateCompleted = "completed"
	DelegateFailed    = "failed"
	DelegateCancelled = "cancelled"
)

type delegateHookContextKey struct{}
type delegateIdentityContextKey struct{}

type delegateIdentity struct {
	mu        sync.RWMutex
	id        string
	task      string
	model     string
	startedAt time.Time
}

var delegateIDCounter atomic.Uint64

func nextDelegateID() string {
	return "delegate-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" + strconv.FormatUint(delegateIDCounter.Add(1), 10)
}

// WithDelegateHook returns a context that receives delegate lifecycle and
// progress events. A hook can be called concurrently when multiple children
// are running, so callers must make their hook safe for concurrent use.
func WithDelegateHook(ctx context.Context, hook func(DelegateEvent)) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, delegateHookContextKey{}, hook)
}

func withDelegateIdentity(ctx context.Context, id, task string, started time.Time) context.Context {
	return context.WithValue(ctx, delegateIdentityContextKey{}, &delegateIdentity{
		id: id, task: task, model: "", startedAt: started,
	})
}

// fillID és l'id del fill del context ("" fora d'un fill).
func fillID(ctx context.Context) string {
	if identity, ok := ctx.Value(delegateIdentityContextKey{}).(*delegateIdentity); ok {
		return identity.id
	}
	return ""
}

func setDelegateModel(ctx context.Context, model string) {
	if identity, ok := ctx.Value(delegateIdentityContextKey{}).(*delegateIdentity); ok {
		identity.mu.Lock()
		identity.model = model
		identity.mu.Unlock()
	}
}

func emitDelegateEvent(ctx context.Context, event DelegateEvent) {
	if ctx == nil {
		return
	}
	hook, _ := ctx.Value(delegateHookContextKey{}).(func(DelegateEvent))
	if hook == nil {
		return
	}
	if identity, ok := ctx.Value(delegateIdentityContextKey{}).(*delegateIdentity); ok {
		identity.mu.RLock()
		if event.ID == "" {
			event.ID = identity.id
		}
		if event.Task == "" {
			event.Task = identity.task
		}
		if event.Model == "" {
			event.Model = identity.model
		}
		if event.StartedAt.IsZero() {
			event.StartedAt = identity.startedAt
		}
		identity.mu.RUnlock()
	}
	if event.Agent == "" {
		if def := SubagentFromContext(ctx); def != nil {
			event.Agent = def.Name
		}
	}
	now := time.Now().UTC()
	if event.StartedAt.IsZero() {
		event.StartedAt = now
	}
	event.UpdatedAt = now
	event.Output = delegatePreview(event.Output)
	// Real consumption travels with the context (UsageMeter): every event
	// carries the child's running totals so the room can show tokens
	// without a second channel. A missing meter means zero calls.
	if meter := llm.UsageFrom(ctx); meter != nil {
		u, calls, with := meter.Total()
		event.UpTokens, event.DownTokens = u.PromptTokens, u.CompletionTokens
		event.Calls = calls
		event.UsageKnown = with > 0
	}
	// Telemetry must not break a real delegated task if an observer panics.
	func() {
		defer func() { _ = recover() }()
		hook(event)
	}()
}

func delegatePreview(value string) string {
	const maxRunes = 2000
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "…"
}
