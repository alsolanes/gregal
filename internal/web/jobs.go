package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/runs"
)

// jobSystem obliga format butlletí markdown en execucions desateses. La
// llengua la fixa la directiva d'idioma que tanca la consigna (executeJob).
const jobSystem = "You are the writer of a bulletin. You work in inspect mode: read-only (web search, reading pages and the project). Do not modify anything or run destructive actions. Produce the answer as well-structured markdown: headline, date, summary, key points with links to the sources and, if relevant, a comparison table. Reply with the bulletin only, no preamble."

// ExternalJobScheduler diu si els jobs programats els dispara algú de fora
// (GREGAL_JOBS_SCHEDULER=external): l'app Android programa una alarma del
// sistema i crida /api/jobs/run amb scheduled, perquè el backend només viu
// mentre l'app és viva. Amb dos programadors el mateix job es disputaria.
func ExternalJobScheduler() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("GREGAL_JOBS_SCHEDULER")), "external")
}

// jobLoop revisa cada 30 s si toca executar algun job.
func (s *Server) jobLoop(ctx context.Context) {
	if ExternalJobScheduler() {
		return
	}
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.runDueJobs()
		}
	}
}

func (s *Server) runDueJobs() {
	if s.jobs == nil {
		return
	}
	s.mu.Lock()
	// Els jobs programats també passen per runs.Queue: si un torn interactiu
	// ocupa el worker, s'encuen amb prioritat de fons i no escriuen en
	// paral·lel al mateix workspace.
	if s.jobRunning {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	list, err := s.jobs.List()
	if err != nil {
		return
	}
	now := time.Now()
	for _, j := range list {
		if jobs.Due(j, now) {
			// Un per tick: evita solapaments i pics al model local.
			_, _ = s.runJob(j.ID, false)
			return
		}
	}
}

// runJob encua un job a la cua compartida i espera que acabi per conservar el
// contracte intern síncron del scheduler.
// manual=true permet disparar-lo encara que estigui deshabilitat, però mai
// si l'agent interactiu està ocupat (el comportament de l'API manual).
// Un pànic (eina, JSON, provider) no tomba mai el servidor: queda com a
// execució d'error i el scheduler continua al següent tick.
func (s *Server) runJob(id string, manual bool) (run jobs.Run, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pànic recuperat: %v", r)
			run.Status = "error"
			run.Error = err.Error()
			run.FinishedAt = time.Now()
			if run.JobID != "" && s.jobs != nil {
				_ = s.jobs.AddRun(run)
				_ = s.jobs.MarkRun(run.JobID, run.FinishedAt, "error")
			}
		}
	}()
	if s.jobs == nil {
		return jobs.Run{}, fmt.Errorf("jobs desactivats")
	}
	list, err := s.jobs.List()
	if err != nil {
		return jobs.Run{}, err
	}
	var job *jobs.Job
	for i, j := range list {
		if j.ID == id {
			job = &list[i]
			break
		}
	}
	if job == nil {
		return jobs.Run{}, fmt.Errorf("job inexistent")
	}
	if !job.Enabled && !manual {
		return jobs.Run{}, fmt.Errorf("job deshabilitat")
	}
	s.mu.Lock()
	if manual && s.agentBusy {
		s.mu.Unlock()
		return jobs.Run{}, fmt.Errorf("agent ocupat: torna-ho a provar")
	}
	if s.jobRunning {
		s.mu.Unlock()
		return jobs.Run{}, fmt.Errorf("ja hi ha un job en marxa")
	}
	s.jobRunning = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.jobRunning = false
		s.mu.Unlock()
	}()

	result := make(chan jobResult, 1)
	exec := func(ctx context.Context, queued runs.Run) error {
		jobRun, runErr := s.executeJob(ctx, *job, queued)
		result <- jobResult{run: jobRun, err: runErr}
		return runErr
	}
	queued, _, submitErr := s.Hub().Queue().Submit(exec, runs.Run{
		Session:   s.id,
		Workspace: runs.WorkspaceKey(s.cwd),
		Priority:  runs.PriorityBackground,
	})
	if submitErr != nil {
		return jobs.Run{}, submitErr
	}
	s.appendRunEvent(queued, "run_queued", "job encuat")
	s.watchRun(queued)
	<-s.Hub().Queue().Done(queued.ID)

	select {
	case got := <-result:
		run, err = got.run, got.err
	default:
		// Queue només pot arribar aquí si el torn s'ha cancel·lat o si ha
		// recuperat un pànic abans d'entrar al callback.
		finished, _ := s.Hub().Queue().Get(queued.ID)
		run = jobs.Run{
			JobID: job.ID, JobName: job.Name, StartedAt: queued.StartedAt,
			FinishedAt: finished.FinishedAt,
			Status:     "error",
			Error:      "execució de job interrompuda abans de començar",
		}
		if finished.Err != "" {
			run.Error = finished.Err
		}
		if run.FinishedAt.IsZero() {
			run.FinishedAt = time.Now()
		}
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, run.FinishedAt, run.Status)
		err = fmt.Errorf("%s", run.Error)
	}
	return run, err
}

type jobResult struct {
	run jobs.Run
	err error
}

