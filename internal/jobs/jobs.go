package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Job és una tasca programada (v1: diària, per interval o manual).
type Job struct {
	Wind       string     `json:"wind,omitempty"`
	WindOwner  string     `json:"wind_owner,omitempty"`
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prompt     string     `json:"prompt"`
	Kind       string     `json:"kind"` // "daily" | "interval" | "event" | "remind" | "manual"
	Time       string     `json:"time"` // "HH:MM" (daily, hora local)
	IntervalH  float64    `json:"interval_h"`
	Enabled    bool       `json:"enabled"`
	Notify     bool       `json:"notify"`
	LastAt     *time.Time `json:"last_at,omitempty"`
	LastStatus string     `json:"last_status,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	// NextAt és la franja puntual del kind "event" (el cor del vent): el
	// moment exacte que el mateix vent s'ha programat per tornar a mirar
	// (p. ex. just després del partit que vigila). El l'actualitza quan
	// corre; qui l'encén, l'app o un run manual, el dispara d'immediat.
	NextAt *time.Time `json:"next_at,omitempty"`
}

// Run és una execució (la sortida és markdown llest per renderitzar ric).
type Run struct {
	Wind       string    `json:"wind,omitempty"`
	WindOwner  string    `json:"wind_owner,omitempty"`
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"`
	JobName    string    `json:"job_name"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Status     string    `json:"status"` // "ok" | "error"
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// DefaultDir retorna ~/.local/share/gregal/jobs.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "gregal", "jobs")
	}
	return filepath.Join(home, ".local", "share", "gregal", "jobs")
}

// Store guarda jobs + runs en disc (JSON).
type Store struct {
	mu  sync.Mutex
	dir string
}

// New obre (creant) el directori.
func New(dir string) (*Store, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	if err := os.MkdirAll(filepath.Join(dir, "runs"), 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) jobsPath() string { return filepath.Join(s.dir, "jobs.json") }

// List retorna els jobs ordenats per nom.
func (s *Store) List() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list()
}

func (s *Store) list() ([]Job, error) {
	raw, err := os.ReadFile(s.jobsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Job{}, nil
		}
		return nil, err
	}
	var jobs []Job
	if err := json.Unmarshal(raw, &jobs); err != nil {
		return nil, err
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })
	return jobs, nil
}

// Save crea o actualitza un job (ID buit = nou, amb ID generat).
func (s *Store) Save(j Job) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.list()
	if err != nil {
		return Job{}, err
	}
	j.Name = trim(j.Name, 80)
	j.Prompt = trim(j.Prompt, 4000)
	if j.Kind != "daily" && j.Kind != "interval" && j.Kind != "event" && j.Kind != "remind" {
		j.Kind = "manual"
	}
	if j.ID == "" {
		j.ID = fmt.Sprintf("job-%s", time.Now().Format("20060102-150405"))
		j.CreatedAt = time.Now()
	}
	found := false
	for i, e := range jobs {
		if e.ID == j.ID {
			j.CreatedAt = e.CreatedAt
			if j.LastAt == nil {
				j.LastAt = e.LastAt
				j.LastStatus = e.LastStatus
			}
			if j.NextAt == nil {
				j.NextAt = e.NextAt
			}
			jobs[i] = j
			found = true
			break
		}
	}
	if !found {
		if j.CreatedAt.IsZero() {
			j.CreatedAt = time.Now()
		}
		jobs = append(jobs, j)
	}
	return j, s.persist(jobs)
}

// Delete esborra un job (les execucions es conserven).
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.list()
	if err != nil {
		return err
	}
	out := jobs[:0]
	for _, j := range jobs {
		if j.ID != id {
			out = append(out, j)
		}
	}
	return s.persist(out)
}

// MarkRun actualitza l'última execució del job.
// Has diu si el job encara existeix (un run manual d'una rutina esborrada
// no faria res i cal poder-ho rebutjar).
func (s *Store) Has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.list()
	if err != nil {
		return false
	}
	for _, j := range jobs {
		if j.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) MarkRun(id string, at time.Time, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.list()
	if err != nil {
		return err
	}
	for i, j := range jobs {
		if j.ID == id {
			jobs[i].LastAt = &at
			jobs[i].LastStatus = status
			break
		}
	}
	return s.persist(jobs)
}

// SetNextAt reprograma la franja d'un job d'esdeveniment (el cor del vent)
// sense tocar res més: ni LastAt ni LastStatus ni la programació d'altres
// jobs. Job desconegut = error, mai crea res.
func (s *Store) SetNextAt(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.list()
	if err != nil {
		return err
	}
	found := false
	for i, j := range jobs {
		if j.ID == id {
			jobs[i].NextAt = &at
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("job desconegut: %s", id)
	}
	return s.persist(jobs)
}

func (s *Store) persist(jobs []Job) error {
	raw, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.jobsPath(), raw, 0o600)
}

// AddRun desa una execució i poda a les últimes 50.
func (s *Store) AddRun(r Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = fmt.Sprintf("run-%s", time.Now().Format("20060102-150405"))
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, "runs", r.ID+".json"), raw, 0o600); err != nil {
		return err
	}
	entries, _ := os.ReadDir(filepath.Join(s.dir, "runs"))
	if len(entries) > 200 {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries[:len(entries)-200] {
			_ = os.Remove(filepath.Join(s.dir, "runs", e.Name()))
		}
	}
	return nil
}

// Runs retorna les últimes execucions (recents primer, límit n; jobID buit = totes).
func (s *Store) Runs(jobID string, n int) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.dir, "runs"))
	if err != nil {
		if os.IsNotExist(err) {
			return []Run{}, nil
		}
		return nil, err
	}
	runs := []Run{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(s.dir, "runs", e.Name()))
		if err != nil {
			continue
		}
		var r Run
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		if jobID != "" && r.JobID != jobID {
			continue
		}
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].StartedAt.After(runs[j].StartedAt) })
	if n > 0 && len(runs) > n {
		runs = runs[:n]
	}
	return runs, nil
}

// NextDue calcula la franja programada vigent: si ja ha passat i no s'ha
// executat, retorna el passat (Due() dirà que toca); si ja s'ha cobert,
// la següent franja. Zero = manual o deshabilitat.
func NextDue(j Job, now time.Time) time.Time {
	if !j.Enabled {
		return time.Time{}
	}
	switch j.Kind {
	case "daily":
		hm, err := time.Parse("15:04", j.Time)
		if err != nil {
			return time.Time{}
		}
		slot := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), 0, 0, now.Location())
		if slot.After(now) {
			slot = slot.Add(-24 * time.Hour)
		}
		if j.LastAt != nil && !j.LastAt.Before(slot) {
			return slot.Add(24 * time.Hour)
		}
		return slot
	case "interval":
		if j.IntervalH <= 0 {
			return time.Time{}
		}
		if j.LastAt == nil {
			return now // primera execució immediata
		}
		return j.LastAt.Add(time.Duration(j.IntervalH * float64(time.Hour)))
	case "event", "remind":
		// Franja puntual: el cor del vent (event) o un recordatori d'un vent
		// (remind, d'una sola execució). Toca quan arriba next_at i no torna
		// a tocar fins que es mogui o es cobreixi (LastAt >= NextAt).
		if j.NextAt == nil {
			return time.Time{}
		}
		if j.LastAt != nil && !j.LastAt.Before(*j.NextAt) {
			return time.Time{}
		}
		return *j.NextAt
	default:
		return time.Time{}
	}
}

// Due diu si cal executar ara: quan arriba la franja, mai abans. No cal
// marge pel tick: NextDue torna la franja passada fins que una execució la
// cobreix, així que un tick tardà la troba igual. Un marge d'avançada feia
// sonar un «d'aquí a 1 minut» als pocs segons, i una diària a les 08:59 que
// acabés abans de les 09:00 deixava la franja sense cobrir i corria dos cops.
func Due(j Job, now time.Time) bool {
	next := NextDue(j, now)
	if next.IsZero() {
		return false
	}
	if j.Kind == "interval" && j.LastAt == nil {
		return true
	}
	return !now.Before(next)
}

func trim(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
