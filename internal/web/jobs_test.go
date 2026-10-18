package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/runs"
	"gregal/internal/tools"
)

func TestHandleRewindRestaura(t *testing.T) {
	s := goalTestServer(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "nota.txt")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	tools.Active.Snap(path)
	if err := os.WriteFile(path, []byte("modificat"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := postJSON(t, s.handleRewind, "{}")
	if rec.Code != 200 {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out["summary"], "restaurats") {
		t.Fatalf("resum pobre: %q", out["summary"])
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "original" {
		t.Fatalf("no restaurat: %q", raw)
	}
	// Segona crida: res a desfer.
	rec2 := postJSON(t, s.handleRewind, "{}")
	var out2 map[string]string
	_ = json.Unmarshal(rec2.Body.Bytes(), &out2)
	if out2["summary"] != "res a desfer" {
		t.Fatalf("segona crida: %q", out2["summary"])
	}
}

func TestJobRunPassaPerCuaBackgroundIWorkspace(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	var err error
	s.jobs, err = jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	job, err := s.jobs.Save(jobs.Job{
		ID: "job-cua", Name: "Cua", Prompt: "fes un butlletí", Kind: "manual", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, entered, release := v2Provider(t, "butlletí acabat")
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}

	type result struct {
		run jobs.Run
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		run, runErr := s.runJob(job.ID, true)
		resultCh <- result{run: run, err: runErr}
	}()
	<-entered

	// El provider està retingut: el job ha d'estar en marxa, però conserva
	// les marques de la cua i el workspace canònic.
	q := s.Hub().Queue()
	var background runs.Run
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, candidate := range q.ListSession(s.id) {
			if candidate.Priority == runs.PriorityBackground {
				background = candidate
			}
		}
		if background.ID != 0 && background.State == runs.Running {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if background.ID == 0 || background.State != runs.Running {
		t.Fatal("el job no ha arribat a running dins la cua")
	}
	if background.Workspace != runs.WorkspaceKey(s.cwd) {
		t.Fatalf("workspace del job: %q, volia %q", background.Workspace, runs.WorkspaceKey(s.cwd))
	}

	// Una sessió diferent que comparteix workspace no pot entrar fins que
	// s'allibera el job: la serialització ja no depèn de jobRunning.
	started := make(chan struct{})
	other, _, err := q.Submit(func(context.Context, runs.Run) error {
		close(started)
		return nil
	}, runs.Run{Session: "altra-sessio", Workspace: runs.WorkspaceKey(s.cwd)})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
		t.Fatal("un torn del mateix workspace ha començat en paral·lel al job")
	case <-time.After(100 * time.Millisecond):
	}
	if waiting, ok := q.Get(other.ID); !ok || waiting.State != runs.Queued {
		t.Fatalf("el torn conflictiu no queda en cua: %+v %v", waiting, ok)
	}

	close(release)
	got := <-resultCh
	if got.err != nil || got.run.Status != "ok" {
		t.Fatalf("job: run=%+v err=%v", got.run, got.err)
	}
	select {
	case <-q.Done(other.ID):
	case <-time.After(2 * time.Second):
		t.Fatal("el torn que esperava el workspace no ha acabat")
	}
}

// TestHandleJobsNextDueAt: l'app Android programa l'alarma amb next_due_at,
// així que ha de ser una hora absoluta (RFC 3339) i només per als jobs que
// en tenen (manuals i pausats, cap).
func TestHandleJobsNextDueAt(t *testing.T) {
	s := goalTestServer(t)
	var err error
	s.jobs, err = jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range []jobs.Job{
		{ID: "a", Name: "Notícies", Prompt: "p", Kind: "daily", Time: "08:00", Enabled: true},
		{ID: "b", Name: "Manual", Prompt: "p", Kind: "manual", Enabled: true},
		{ID: "c", Name: "Pausat", Prompt: "p", Kind: "daily", Time: "08:00"},
	} {
		if _, err := s.jobs.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	s.handleJobs(rec, httptest.NewRequest(http.MethodGet, "/api/jobs", nil))
	var out struct {
		Jobs []struct {
			ID        string `json:"id"`
			NextDueAt string `json:"next_due_at"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, j := range out.Jobs {
		got[j.ID] = j.NextDueAt
	}
	at, err := time.Parse(time.RFC3339, got["a"])
	if err != nil {
		t.Fatalf("next_due_at del diari: %q (%v)", got["a"], err)
	}
	if at.Hour() != 8 || at.Minute() != 0 || time.Since(at) > 25*time.Hour {
		t.Fatalf("franja inesperada: %v", at)
	}
	if got["b"] != "" || got["c"] != "" {
		t.Fatalf("manual i pausat sense hora: %q %q", got["b"], got["c"])
	}
}

// TestJobRunScheduled: la franja que dispara l'alarma d'Android no rebota
// amb un torn interactiu en marxa (s'encua darrere, com al jobLoop), però
// un job pausat es refusa de seguida perquè ningú n'esperi el resultat.
func TestJobRunScheduled(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	var err error
	s.jobs, err = jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range []jobs.Job{
		{ID: "on", Name: "Actiu", Prompt: "p", Kind: "daily", Time: "08:00", Enabled: true},
		{ID: "off", Name: "Pausat", Prompt: "p", Kind: "daily", Time: "08:00"},
	} {
		if _, err := s.jobs.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	s.mu.Lock()
	s.agentBusy = true
	s.mu.Unlock()
	if rec := postJSON(t, s.handleJobRun, `{"id":"on"}`); rec.Code != 409 {
		t.Fatalf("manual amb l'agent ocupat: %d", rec.Code)
	}
	if rec := postJSON(t, s.handleJobRun, `{"id":"off","scheduled":true}`); rec.Code != 409 {
		t.Fatalf("programat i pausat: %d", rec.Code)
	}
	if rec := postJSON(t, s.handleJobRun, `{"id":"on","scheduled":true}`); rec.Code != 200 {
		t.Fatalf("programat amb l'agent ocupat: %d %s", rec.Code, rec.Body.String())
	}
	// Sense proveïdor acaba en error, però acaba: queda l'execució.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if list, _ := s.jobs.Runs("on", 1); len(list) == 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("la franja programada no ha deixat cap execució")
}

func TestExternalJobScheduler(t *testing.T) {
	t.Setenv("GREGAL_JOBS_SCHEDULER", "external")
	if !ExternalJobScheduler() {
		t.Fatal("GREGAL_JOBS_SCHEDULER=external hauria d'aturar el jobLoop")
	}
	done := make(chan struct{})
	go func() { (&Server{}).jobLoop(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("el jobLoop no surt amb el programador extern")
	}
	t.Setenv("GREGAL_JOBS_SCHEDULER", "")
	if ExternalJobScheduler() {
		t.Fatal("per defecte el programa el mateix backend")
	}
}
