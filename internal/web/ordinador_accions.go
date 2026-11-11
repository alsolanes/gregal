package web

// On actua l'agent a l'ordinador (paquet B13, pla v2).
//
// Les eines del navegador avisen de cada acció de punter
// (tools.SetComputerActionSink). Aquí:
//
//   - es redacta (secrets del config i patrons de tokens) i s'escriu com a
//     esdeveniment `computer_action` al registre durable, amb el torn, el
//     pas de l'encàrrec i el propietari;
//   - la captura petita i una còpia de l'acció es desen a
//     <dades>/ordinador-accions/<propietari>/ per a la reproducció (com a
//     molt computerKeep per propietari);
//   - el visor (/ordinador/visor/accions*) i l'API (/api/ordinador/accions*)
//     només en serveixen les del seu propietari.
//
// L'esdeveniment no porta mai la captura, ni text de camps secrets.

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/events"
	"gregal/internal/tools"
	"gregal/internal/transcript"
)

// ComputerActionKind és el kind de l'esdeveniment.
const ComputerActionKind = "computer_action"

// computerKeep és quantes accions (i captures) es guarden per propietari.
const computerKeep = 120

// computerActionView és el que veuen el visor i l'API: l'acció més el torn,
// el pas i la captura (id), mai la imatge.
type computerActionView struct {
	ID   uint64    `json:"id,omitempty"`
	At   time.Time `json:"at"`
	Run  int       `json:"run,omitempty"`
	Step string    `json:"step,omitempty"`
	Shot string    `json:"shot,omitempty"`
	tools.ComputerAction
}

var computerShotRe = regexp.MustCompile(`^[0-9a-f]{20}$`)

// computerStore desa les accions recents i les captures per propietari.
type computerStore struct {
	mu  sync.Mutex
	dir string
}

func (c *computerStore) ownerDir(owner string) string {
	k := owner
	if len(k) > 32 {
		k = k[:32]
	}
	k = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '_'
	}, k)
	return filepath.Join(c.dir, k)
}

