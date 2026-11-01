package web

// Mode Kanban (experimental): un tauler on cada targeta és un TUI de Gregal
// viu dins d'un pseudoterminal. La targeta es mou de columna segons l'estat
// que el mateix TUI publica (internal/tui/kanban.go) i el servidor manté la
// pantalla de cada terminal amb un emulador VT propi (internal/vterm): el
// navegador només pinta les línies que rep, i les targetes minimitzades no
// costen res més que un resum.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/knowledge"
	"gregal/internal/mcp"
	"gregal/internal/pty"
	"gregal/internal/tui"
	"gregal/internal/vterm"
)

// Columnes del tauler, en ordre.
const (
	kbDefinition = "definition" // esperant tasca, o planificant
	kbReady      = "ready"      // té tasca i espera torn per arrencar
	kbProgress   = "progress"   // treballant
	kbReview     = "review"     // espera l'usuari: permís, pregunta o pla
	kbDone       = "done"       // ha acabat el torn
)

var kanbanColumns = []string{kbDefinition, kbReady, kbProgress, kbReview, kbDone}

// kanbanColumnFor tradueix l'estat que publica el TUI a una columna. Un
// estat buit vol dir que el terminal encara arrenca o espera lloc.
func kanbanColumnFor(activity string) string {
	switch activity {
	case tui.KanbanIdle, tui.KanbanPlanning:
		return kbDefinition
	case tui.KanbanWorking:
		return kbProgress
	case tui.KanbanApproval, tui.KanbanQuestion, tui.KanbanPlanReview:
		return kbReview
	case tui.KanbanDone:
		return kbDone
	}
	return kbReady
}

func validKanbanColumn(c string) bool {
	for _, k := range kanbanColumns {
		if k == c {
			return true
		}
	}
	return false
}

// Límits: quants terminals vius alhora (la resta espera a «Preparada») i
// quantes targetes pot tenir un tauler.
const (
	kanbanDefaultLive = 6
	kanbanMaxCards    = 40
	kanbanCols        = 120
	kanbanRows        = 34
)

func kanbanMaxLive() int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("GREGAL_KANBAN_MAX"))); err == nil && n > 0 && n <= 32 {
		return n
	}
	return kanbanDefaultLive
}

