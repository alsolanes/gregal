package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/arena"
)

// arenaFakeMatches desa n partits acabats d'uns 150 KB cadascun (com els de
// debò: jugades amb dades, decisions, comentaris i informes). Els parells
// són de «local», amb informe.
func arenaFakeMatches(tb testing.TB, s *Server, n int) {
	tb.Helper()
	var ev []arena.Event
	data := json.RawMessage(`{"x":` + strings.Repeat("1", 300) + `}`)
	for i := 0; i < 300; i++ {
		ev = append(ev, arena.Event{N: i + 1, Kind: "run", Text: "Run up the middle for 4 yards.", Score: [2]int{7, 3}, Decision: i/2 + 1, Data: data})
	}
	var decs []arenaDecision
	for i := 0; i < 200; i++ {
		decs = append(decs, arenaDecision{N: i/2 + 1, Side: i % 2, Short: "Q1 1&10 @25", Decision: json.RawMessage(`{"play":"run_inside","target":"","tempo":"normal"}`),
			Why: "They keep blitzing on first down, so we run inside.", Rule: "If they blitz on 1st down: run_inside."})
	}
	fb := &arenaFeedback{Scores: map[string]int{"clarity": 4, "coverage": 3, "adaptivity": 3, "specificity": 2, "consistency": 5}, Summary: strings.Repeat("Good. ", 40)}
	base := time.Now().Add(-48 * time.Hour).Unix()
	for i := 0; i < n; i++ {
		owner := "bob"
		if i%2 == 0 {
			owner = "local"
		}
		m := &arenaMatch{ID: fmt.Sprintf("nfl-fake-%04d", i), Game: "nfl", Kind: "practice", Stage: "Practice", Status: "final", Started: base + int64(i)*60,
			Owners: [2]string{owner, "bot:rookie"}, Names: [2]string{owner, "Bot Rookie"}, Score: [2]int{21, 14}, Winner: 0,
			MD: [2]string{strings.Repeat("- a rule\n", 300), strings.Repeat("- b rule\n", 300)}, Events: ev, Decisions: decs,
			Feedback: [2]*arenaFeedback{fb, nil}}
		if i%2 == 1 {
			// Els antics, sagnats: s'han de continuar llegint.
			raw, _ := json.MarshalIndent(m, "", " ")
			_ = os.WriteFile(s.matchPath(m.ID), raw, 0o600)
			continue
		}
		arenaWrite(s.matchPath(m.ID), m)
	}
}

func arenaGET(s *Server) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handleArena(rec, httptest.NewRequest(http.MethodGet, "/api/arena", nil))
	return rec
}

