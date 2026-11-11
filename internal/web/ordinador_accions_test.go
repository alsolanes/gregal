package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/events"
	"gregal/internal/tools"
)

// B13 — on actua l'agent a l'ordinador. Sense navegador ni VM: les accions
// s'injecten pel mateix receptor que fan servir les eines (el Hub el
// connecta en crear-se) i el visor és el de B3 amb els seus dobles.

const fakeGitHubToken = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"

func accioDe(scope, kind, target, text string) tools.ComputerAction {
	return tools.ComputerAction{
		Kind: kind, Owner: tools.BrowserOwnerKey(scope), X: 0.5, Y: 0.25,
		Rect: []float64{0.4, 0.2, 0.2, 0.1}, Target: target, Text: text,
		View: tools.ComputerView{W: 1000, H: 600}, Shot: []byte("JPEG-" + target), At: time.Now(),
	}
}

func TestComputerActionRegistreIPropietari(t *testing.T) {
	ambVisorHost(t)
	s, h := knowledgeTestServer(t)
	hub := s.Hub()
	carla, aleix := &User{Name: "carla"}, &User{Name: "aleix"}
	sc := hub.SessionFor(carla, "default")
	sa := hub.SessionFor(aleix, "vent-1")
	sc.mu.Lock()
	sc.active = &activeAgent{ID: 7}
	sc.todo = []tools.TodoItem{{Title: "Busca el vol", Status: "done"}, {Title: "Omple el formulari", Status: "working"}}
	sc.mu.Unlock()

	hub.recordComputerAction(sc.id, accioDe(sc.id, tools.ComputerType, "Notes", "clau "+fakeGitHubToken))
	masked := accioDe(sc.id, tools.ComputerType, "Contrasenya", tools.MaskedText)
	masked.Masked = true
	hub.recordComputerAction(sc.id, masked)
	hub.recordComputerAction(sa.id, accioDe(sa.id, tools.ComputerClick, "Reserva", ""))

	// Al registre: kind propi, l'usuari de la sessió, el torn i cap secret.
	evs, err := hub.eventStore.AfterMatch(0, 100, func(e events.Event) bool { return e.Kind == ComputerActionKind })
	if err != nil || len(evs) != 3 {
		t.Fatalf("events: %d %v", len(evs), err)
	}
	if evs[0].User != "carla" || evs[0].RunID != 7 || evs[2].User != "aleix" {
		t.Fatalf("usuari/torn: %+v", evs)
	}
	for _, e := range evs {
		raw, _ := json.Marshal(e)
		if strings.Contains(string(raw), fakeGitHubToken) || strings.Contains(string(raw), "JPEG-") {
			t.Fatalf("secret o captura a l'event: %s", raw)
		}
	}

	// API: la Carla veu les seues dues, amb el pas i la captura.
	code, out := apiAs(t, h, carla, "GET", "/api/ordinador/accions", "")
	acts, _ := out["actions"].([]any)
	if code != 200 || len(acts) != 2 || out["run"] != float64(7) {
		t.Fatalf("carla: %d %v", code, out)
	}
	first := acts[0].(map[string]any)
	if first["step"] != "2/2 · Omple el formulari" || !strings.Contains(fmt.Sprint(first["text"]), "[REDACTED]") {
		t.Fatalf("pas o redacció: %v", first)
	}
	second := acts[1].(map[string]any)
	if second["masked"] != true || second["text"] != tools.MaskedText {
		t.Fatalf("emmascarat: %v", second)
	}
	shotURL := fmt.Sprint(first["shotUrl"])
	if !strings.HasPrefix(shotURL, "/api/ordinador/accions/captura?id=") {
		t.Fatalf("shotUrl: %q", shotURL)
	}
	r := httptest.NewRequest("GET", shotURL, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, carla)))
	if w.Code != 200 || w.Body.String() != "JPEG-Notes" || w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("captura: %d %q", w.Code, w.Body.String())
	}

	// Segona identitat: l'Aleix no veu res de la Carla, ni la captura.
	_, out = apiAs(t, h, aleix, "GET", "/api/ordinador/accions", "")
	acts, _ = out["actions"].([]any)
	if len(acts) != 1 || acts[0].(map[string]any)["target"] != "Reserva" {
		t.Fatalf("aleix: %v", out)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, aleix)))
	if w.Code != http.StatusNotFound {
		t.Fatalf("captura d'un altre: %d", w.Code)
	}
}