// kanbanCard és una targeta. Els camps amb json es desen; el terminal viu no.
type kanbanCard struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Prompt    string    `json:"prompt,omitempty"`
	Dir       string    `json:"dir"`
	Column    string    `json:"column"`
	Activity  string    `json:"activity,omitempty"`
	Manual    string    `json:"manual,omitempty"`     // columna triada a mà
	ManualFor string    `json:"manual_for,omitempty"` // estat en què s'ha triat
	Detail    string    `json:"detail,omitempty"`
	Intent    string    `json:"intent,omitempty"`
	Task      string    `json:"task,omitempty"`
	Turns     int       `json:"turns,omitempty"`
	Order     int64     `json:"order"`
	Cols      int       `json:"cols,omitempty"`
	Rows      int       `json:"rows,omitempty"`
	Exit      *int      `json:"exit,omitempty"`
	Tail      []string  `json:"tail,omitempty"` // últimes línies pintades
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`

	term   *kanbanTerm
	queued bool
	sent   bool // la tasca inicial ja s'ha enviat a algun TUI
}

// recompute posa la columna efectiva. Una columna triada a mà val mentre
// l'estat de la feina sigui el mateix que quan es va triar: quan el TUI
// canvia d'estat, torna a manar l'estat.
func (c *kanbanCard) recompute() {
	if c.Manual != "" && c.ManualFor != c.Activity {
		c.Manual, c.ManualFor = "", ""
	}
	if c.Manual != "" {
		c.Column = c.Manual
		return
	}
	c.Column = kanbanColumnFor(c.Activity)
}

type kanbanTerm struct {
	p          pty.PTY
	vt         *vterm.Term
	statusPath string
	started    time.Time
	lastOut    atomic.Int64 // UnixMilli de l'última sortida
	done       chan struct{}
	statusMod  time.Time
}

func (t *kanbanTerm) alive() bool {
	if t == nil {
		return false
	}
	select {
	case <-t.done:
		return false
	default:
		return true
	}
}

// Ganxos substituïbles pels tests: cap test engega un TUI de debò.
var (
	kanbanSpawn = pty.Start
	kanbanExe   = os.Executable
)

type kanbanBoard struct {
	mu      sync.Mutex
	path    string
	dir     string // <data>/kanban
	cfgPath string
	cards   []*kanbanCard
}

var (
	kanbanBoardsMu sync.Mutex
	kanbanBoards   = map[string]*kanbanBoard{}
)

func kanbanDataDir() string { return filepath.Join(knowledge.DataDir(), "kanban") }

// kanbanBoardFor carrega (un cop) el tauler d'un usuari.
func kanbanBoardFor(user, cfgPath string) *kanbanBoard {
	kanbanBoardsMu.Lock()
	defer kanbanBoardsMu.Unlock()
	if b := kanbanBoards[user]; b != nil {
		if cfgPath != "" {
			b.cfgPath = cfgPath
		}
		return b
	}
	dir := kanbanDataDir()
	name := "board.json"
	if user != "" {
		name = "board-" + safeKanbanName(user) + ".json"
	}
	b := &kanbanBoard{path: filepath.Join(dir, name), dir: dir, cfgPath: cfgPath}
	b.load()
	kanbanBoards[user] = b
	return b
}

func safeKanbanName(s string) string {
	var out strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			out.WriteRune(r)
		} else {
			out.WriteByte('_')
		}
	}
	return out.String()
}

// load llegeix el tauler. Cap terminal sobreviu al servidor: les targetes
// tornen aturades i es poden reengegar.
func (b *kanbanBoard) load() {
	raw, err := os.ReadFile(b.path)
	if err != nil {
		return
	}
	var saved struct {
		Cards []*kanbanCard `json:"cards"`
	}
	if json.Unmarshal(raw, &saved) != nil {
		return
	}
	for _, c := range saved.Cards {
		if c == nil || c.ID == "" {
			continue
		}
		if c.Exit == nil {
			code := -1
			c.Exit = &code
		}
		c.sent = c.sent || c.Activity != "" && c.Activity != tui.KanbanIdle
		c.recompute()
		b.cards = append(b.cards, c)
	}
}

// saveLocked desa el tauler. Cal tenir b.mu.
func (b *kanbanBoard) saveLocked() {
	raw, err := json.MarshalIndent(map[string]any{"cards": b.cards}, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(b.dir, 0o700) != nil {
		return
	}
	tmp := b.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, b.path)
	}
}

func (b *kanbanBoard) find(id string) *kanbanCard {
	for _, c := range b.cards {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (b *kanbanBoard) liveLocked() int {
	n := 0
	for _, c := range b.cards {
		if c.term.alive() {
			n++
		}
	}
	return n
}

func newKanbanID() string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

// create afegeix una targeta i l'engega (o la deixa a la cua si ja hi ha
// massa terminals vius).
func (b *kanbanBoard) create(title, prompt, dir string) (*kanbanCard, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.cards) >= kanbanMaxCards {
		return nil, errors.New("too many cards: delete some first")
	}
	now := time.Now().UTC()
	if title == "" {
		title = kanbanTitleFrom(prompt)
	}
	if title == "" {
		title = "Terminal " + strconv.Itoa(len(b.cards)+1)
	}
	c := &kanbanCard{ID: newKanbanID(), Title: title, Prompt: prompt, Dir: dir, Order: now.UnixNano(),
		Created: now, Updated: now, Cols: kanbanCols, Rows: kanbanRows}
	b.cards = append(b.cards, c)
	err := b.startLocked(c)
	c.recompute()
	b.saveLocked()
	return c, err
}

func kanbanTitleFrom(prompt string) string {
	p := strings.Join(strings.Fields(prompt), " ")
	r := []rune(p)
	if len(r) > 60 {
		return string(r[:57]) + "…"
	}
	return p
}

// startLocked engega el TUI de la targeta, o l'encua si no hi ha lloc.
func (b *kanbanBoard) startLocked(c *kanbanCard) error {
	if c.term.alive() {
		return nil
	}
	c.Exit, c.Activity, c.Detail, c.Intent = nil, "", "", ""
	c.Manual, c.ManualFor = "", ""
	if b.liveLocked() >= kanbanMaxLive() {
		c.queued = true
		c.Detail = "queued"
		c.recompute()
		return nil
	}
	c.queued = false
	exe, err := kanbanExe()
	if err != nil {
		return b.failLocked(c, err)
	}
	statusPath := filepath.Join(b.dir, "status", c.ID+".json")
	_ = os.MkdirAll(filepath.Dir(statusPath), 0o700)
	_ = os.Remove(statusPath)
	env := kanbanEnv(statusPath)
	if c.Prompt != "" && !c.sent {
		env = append(env, tui.KanbanPromptEnv+"="+c.Prompt)
		c.sent = true
	}
	var args []string
	if b.cfgPath != "" {
		args = append(args, "--config", b.cfgPath)
	}
	cols, rows := c.Cols, c.Rows
	if cols <= 0 || rows <= 0 {
		cols, rows = kanbanCols, kanbanRows
	}
	t := &kanbanTerm{statusPath: statusPath, started: time.Now(), done: make(chan struct{})}
	// Les respostes de l'emulador (posició del cursor, colors) tornen al
	// TUI per l'entrada, com ho faria un terminal de debò.
	t.vt = vterm.New(cols, rows, func(r []byte) {
		if t.p != nil {
			_, _ = t.p.Write(r)
		}
	})
	p, err := kanbanSpawn(pty.Spec{Path: exe, Args: args, Dir: c.Dir, Env: env, Cols: cols, Rows: rows})
	if err != nil {
		return b.failLocked(c, err)
	}
	t.p = p
	c.term = t
	c.Updated = time.Now().UTC()
	go t.pump()
	go b.watch(c, t)
	return nil
}

func (b *kanbanBoard) failLocked(c *kanbanCard, err error) error {
	code := -1
	c.Exit = &code
	c.Detail = err.Error()
	c.recompute()
	return err
}

// kanbanEnv és l'entorn del TUI fill: el del servidor (amb GREGAL_DATA_DIR,
// GREGAL_AUTH i companyia, així l'aïllament es manté) sense les marques que
// li prohibirien obrir el TUI, i amb un terminal de 256 colors.
func kanbanEnv(statusPath string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case mcp.ChildEnv, tui.KanbanStatusEnv, tui.KanbanPromptEnv, "TERM", "COLORTERM", "NO_COLOR":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TERM=xterm-256color", "COLORTERM=truecolor", tui.KanbanStatusEnv+"="+statusPath)
}

// pump passa la sortida del pseudoterminal a l'emulador.
func (t *kanbanTerm) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := t.p.Read(buf)
		if n > 0 {
			_, _ = t.vt.Write(buf[:n])
			t.lastOut.Store(time.Now().UnixMilli())
		}
		if err != nil {
			return
		}
	}
}

// watch segueix el fitxer d'estat mentre el procés viu, i en acabar marca
// la targeta com a aturada i engega la següent de la cua.
func (b *kanbanBoard) watch(c *kanbanCard, t *kanbanTerm) {
	exited := make(chan int, 1)
	go func() {
		code, _ := t.p.Wait()
		exited <- code
	}()
	tick := time.NewTicker(400 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case code := <-exited:
			b.readStatus(c, t)
			_ = t.p.Close()
			b.mu.Lock()
			if c.term == t {
				c.Exit = &code
				c.Tail = kanbanTail(t.vt, 6)
				c.Updated = time.Now().UTC()
			}
			// Dins del lock: qui vegi el terminal mort ja hi veu el codi.
			close(t.done)
			b.startQueuedLocked()
			b.saveLocked()
			b.mu.Unlock()
			return
		case <-tick.C:
			b.readStatus(c, t)
		}
	}
}

// readStatus llegeix el fitxer d'estat si ha canviat.
func (b *kanbanBoard) readStatus(c *kanbanCard, t *kanbanTerm) {
	fi, err := os.Stat(t.statusPath)
	if err != nil || fi.ModTime().Equal(t.statusMod) {
		return
	}
	raw, err := os.ReadFile(t.statusPath)
	if err != nil {
		return // el TUI l'està reescrivint: el proper tic
	}
	var st tui.KanbanStatus
	if json.Unmarshal(raw, &st) != nil {
		return
	}
	t.statusMod = fi.ModTime()
	b.applyStatus(c, t, st)
}

func (b *kanbanBoard) applyStatus(c *kanbanCard, t *kanbanTerm, st tui.KanbanStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if c.term != t {
		return
	}
	changed := c.Activity != st.State
	c.Activity, c.Detail, c.Intent, c.Task, c.Turns = st.State, st.Detail, st.Intent, st.Task, st.Turns
	c.recompute()
	c.Updated = time.Now().UTC()
	if changed {
		// La pantalla es desa amb cada canvi d'estat: si el servidor mor
		// de cop, la targeta aturada encara ensenya on era.
		c.Tail = kanbanTail(t.vt, 6)
		b.saveLocked()
	}
}

func (b *kanbanBoard) startQueuedLocked() {
	var queued []*kanbanCard
	for _, c := range b.cards {
		if c.queued {
			queued = append(queued, c)
		}
	}
	sort.Slice(queued, func(i, j int) bool { return queued[i].Order < queued[j].Order })
	for _, c := range queued {
		if b.liveLocked() >= kanbanMaxLive() {
			return
		}
		_ = b.startLocked(c)
		c.recompute()
	}
}

// kanbanTail torna les últimes n files no buides de la pantalla, pintades.
// Al TUI de Gregal, el més útil per a una targeta petita és la conversa i
// la barra d'estat, no el quadre d'escriure: si troba la vora de dalt del
// composer (╭) a baix de tot, agafa el que hi ha a sobre i hi afegeix
// l'última línia (la barra d'estat).
func kanbanTail(vt *vterm.Term, n int) []string {
	lines := vt.Lines()
	text := vt.Text()
	last := len(text) - 1
	for last >= 0 && strings.TrimSpace(text[last]) == "" {
		last--
	}
	if last < 0 {
		return nil
	}
	end, status := last+1, -1
	for i := last; i >= 0 && i > last-12; i-- {
		if strings.HasPrefix(strings.TrimSpace(text[i]), "╭") {
			end, status = i, last
			break
		}
	}
	want := n
	if status >= 0 {
		want--
	}
	var out []string
	for i := end - 1; i >= 0 && len(out) < want; i-- {
		if t := strings.TrimSpace(strings.Trim(text[i], "│ ")); t == "" || strings.Trim(t, "─╌·-") == "" {
			continue
		}
		out = append([]string{vterm.LineHTML(lines[i], -1)}, out...)
	}
	if status >= 0 {
		out = append(out, vterm.LineHTML(lines[status], -1))
	}
	return out
}

// kill atura el terminal d'una targeta (o la treu de la cua).
func (b *kanbanBoard) killLocked(c *kanbanCard) {
	if c.queued {
		c.queued = false
		code := -1
		c.Exit = &code
		c.Detail = ""
	}
	if c.term.alive() {
		_ = c.term.p.Kill()
	}
}

// stopAll atura tots els terminals (en tancar el servidor).
func kanbanStopAll() {
	kanbanBoardsMu.Lock()
	boards := make([]*kanbanBoard, 0, len(kanbanBoards))
	for _, b := range kanbanBoards {
		boards = append(boards, b)
	}
	kanbanBoardsMu.Unlock()
	for _, b := range boards {
		b.mu.Lock()
		for _, c := range b.cards {
			c.queued = false
			if c.term.alive() {
				_ = c.term.p.Kill()
			}
		}
		b.saveLocked()
		b.mu.Unlock()
	}
}

// view és el que veu el client d'una targeta.
func (c *kanbanCard) view() map[string]any {
	alive := c.term.alive()
	v := map[string]any{
		"id": c.ID, "title": c.Title, "prompt": c.Prompt, "dir": c.Dir,
		"column": c.Column, "activity": c.Activity, "manual": c.Manual != "",
		"detail": c.Detail, "intent": c.Intent, "task": c.Task, "turns": c.Turns,
		"alive": alive, "queued": c.queued, "order": c.Order,
		"created": c.Created, "updated": c.Updated,
	}
	if c.Exit != nil && !alive {
		v["exit"] = *c.Exit
	}
	if alive {
		v["pid"] = c.term.p.Pid()
		v["tail"] = kanbanTail(c.term.vt, 6)
		if ms := c.term.lastOut.Load(); ms > 0 {
			v["idle_ms"] = time.Now().UnixMilli() - ms
		}
		if title := c.term.vt.Title(); title != "" {
			v["term_title"] = title
		}
	} else {
		v["tail"] = c.Tail
	}
	return v
}

func (b *kanbanBoard) snapshot() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	cards := append([]*kanbanCard(nil), b.cards...)
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].Order < cards[j].Order })
	out := make([]map[string]any, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.view())
	}
	return map[string]any{"columns": kanbanColumns, "cards": out, "live": b.liveLocked(), "max_live": kanbanMaxLive()}
}

// --- HTTP ---

// kanbanAccess: engegar TUIs és com obrir un terminal al servidor. Només
// l'amo (ús local) o un administrador.
func (s *Server) kanbanBoard(w http.ResponseWriter, r *http.Request) *kanbanBoard {
	u := userFrom(r)
	if u != nil && !u.Admin {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "kanban mode needs an administrator"})
		return nil
	}
	name := ""
	if u != nil {
		name = u.Name
	}
	return kanbanBoardFor(name, s.cfgPath)
}

func kanbanErr(w http.ResponseWriter, code int, msg string) {
	writeJSONStatus(w, code, map[string]string{"error": msg})
}

// handleKanban: GET el tauler; POST crea una targeta {title?,prompt?,dir?}.
func (s *Server) handleKanban(w http.ResponseWriter, r *http.Request) {
	b := s.kanbanBoard(w, r)
	if b == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, b.snapshot())
	case http.MethodPost:
		var req struct {
			Title  string `json:"title"`
			Prompt string `json:"prompt"`
			Dir    string `json:"dir"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req) != nil {
			kanbanErr(w, 400, "invalid JSON")
			return
		}
		dir := strings.TrimSpace(req.Dir)
		if dir == "" {
			s.mu.Lock()
			dir = s.cwd
			s.mu.Unlock()
			if a := userFrom(r).Arrel(); a != "" && dir == "" {
				dir = a
			}
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			kanbanErr(w, 400, "folder not found: "+dir)
			return
		}
		if err := userFrom(r).Allow(dir); err != nil {
			kanbanErr(w, 403, err.Error())
			return
		}
		title := strings.TrimSpace(req.Title)
		if len([]rune(title)) > 120 {
			title = string([]rune(title)[:120])
		}
		c, err := b.create(title, strings.TrimSpace(req.Prompt), dir)
		if c == nil {
			kanbanErr(w, 400, err.Error())
			return
		}
		b.mu.Lock()
		v := c.view()
		b.mu.Unlock()
		if err != nil {
			v["error"] = err.Error()
		}
		writeJSON(w, v)
	default:
		kanbanErr(w, 405, "GET or POST")
	}
}

