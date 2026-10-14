package web

// Capacitats d'agent a la web: apostes de confiança (event bet i
// /api/calibration), crític del pla (/api/plan) i best-of-N en worktrees
// (/api/bestof). Les rutes es registren des de registerCapsRoutes perquè
// serve.go només hi tingui una línia.
//
//   GET    /api/calibration[?scope=session|project|all]  resum Brier de les apostes
//   POST   /api/bestof          {task, n, models?, verify?, max_steps?}  arrenca N intents
//   GET    /api/bestof[?id=]    llista o comparació d'un best-of
//   POST   /api/bestof/apply    {id, attempt, keep?}  porta l'intent al repo i llença la resta
//   DELETE /api/bestof?id=      atura i llença tots els intents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/runs"
)

// registerCapsRoutes registra les rutes d'aquest fitxer al mux.
func registerCapsRoutes(h *Hub, handle func(string, http.HandlerFunc)) {
	handle("/api/calibration", h.route((*Server).handleCalibration))
	handle("/api/bestof", h.route((*Server).handleBestOf))
	handle("/api/bestof/apply", h.route((*Server).handleBestOfApply))
}

// betPayload és l'event bet (tot text: recordActive el llegeix com a
// map[string]string).
func betPayload(id string, a agent.Aposta) map[string]string {
	return map[string]string{
		"id":      id,
		"p":       strconv.Itoa(int(a.P*100 + 0.5)),
		"passed":  strconv.FormatBool(a.Passed),
		"command": a.Command,
		"source":  a.Source,
		"text":    agent.BetChip(a) + " · " + a.Command,
	}
}

// handleCalibration resumeix les apostes de la sessió, del projecte o de
// totes.
func (s *Server) handleCalibration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "només GET", 405)
		return
	}
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "project"
	}
	session, project := "", ""
	switch scope {
	case "session":
		session = s.id
	case "project":
		project = s.sessionCwd()
	case "all":
	default:
		http.Error(w, "scope ha de ser session, project o all", 400)
		return
	}
	c := agent.Calibra(agent.LlegeixApostes(session, project))
	writeJSON(w, map[string]any{"scope": scope, "enabled": agent.BetsMode(s.cfg) != "off", "mode": agent.BetsMode(s.cfg), "calibration": c})
}

// planCritic passa el pla pel crític (plan.critic) i torna el resultat per
// a la resposta de /api/plan (nil amb el crític apagat).
func (s *Server) planCritic(ctx context.Context, task, plan string) *agent.PlanCritique {
	if !s.cfg.PlanCriticOn() {
		return nil
	}
	c, _ := agent.CriticaPla(ctx, s.client, s.cfg, task, plan, s.sessionCwd())
	return &c
}

// --- best-of ---

type bestOfRun struct {
	mu      sync.Mutex
	owner   string
	dir     string
	res     agent.BestOfResult
	running bool
	applied int
	message string
	started time.Time
	cancel  context.CancelFunc
}

func (b *bestOfRun) snapshot() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	atts := append([]agent.BestOfAttempt(nil), b.res.Attempts...)
	if atts == nil {
		atts = []agent.BestOfAttempt{}
	}
	return map[string]any{
		"id": b.res.ID, "task": b.res.Task, "main": b.res.Main, "attempts": atts,
		"recommended": b.res.Recommended, "running": b.running, "applied": b.applied,
		"message": b.message, "started_at": b.started.UTC().Format(time.RFC3339),
	}
}

var (
	bestOfMu   sync.Mutex
	bestOfRuns = map[string]*bestOfRun{}
)

func getBestOf(id, owner string) *bestOfRun {
	bestOfMu.Lock()
	defer bestOfMu.Unlock()
	b := bestOfRuns[id]
	if b == nil || b.owner != owner {
		return nil
	}
	return b
}

// bestOfOwner identifica la sessió propietària. Hub() primer: és qui fixa
// l'id de la sessió per defecte.
func (s *Server) bestOfOwner() string {
	s.Hub()
	return s.usuari() + "\x00" + s.id
}

func (s *Server) handleBestOf(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if id := r.URL.Query().Get("id"); id != "" {
			b := getBestOf(id, s.bestOfOwner())
			if b == nil {
				http.Error(w, "best-of inexistent", 404)
				return
			}
			writeJSON(w, b.snapshot())
			return
		}
		bestOfMu.Lock()
		var list []*bestOfRun
		for _, b := range bestOfRuns {
			if b.owner == s.bestOfOwner() {
				list = append(list, b)
			}
		}
		bestOfMu.Unlock()
		sort.Slice(list, func(i, j int) bool { return list[i].started.After(list[j].started) })
		out := []map[string]any{}
		for _, b := range list {
			out = append(out, b.snapshot())
		}
		writeJSON(w, map[string]any{"runs": out})
	case http.MethodPost:
		s.startBestOf(w, r)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		b := getBestOf(id, s.bestOfOwner())
		if b == nil {
			http.Error(w, "best-of inexistent", 404)
			return
		}
		b.cancel()
		b.mu.Lock()
		atts := append([]agent.BestOfAttempt(nil), b.res.Attempts...)
		b.mu.Unlock()
		removed := agent.BestOfDiscard(b.dir, atts)
		bestOfMu.Lock()
		delete(bestOfRuns, id)
		bestOfMu.Unlock()
		if removed == nil {
			removed = []string{}
		}
		writeJSON(w, map[string]any{"id": id, "removed": removed})
	default:
		http.Error(w, "mètode no permès", 405)
	}
}

