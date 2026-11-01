package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/pty"
	"gregal/internal/tui"
	"gregal/internal/vterm"
)

// fakePTY fa de TUI: el que s'hi escriu queda apuntat, la sortida l'escriu
// el test i Kill el fa acabar.
type fakePTY struct {
	mu     sync.Mutex
	input  strings.Builder
	cols   int
	rows   int
	outR   *io.PipeReader
	outW   *io.PipeWriter
	exit   chan int
	killed sync.Once
}

func newFakePTY(cols, rows int) *fakePTY {
	r, w := io.Pipe()
	return &fakePTY{cols: cols, rows: rows, outR: r, outW: w, exit: make(chan int, 1)}
}

func (f *fakePTY) Read(b []byte) (int, error) { return f.outR.Read(b) }
func (f *fakePTY) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.input.Write(b)
}
func (f *fakePTY) Resize(c, r int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cols, f.rows = c, r
	return nil
}
func (f *fakePTY) Kill() error {
	f.killed.Do(func() { f.exit <- 137 })
	return nil
}
func (f *fakePTY) Wait() (int, error) {
	code := <-f.exit
	f.exit <- code
	return code, nil
}
func (f *fakePTY) Close() error { return f.outW.Close() }
func (f *fakePTY) Pid() int     { return 4242 }
func (f *fakePTY) typed() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.input.String()
}

type kanbanFixture struct {
	t     *testing.T
	s     *Server
	mu    sync.Mutex
	specs []pty.Spec
	ptys  []*fakePTY
}

func newKanbanFixture(t *testing.T) *kanbanFixture {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_KANBAN_MAX", "")
	f := &kanbanFixture{t: t, s: hubTestServer(t)}
	f.s.cwd = t.TempDir()
	oldSpawn, oldExe := kanbanSpawn, kanbanExe
	kanbanSpawn = func(spec pty.Spec) (pty.PTY, error) {
		p := newFakePTY(spec.Cols, spec.Rows)
		f.mu.Lock()
		f.specs = append(f.specs, spec)
		f.ptys = append(f.ptys, p)
		f.mu.Unlock()
		return p, nil
	}
	kanbanExe = func() (string, error) { return "/fake/gregal", nil }
	resetKanbanBoards()
	t.Cleanup(func() {
		kanbanStopAll()
		waitKanbanStopped(t)
		kanbanSpawn, kanbanExe = oldSpawn, oldExe
		resetKanbanBoards()
	})
	return f
}

// waitKanbanStopped espera que cada terminal aturat hagi desat el tauler:
// si no, l'escriptura arriba quan el test ja esborra el directori temporal.
func waitKanbanStopped(t *testing.T) {
	kanbanBoardsMu.Lock()
	var terms []*kanbanTerm
	for _, b := range kanbanBoards {
		b.mu.Lock()
		for _, c := range b.cards {
			if c.term != nil {
				terms = append(terms, c.term)
			}
		}
		b.mu.Unlock()
	}
	kanbanBoardsMu.Unlock()
	for _, term := range terms {
		select {
		case <-term.done:
		case <-time.After(5 * time.Second):
			t.Error("un terminal no s'ha aturat")
		}
	}
	// L'últim desat va just després de tancar done, sota el mateix lock.
	kanbanBoardsMu.Lock()
	for _, b := range kanbanBoards {
		b.mu.Lock()
		b.mu.Unlock()
	}
	kanbanBoardsMu.Unlock()
}

func resetKanbanBoards() {
	kanbanBoardsMu.Lock()
	kanbanBoards = map[string]*kanbanBoard{}
	kanbanBoardsMu.Unlock()
}

func (f *kanbanFixture) pty(i int) (*fakePTY, pty.Spec) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ptys[i], f.specs[i]
}

