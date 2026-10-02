// Package runs implementa la cua d'execucions compartida del servei.
//
// Regles del pla (secció 5): una conversa té com a màxim un torn
// executant-se; els missatges addicionals entren en una cua visible; dos
// torns capaços d'escriure que comparteixen directori s'executen en sèrie.
// La prioritat separa el treball interactiu del desatès sense deixar aquest
// esperant indefinidament.
//
// La cua és en memòria: la durabilitat la dona el registre d'events, que
// recorda queued/started i la fi de cada execució. Si el procés mor, els
// torns no encesos desapareixen i el client pot tornar-los a enviar
// (la idempotència per clau evita duplicats mentre el procés viu).
package runs

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// State és el cicle de vida d'una execució.
type State string

const (
	Queued    State = "queued"
	Running   State = "running"
	Completed State = "completed"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

// Active diu si l'execució encara pot canviar d'estat.
func (s State) Active() bool { return s == Queued || s == Running }

// Prioritats. Menor número = surt abans de la cua de la seva sessió. El
// treball interactiu passa davant del desatès, però la serialització per
// workspace impedeix que un job gamin mai: espera el seu torn com tothom.
const (
	PriorityInteractive = 0
	PriorityBackground  = 10
)

// ErrCancelled el retorna l'Executor quan el ctx s'ha tallat: la cua marca
// l'execució com a cancel·lada i no com a fallida.
var ErrCancelled = errors.New("runs: execució cancel·lada")

// ErrQueueFull: massa torns pendents per a la mateixa conversa. El client
// ha de mostrar-ho en comptes d'encuar a cegues.
var ErrQueueFull = errors.New("runs: massa torns pendents per a aquesta conversa")

// maxPerSession és el topall de torns vius (pendents o en marxa) per sessió.
const maxPerSession = 8

// Run descriu una execució de la cua.
type Run struct {
	ID         int64     `json:"id"`
	Session    string    `json:"session,omitempty"`   // clau de sessió dins del servei
	Workspace  string    `json:"workspace,omitempty"` // clau canònica del directori
	Priority   int       `json:"priority,omitempty"`  // menor = abans
	Key        string    `json:"key,omitempty"`       // clau d'idempotència del client
	State      State     `json:"state"`
	Err        string    `json:"error,omitempty"`
	EnqueuedAt time.Time `json:"enqueued_at,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// Executor executa el torn. Rep un ctx que Cancel talla i emet els events
// pel seu compte (SSE, registre durable o tots dos). L'error decideix l'estat
// final: nil → completed, ErrCancelled → cancelled, altres → failed.
type Executor func(ctx context.Context, run Run) error

type slot struct {
	run    Run
	exec   Executor
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	seq    int64 // ordre d'arribada dins la sessió
}

// Queue és la cua compartida del procés. LesSessions ("converses") avancen
// en paral·lel; els torns que comparteixen Workspace s'executen en sèrie.
type Queue struct {
	mu      sync.Mutex
	nextID  int64
	nextSeq int64
	byID    map[int64]*slot
	pend    map[string][]*slot // sessió → pendents
	running map[string]*slot   // sessió → torn en execució (el worker el desencua)
	worker  map[string]bool    // sessió → worker viu
	ws      map[string]chan struct{}
}

// New crea la cua buida.
func New() *Queue {
	return &Queue{
		byID:    map[int64]*slot{},
		pend:    map[string][]*slot{},
		running: map[string]*slot{},
		worker:  map[string]bool{},
		ws:      map[string]chan struct{}{},
	}
}

// WorkspaceKey normalitza un directori per agrupar torns del mateix
// workspace. A Windows el sistema de fitxers no distingeix majúscules, i
// C:\a\app amb barra final és el mateix directori que C:\a\app\.
func WorkspaceKey(dir string) string {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." {
		return ""
	}
	if runtime.GOOS == "windows" {
		dir = strings.ToLower(dir)
	}
	return dir
}

// Submit encua un torn. Si una execució viva (o ja acabada) de la mateixa
// sessió duu la mateixa clau d'idempotència, retorna l'existent sense
// encuar res (existing=true). Compte amb la cancel·lació: un torn
// cancel·lat amb la mateixa clau es pot tornar a enviar.
func (q *Queue) Submit(exec Executor, r Run) (run Run, existing bool, err error) {
	if exec == nil {
		return Run{}, false, errors.New("runs: execució buida")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if r.Key != "" {
		for _, sl := range q.byID {
			if sl.run.Session == r.Session && sl.run.Key == r.Key && sl.run.State != Cancelled {
				return sl.run, true, nil
			}
		}
	}
	vius := 0
	if q.running[r.Session] != nil {
		vius++
	}
	if vius+len(q.pend[r.Session]) >= maxPerSession {
		return Run{}, false, ErrQueueFull
	}
	q.nextSeq++
	q.nextID++
	run = r
	run.ID = q.nextID
	// Normalize at the queue boundary as well as at the HTTP/TUI adapters.
	// The queue is the owner of workspace serialization, so accepting a raw
	// path here would let another caller bypass the lock with a trailing
	// separator, a relative spelling, or (on Windows) different casing.
	run.Workspace = WorkspaceKey(run.Workspace)
	run.Key = strings.TrimSpace(run.Key)
	run.State = Queued
	run.EnqueuedAt = time.Now().UTC()
	ctx, cancel := context.WithCancel(context.Background())
	sl := &slot{run: run, exec: exec, ctx: ctx, cancel: cancel, done: make(chan struct{}), seq: q.nextSeq}
	q.byID[run.ID] = sl
	q.pend[r.Session] = append(q.pend[r.Session], sl)
	if !q.worker[r.Session] {
		q.worker[r.Session] = true
		go q.work(r.Session)
	}
	return run, false, nil
}

// work és el bucle d'una sessió: desencua per prioritat i ordre d'arribada,
// espera el token del workspace i executa un torn a la vegada.
func (q *Queue) work(sess string) {
	for {
		q.mu.Lock()
		sl := q.popLocked(sess)
		if sl == nil {
			delete(q.worker, sess)
			q.mu.Unlock()
			return
		}
		q.running[sess] = sl
		q.mu.Unlock()

		ws := sl.run.Workspace
		held := false
		if ws != "" {
			select {
			case <-sl.ctx.Done():
				q.finish(sl, Cancelled, "")
				q.clearRunning(sess)
				continue
			case <-q.token(ws):
				held = true
			}
		}

		// Cancel·lat mentre esperava el workspace? L'estat es comprova i es
		// fixa dins el mateix bloqueig per no ficar-ne un de ja tancat.
		q.mu.Lock()
		if !sl.run.State.Active() {
			q.mu.Unlock()
			if held {
				q.release(ws)
			}
			q.clearRunning(sess)
			continue
		}
		sl.run.State = Running
		sl.run.StartedAt = time.Now().UTC()
		q.mu.Unlock()

		var execErr error
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					execErr = errors.New("torn interromput: pànic a l'executor")
				}
			}()
			execErr = sl.exec(sl.ctx, sl.run)
		}()

		switch {
		case execErr == nil:
			q.finish(sl, Completed, "")
		case errors.Is(execErr, ErrCancelled) || sl.ctx.Err() != nil:
			q.finish(sl, Cancelled, "")
		default:
			q.finish(sl, Failed, execErr.Error())
		}
		q.clearRunning(sess)
		if held {
			q.release(ws)
		}
	}
}

func (q *Queue) clearRunning(sess string) {
	q.mu.Lock()
	delete(q.running, sess)
	q.mu.Unlock()
}

// popLocked tria el següent pendent viu de la sessió (prioritat menor, després
// ordre d'arribada) i el treu de la llista. Neteja els cancel·lats.
func (q *Queue) popLocked(sess string) *slot {
	lst := q.pend[sess]
	best := -1
	for i, sl := range lst {
		if !sl.run.State.Active() {
			continue
		}
		if best == -1 || sl.run.Priority < lst[best].run.Priority ||
			(sl.run.Priority == lst[best].run.Priority && sl.seq < lst[best].seq) {
			best = i
		}
	}
	if best == -1 {
		delete(q.pend, sess)
		return nil
	}
	sl := lst[best]
	q.pend[sess] = append(lst[:best], lst[best+1:]...)
	return sl
}

// finish tanca el cicle de vida d'un slot. Únic punt que escriu estat final
// i tanca done, sempre dins el bloqueig: ni Cancel ni el worker poden
// duplicar el close del canal.
func (q *Queue) finish(sl *slot, st State, msg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !sl.run.State.Active() {
		return
	}
	sl.run.State = st
	sl.run.Err = msg
	sl.run.FinishedAt = time.Now().UTC()
	sl.cancel()
	close(sl.done)
}

// Cancel talla una execució viva. Si encara estava en cua, no s'executarà
// mai. Retorna cert si n'hi havia cap de viva amb aquest ID.
func (q *Queue) Cancel(id int64) bool {
	q.mu.Lock()
	sl := q.byID[id]
	if sl == nil || !sl.run.State.Active() {
		q.mu.Unlock()
		return false
	}
	wasQueued := sl.run.State == Queued
	if wasQueued {
		sl.run.State = Cancelled
		sl.run.FinishedAt = time.Now().UTC()
		sl.cancel()
		close(sl.done)
	}
	q.mu.Unlock()
	if !wasQueued {
		// En marxa: el ctx talla l'executor, i el worker marca l'estat final.
		sl.cancel()
	}
	return true
}

// CancelSession cancel·la tot el que sigui viu d'una sessió: el torn en marxa
// i tots els pendents. Retorna quants ha tocat. La distinció important amb
// Cancel: aquí el qui crida no coneix els IDs («talla-ho tot i escolta'm»,
// l'ordre /ara del bot), així que recorre la sessió sencera. Els torns en
// cua es marquen Cancelled i tanquen el seu done dins el bloqueig (com fa
// Cancel); el torn en marxa només es cancel·la el context i el worker hi
// posa l'estat final.
func (q *Queue) CancelSession(sess string) int {
	q.mu.Lock()
	n := 0
	for _, sl := range q.pend[sess] {
		if sl.run.State != Queued {
			continue
		}
		sl.run.State = Cancelled
		sl.run.FinishedAt = time.Now().UTC()
		sl.cancel()
		close(sl.done)
		n++
	}
	delete(q.pend, sess)
	if cur := q.running[sess]; cur != nil && cur.run.State == Running {
		cur.cancel()
		n++
	}
	q.mu.Unlock()
	return n
}

// Get retorna una còpia de l'execució.
func (q *Queue) Get(id int64) (Run, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	sl := q.byID[id]
	if sl == nil {
		return Run{}, false
	}
	return sl.run, true
}

// Done és el canal que es tanca quan l'execució acaba (bé o malament).
func (q *Queue) Done(id int64) <-chan struct{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	if sl := q.byID[id]; sl != nil {
		return sl.done
	}
	c := make(chan struct{})
	close(c)
	return c
}

// ListSession torna les execucions d'una sessió, de més nova a més vella.
func (q *Queue) ListSession(sess string) []Run {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := []Run{}
	for _, sl := range q.byID {
		if sl.run.Session == sess {
			out = append(out, sl.run)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// token retorna el canal-token d'un workspace, creant-lo si cal. Un sol
// valor dins un canal bufferitzat fa de verrelligam; els receptors
// bloquejats surten en FIFO, així que ningú morí de fam.
func (q *Queue) token(ws string) <-chan struct{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	ch, ok := q.ws[ws]
	if !ok {
		ch = make(chan struct{}, 1)
		ch <- struct{}{}
		q.ws[ws] = ch
	}
	return ch
}

func (q *Queue) release(ws string) {
	q.mu.Lock()
	ch := q.ws[ws]
	q.mu.Unlock()
	if ch != nil {
		ch <- struct{}{}
	}
}