// El visor rep en directe només les accions del seu propietari. Els temps
// els marca el canal on el lector deixa cada esdeveniment, no cap sleep.
func TestComputerActionVisorEnDirecte(t *testing.T) {
	ambVisorHost(t)
	s, h := knowledgeTestServer(t)
	hub := s.Hub()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	carla, aleix := &User{Name: "carla"}, &User{Name: "aleix"}
	sc := hub.SessionFor(carla, "default")
	sa := hub.SessionFor(aleix, "default")

	_, out := apiAs(t, h, carla, "POST", "/api/ordinador/viewer", `{}`)
	waitBackground()
	resp, err := noRedirect().Get(srv.URL + fmt.Sprint(out["url"]))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+visorPrefix+"accions", nil)
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	stream, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if stream.StatusCode != 200 {
		t.Fatalf("stream: %d", stream.StatusCode)
	}
	got := make(chan map[string]any, 8)
	ready := make(chan struct{})
	go func() {
		rd := bufio.NewReader(stream.Body)
		first := true
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				close(got)
				return
			}
			if first && strings.HasPrefix(line, "retry:") {
				first = false
				close(ready)
			}
			if strings.HasPrefix(line, "data: ") {
				var m map[string]any
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m)
				got <- m
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("el stream no ha començat")
	}
	hub.recordComputerAction(sa.id, accioDe(sa.id, tools.ComputerClick, "D'en Aleix", ""))
	hub.recordComputerAction(sc.id, accioDe(sc.id, tools.ComputerClick, "Reserva", ""))
	select {
	case m := <-got:
		if m["target"] != "Reserva" || m["type"] != "click" || m["owner"] != tools.OwnerKeyFor("carla") {
			t.Fatalf("primer event del visor: %v", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("l'acció no ha arribat al visor")
	}

	// La reproducció del visor: mateixes dades, captura pel camí del visor.
	rec, _ := http.NewRequest("GET", srv.URL+visorPrefix+"accions/recents", nil)
	for _, c := range resp.Cookies() {
		rec.AddCookie(c)
	}
	r2, err := http.DefaultClient.Do(rec)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Actions []map[string]any `json:"actions"`
	}
	_ = json.NewDecoder(r2.Body).Decode(&body)
	r2.Body.Close()
	if len(body.Actions) != 1 || !strings.HasPrefix(fmt.Sprint(body.Actions[0]["shotUrl"]), "captura/") {
		t.Fatalf("recents: %v", body.Actions)
	}
	shot, _ := http.NewRequest("GET", srv.URL+visorPrefix+fmt.Sprint(body.Actions[0]["shotUrl"]), nil)
	for _, c := range resp.Cookies() {
		shot.AddCookie(c)
	}
	r3, err := http.DefaultClient.Do(shot)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r3.Body)
	r3.Body.Close()
	if string(b) != "JPEG-Reserva" {
		t.Fatalf("captura del visor: %d %q", r3.StatusCode, b)
	}
	// Sense galeta, res.
	r4, _ := http.Get(srv.URL + visorPrefix + "accions/recents")
	r4.Body.Close()
	if r4.StatusCode != http.StatusUnauthorized {
		t.Fatalf("recents sense sessió: %d", r4.StatusCode)
	}
}

func TestComputerStoreRetallaICaptures(t *testing.T) {
	dir := t.TempDir()
	c := &computerStore{dir: dir}
	owner := tools.OwnerKeyFor("carla")
	var first string
	for i := 0; i < computerKeep+5; i++ {
		v := computerActionView{Run: 1, Shot: newShotID(), At: time.Now()}
		v.Owner, v.Kind = owner, tools.ComputerClick
		if i == 0 {
			first = v.Shot
		}
		c.save(v, []byte("x"))
	}
	if got := c.recent(owner, 0, 1000); len(got) != computerKeep {
		t.Fatalf("es guarden %d", len(got))
	}
	if _, ok := c.shot(owner, first); ok {
		t.Fatal("la captura més vella s'havia d'esborrar")
	}
	if _, ok := c.shot(owner, "../../etc"); ok {
		t.Fatal("id invàlid")
	}
}