func (f *kanbanFixture) call(h func(*Server, http.ResponseWriter, *http.Request), method, url, body string) map[string]any {
	f.t.Helper()
	w := httptest.NewRecorder()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	h(f.s, w, httptest.NewRequest(method, url, rd))
	if w.Code != 200 {
		f.t.Fatalf("%s %s: %d %s", method, url, w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

func (f *kanbanFixture) card(id string) map[string]any {
	f.t.Helper()
	board := f.call((*Server).handleKanban, "GET", "/api/kanban", "")
	for _, c := range board["cards"].([]any) {
		m := c.(map[string]any)
		if m["id"] == id {
			return m
		}
	}
	f.t.Fatalf("la targeta %s no és al tauler", id)
	return nil
}

var kanbanStatusBump int

func writeKanbanStatus(t *testing.T, spec pty.Spec, state string) {
	t.Helper()
	var path string
	for _, kv := range spec.Env {
		if v, ok := strings.CutPrefix(kv, tui.KanbanStatusEnv+"="); ok {
			path = v
		}
	}
	if path == "" {
		t.Fatal("el TUI no rep el camí del fitxer d'estat")
	}
	raw, _ := json.Marshal(tui.KanbanStatus{State: state, Detail: "detall " + state, Turns: 1, At: time.Now()})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	// La resolució del mtime pot ser grollera: que el següent canvi es vegi.
	kanbanStatusBump++
	future := time.Now().Add(time.Duration(kanbanStatusBump) * time.Second)
	_ = os.Chtimes(path, future, future)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("temps esgotat esperant: %s", what)
}

func TestKanbanColumnFor(t *testing.T) {
	cases := map[string]string{
		"":                   kbReady,
		tui.KanbanIdle:       kbDefinition,
		tui.KanbanPlanning:   kbDefinition,
		tui.KanbanWorking:    kbProgress,
		tui.KanbanApproval:   kbReview,
		tui.KanbanQuestion:   kbReview,
		tui.KanbanPlanReview: kbReview,
		tui.KanbanDone:       kbDone,
		"desconegut":         kbReady,
	}
	for in, want := range cases {
		if got := kanbanColumnFor(in); got != want {
			t.Errorf("%q → %s, volia %s", in, got, want)
		}
	}
}

func TestKanbanManualColumnHoldsUntilStateChanges(t *testing.T) {
	c := &kanbanCard{Activity: tui.KanbanDone}
	c.recompute()
	if c.Column != kbDone {
		t.Fatal(c.Column)
	}
	c.Manual, c.ManualFor = kbReview, c.Activity
	c.recompute()
	if c.Column != kbReview {
		t.Fatalf("la columna triada a mà no s'aplica: %s", c.Column)
	}
	c.Activity = tui.KanbanWorking
	c.recompute()
	if c.Column != kbProgress || c.Manual != "" {
		t.Fatalf("un canvi d'estat hauria de tornar a manar: %s (manual %q)", c.Column, c.Manual)
	}
}

func TestKanbanCardLifecycle(t *testing.T) {
	f := newKanbanFixture(t)
	dir := t.TempDir()
	body, _ := json.Marshal(map[string]string{"prompt": "arregla els tests", "dir": dir})
	c := f.call((*Server).handleKanban, "POST", "/api/kanban", string(body))
	id, _ := c["id"].(string)
	if id == "" || c["alive"] != true || c["column"] != kbReady || c["title"] != "arregla els tests" {
		t.Fatalf("targeta nova: %v", c)
	}
	p, spec := f.pty(0)
	if spec.Path != "/fake/gregal" || spec.Dir != dir || len(spec.Args) != 2 || spec.Args[0] != "--config" {
		t.Fatalf("spec: %+v", spec)
	}
	env := strings.Join(spec.Env, "\n")
	if !strings.Contains(env, tui.KanbanPromptEnv+"=arregla els tests") || !strings.Contains(env, "TERM=xterm-256color") {
		t.Fatal("el TUI no rep la tasca o el terminal")
	}

	// L'estat que publica el TUI mou la targeta.
	writeKanbanStatus(t, spec, tui.KanbanWorking)
	waitFor(t, "progress", func() bool { return f.card(id)["column"] == kbProgress })
	if f.card(id)["detail"] != "detall working" {
		t.Fatalf("detall: %v", f.card(id)["detail"])
	}

	// La sortida arriba a l'emulador i al resum de la targeta.
	go p.outW.Write([]byte("\x1b[32mhola des del TUI\x1b[0m\r\n"))
	waitFor(t, "tail", func() bool {
		tail, _ := f.card(id)["tail"].([]any)
		return len(tail) > 0 && strings.Contains(tail[0].(string), "hola des del TUI")
	})

	// Moure-la a mà: val fins que el TUI canvia d'estat.
	f.call((*Server).handleKanbanCard, "POST", "/api/kanban/card", `{"id":"`+id+`","action":"move","column":"review"}`)
	if got := f.card(id); got["column"] != kbReview || got["manual"] != true {
		t.Fatalf("moguda: %v", got)
	}
	writeKanbanStatus(t, spec, tui.KanbanDone)
	waitFor(t, "done", func() bool { return f.card(id)["column"] == kbDone })

	// Entrada i mida.
	f.call((*Server).handleKanbanInput, "POST", "/api/kanban/input", `{"id":"`+id+`","data":"ls\r"}`)
	if p.typed() != "ls\r" {
		t.Fatalf("entrada: %q", p.typed())
	}
	f.call((*Server).handleKanbanResize, "POST", "/api/kanban/resize", `{"id":"`+id+`","cols":100,"rows":30}`)
	if p.cols != 100 || p.rows != 30 {
		t.Fatalf("mida: %dx%d", p.cols, p.rows)
	}

	f.call((*Server).handleKanbanCard, "POST", "/api/kanban/card", `{"id":"`+id+`","action":"rename","title":"Tests verds"}`)

	// Aturar-la: queda aturada amb el codi i la columna de l'últim estat.
	f.call((*Server).handleKanbanCard, "POST", "/api/kanban/card", `{"id":"`+id+`","action":"stop"}`)
	waitFor(t, "stopped", func() bool { return f.card(id)["alive"] == false })
	if got := f.card(id); got["exit"] != float64(137) || got["column"] != kbDone {
		t.Fatalf("aturada: %v", got)
	}

	// El tauler sobreviu a un reinici del servidor.
	resetKanbanBoards()
	got := f.card(id)
	if got["title"] != "Tests verds" || got["alive"] != false || got["column"] != kbDone {
		t.Fatalf("després de recarregar: %v", got)
	}
	tail, _ := got["tail"].([]any)
	if len(tail) == 0 {
		t.Fatal("la targeta aturada hauria de conservar l'última pantalla")
	}

	// Reengegar-la no torna a enviar la tasca: ja s'havia fet feina.
	f.call((*Server).handleKanbanCard, "POST", "/api/kanban/card", `{"id":"`+id+`","action":"restart"}`)
	_, spec2 := f.pty(1)
	if strings.Contains(strings.Join(spec2.Env, "\n"), tui.KanbanPromptEnv+"=") {
		t.Fatal("el reinici no hauria de repetir la tasca")
	}
	if f.card(id)["alive"] != true {
		t.Fatal("no s'ha reengegat")
	}

	f.call((*Server).handleKanbanCard, "POST", "/api/kanban/card", `{"id":"`+id+`","action":"delete"}`)
	board := f.call((*Server).handleKanban, "GET", "/api/kanban", "")
	if len(board["cards"].([]any)) != 0 {
		t.Fatalf("no s'ha esborrat: %v", board)
	}
}

func TestKanbanQueueRespectsLiveLimit(t *testing.T) {
	f := newKanbanFixture(t)
	t.Setenv("GREGAL_KANBAN_MAX", "1")
	a := f.call((*Server).handleKanban, "POST", "/api/kanban", `{"title":"a"}`)
	b := f.call((*Server).handleKanban, "POST", "/api/kanban", `{"title":"b","prompt":"tasca b"}`)
	if b["queued"] != true || b["alive"] != false || b["column"] != kbReady {
		t.Fatalf("la segona hauria d'esperar a Preparada: %v", b)
	}
	f.call((*Server).handleKanbanCard, "POST", "/api/kanban/card", `{"id":"`+a["id"].(string)+`","action":"stop"}`)
	waitFor(t, "la cua avança", func() bool { return f.card(b["id"].(string))["alive"] == true })
	_, spec := f.pty(1)
	if !strings.Contains(strings.Join(spec.Env, "\n"), tui.KanbanPromptEnv+"=tasca b") {
		t.Fatal("la targeta encuada ha d'arrencar amb la seva tasca")
	}
}

func TestKanbanRejectsBadInput(t *testing.T) {
	f := newKanbanFixture(t)
	for _, tc := range []struct {
		h    func(*Server, http.ResponseWriter, *http.Request)
		url  string
		body string
		code int
	}{
		{(*Server).handleKanban, "/api/kanban", `{"dir":"` + filepath.ToSlash(filepath.Join(t.TempDir(), "no-hi-es")) + `"}`, 400},
		{(*Server).handleKanbanCard, "/api/kanban/card", `{"id":"x","action":"move","column":"review"}`, 404},
		{(*Server).handleKanbanInput, "/api/kanban/input", `{"id":"x","data":"a"}`, 404},
		{(*Server).handleKanbanResize, "/api/kanban/resize", `{"id":"x","cols":2,"rows":2}`, 400},
	} {
		w := httptest.NewRecorder()
		tc.h(f.s, w, httptest.NewRequest("POST", tc.url, strings.NewReader(tc.body)))
		if w.Code != tc.code {
			t.Errorf("%s %s: %d, volia %d (%s)", tc.url, tc.body, w.Code, tc.code, w.Body.String())
		}
	}
	c := f.call((*Server).handleKanban, "POST", "/api/kanban", `{}`)
	w := httptest.NewRecorder()
	f.s.handleKanbanCard(w, httptest.NewRequest("POST", "/api/kanban/card", strings.NewReader(`{"id":"`+c["id"].(string)+`","action":"move","column":"enlloc"}`)))
	if w.Code != 400 {
		t.Fatalf("columna desconeguda: %d", w.Code)
	}
}

func TestKanbanStreamSendsFrames(t *testing.T) {
	f := newKanbanFixture(t)
	c := f.call((*Server).handleKanban, "POST", "/api/kanban", `{"title":"s"}`)
	id := c["id"].(string)
	p, _ := f.pty(0)
	go p.outW.Write([]byte("pantalla viva"))
	waitFor(t, "sortida", func() bool {
		tail, _ := f.card(id)["tail"].([]any)
		return len(tail) > 0
	})

	srv := httptest.NewServer(http.HandlerFunc(f.s.handleKanbanStream))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"?id="+id, nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			var fr struct {
				Full  bool              `json:"full"`
				Cols  int               `json:"cols"`
				Lines map[string]string `json:"lines"`
			}
			if json.Unmarshal([]byte(data), &fr) != nil {
				continue
			}
			if !fr.Full || fr.Cols != kanbanCols || !strings.Contains(fr.Lines["0"], "pantalla viva") {
				t.Fatalf("primer frame: %s", data)
			}
			return
		}
	}
	t.Fatal("el stream no ha enviat cap frame")
}

// Al TUI, el resum de la targeta salta el quadre del composer.
func TestKanbanTailSkipsComposer(t *testing.T) {
	vt := vtermForTest(40, 10)
	_, _ = vt.Write([]byte("YOU fes x\r\n\r\nfet: x\r\n\r\n\r\n\r\n╭──────╮\r\n│ ❯    │\r\n╰──────╯\r\nACT ready"))
	tail := kanbanTail(vt, 4)
	joined := strings.Join(tail, "|")
	if len(tail) != 3 || !strings.Contains(tail[0], "YOU fes x") || !strings.Contains(tail[1], "fet: x") || !strings.Contains(tail[2], "ACT ready") || strings.Contains(joined, "❯") {
		t.Fatalf("tail: %q", tail)
	}
}

func vtermForTest(c, r int) *vterm.Term { return vterm.New(c, r, nil) }