// save afegeix l'acció i la captura del propietari i retalla les velles.
func (c *computerStore) save(v computerActionView, shot []byte) {
	if c == nil || c.dir == "" || v.Owner == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	dir := c.ownerDir(v.Owner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	if len(shot) > 0 && v.Shot != "" {
		_ = os.WriteFile(filepath.Join(dir, v.Shot+".jpg"), shot, 0o600)
	}
	line, err := json.Marshal(v)
	if err != nil {
		return
	}
	list := c.readLocked(dir)
	list = append(list, line)
	if len(list) > computerKeep {
		for _, old := range list[:len(list)-computerKeep] {
			var o computerActionView
			if json.Unmarshal(old, &o) == nil && computerShotRe.MatchString(o.Shot) {
				_ = os.Remove(filepath.Join(dir, o.Shot+".jpg"))
			}
		}
		list = list[len(list)-computerKeep:]
	}
	tmp := filepath.Join(dir, "accions.jsonl.tmp")
	if err := os.WriteFile(tmp, append(bytes.Join(list, []byte{'\n'}), '\n'), 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(dir, "accions.jsonl"))
}

func (c *computerStore) readLocked(dir string) [][]byte {
	f, err := os.Open(filepath.Join(dir, "accions.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out [][]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 16<<10), 1<<20)
	for sc.Scan() {
		if b := bytes.TrimSpace(sc.Bytes()); len(b) > 0 {
			out = append(out, append([]byte(nil), b...))
		}
	}
	return out
}

// recent torna les últimes accions del propietari: les de l'últim torn que
// en va fer (o de run, si n'hi ha), com a molt limit, de la més vella a la
// més nova.
func (c *computerStore) recent(owner string, run, limit int) []computerActionView {
	if c == nil || c.dir == "" || owner == "" {
		return nil
	}
	c.mu.Lock()
	lines := c.readLocked(c.ownerDir(owner))
	c.mu.Unlock()
	var all []computerActionView
	for _, l := range lines {
		var v computerActionView
		if json.Unmarshal(l, &v) == nil && v.Owner == owner {
			all = append(all, v)
		}
	}
	if len(all) == 0 {
		return nil
	}
	if run <= 0 {
		run = all[len(all)-1].Run
	}
	var out []computerActionView
	for _, v := range all {
		if v.Run == run {
			out = append(out, v)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// shot torna la captura d'una acció del propietari.
func (c *computerStore) shot(owner, id string) ([]byte, bool) {
	if c == nil || c.dir == "" || owner == "" || !computerShotRe.MatchString(id) {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(c.ownerDir(owner), id+".jpg"))
	return b, err == nil
}

func newShotID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

// recordComputerAction és el receptor de tools.SetComputerActionSink.
func (h *Hub) recordComputerAction(scope string, a tools.ComputerAction) {
	if h == nil {
		return
	}
	h.mu.RLock()
	s := h.sess[scope]
	h.mu.RUnlock()
	v := computerActionView{At: a.At, ComputerAction: a}
	if v.At.IsZero() {
		v.At = time.Now()
	}
	v.At = v.At.UTC()
	user, session := "", ""
	red := transcript.NewRedactor(h.def.redactOptions(false))
	if s != nil {
		user, session = s.usuari(), s.id
		red = transcript.NewRedactor(s.redactOptions(false))
		s.mu.Lock()
		if s.active != nil {
			v.Run = s.active.ID
		}
		v.Step = todoStep(s.todo)
		s.mu.Unlock()
	}
	// Defensa en profunditat: el text ja ve emmascarat dels camps secrets;
	// aquí també fora tokens i claus que s'hagin escrit en un camp normal.
	v.Text = red.Redact(v.Text)
	v.Target = red.Redact(v.Target)
	v.Step = red.Redact(v.Step)
	v.Shot = ""
	if len(a.Shot) > 0 {
		v.Shot = newShotID()
	}
	payload, err := json.Marshal(v)
	if err == nil {
		ev, aerr := h.appendComputerEvent(events.Event{
			SessionID: session, User: user, RunID: v.Run, Kind: ComputerActionKind,
			Text: computerActionText(v), Payload: payload,
		})
		if aerr == nil {
			v.ID = ev.ID
		}
	}
	h.computer.save(v, a.Shot)
}

func (h *Hub) appendComputerEvent(e events.Event) (events.Event, error) {
	if h.eventStore == nil {
		return events.Event{}, fmt.Errorf("registre d'events no disponible")
	}
	return h.eventStore.Append(e)
}

// todoStep és el pas en curs de l'encàrrec («2/5 · Omple el formulari»).
func todoStep(items []tools.TodoItem) string {
	for i, it := range items {
		if it.Status == "working" {
			return fmt.Sprintf("%d/%d · %s", i+1, len(items), strings.TrimSpace(it.Title))
		}
	}
	return ""
}

// computerActionText és el resum curt de l'esdeveniment (registre, logs de
// depuració). Els clients munten la frase planera amb el payload.
func computerActionText(v computerActionView) string {
	t := v.Kind
	if v.Target != "" {
		t += ": " + v.Target
	}
	return t
}

// computerOwnerOf és la clau del propietari de les accions d'un usuari.
func computerOwnerOf(name string) string { return tools.OwnerKeyFor(name) }

// computerEventFor llegeix l'acció d'un esdeveniment si és del propietari.
func computerEventFor(e events.Event, owner string) (computerActionView, bool) {
	if e.Kind != ComputerActionKind || len(e.Payload) == 0 {
		return computerActionView{}, false
	}
	var v computerActionView
	if json.Unmarshal(e.Payload, &v) != nil || v.Owner != owner {
		return computerActionView{}, false
	}
	v.ID = e.ID
	return v, true
}

// computerPoll és cada quan el stream del visor mira si hi ha accions noves.
// Amb 80 ms, el ressalt arriba bastant abans dels 300 ms de l'acceptació.
var computerPoll = 80 * time.Millisecond

// serveComputerStream és l'SSE de les accions del propietari, des d'ara (o
// des de ?after=). Tanca quan el client se'n va.
func (h *Hub) serveComputerStream(w http.ResponseWriter, r *http.Request, owner string) {
	fl, ok := w.(http.Flusher)
	if !ok || h.eventStore == nil {
		http.Error(w, "SSE no disponible", http.StatusServiceUnavailable)
		return
	}
	after := h.eventStore.Cursor()
	if v, err := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64); err == nil && v < after {
		after = v
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	if _, err := fmt.Fprint(w, "retry: 1000\n\n"); err != nil {
		return
	}
	fl.Flush()
	poll := time.NewTicker(computerPoll)
	defer poll.Stop()
	beat := time.NewTicker(15 * time.Second)
	defer beat.Stop()
	keep := func(e events.Event) bool { _, ok := computerEventFor(e, owner); return ok }
	for {
		got, err := h.eventStore.AfterMatch(after, 200, keep)
		if err != nil {
			return
		}
		for _, e := range got {
			v, _ := computerEventFor(e, owner)
			b, _ := json.Marshal(v)
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, ComputerActionKind, b); err != nil {
				return
			}
			after = e.ID
		}
		if len(got) > 0 {
			fl.Flush()
			continue
		}
		// Res de nou: avancem el cursor fins al final perquè la propera
		// lectura no torni a recórrer els esdeveniments d'altres.
		if c := h.eventStore.Cursor(); c > after {
			after = c
		}
		select {
		case <-r.Context().Done():
			return
		case <-poll.C:
		case <-beat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

// computerRecentJSON és la resposta de la reproducció: les accions i, per a
// cadascuna amb captura, el camí relatiu on es pot demanar.
func (h *Hub) computerRecentJSON(owner string, r *http.Request, shotBase string) map[string]any {
	run, _ := strconv.Atoi(r.URL.Query().Get("run"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > computerKeep {
		limit = 40
	}
	list := h.computer.recent(owner, run, limit)
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		b, _ := json.Marshal(v)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if v.Shot != "" {
			m["shotUrl"] = shotBase + v.Shot
		}
		out = append(out, m)
	}
	resp := map[string]any{"actions": out}
	if len(list) > 0 {
		resp["run"] = list[len(list)-1].Run
	}
	return resp
}

func serveComputerShot(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(b)
}

// handleOrdinadorAccions és GET /api/ordinador/accions (reproducció de les
// últimes accions de l'agent a l'ordinador del qui pregunta) i
// /api/ordinador/accions/captura?id=… (la captura d'una).
func (s *Server) handleOrdinadorAccions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, name := ordinadorCaller(r)
	owner := computerOwnerOf(name)
	h := s.Hub()
	writeJSON(w, h.computerRecentJSON(owner, r, "/api/ordinador/accions/captura?id="))
}

func (s *Server) handleOrdinadorAccionsCaptura(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, name := ordinadorCaller(r)
	b, ok := s.Hub().computer.shot(computerOwnerOf(name), r.URL.Query().Get("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	serveComputerShot(w, b)
}
