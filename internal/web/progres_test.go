package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// emissor recull els events del conductor (segur per a concurrència).
type emissor struct {
	mu  sync.Mutex
	evs []emes
}

type emes struct {
	kind string
	data map[string]string
}

func (e *emissor) emit(kind string, value any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, _ := value.(map[string]string)
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	e.evs = append(e.evs, emes{kind, cp})
}

func (e *emissor) tots() []emes {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]emes(nil), e.evs...)
}

func (e *emissor) progres() []map[string]string {
	var out []map[string]string
	for _, ev := range e.tots() {
		if ev.kind == "progress" {
			out = append(out, ev.data)
		}
	}
	return out
}

func compte(t *testing.T, ev map[string]string, camp string) int {
	t.Helper()
	n, err := strconv.Atoi(ev[camp])
	if err != nil {
		t.Fatalf("%s no és un enter decimal: %+v", camp, ev)
	}
	return n
}

// Amb un rellotge fals: canvis d'estat de seguida, comptadors com a molt
// cada intervalProgres, i un últim total en tancar la crida només si ha
// canviat res.
func TestProgresTornThrottleIEstats(t *testing.T) {
	em := &emissor{}
	p := nouProgresTorn(em.emit)
	ara := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	p.ara = func() time.Time { return ara }
	avança := func(d time.Duration) { ara = ara.Add(d) }

	p.iniciModel()
	avança(2 * time.Minute) // a la cua del proveïdor: cap byte
	p.delta(llm.ProgressFirst, 0)
	p.delta(llm.ProgressReasoning, 10)
	avança(time.Second)
	p.delta(llm.ProgressReasoning, 10) // dins l'interval: res
	avança(2 * time.Second)
	p.delta(llm.ProgressReasoning, 5) // 3 s des de l'última: surt
	avança(time.Second)
	p.delta(llm.ProgressText, 4)
	p.delta(llm.ProgressTool, 7)
	p.fiModel() // ha canviat: surt el total
	p.fiModel() // res de nou: no surt

	got := em.progres()
	if len(got) != 4 {
		t.Fatalf("emissions=%d, volia 4: %+v", len(got), got)
	}
	since := "2026-10-13T10:00:00Z"
	for i, want := range []map[string]string{
		{"phase": "model", "state": "waiting", "reasoning": "0", "text": "0", "tool": "0", "since": since},
		{"phase": "model", "state": "receiving", "reasoning": "0", "text": "0", "tool": "0", "since": since},
		{"phase": "model", "state": "receiving", "reasoning": "25", "text": "0", "tool": "0", "since": since},
		{"phase": "model", "state": "receiving", "reasoning": "25", "text": "4", "tool": "7", "since": since},
	} {
		if fmt.Sprint(got[i]) != fmt.Sprint(want) {
			t.Fatalf("emissió %d = %v\nvolia        %v", i, got[i], want)
		}
	}

	// Un reintent torna a "waiting" i el primer tros següent, a "receiving".
	p.esperaModel()
	p.esperaModel() // ja espera: no es repeteix
	p.delta(llm.ProgressFirst, 0)
	got = em.progres()
	if len(got) != 6 || got[4]["state"] != "waiting" || got[5]["state"] != "receiving" || got[5]["reasoning"] != "25" {
		t.Fatalf("reintent mal reflectit: %+v", got[4:])
	}

	// Fase d'eines: noms, sense comptadors; els deltes tardans no hi diuen res.
	avança(time.Minute)
	p.faseNova(faseEines, []string{"web_fetch", "read"})
	p.delta(llm.ProgressReasoning, 99)
	p.fiModel()
	got = em.progres()
	last := got[len(got)-1]
	if len(got) != 7 || last["phase"] != "tools" || last["state"] != "running" || last["tools"] != "web_fetch,read" ||
		last["since"] != "2026-10-13T10:03:04Z" || last["reasoning"] != "" {
		t.Fatalf("fase d'eines: %+v", got[6:])
	}
	if txt := textProgres(last); txt != "tools · web_fetch,read" {
		t.Fatalf("text durable=%q", txt)
	}
	if txt := textProgres(got[1]); txt != "model · receiving" {
		t.Fatalf("text durable=%q", txt)
	}
}

