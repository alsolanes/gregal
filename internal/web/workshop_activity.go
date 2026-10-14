package web

import (
	"context"
	"errors"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/team"
)

// A bounded snapshot of the latest real turn, scoped to its owning session.
// It survives completion so a fast child does not disappear between polls.
const maxWorkshopAgents = 32

// maxWorkshopHistory bounds the inspectable step timeline per turn:
// inspection only (the engine runs forward), newest last.
const maxWorkshopHistory = 40

type workshopRunKey struct{}

// workshopHistoryEvent is one rung of the turn timeline: what the
// coordinator did at each step, in order. Retries included.
type workshopHistoryEvent struct {
	Step    int       `json:"step"`
	State   string    `json:"state"`
	Tool    string    `json:"tool,omitempty"`
	Message string    `json:"message,omitempty"`
	At      time.Time `json:"at"`
}

type workshopActivity struct {
	State string `json:"state"`
	Task  string `json:"task"`
	Model string `json:"model,omitempty"`
	Step  int    `json:"step,omitempty"`
	Limit int    `json:"limit,omitempty"`
	// Kind marks team pipeline runs ("team"); agent turns leave it empty.
	// The room renders both, so one room replaces two 2D views.
	Kind       string         `json:"kind,omitempty"`
	Flow       *team.FlowSpec `json:"flow,omitempty"`
	Tool       string         `json:"tool,omitempty"`
	Message    string         `json:"message,omitempty"`
	UpTokens   int            `json:"up_tokens,omitempty"`
	DownTokens int            `json:"down_tokens,omitempty"`
	Calls      int            `json:"calls,omitempty"`
	// TokensSource is "api" when every call declared usage, "mixed" when
	// only some did, and "" when the provider said nothing (calls alone).
	TokensSource string `json:"tokens_source,omitempty"`
	// CostUSD is set only when the model has a price; never estimated.
	CostUSD *float64               `json:"cost_usd,omitempty"`
	History []workshopHistoryEvent `json:"history,omitempty"`
	// Evicted counts children dropped to stay bounded; Trimmed counts
	// history rungs dropped. The room shows them ("+N anteriors") so a
	// cap never looks like the whole story.
	Evicted   int                   `json:"agents_evicted,omitempty"`
	Trimmed   int                   `json:"history_trimmed,omitempty"`
	StartedAt time.Time             `json:"started_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	Agents    []agent.DelegateEvent `json:"agents"`
	// Role és el carril que té el torn ara (workshop_roles.go); Lanes, la
	// línia de temps de passos atribuïts a rols; Handoffs, els relleus.
	Role         string                   `json:"role,omitempty"`
	Lanes        []workshopLane           `json:"lanes,omitempty"`
	LanesTrimmed int                      `json:"lanes_trimmed,omitempty"`
	Handoffs     []workshopHandoff        `json:"handoffs,omitempty"`
	RoleUsage    map[string]workshopUsage `json:"role_usage,omitempty"`
	// Comptadors interns: el número de carril/relleu i l'última lectura
	// del comptador de tokens (per repartir deltes per rol).
	laneSeq                     int
	lastUp, lastDown, lastCalls int
}

func workshopText(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return string(runes)
}

func (s *Server) beginWorkshop(task, model string, limit int) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workshopSeq++
	now := time.Now().UTC()
	s.activity = &workshopActivity{State: "thinking", Task: workshopText(task, 2000), Model: model,
		Limit: limit, StartedAt: now, UpdatedAt: now, Agents: []agent.DelegateEvent{}, Role: roleCoordinator}
	return s.workshopSeq
}

func (s *Server) updateWorkshop(generation uint64, state string, step, limit int, tool, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil {
		return
	}
	a := s.activity
	a.State, a.Step, a.Limit = state, step, limit
	a.Tool, a.Message = workshopText(tool, 200), workshopText(message, 500)
	a.UpdatedAt = time.Now().UTC()
	a.History = append(a.History, workshopHistoryEvent{
		Step: step, State: state, Tool: a.Tool, Message: a.Message, At: a.UpdatedAt,
	})
	if len(a.History) > maxWorkshopHistory {
		a.Trimmed += len(a.History) - maxWorkshopHistory
		a.History = append([]workshopHistoryEvent{}, a.History[len(a.History)-maxWorkshopHistory:]...)
	}
}

func (s *Server) noteDelegate(generation uint64, event agent.DelegateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil || event.ID == "" {
		return
	}
	a := s.activity
	if a.State == "completed" || a.State == "failed" || a.State == "cancelled" {
		return
	}
	event.Task = workshopText(event.Task, 2000)
	event.Output = workshopText(event.Output, 2000)
	event.Message = workshopText(event.Message, 500)
	for i, old := range a.Agents {
		if old.ID != event.ID {
			continue
		}
		if event.UpdatedAt.Before(old.UpdatedAt) {
			return
		}
		// Lifecycle events can omit the last tool/step; retain observed progress.
		if event.Step == 0 {
			event.Step = old.Step
		}
		if event.MaxSteps == 0 {
			event.MaxSteps = old.MaxSteps
		}
		// Progress pings ("step N", "model response received") carry no
		// tool/output; keep showing the last observed tool and its output
		// so a fast child stays visible between tool calls. A fresh tool
		// resets its own output instead of showing stale text.
		if event.Tool == "" {
			event.Tool = old.Tool
			if event.Output == "" {
				event.Output = old.Output
			}
		}
		if event.Model == "" {
			event.Model = old.Model
		}
		if event.Task == "" {
			event.Task = old.Task
		}
		if event.Agent == "" {
			event.Agent = old.Agent
		}
		if event.StartedAt.IsZero() {
			event.StartedAt = old.StartedAt
		}
		a.Agents[i] = event
		a.UpdatedAt = time.Now().UTC()
		return
	}
	if len(a.Agents) >= maxWorkshopAgents {
		for i, old := range a.Agents {
			if old.Status != agent.DelegateWorking {
				a.Agents = append(a.Agents[:i], a.Agents[i+1:]...)
				a.Evicted++
				break
			}
		}
		if len(a.Agents) >= maxWorkshopAgents {
			return
		}
	}
	a.Agents = append(a.Agents, event)
	a.UpdatedAt = time.Now().UTC()
}

// touchWorkshop appends a timeline rung keeping the current state and
// step: for events (handoffs, feedback) that move no progress themselves.
func (s *Server) touchWorkshop(generation uint64, tool, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil {
		return
	}
	a := s.activity
	now := time.Now().UTC()
	a.History = append(a.History, workshopHistoryEvent{
		Step: a.Step, State: a.State, Tool: workshopText(tool, 200), Message: workshopText(message, 500), At: now,
	})
	if len(a.History) > maxWorkshopHistory {
		a.Trimmed += len(a.History) - maxWorkshopHistory
		a.History = append([]workshopHistoryEvent{}, a.History[len(a.History)-maxWorkshopHistory:]...)
	}
	a.UpdatedAt = now
}

// noteTeamEvent translates one team pipeline event into the shared room
// activity, so pipeline runs render as bays next to agent sessions. Model
// is the resolved model for the agent ("" when unknown); the shared
// retention rules (2000/500 chars, progress preservation) apply.
func (s *Server) noteTeamEvent(generation uint64, event team.Event, model string) {
	switch event.Type {
	case "working":
		state := "tools"
		switch event.Activity {
		case "plan":
			state = "thinking"
		case "review":
			state = "reviewing"
		}
		now := time.Now().UTC()
		s.noteDelegate(generation, agent.DelegateEvent{
			ID: event.Agent, Status: agent.DelegateWorking, Model: model,
			StartedAt: now, UpdatedAt: now,
		})
		s.updateWorkshop(generation, state, 0, 0, "", event.Message)
	case "completed":
		now := time.Now().UTC()
		s.noteDelegate(generation, agent.DelegateEvent{
			ID: event.Agent, Status: agent.DelegateCompleted, Model: model,
			Output: event.Output, StartedAt: now, UpdatedAt: now,
		})
	case "handoff":
		s.touchWorkshop(generation, "", "")
	case "discussion":
		s.touchWorkshop(generation, "", event.Message)
	}
}

// noteUsage snapshots the turn meter into the live activity. The model in
// force (fallback included) prices the cost; without a price there is no
// cost, only tokens. Without declared usage the call count still shows.
func (s *Server) noteUsage(generation uint64, meter *llm.UsageMeter, provider string) {
	if meter == nil {
		return
	}
	u, calls, with := meter.Total()
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil {
		return
	}
	a := s.activity
	a.Calls = calls
	a.noteRoleUsageLocked(u.PromptTokens, u.CompletionTokens, calls)
	if with == 0 {
		return
	}
	a.UpTokens, a.DownTokens = u.PromptTokens, u.CompletionTokens
	a.TokensSource = "api"
	if with < calls {
		a.TokensSource = "mixed"
	}
	if usd, ok := agent.CostUSD(provider, a.Model, u.PromptTokens, u.CompletionTokens); ok {
		a.CostUSD = &usd
	} else {
		a.CostUSD = nil
	}
	a.UpdatedAt = time.Now().UTC()
}

func (s *Server) finishWorkshop(generation uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil {
		return
	}
	a := s.activity
	a.State, a.Tool = "completed", ""
	if errors.Is(err, context.Canceled) {
		a.State = "cancelled"
	} else if err != nil {
		a.State = "failed"
	}
	a.Message = ""
	if err != nil {
		// Never store provider errors raw: they may carry
		// credential-bearing URLs into the room (and its polls).
		a.Message = workshopText(redactError(err), 500)
	}
	a.UpdatedAt = time.Now().UTC()
	for i := range a.Lanes {
		if !a.Lanes[i].Done {
			a.Lanes[i].Done = true
			a.Lanes[i].DurationMs = a.UpdatedAt.Sub(a.Lanes[i].StartedAt).Milliseconds()
		}
	}
	// Torn acabat bé: la feina torna al coordinador, que respon.
	if a.State == "completed" && a.Role != "" && a.Role != roleCoordinator && len(a.Lanes) > 0 {
		s.addHandoffLocked(a.Step, a.Role, roleCoordinator, "report", 0, "", nil, a.UpdatedAt)
		a.Role = roleCoordinator
	}
	for i := range a.Agents {
		if a.Agents[i].Status == agent.DelegateWorking {
			a.Agents[i].Status = "cancelled"
			a.Agents[i].Tool = ""
			a.Agents[i].UpdatedAt = a.UpdatedAt
		}
	}
}

// Caller holds s.mu. Copy the slices before JSON encoding outside that lock.
func (s *Server) workshopSnapshotLocked() *workshopActivity {
	if s.activity == nil {
		return nil
	}
	copy := *s.activity
	copy.Agents = append([]agent.DelegateEvent{}, s.activity.Agents...)
	copy.History = append([]workshopHistoryEvent{}, s.activity.History...)
	copy.Lanes = append([]workshopLane{}, s.activity.Lanes...)
	copy.Handoffs = append([]workshopHandoff{}, s.activity.Handoffs...)
	if s.activity.RoleUsage != nil {
		copy.RoleUsage = make(map[string]workshopUsage, len(s.activity.RoleUsage))
		for k, v := range s.activity.RoleUsage {
			copy.RoleUsage[k] = v
		}
	}
	return &copy
}