// handleKanbanCard: POST {id, action: move|rename|stop|restart|delete,
// column?, title?}.
func (s *Server) handleKanbanCard(w http.ResponseWriter, r *http.Request) {
	b := s.kanbanBoard(w, r)
	if b == nil {
		return
	}
	if r.Method != http.MethodPost {
		kanbanErr(w, 405, "POST")
		return
	}
	var req struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Column string `json:"column"`
		Title  string `json:"title"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req) != nil {
		kanbanErr(w, 400, "invalid JSON")
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.find(req.ID)
	if c == nil {
		kanbanErr(w, 404, "unknown card")
		return
	}
	switch req.Action {
	case "move":
		if !validKanbanColumn(req.Column) {
			kanbanErr(w, 400, "unknown column")
			return
		}
		c.Manual, c.ManualFor = req.Column, c.Activity
		if req.Column == kanbanColumnFor(c.Activity) {
			c.Manual, c.ManualFor = "", ""
		}
		c.Order = time.Now().UTC().UnixNano()
		c.recompute()
	case "rename":
		t := strings.TrimSpace(req.Title)
		if t == "" {
			kanbanErr(w, 400, "empty title")
			return
		}
		if len([]rune(t)) > 120 {
			t = string([]rune(t)[:120])
		}
		c.Title = t
	case "stop":
		b.killLocked(c)
		c.recompute()
	case "restart":
		if c.term.alive() || c.queued {
			kanbanErr(w, 409, "card is still running")
			return
		}
		// Si no va arribar a fer res, la tasca inicial es torna a enviar.
		if c.Activity == "" || c.Activity == tui.KanbanIdle {
			c.sent = false
		}
		if err := b.startLocked(c); err != nil {
			b.saveLocked()
			kanbanErr(w, 500, err.Error())
			return
		}
		c.recompute()
	case "delete":
		b.killLocked(c)
		if c.term != nil {
			_ = os.Remove(c.term.statusPath)
		}
		for i, x := range b.cards {
			if x == c {
				b.cards = append(b.cards[:i], b.cards[i+1:]...)
				break
			}
		}
		b.saveLocked()
		writeJSON(w, map[string]any{"ok": true})
		return
	default:
		kanbanErr(w, 400, "unknown action")
		return
	}
	c.Updated = time.Now().UTC()
	b.saveLocked()
	writeJSON(w, c.view())
}

func (b *kanbanBoard) liveTerm(id string) (*kanbanCard, *kanbanTerm) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.find(id)
	if c == nil {
		return nil, nil
	}
	if !c.term.alive() {
		return c, nil
	}
	return c, c.term
}

// handleKanbanInput: POST {id, data}: tecles cap al TUI, ja codificades
// com les enviaria un terminal.
func (s *Server) handleKanbanInput(w http.ResponseWriter, r *http.Request) {
	b := s.kanbanBoard(w, r)
	if b == nil {
		return
	}
	if r.Method != http.MethodPost {
		kanbanErr(w, 405, "POST")
		return
	}
	var req struct {
		ID   string `json:"id"`
		Data string `json:"data"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req) != nil {
		kanbanErr(w, 400, "invalid JSON")
		return
	}
	c, t := b.liveTerm(req.ID)
	if c == nil {
		kanbanErr(w, 404, "unknown card")
		return
	}
	if t == nil {
		kanbanErr(w, 409, "terminal stopped")
		return
	}
	if _, err := t.p.Write([]byte(req.Data)); err != nil {
		kanbanErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleKanbanResize: POST {id, cols, rows}.
func (s *Server) handleKanbanResize(w http.ResponseWriter, r *http.Request) {
	b := s.kanbanBoard(w, r)
	if b == nil {
		return
	}
	if r.Method != http.MethodPost {
		kanbanErr(w, 405, "POST")
		return
	}
	var req struct {
		ID   string `json:"id"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req) != nil || req.Cols < 20 || req.Rows < 5 || req.Cols > 400 || req.Rows > 200 {
		kanbanErr(w, 400, "cols 20-400 and rows 5-200")
		return
	}
	b.mu.Lock()
	c := b.find(req.ID)
	if c != nil {
		c.Cols, c.Rows = req.Cols, req.Rows
	}
	var t *kanbanTerm
	if c != nil && c.term.alive() {
		t = c.term
	}
	b.mu.Unlock()
	if c == nil {
		kanbanErr(w, 404, "unknown card")
		return
	}
	if t != nil {
		t.vt.Resize(req.Cols, req.Rows)
		_ = t.p.Resize(req.Cols, req.Rows)
	}
	writeJSON(w, map[string]any{"ok": true, "cols": req.Cols, "rows": req.Rows})
}

// handleKanbanStream: SSE amb la pantalla d'una targeta (?id=). El primer
// event «frame» porta totes les files; els següents, només les que han
// canviat. Si el terminal s'atura, ho diu i continua esperant un reinici.
func (s *Server) handleKanbanStream(w http.ResponseWriter, r *http.Request) {
	b := s.kanbanBoard(w, r)
	if b == nil {
		return
	}
	id := r.URL.Query().Get("id")
	if c, _ := b.liveTerm(id); c == nil {
		kanbanErr(w, 404, "unknown card")
		return
	}
	emit := sse(w)
	ctx := r.Context()
	var (
		prev     []string
		lastSeq  uint64
		lastTerm *kanbanTerm
		lastPing = time.Now()
		sentDead bool
	)
	frame := time.NewTicker(50 * time.Millisecond)
	defer frame.Stop()
	for {
		c, t := b.liveTerm(id)
		if c == nil {
			emit("gone", map[string]any{"id": id})
			return
		}
		if t != lastTerm {
			prev, lastSeq, lastTerm, sentDead = nil, 0, t, false
		}
		switch {
		case t == nil && !sentDead:
			b.mu.Lock()
			v := c.view()
			b.mu.Unlock()
			emit("stopped", v)
			sentDead = true
		case t != nil:
			if seq := t.vt.Seq(); seq != lastSeq || prev == nil {
				lastSeq = seq
				f, lines := kanbanFrame(t, prev)
				prev = lines
				emit("frame", f)
				lastPing = time.Now()
			}
		}
		if time.Since(lastPing) > 15*time.Second {
			emit("ping", map[string]any{})
			lastPing = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-frame.C:
		}
	}
}

// kanbanFrame pinta la pantalla i torna només les files que han canviat
// respecte de prev (totes si prev és nil o la mida ha canviat).
func kanbanFrame(t *kanbanTerm, prev []string) (map[string]any, []string) {
	cols, rows := t.vt.Size()
	cx, cy, cv := t.vt.Cursor()
	lines := t.vt.Lines()
	modes := t.vt.Modes()
	out := make([]string, len(lines))
	changed := map[string]string{}
	full := prev == nil || len(prev) != len(lines)
	for i, row := range lines {
		curX := -1
		if cv && i == cy {
			curX = cx
		}
		out[i] = vterm.LineHTML(row, curX)
		if full || prev[i] != out[i] {
			changed[strconv.Itoa(i)] = out[i]
		}
	}
	return map[string]any{
		"cols": cols, "rows": rows, "full": full, "lines": changed,
		"app_cursor": modes.AppCursor, "paste": modes.BracketedPaste,
		"mouse": int(modes.Mouse), "mouse_sgr": modes.MouseSGR, "title": t.vt.Title(),
	}, out
}