// executeJob és l'executor de runs.Queue. El context ve de la cua, així que
// una cancel·lació del torn allibera també la inferència i no deixa el job
// escrivint fora del token del workspace.
func (s *Server) executeJob(ctx context.Context, job jobs.Job, queued runs.Run) (jobs.Run, error) {
	s.appendRunEvent(queued, "run_started", "job iniciat")
	start := time.Now()
	jobCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	// Els jobs de vent (id wind-<vent>) tenen consigna pròpia: el vent vigila
	// el seu tema amb cerques web i només parla si troba algo que valgui la
	// pena. El missatge entra al seu xat (company-<vent>).
	if id, isWind := windJobID(job.ID); isWind && s.winds != nil {
		if w, ok := s.winds.ByID(id); ok {
			return s.executeWindHeartbeat(ctx, job, queued, w)
		}
	}
	// Loop no té system prompt: la consigna va davant la tasca, amb la
	// memòria del projecte si n'hi ha.
	brief := jobSystem + "\n\nTask:\n" + job.Prompt
	if info := agent.ContextProjecte(s.cwd); info != "" {
		brief += "\n\n" + info
	}
	brief += "\n\n" + config.LanguageDirective(s.cfg.Lang())
	res, runErr := s.runReadOnlyLoop(jobCtx, brief, s.cfg.Agent.MaxSteps)
	out := res.text
	if res.parcial {
		// Un job desatès no pot continuar sol: el butlletí diu que és parcial.
		out = "[partial: " + res.motiu + "]\n\n" + out
	}
	end := time.Now()
	run := jobs.Run{
		JobID: job.ID, JobName: job.Name, StartedAt: start, FinishedAt: end,
	}
	if runErr != nil {
		run.Status = "error"
		run.Error = runErr.Error()
	} else {
		run.Status = "ok"
		run.Output = strings.TrimSpace(out)
	}
	_ = s.jobs.AddRun(run)
	_ = s.jobs.MarkRun(job.ID, end, run.Status)
	return run, runErr
}

// --- API de jobs ---

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		http.Error(w, "jobs desactivats", 500)
		return
	}
	list, err := s.jobs.List()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	now := time.Now()
	type jobOut struct {
		jobs.Job
		NextDue string `json:"next_due,omitempty"`
		// NextDueAt és la mateixa franja en RFC 3339, per a qui programa
		// fora del procés (l'app Android desperta el backend amb una alarma).
		NextDueAt string `json:"next_due_at,omitempty"`
	}
	out := make([]jobOut, 0, len(list))
	for _, j := range list {
		o := jobOut{Job: j}
		if nd := jobs.NextDue(j, now); !nd.IsZero() {
			o.NextDue = nd.Format("02-01 15:04")
			o.NextDueAt = nd.Format(time.RFC3339)
		}
		out = append(out, o)
	}
	writeJSON(w, map[string]any{"jobs": out})
}

func (s *Server) handleJobSave(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		http.Error(w, "jobs desactivats", 500)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	_, hasNotify := body["notify"]
	var j jobs.Job
	if raw, err := json.Marshal(body); err != nil || json.Unmarshal(raw, &j) != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	if strings.TrimSpace(j.Name) == "" || strings.TrimSpace(j.Prompt) == "" {
		http.Error(w, "nom i prompt obligatoris", 400)
		return
	}
	if j.ID == "" && !hasNotify {
		// Per defecte els jobs nous avisen per Telegram (es pot desmarcar).
		j.Notify = true
	}
	saved, err := s.jobs.Save(j)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"job": saved})
}

func (s *Server) handleJobDelete(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		http.Error(w, "jobs desactivats", 500)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "id obligatori", 400)
		return
	}
	if err := s.jobs.Delete(req.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleJobRun dispara en segon pla i retorna acceptat: l'app fa polling a runs.
// Amb scheduled=true és la franja programada que dispara un programador
// extern: com al jobLoop, s'encua darrere d'un torn interactiu en lloc de
// tornar 409, i un job pausat no corre.
func (s *Server) handleJobRun(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		http.Error(w, "jobs desactivats", 500)
		return
	}
	var req struct {
		ID        string `json:"id"`
		Scheduled bool   `json:"scheduled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "id obligatori", 400)
		return
	}
	if req.Scheduled {
		// Comprovat ara i no dins la goroutine: si no, qui crida esperaria
		// una execució que mai arribarà.
		list, err := s.jobs.List()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		enabled := false
		for _, j := range list {
			if j.ID == req.ID {
				enabled = j.Enabled
			}
		}
		if !enabled {
			http.Error(w, "job inexistent o pausat", 409)
			return
		}
	}
	s.mu.Lock()
	busy := s.jobRunning || (s.agentBusy && !req.Scheduled)
	s.mu.Unlock()
	if busy {
		http.Error(w, "agent ocupat: torna-ho a provar", 409)
		return
	}
	go func() {
		defer func() { _ = recover() }()
		_, _ = s.runJob(req.ID, !req.Scheduled)
	}()
	writeJSON(w, map[string]bool{"accepted": true})
}

func (s *Server) handleJobRuns(w http.ResponseWriter, r *http.Request) {
	if s.jobs == nil {
		http.Error(w, "jobs desactivats", 500)
		return
	}
	runs, err := s.jobs.Runs(r.URL.Query().Get("job"), 30)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"runs": runs})
}