// GET /api/arena amb 200 partits (30 MB): la primera vegada llegeix les
// capçaleres i després surten de la cache (abans: 2,2 s amb 117 partits).
func TestArenaStateFastWithManyMatches(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)
	arenaFakeMatches(t, s, 200)
	t0 := time.Now()
	rec := arenaGET(s)
	cold := time.Since(t0)
	if rec.Code != 200 {
		t.Fatalf("GET: %d", rec.Code)
	}
	var st struct {
		Matches  []map[string]any            `json:"matches"`
		Progress map[string][]map[string]any `json:"progress"`
		Left     int                         `json:"practice_left"`
		Slots    arenaSlotCfg                `json:"slots"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Matches) != 30 || len(st.Progress["nfl"]) != 20 || st.Progress["nfl"][19]["coach"].(float64) != 17 {
		t.Fatalf("estat: %d partits, progrés %v", len(st.Matches), st.Progress["nfl"])
	}
	if st.Matches[0]["events"] != nil || st.Matches[0]["decisions"] != nil {
		t.Fatalf("la llista no porta les jugades: %v", st.Matches[0])
	}
	if st.Slots.Official != 1 || st.Slots.Practice != 2 {
		t.Fatalf("torns per defecte: %+v", st.Slots)
	}
	// Fred de debò però amb els fitxers ja vistos per l'antivirus: es buida la cache.
	arenaSumMu.Lock()
	clear(arenaSums)
	arenaSumMu.Unlock()
	t0 = time.Now()
	arenaGET(s)
	cold2 := time.Since(t0)
	var warm time.Duration
	for i := 0; i < 5; i++ {
		t0 = time.Now()
		arenaGET(s)
		if d := time.Since(t0); warm == 0 || d < warm {
			warm = d
		}
	}
	t.Logf("GET /api/arena amb 200 partits: fred %v (primer cop, %v amb els fitxers ja oberts), calent %v (%d bytes)", cold, cold2, warm, rec.Body.Len())
	if warm > 150*time.Millisecond { // objectiu < 50 ms; marge per a màquines lentes
		t.Fatalf("GET /api/arena massa lent: %v", warm)
	}
	// Un partit que canvia es torna a llegir (la cache mira data i mida).
	m := s.arenaLoadMatch("nfl-fake-0198")
	m.Score = [2]int{99, 0}
	arenaWrite(s.matchPath(m.ID), m)
	for _, x := range s.arenaMatchList() {
		if x.ID == m.ID && x.Score[0] != 99 {
			t.Fatalf("la cache no s'ha invalidat: %+v", x.Score)
		}
	}
	waitBackground()
}

func BenchmarkArenaState200(b *testing.B) {
	b.Setenv("GREGAL_DATA_DIR", b.TempDir())
	s, _ := companionTestServer(&testing.T{})
	arenaFakeMatches(b, s, 200)
	arenaGET(s)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arenaGET(s)
	}
}

// Les consultes lleugeres del visor: dafter, lafter i el .md només al principi.
func TestArenaMatchDeltaPolling(t *testing.T) {
	s := cornerTestServer(t)
	m := &arenaMatch{ID: "nfl-delta", Game: "nfl", Kind: "practice", Status: "final", Owners: [2]string{"local", "bot:rookie"}, Winner: 0,
		MD: [2]string{"# Mine", "# Bot"}, Lines: []arenaLine{{N: 1, Text: "a"}, {N: 2, Text: "b"}, {N: 3, Text: "c"}}}
	for i := 0; i < 10; i++ {
		m.Events = append(m.Events, arena.Event{N: i + 1, Kind: "run"})
		m.Decisions = append(m.Decisions, arenaDecision{N: i + 1, Side: i % 2, Decision: json.RawMessage(`{}`)})
	}
	arenaWrite(s.matchPath(m.ID), m)
	get := func(q string) map[string]json.RawMessage {
		rec := httptest.NewRecorder()
		s.handleArenaMatch(rec, httptest.NewRequest(http.MethodGet, "/api/arena/match?id=nfl-delta"+q, nil))
		var out map[string]json.RawMessage
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	count := func(raw json.RawMessage) int {
		var a []any
		json.Unmarshal(raw, &a)
		return len(a)
	}
	full := get("")
	if count(full["decisions"]) != 10 || count(full["lines"]) != 3 || !strings.Contains(string(full["md"]), "# Mine") || full["decisions_total"] != nil {
		t.Fatalf("sense paràmetres, com sempre: %s", full["md"])
	}
	d := get("&after=4&dafter=7&lafter=2")
	if count(d["events"]) != 6 || count(d["decisions"]) != 3 || count(d["lines"]) != 1 || d["md"] != nil ||
		string(d["decisions_total"]) != "10" || string(d["lines_total"]) != "3" || d["followed"] == nil {
		t.Fatalf("delta: %d jugades, %d decisions, %d línies, md %s, totals %s %s", count(d["events"]), count(d["decisions"]), count(d["lines"]), d["md"], d["decisions_total"], d["lines_total"])
	}
	if d := get("&after=4&dafter=99"); count(d["decisions"]) != 0 {
		t.Fatalf("dafter més enllà del final: %s", d["decisions"])
	}
}

// Una resposta amb un valor que no és a la llista es torna a demanar dient
// què no anava bé; el prompt comença per la part fixa i la situació va al final.
func TestArenaDecideFixupAndOrder(t *testing.T) {
	s := cornerTestServer(t)
	old := arenaModel
	var mu sync.Mutex
	var asks []string
	arenaModel = func(_ *Server, _ context.Context, system, user string, n int) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		asks = append(asks, user)
		if n > arenaRetryTokens {
			t.Errorf("max tokens: %d", n)
		}
		switch len(asks) {
		case 1:
			return `{"decision":{"call":"zone_blitz"},"why":"x","rule":""}`, nil
		case 2:
			return `not json at all`, nil
		}
		return `{"decision":{"call":"cover2"},"why":"deep","rule":""}`, nil
	}
	t.Cleanup(func() { arenaModel = old })
	g, _ := arena.Get("nfl")
	p := arena.Prompt{Situation: "You are the DEFENSE of X.", Schema: `{"call":"base|blitz|cover2|cover3|man|goal_line|prevent"}` + "\n- base: balanced."}
	a, fb, st := s.arenaDecideStats(context.Background(), g, "# md", p, true)
	if fb || string(a.Decision) != `{"call":"cover2"}` || st.Calls != 3 || st.Retries != 2 {
		t.Fatalf("decisió: %s fb=%v %+v", a.Decision, fb, st)
	}
	if !strings.HasPrefix(asks[0], "DECISION FORMAT:\n") || !strings.HasSuffix(asks[0], "SITUATION:\nYou are the DEFENSE of X.") {
		t.Fatalf("ordre del prompt: %q", asks[0])
	}
	if !strings.HasPrefix(asks[1], asks[0]) || !strings.Contains(asks[1], `value "zone_blitz" is not allowed for "call"`) || !strings.Contains(asks[2], "not valid JSON") {
		t.Fatalf("reintents: %q / %q", asks[1], asks[2])
	}
	// La mateixa pregunta en una pràctica: de la memòria, sense cridar el model.
	if a2, _, st2 := s.arenaDecideStats(context.Background(), g, "# md", p, true); !st2.Memo || st2.Calls != 0 || string(a2.Decision) != string(a.Decision) || len(asks) != 3 {
		t.Fatalf("memòria: %+v", st2)
	}
	if arenaCheckDecision(json.RawMessage(`{"play":"PASS_SHORT","target":"","tempo":"hurry","extra":1}`), `{"play":"run_inside|pass_short","target":"wr1|rb|","tempo":"normal|hurry|milk"}`) != "" {
		t.Fatal("valors vàlids (majúscules, buit permès, camps de més) no s'han de refusar")
	}
}

// Els torns: oficials i pràctiques per separat, amb la posició a la cua.
func TestArenaSlotQueue(t *testing.T) {
	q := &arenaSlotQueue{running: map[string]int{}}
	limit := func(c string) int {
		if c == "official" {
			return 1
		}
		return 2
	}
	ctx := context.Background()
	if !q.acquire(ctx, "o1", "official", limit) || !q.acquire(ctx, "p1", "practice", limit) || !q.acquire(ctx, "p2", "practice", limit) {
		t.Fatal("hi ha torns lliures")
	}
	got := make(chan string, 3)
	go func() { q.acquire(ctx, "p3", "practice", limit); got <- "p3" }()
	go func() { q.acquire(ctx, "o2", "official", limit); got <- "o2" }()
	cctx, cancel := context.WithCancel(ctx)
	stopped := make(chan bool)
	time.Sleep(20 * time.Millisecond)
	go func() { stopped <- q.acquire(cctx, "p4", "practice", limit) }()
	time.Sleep(20 * time.Millisecond)
	if q.position("p3") != 1 || q.position("p4") != 2 || q.position("o2") != 1 || q.position("p1") != 0 {
		t.Fatalf("posicions: p3 %d p4 %d o2 %d", q.position("p3"), q.position("p4"), q.position("o2"))
	}
	cancel()
	if <-stopped {
		t.Fatal("un partit aturat a la cua no juga")
	}
	q.release("practice", limit)
	if w := <-got; w != "p3" {
		t.Fatalf("havia d'entrar p3, no %s", w)
	}
	q.release("official", limit)
	if w := <-got; w != "o2" {
		t.Fatalf("havia d'entrar o2, no %s", w)
	}
	if q.running["practice"] != 2 || q.running["official"] != 1 || len(q.waiting) != 0 {
		t.Fatalf("estat: %+v %d", q.running, len(q.waiting))
	}
}

// Els torns es configuren (admin) i la clau d'espectador no els esborra.
func TestArenaSlotsSetting(t *testing.T) {
	s := cornerTestServer(t)
	rec := postJSON(t, s.handleArena, `{"action":"slots","official":2,"practice":30}`)
	var c arenaSlotCfg
	json.Unmarshal(rec.Body.Bytes(), &c)
	if c.Official != 2 || c.Practice != arenaMaxSlots {
		t.Fatalf("torns: %d %s", rec.Code, rec.Body.String())
	}
	s.arenaSpectatorKey(true)
	if got := s.arenaSlots(); got.Official != 2 || got.Practice != arenaMaxSlots || s.arenaSettings().Spectator == "" {
		t.Fatalf("després de la clau d'espectador: %+v", got)
	}
}

// L'historial del .md: data, hash curt i mida, i el text d'una versió a demanda.
func TestArenaMDHistory(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","md":"# One"}`)
	time.Sleep(1100 * time.Millisecond) // les versions van per segons
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","md":"# Two"}`)
	var st struct {
		Mine map[string]struct {
			History []struct {
				At      int64  `json:"at"`
				Hash    string `json:"hash"`
				Size    int    `json:"size"`
				Current bool   `json:"current"`
			} `json:"history"`
		} `json:"mine"`
	}
	json.Unmarshal(arenaGET(s).Body.Bytes(), &st)
	h := st.Mine["nfl"].History
	if len(h) != 2 || h[0].Size != 5 || len(h[0].Hash) != 8 || !h[1].Current || h[0].Current {
		t.Fatalf("historial: %+v", h)
	}
	rec := postJSON(t, s.handleArena, fmt.Sprintf(`{"action":"version","game":"nfl","at":%d}`, h[0].At))
	if !strings.Contains(rec.Body.String(), "# One") {
		t.Fatalf("versió: %d %s", rec.Code, rec.Body.String())
	}
	waitBackground()
}