func (s *Server) startBestOf(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Task     string   `json:"task"`
		N        int      `json:"n"`
		Models   []string `json:"models"`
		Verify   []string `json:"verify"`
		MaxSteps int      `json:"max_steps"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Task) == "" {
		http.Error(w, "cal task", 400)
		return
	}
	if req.N == 0 {
		req.N = 2
	}
	if req.N < agent.BestOfMin || req.N > agent.BestOfMax {
		http.Error(w, fmt.Sprintf("n ha d'anar de %d a %d", agent.BestOfMin, agent.BestOfMax), 400)
		return
	}
	if req.MaxSteps < 0 || req.MaxSteps > 200 {
		http.Error(w, "max_steps fora de rang (1-200)", 400)
		return
	}
	dir := s.sessionCwd()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	b := &bestOfRun{owner: s.bestOfOwner(), dir: dir, running: true, started: time.Now(), cancel: cancel}
	id := agent.NewBestOfID()
	bestOfMu.Lock()
	for bestOfRuns[id] != nil {
		id += "x"
	}
	bestOfRuns[id] = b
	bestOfMu.Unlock()
	b.res = agent.BestOfResult{ID: id, Task: req.Task, Recommended: -1}

	s.mu.Lock()
	cfg := s.cfg
	mode := s.mode
	s.mu.Unlock()
	queue := s.Hub().Queue()
	opts := agent.BestOfOpts{
		ID: id, Task: strings.TrimSpace(req.Task), N: req.N, Models: req.Models, Dir: dir,
		Mode: mode, MaxSteps: req.MaxSteps, AutoApprove: s.permissiveNow(), Verify: req.Verify,
		Score: agent.BestOfScorer(s.client, cfg),
		// Cada intent passa per la cua d'execucions compartida, amb una
		// sessió pròpia i el workspace del seu worktree: corren en
		// paral·lel entre ells i en sèrie amb qualsevol altre torn del
		// mateix directori.
		Run: func(ctx context.Context, c *config.Config, o agent.RunOpts) (agent.RunResult, error) {
			var out agent.RunResult
			var runErr error
			exec := func(rctx context.Context, _ runs.Run) error {
				out, runErr = agent.RunNonInteractiveOpts(rctx, s.client, c, o)
				if rctx.Err() != nil {
					return runs.ErrCancelled
				}
				return runErr
			}
			queued, _, err := queue.Submit(exec, runs.Run{
				Session:   "bestof-" + id + "-" + o.Role,
				Workspace: runs.WorkspaceKey(o.Dir),
				Priority:  runs.PriorityBackground,
			})
			if err != nil {
				return out, err
			}
			select {
			case <-queue.Done(queued.ID):
			case <-ctx.Done():
				queue.Cancel(queued.ID)
				<-queue.Done(queued.ID)
				return out, ctx.Err()
			}
			if runErr == nil {
				if fin, ok := queue.Get(queued.ID); ok && fin.Err != "" && fin.State != runs.Completed {
					runErr = fmt.Errorf("%s", fin.Err)
				}
			}
			return out, runErr
		},
		OnUpdate: func(a agent.BestOfAttempt) {
			b.mu.Lock()
			defer b.mu.Unlock()
			for len(b.res.Attempts) < a.Index {
				b.res.Attempts = append(b.res.Attempts, agent.BestOfAttempt{Index: len(b.res.Attempts) + 1, State: agent.BestOfQueued, Files: []string{}, Checks: []agent.AutonomousCheck{}})
			}
			b.res.Attempts[a.Index-1] = a
		},
	}
	go func() {
		defer cancel()
		res, err := agent.RunBestOf(ctx, s.client, cfg, opts)
		b.mu.Lock()
		defer b.mu.Unlock()
		b.running = false
		if res.Main != "" {
			b.res.Main = res.Main
		}
		if len(res.Attempts) > 0 {
			b.res.Attempts = res.Attempts
		}
		b.res.Recommended = res.Recommended
		if err != nil {
			b.message = err.Error()
		}
	}()
	writeJSONStatus(w, http.StatusAccepted, b.snapshot())
}

func (s *Server) handleBestOfApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Attempt int    `json:"attempt"`
		Keep    bool   `json:"keep"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "json invàlid", 400)
		return
	}
	b := getBestOf(req.ID, s.bestOfOwner())
	if b == nil {
		http.Error(w, "best-of inexistent", 404)
		return
	}
	b.mu.Lock()
	running := b.running
	atts := append([]agent.BestOfAttempt(nil), b.res.Attempts...)
	b.mu.Unlock()
	if running {
		http.Error(w, "els intents encara corren", 409)
		return
	}
	s.mu.Lock()
	busy := s.agentBusy
	s.mu.Unlock()
	if busy {
		http.Error(w, "agent ocupat: torna-ho a provar quan acabi el torn", 409)
		return
	}
	msg, removed, err := agent.BestOfApply(b.dir, atts, req.Attempt, req.Keep)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	b.mu.Lock()
	b.applied, b.message = req.Attempt, msg
	b.mu.Unlock()
	if removed == nil {
		removed = []string{}
	}
	writeJSON(w, map[string]any{"id": req.ID, "applied": req.Attempt, "message": msg, "removed": removed})
}
