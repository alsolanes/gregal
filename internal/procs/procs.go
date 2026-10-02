// Package procs gestiona processos llargs en segon pla.
//
// El bash de l'agent té un topall de 2 minuts: prou per a `go test ./...`
// d'un paquet, insuficient per a `npm run dev`, una suite sencera o un
// build. Aquí els processos viuen fora del torn: s'engeguen, se'n llegeix
// la sortida a trossos i es maten quan cal.
package procs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/shell"
)

// MaxLines és el búfer circular de sortida per procés.
const MaxLines = 5000

// Line és una línia de sortida amb el seu origen.
type Line struct {
	N      int    `json:"n"`
	Stream string `json:"stream"` // "out" o "err"
	Text   string `json:"text"`
}

// Proc és un procés en segon pla.
type Proc struct {
	ID      string    `json:"id"`
	Cmd     string    `json:"cmd"`
	Dir     string    `json:"dir"`
	Session string    `json:"session"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitempty"`
	Exit    *int      `json:"exit,omitempty"`
	Running bool      `json:"running"`

	mu     sync.Mutex
	lines  []Line
	first  int // número de la primera línia que encara tenim
	total  int
	cancel func()
	done   chan struct{}
}

// Snapshot és la vista serialitzable d'un procés (sense mutex).
type Snapshot struct {
	ID      string `json:"id"`
	Cmd     string `json:"cmd"`
	Dir     string `json:"dir"`
	Session string `json:"session"`
	Started string `json:"started"`
	Running bool   `json:"running"`
	Exit    *int   `json:"exit,omitempty"`
	Lines   int    `json:"lines"`
}

// Store guarda els processos vius i els acabats recentment.
type Store struct {
	mu    sync.Mutex
	procs map[string]*Proc
	order []string
	seq   int64
}

// New crea un magatzem buit.
func New() *Store { return &Store{procs: map[string]*Proc{}} }

// MaxProcs limita quants processos es recorden alhora.
const MaxProcs = 40

// Start engega `sh -c cmd` a dir i retorna el procés.
func (s *Store) Start(session, dir, cmd string) (*Proc, error) {
	if cmd == "" {
		return nil, errors.New("cal una ordre")
	}
	// L'intèrpret el tria internal/shell (sh, el sh del Git a Windows, o cmd):
	// amb "sh" a pèl el portable de Windows no podia engegar cap procés.
	// Les rutes Windows es tradueixen com a l'eina bash.
	name, args := shell.Argv(shell.NormalitzaPathsWindows(cmd))
	c := exec.Command(name, args...)
	c.Dir = dir
	setGroup(c)
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	id := "p" + strconv.FormatInt(atomic.AddInt64(&s.seq, 1), 10)
	p := &Proc{
		ID: id, Cmd: cmd, Dir: dir, Session: session,
		Started: time.Now(), Running: true, done: make(chan struct{}),
	}
	p.cancel = func() { killGroup(c) }
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); p.pump("out", stdout) }()
	go func() { defer wg.Done(); p.pump("err", stderr) }()
	go func() {
		wg.Wait()
		err := c.Wait()
		code := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			} else {
				code = -1
				p.add("err", err.Error())
			}
		}
		p.mu.Lock()
		p.Running = false
		p.Ended = time.Now()
		p.Exit = &code
		p.mu.Unlock()
		close(p.done)
	}()
	s.mu.Lock()
	s.procs[id] = p
	s.order = append(s.order, id)
	s.evictLocked()
	s.mu.Unlock()
	return p, nil
}

// evictLocked treu els acabats més antics quan se'n van acumulant.
func (s *Store) evictLocked() {
	for len(s.order) > MaxProcs {
		for i, id := range s.order {
			p := s.procs[id]
			if p == nil || !p.IsRunning() {
				delete(s.procs, id)
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
			if i == len(s.order)-1 {
				return // tots corrent: no toquem res
			}
		}
	}
}

func (p *Proc) pump(stream string, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		p.add(stream, sc.Text())
	}
}

func (p *Proc) add(stream, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.total++
	p.lines = append(p.lines, Line{N: p.total, Stream: stream, Text: text})
	if len(p.lines) > MaxLines {
		drop := len(p.lines) - MaxLines
		p.lines = p.lines[drop:]
		p.first += drop
	}
}

// IsRunning diu si encara corre.
func (p *Proc) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Running
}

// Output retorna les línies amb N > from i si el procés ja ha acabat.
func (p *Proc) Output(from int) ([]Line, bool, *int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []Line
	for _, l := range p.lines {
		if l.N > from {
			out = append(out, l)
		}
	}
	return out, !p.Running, p.Exit
}

// Wait espera que acabi (amb topall) i diu si ha acabat.
func (p *Proc) Wait(d time.Duration) bool {
	select {
	case <-p.done:
		return true
	case <-time.After(d):
		return false
	}
}

// Kill mata el procés.
func (p *Proc) Kill() {
	if p.cancel != nil {
		p.cancel()
	}
}

// Get retorna un procés per id.
func (s *Store) Get(id string) *Proc {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.procs[id]
}

// Kill mata el procés id. Fals si no existeix.
func (s *Store) Kill(id string) bool {
	p := s.Get(id)
	if p == nil {
		return false
	}
	p.Kill()
	p.Wait(3 * time.Second)
	return true
}

// KillSession mata tots els processos d'una sessió (en tancar-la).
func (s *Store) KillSession(session string) int {
	s.mu.Lock()
	var victims []*Proc
	for _, p := range s.procs {
		if p.Session == session && p.IsRunning() {
			victims = append(victims, p)
		}
	}
	s.mu.Unlock()
	for _, p := range victims {
		p.Kill()
	}
	return len(victims)
}

// List retorna el resum de tots els processos, els més nous primer.
func (s *Store) List(session string) []Snapshot {
	s.mu.Lock()
	ids := append([]string(nil), s.order...)
	byID := make(map[string]*Proc, len(s.procs))
	for k, v := range s.procs {
		byID[k] = v
	}
	s.mu.Unlock()
	out := make([]Snapshot, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		p := byID[ids[i]]
		if p == nil || (session != "" && p.Session != session) {
			continue
		}
		p.mu.Lock()
		out = append(out, Snapshot{
			ID: p.ID, Cmd: p.Cmd, Dir: p.Dir, Session: p.Session,
			Started: p.Started.Format(time.RFC3339), Running: p.Running,
			Exit: p.Exit, Lines: p.total,
		})
		p.mu.Unlock()
	}
	return out
}

// Tail dona les últimes n línies en text pla (per a l'agent).
func (p *Proc) Tail(n int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	start := 0
	if len(p.lines) > n {
		start = len(p.lines) - n
	}
	out := ""
	for _, l := range p.lines[start:] {
		if l.Stream == "err" {
			out += "! " + l.Text + "\n"
			continue
		}
		out += l.Text + "\n"
	}
	if out == "" {
		return "(sense sortida encara)"
	}
	return out
}

// Describe resumeix l'estat per a l'agent.
func (p *Proc) Describe() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Running {
		return fmt.Sprintf("[%s] EN MARXA · %s · %d línies", p.ID, p.Cmd, p.total)
	}
	code := 0
	if p.Exit != nil {
		code = *p.Exit
	}
	return fmt.Sprintf("[%s] ACABAT (codi %d) · %s · %d línies", p.ID, code, p.Cmd, p.total)
}