// Integració amb el conductor: un proveïdor que raona a poc a poc en mode
// recuperació (el text no surt fins al final) ha de produir progress
// model/waiting → receiving amb comptadors creixents i limitats, una fase
// tools amb el nom de l'eina, i senyal de vida abans del text final.
func TestConduirTornEmetProgresDuranteRaonamentLent(t *testing.T) {
	vell := intervalProgres
	intervalProgres = 60 * time.Millisecond
	defer func() { intervalProgres = vell }()

	const trossos, mida = 12, 5
	var peticions atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := peticions.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		linia := func(payload string) {
			fmt.Fprint(w, "data: "+payload+"\n\n")
			if fl != nil {
				fl.Flush()
			}
		}
		if n == 1 {
			for i := 0; i < trossos; i++ {
				linia(`{"choices":[{"delta":{"reasoning_content":"abcde"}}]}`)
				time.Sleep(25 * time.Millisecond)
			}
			linia(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"g1","type":"function","function":{"name":"glob","arguments":"{\"pattern\":\"*.go\"}"}}]},"finish_reason":"tool_calls"}]}`)
			linia(`[DONE]`)
			return
		}
		time.Sleep(80 * time.Millisecond) // cua del proveïdor
		for i := 0; i < 6; i++ {
			linia(`{"choices":[{"delta":{"reasoning_content":"xyz"}}]}`)
			time.Sleep(25 * time.Millisecond)
		}
		linia(`{"choices":[{"delta":{"content":"fet"},"finish_reason":"stop"}]}`)
		linia(`[DONE]`)
	}))
	defer provider.Close()

	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL}}
	role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}
	em := &emissor{}
	reply, _, err := s.conduirTorn(context.Background(), "busca fitxers", agent.ModeCode, "code", s.cfg.Providers["p"], role, 3, t.TempDir(), nil, em.emit)
	if err != nil || reply != "fet" {
		t.Fatalf("reply=%q err=%v", reply, err)
	}

	// Trossos per crida: model (1a) → tools → model (2a).
	var crida1, crida2 []map[string]string
	var eines map[string]string
	for _, ev := range em.progres() {
		switch {
		case ev["phase"] == "tools":
			eines = ev
		case ev["phase"] == "model" && eines == nil:
			crida1 = append(crida1, ev)
		case ev["phase"] == "model":
			crida2 = append(crida2, ev)
		default:
			t.Fatalf("fase inesperada: %+v", ev)
		}
	}
	if eines == nil || eines["tools"] != "glob" || eines["state"] != "running" {
		t.Fatalf("falta la fase tools amb glob: %+v", em.progres())
	}
	var inicis []time.Time
	for _, crida := range [][]map[string]string{crida1, crida2} {
		if len(crida) < 3 {
			t.Fatalf("massa poques actualitzacions del model: %+v", crida)
		}
		if crida[0]["state"] != "waiting" || crida[0]["reasoning"] != "0" || crida[1]["state"] != "receiving" {
			t.Fatalf("el model ha de passar de waiting a receiving: %+v", crida[:2])
		}
		inici, err := time.Parse(time.RFC3339Nano, crida[0]["since"])
		if err != nil {
			t.Fatalf("since no és RFC3339Nano: %v", err)
		}
		inicis = append(inicis, inici)
		prev := -1
		for _, ev := range crida {
			if ev["since"] != crida[0]["since"] {
				t.Fatalf("since ha de ser l'inici de la fase: %+v", crida)
			}
			r := compte(t, ev, "reasoning")
			if r < prev {
				t.Fatalf("comptador decreixent: %+v", crida)
			}
			prev = r
		}
	}
	if ti, err := time.Parse(time.RFC3339Nano, eines["since"]); err != nil || ti.Before(inicis[0]) || inicis[1].Before(ti) {
		t.Fatalf("since fora d'ordre: model %v, tools %v, model %v", inicis[0], eines["since"], inicis[1])
	}
	// Totals finals (l'última actualització de cada crida porta el total).
	fi1, fi2 := crida1[len(crida1)-1], crida2[len(crida2)-1]
	if compte(t, fi1, "reasoning") != trossos*mida || compte(t, fi1, "tool") != len(`{"pattern":"*.go"}`) {
		t.Fatalf("total de la 1a crida: %+v", fi1)
	}
	if compte(t, fi2, "reasoning") != 18 || compte(t, fi2, "text") != 3 {
		t.Fatalf("total de la 2a crida: %+v", fi2)
	}
	// Throttle: 300 ms de raonament amb un interval de 60 ms no poden donar
	// més de waiting + receiving + ~6 actualitzacions + el total final.
	if len(crida1) > 10 {
		t.Fatalf("massa emissions (%d): el throttle no funciona: %+v", len(crida1), crida1)
	}
	// Mode recuperació: el text surt al final, però abans ja hi ha hagut
	// senyal de vida amb raonament comptat.
	evs := em.tots()
	viu, token := -1, -1
	for i, ev := range evs {
		if viu < 0 && ev.kind == "progress" && ev.data["since"] == crida2[0]["since"] && ev.data["reasoning"] != "0" {
			viu = i
		}
		if token < 0 && ev.kind == "token" {
			token = i
		}
	}
	if viu < 0 || token < 0 || viu > token {
		t.Fatalf("cap progrés del raonament abans del text retingut (progress %d, token %d): %+v", viu, token, evs)
	}
}

// El registre durable lliura progress amb payload estructurat (només els
// camps del contracte) i un text curt; /api/active no l'afegeix a la seva
// llista d'activitat, només en guarda l'últim a progress.
func TestProgressDurableIForaDeLActivitat(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.Hub()
	s.mu.Lock()
	s.active = &activeAgent{ID: 7}
	s.mu.Unlock()

	s.recordActive("progress", map[string]string{
		"phase": "model", "state": "receiving", "reasoning": "120", "text": "0", "tool": "0",
		"since": "2026-10-13T10:00:00Z", "secret": "no-desar",
	})
	s.recordActive("tool_call", map[string]string{"id": "c1", "name": "read", "args": "{}"})

	w := httptest.NewRecorder()
	s.handleEvents(w, httptest.NewRequest(http.MethodGet, "/api/v2/events?after=0&limit=10", nil))
	var got struct {
		Events []struct {
			Kind    string          `json:"kind"`
			Text    string          `json:"text"`
			RunID   int             `json:"run_id"`
			Payload json.RawMessage `json:"payload"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 || got.Events[0].Kind != "progress" || got.Events[0].Text != "model · receiving" || got.Events[0].RunID != 7 {
		t.Fatalf("progress absent o mal format al registre: %+v", got.Events)
	}
	want := `{"phase":"model","reasoning":"120","since":"2026-10-13T10:00:00Z","state":"receiving","text":"0","tool":"0"}`
	if string(got.Events[0].Payload) != want {
		t.Fatalf("payload:\n got %s\nwant %s", got.Events[0].Payload, want)
	}

	w = httptest.NewRecorder()
	s.handleActive(w, httptest.NewRequest(http.MethodGet, "/api/active", nil))
	var act struct {
		Active struct {
			Events   []liveEvent       `json:"events"`
			Progress map[string]string `json:"progress"`
		} `json:"active"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &act); err != nil {
		t.Fatal(err)
	}
	for _, ev := range act.Active.Events {
		if ev.Kind == "progress" {
			t.Fatalf("progress no ha d'omplir la llista d'activitat: %+v", act.Active.Events)
		}
	}
	if len(act.Active.Events) != 1 || act.Active.Progress["state"] != "receiving" || act.Active.Progress["secret"] != "" {
		t.Fatalf("/api/active: %+v", act.Active)
	}
}

// Un checkpoint sense comprovacions ni revisió: l'event estructurat surt
// igual (checks [] i no null) però cap línia d'estat buida de contingut.
// Amb comprovacions, la línia diu el resultat en l'idioma de la sessió.
func TestCheckpointAutonomChecksBuitsISenseEstatInutil(t *testing.T) {
	s := goalTestServer(t)
	// Sense revisió (aniria a un proveïdor de veritat): només comprovacions.
	s.cfg.Agent.Autonomous.ReviewEvery = 1000
	torn := agent.NouTorn(agent.OpcionsTorn{Cfg: s.cfg, Mode: agent.ModeAutonomous, Tasca: "x"})
	em := &emissor{}
	var crus []any
	emit := func(kind string, v any) {
		if kind == "autonomous_checkpoint" {
			crus = append(crus, v)
		}
		em.emit(kind, v)
	}
	s.checkpointAutonom(context.Background(), torn, emit, t.TempDir())
	if len(crus) != 1 {
		t.Fatalf("checkpoint estructurat absent: %+v", em.tots())
	}
	raw, _ := json.Marshal(crus[0])
	if !strings.Contains(string(raw), `"checks":[]`) {
		t.Fatalf("checks ha de ser [] i no null: %s", raw)
	}
	for _, ev := range em.tots() {
		if ev.kind == "status" {
			t.Fatalf("status sense res a dir: %+v", ev)
		}
	}
	// El motor tampoc no hi ha d'afegir "autònom · checkpoint N".
	if evs := senseAvisosCheckpoint(torn.RepCheckpoint("nota", ""), ""); len(evs) != 0 {
		t.Fatalf("avisos del checkpoint no filtrats: %+v", evs)
	}

	s.cfg.Agent.Autonomous.Verify = []string{"exit 0", "exit 3"}
	em2 := &emissor{}
	s.checkpointAutonom(context.Background(), torn, em2.emit, t.TempDir())
	var estat string
	for _, ev := range em2.tots() {
		if ev.kind == "status" {
			estat = ev.data["message"]
		}
	}
	if estat != "Autonomous checkpoint 2: 1/2 checks passed (failing: exit 3)" {
		t.Fatalf("status=%q", estat)
	}
	s.cfg.Language = "ca"
	if got := s.resumCheckpoint(4, nil, "CAL REVISAR: falten tests"); got != "Checkpoint autònom 4: la revisió demana canvis: falten tests" {
		t.Fatalf("resum ca=%q", got)
	}
}
