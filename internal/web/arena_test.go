package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"gregal/internal/arena"
)

// arenaStub fa de model: decideix, escriu bots i dona informes.
func arenaStub(t *testing.T) {
	t.Helper()
	old := arenaModel
	arenaModel = func(_ *Server, _ context.Context, system, user string, _ int) (string, error) {
		switch {
		case strings.Contains(system, "GREGAL ARENA") && strings.Contains(user, "preview the clash"):
			return `{"text":"Dos estils oposats: compte amb el blitz."}`, nil
		case strings.Contains(system, "GREGAL ARENA") && strings.Contains(user, "CASTER"):
			var lines []string
			for _, m := range regexp.MustCompile(`(?m)^(\d+) \|`).FindAllStringSubmatch(user, -1) {
				lines = append(lines, `{"n":`+m[1]+`,"text":"Quina jugada!"}`)
			}
			return `{"lines":[` + strings.Join(lines, ",") + `]}`, nil
		case strings.Contains(system, "GREGAL ARENA"):
			// Els moments: dos de bons (dels candidats), un d'una decisió que no
			// existeix i un amb una regla que no és al .md (s'han de descartar).
			var mo []string
			for _, c := range regexp.MustCompile(`(?m)^d(\d+) side(\d) .*\| rule: (.*)$`).FindAllStringSubmatch(user, -1) {
				if len(mo) == 2 {
					break
				}
				rule, _ := json.Marshal(strings.TrimSpace(c[3]))
				mo = append(mo, `{"n":`+c[1]+`,"side":`+c[2]+`,"rule":`+string(rule)+`,"what":"A big play.","why":"The rule called it."}`)
			}
			if len(mo) > 0 {
				mo = append(mo, `{"n":99999,"side":0,"rule":"Run on first down.","what":"x","why":"y"}`, `{"n":1,"side":0,"rule":"Always punt on every single down, no matter what.","what":"x","why":"y"}`)
			}
			return `{"headline":"La regla del blitz","text":"Ho va decidir una regla.","side":0,"rule":"Run on first down.","moments":[` + strings.Join(mo, ",") + `]}`, nil
		case strings.Contains(system, "You write strategy .md files"):
			return "# Bot strategy\n\n- Run on first down.\n- Cover2 on 3rd and long.", nil
		case strings.Contains(system, "prompt-engineering coach"):
			return `{"scores":{"clarity":4,"coverage":2,"adaptivity":2,"specificity":3,"consistency":5},"summary":"Bona base.","strengths":["clar"],"mistakes":[{"moment":"d3","what":"x","md_issue":"y","fix":"z"}],"suggestions":["Afegeix si/llavors."]}`, nil
		case strings.Contains(user, "You are the OFFENSE"):
			if strings.Contains(system, "ALWAYS DEEP") {
				return `{"decision":{"play":"pass_deep","target":"wr1"},"why":"sempre llarg","rule":"ALWAYS DEEP"}`, nil
			}
			return `{"decision":{"play":"run_inside"},"why":"llegeixo","rule":"Run on first down."}`, nil
		default:
			return `{"decision":{"call":"cover2"},"why":"profund","rule":"Cover2 on 3rd and long."}`, nil
		}
	}
	oldWait := arenaRetryWait
	arenaRetryWait = 0
	t.Cleanup(func() { arenaModel = old; arenaRetryWait = oldWait })
}

func waitMatch(t *testing.T, s *Server, id string) *arenaMatch {
	t.Helper()
	for i := 0; i < 600; i++ {
		if m := s.arenaLoadMatch(id); m != nil && (m.Status == "final" || m.Status == "failed" || m.Status == "stopped") {
			waitBackground()
			return s.arenaLoadMatch(id)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("el partit %s no acaba", id)
	return nil
}

func TestArenaPracticeAndCompetition(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)

	// Bots escrits pel model.
	postJSON(t, s.handleArena, `{"action":"seed_bots","games":["nfl"]}`)
	waitBackground()
	if n := len(s.arenaEntries("nfl")); n != len(arenaBotStyles) {
		t.Fatalf("bots: %d", n)
	}
	// El meu .md, amb historial de versions.
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","display":"Aleix","md":"# ALWAYS DEEP\nThrow deep."}`)
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","display":"Aleix","md":"# Balanced\nRun on first down."}`)
	if e := s.arenaEntry("nfl", "local"); e == nil || len(e.Versions) != 1 || !strings.Contains(e.MD, "Balanced") {
		t.Fatalf("entrada: %+v", e)
	}
	if rec := postJSON(t, s.handleArena, `{"action":"save","game":"nfl","md":"`+strings.Repeat("x", arenaMaxMD+1)+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("un .md massa llarg s'havia de refusar")
	}

	// Partit de prova contra un bot.
	rec := postJSON(t, s.handleArena, `{"action":"practice","game":"nfl","opponent":"bot:rookie"}`)
	var started struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &started)
	if started.ID == "" {
		t.Fatalf("practice: %d %s", rec.Code, rec.Body.String())
	}
	m := waitMatch(t, s, started.ID)
	if m.Status != "final" || len(m.Events) < 60 || len(m.Decisions) < 100 {
		t.Fatalf("partit: %s, %d esdeveniments, %d decisions", m.Status, len(m.Events), len(m.Decisions))
	}
	// L'informe del coach només per a la persona: el bot de pràctica no en té.
	meSide := 0
	if m.Owners[1] == "local" {
		meSide = 1
	}
	if m.Decisions[0].Why == "" || m.Decisions[0].Rule == "" || m.Feedback[meSide] == nil || m.Feedback[meSide].Scores["clarity"] != 4 {
		t.Fatalf("decisions/informe: %+v %+v", m.Decisions[0], m.Feedback[meSide])
	}
	if m.Feedback[1-meSide] != nil {
		t.Fatalf("el bot d'una pràctica no ha de tenir informe: %+v", m.Feedback[1-meSide])
	}
	// Les pràctiques porten les ajudes del format, i el partit diu què ha costat.
	if !m.Hints || m.Stats == nil || m.Stats.Decisions != len(m.Decisions) || m.Stats.Calls < m.Stats.Decisions-m.Stats.Memo || m.Stats.WallMS < 0 {
		t.Fatalf("hints/stats: %v %+v (%d decisions)", m.Hints, m.Stats, len(m.Decisions))
	}
	// Desat sense sagnar.
	if raw, _ := os.ReadFile(s.matchPath(m.ID)); len(raw) == 0 || strings.Contains(string(raw), "\n ") {
		t.Fatalf("el partit s'ha de desar compacte (%d bytes)", len(raw))
	}
	// La retransmissió: prèvia, comentaris lligats a jugades clau i anàlisi.
	if m.Preview == "" || len(m.Lines) == 0 || m.Analysis == nil || m.Analysis.Rule == "" {
		t.Fatalf("retransmissió: prèvia %q, %d comentaris, anàlisi %+v", m.Preview, len(m.Lines), m.Analysis)
	}
	// Els moments del prompt: els del model que són de debò, i la resta del servidor.
	checkMoments(t, m, 3)
	fromModel := 0
	for _, mo := range m.Analysis.Moments {
		if mo.N == 99999 || strings.Contains(mo.Rule, "punt") {
			t.Fatalf("un moment invàlid del model s'havia de descartar: %+v", mo)
		}
		if mo.What == "A big play." {
			fromModel++
		}
	}
	if fromModel == 0 {
		t.Fatalf("els moments bons del model s'havien de quedar: %+v", m.Analysis.Moments)
	}
	hl := map[int]bool{}
	for _, e := range m.Events {
		if e.Highlight {
			hl[e.N] = true
		}
	}
	for _, l := range m.Lines {
		if !hl[l.N] {
			t.Fatalf("un comentari ha d'anar lligat a una jugada clau: %+v", l)
		}
	}
	// El visor rep les jugades noves i el meta només al principi.
	req := httptest.NewRequest(http.MethodGet, "/api/arena/match?id="+m.ID+"&after=5", nil)
	out := httptest.NewRecorder()
	s.handleArenaMatch(out, req)
	var tail arenaMatch
	json.Unmarshal(out.Body.Bytes(), &tail)
	if len(tail.Events) != len(m.Events)-5 || tail.Meta != nil {
		t.Fatalf("after=5: %d esdeveniments", len(tail.Events))
	}
	// Les pràctiques no mouen l'Elo.
	if len(s.arenaRatings()["nfl"]) != 0 {
		t.Fatal("una pràctica no ha de comptar per al rànquing")
	}

	// Competició: 4 bots + jo = un grup de 5 i final.
	rec = postJSON(t, s.handleArenaComp, `{"action":"create","title":"Prova","games":["nfl"],"bots":true}`)
	var comp struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &comp)
	c := s.arenaComp(comp.ID)
	if c == nil || len(c.Groups["nfl"]) != 1 || len(c.Schedule) != 11 {
		t.Fatalf("competició: %+v", c)
	}
	postJSON(t, s.handleArenaComp, `{"action":"start","id":"`+comp.ID+`"}`)
	for i := 0; i < 3000; i++ {
		arenaMu.Lock()
		c = s.arenaComp(comp.ID)
		arenaMu.Unlock()
		if c != nil && c.Status == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitBackground()
	if c.Status != "done" || c.Champion["nfl"] == "" {
		t.Fatalf("la competició havia d'acabar amb campió: %s %+v", c.Status, c.Champion)
	}
	final := c.Schedule[len(c.Schedule)-1]
	if final.Stage != "Final" || !final.Done || final.A == "" || final.B == "" {
		t.Fatalf("final: %+v", final)
	}
	// Els oficials, per defecte sense les ajudes del format.
	if fm := s.arenaLoadMatch(final.Matches[0]); fm == nil || fm.Hints || fm.Feedback[0] == nil || fm.Feedback[1] == nil {
		t.Fatalf("un partit oficial: sense ajudes i amb informe per a tots dos: %+v", fm)
	}
	board := s.arenaRatings()["nfl"]
	if len(board) != 5 {
		t.Fatalf("rànquing: %d jugadors", len(board))
	}
	played, games := 0, 0
	for _, r := range board {
		played += r.Played
	}
	for _, sl := range c.Schedule {
		games += len(sl.Matches)
		// Les eliminatòries, al millor de 3: el guanyador en té 2.
		if sl.Phase == "playoff" && sl.Series[sl.Winner] < 2 && len(sl.Matches) < 5 {
			t.Fatalf("eliminatòria sense sèrie: %+v", sl)
		}
	}
	if played != games*2 || games < 11 {
		t.Fatalf("partits al rànquing: %d (%d partits × 2)", played, games)
	}
}

func TestArenaGroupsAndBracket(t *testing.T) {
	elo := map[string]float64{}
	owners := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	for i, o := range owners {
		elo[o] = float64(2000 - i*10)
	}
	groups := arenaGroups(append([]string(nil), owners...), elo)
	if len(groups) != 3 {
		t.Fatalf("9 jugadors → 3 grups: %v", groups)
	}
	sched := arenaBuildSchedule("nfl", groups)
	stages := map[string]int{}
	for _, sl := range sched {
		stages[sl.Stage]++
	}
	if stages["Group A"] != 3 || stages["Quarterfinal 1"] != 1 || stages["Final"] != 1 {
		t.Fatalf("calendari: %v", stages)
	}
	if a, b := eloUpdate(1500, 1500, 0); a != 1516 || b != 1484 {
		t.Fatalf("elo: %v %v", a, b)
	}
}

func TestArenaSpectatorLinkOnlyWatches(t *testing.T) {
	s := cornerTestServer(t)
	s.Token = "secret"
	h := s.Hub().Handler()
	key := s.arenaSpectatorKey(false)
	do := func(method, path, spect string) int {
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		if spect != "" {
			req.Header.Set("X-Arena-Spectator", spect)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do("GET", "/api/arena/stream", key); c != 200 {
		t.Fatalf("l'espectador ha de poder mirar la retransmissió: %d", c)
	}
	if c := do("GET", "/api/arena/stream", "dolenta"); c != http.StatusUnauthorized {
		t.Fatalf("clau dolenta: %d", c)
	}
	for _, p := range []string{"/api/arena", "/api/state", "/api/winds"} {
		if c := do("GET", p, key); c != http.StatusUnauthorized {
			t.Fatalf("l'espectador no pot entrar a %s: %d", p, c)
		}
	}
	if c := do("POST", "/api/arena/match?id=x", key); c != http.StatusUnauthorized {
		t.Fatalf("l'espectador no pot aturar partits: %d", c)
	}
	if k2 := s.arenaSpectatorKey(true); k2 == key || do("GET", "/api/arena/stream", key) != http.StatusUnauthorized {
		t.Fatal("renovar la clau ha d'invalidar l'enllaç antic")
	}
}

// Un reinici del servidor no pot deixar una competició penjada.
func TestArenaResumeAfterRestart(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)
	postJSON(t, s.handleArena, `{"action":"seed_bots","games":["nfl"]}`)
	waitBackground()
	rec := postJSON(t, s.handleArenaComp, `{"action":"create","title":"Reinici","games":["nfl"],"bots":true}`)
	var comp struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &comp)
	// Com si el servidor hagués caigut a mig partit: competició en marxa i un
	// partit a mitges sense ningú que el jugui.
	arenaMu.Lock()
	c := s.arenaComp(comp.ID)
	c.Status = "running"
	orphan := &arenaMatch{ID: "orfe", Game: "nfl", Kind: "official", Comp: c.ID, Status: "live", Winner: -1}
	c.Schedule[0].Match = orphan.ID
	arenaWrite(s.compPath(c.ID), c)
	arenaWrite(s.matchPath(orphan.ID), orphan)
	arenaMu.Unlock()

	s.arenaResume(context.Background())
	for i := 0; i < 3000; i++ {
		arenaMu.Lock()
		c = s.arenaComp(comp.ID)
		arenaMu.Unlock()
		if c != nil && c.Status == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitBackground()
	if c.Status != "done" || c.Champion["nfl"] == "" {
		t.Fatalf("la competició havia de continuar fins al final: %s", c.Status)
	}
	if m := s.arenaLoadMatch("orfe"); m == nil || m.Status != "stopped" {
		t.Fatalf("el partit a mitges s'ha de donar per aturat: %+v", m)
	}
	if c.Schedule[0].Match == "orfe" || !c.Schedule[0].Done {
		t.Fatalf("el partit que tocava es torna a jugar: %+v", c.Schedule[0])
	}
}

// Un sol clic, sense res preparat: desa la plantilla i juga contra el bot
// plantilla; amb bots, el rival és el primer que encara no has guanyat.
func TestArenaOneClickPractice(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)
	rec := postJSON(t, s.handleArena, `{"action":"practice","game":"nfl"}`)
	var started struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &started)
	if started.ID == "" {
		t.Fatalf("practice sense res: %d %s", rec.Code, rec.Body.String())
	}
	m := waitMatch(t, s, started.ID)
	if m.Owners[0] != "bot:template" && m.Owners[1] != "bot:template" {
		t.Fatalf("rival: %v", m.Owners)
	}
	g, _ := arena.Get("nfl")
	if e := s.arenaEntry("nfl", "local"); e == nil || e.MD != strings.TrimSpace(g.Starter()) && e.MD != g.Starter() {
		t.Fatalf("s'havia de desar la plantilla: %+v", e)
	}
	postJSON(t, s.handleArena, `{"action":"seed_bots","games":["nfl"]}`)
	waitBackground()
	if b := s.arenaNextBot(g, "local"); b == nil || b.Owner != "bot:rookie" {
		t.Fatalf("el primer rival ha de ser el més fàcil: %+v", b)
	}
	// El .md de l'editor es desa en jugar.
	postJSON(t, s.handleArena, `{"action":"practice","game":"nfl","md":"# Nou\nRun on first down."}`)
	waitBackground()
	if e := s.arenaEntry("nfl", "local"); e == nil || !strings.Contains(e.MD, "# Nou") || e.Display != "Jo" && e.Display != "Me" {
		t.Fatalf("entrada: %+v", e)
	}
}

// Un partit oficial on una banda no ha pogut decidir (més del 10% per
// defecte) no compta per a l'Elo i es torna a jugar, amb un límit.
func TestArenaFallbacksDoNotCount(t *testing.T) {
	s := cornerTestServer(t)
	arenaStub(t)
	old := arenaModel
	arenaModel = func(s *Server, ctx context.Context, system, user string, n int) (string, error) {
		if strings.Contains(system, "BROKEN") && strings.Contains(user, "DECISION FORMAT") {
			return "no json here", nil
		}
		return old(s, ctx, system, user, n)
	}
	t.Cleanup(func() { arenaModel = old })
	postJSON(t, s.handleArena, `{"action":"save","game":"nfl","md":"# BROKEN\nNothing useful."}`)
	s.saveEntry(arenaEntry{Owner: "bob", Display: "Bob", Game: "nfl", MD: "# Balanced\nRun on first down."})
	rec := postJSON(t, s.handleArenaComp, `{"action":"create","title":"Fallbacks","games":["nfl"]}`)
	var comp struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &comp)
	postJSON(t, s.handleArenaComp, `{"action":"start","id":"`+comp.ID+`"}`)
	var c *arenaComp
	for i := 0; i < 3000; i++ {
		arenaMu.Lock()
		c = s.arenaComp(comp.ID)
		arenaMu.Unlock()
		if c != nil && c.Status == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitBackground()
	if c == nil || c.Status != "done" {
		t.Fatalf("la competició ha d'acabar igualment")
	}
	if len(s.arenaRatings()["nfl"]) != 0 {
		t.Fatalf("un partit amb decisions per defecte no pot moure l'Elo: %+v", s.arenaRatings()["nfl"])
	}
	if sl := c.Schedule[0]; !sl.Done || len(sl.Matches) != 1 {
		t.Fatalf("el partit s'ha de repetir i després acceptar: %+v", sl)
	}
	if side, rate := arenaFallbackRate(s.arenaLoadMatch(c.Schedule[0].Matches[0])); rate < .5 {
		t.Fatalf("taxa de decisions per defecte: banda %d %.2f", side, rate)
	}
	if got := mdIdentity("# Title\n\nWe attack.\n\n## Rules\n- secret rule"); strings.Contains(got, "secret") {
		t.Fatalf("la identitat no pot portar les regles: %q", got)
	}
}

// El servidor comprova que la regla citada és de debò al .md.
func TestArenaRuleInMD(t *testing.T) {
	md := "# Plan\n- If they run more than 55% of plays: call **goal_line** on 1st down.\n- 3rd & 7+: pass_medium."
	for rule, want := range map[string]bool{
		"If they run more than 55% of plays: call goal_line on 1st down.": true,
		"if they run more than 55% of plays call goal_line":               true,
		"3rd & 7+: pass_medium":      true,
		"Always blitz on third down": false,
		"":                           false,
		"ok":                         false,
	} {
		if got := ruleInMD(rule, md); got != want {
			t.Errorf("ruleInMD(%q) = %v", rule, got)
		}
	}
}

// Els partits de demostració es serveixen com qualsevol partit, amb les
// regles visibles i el que va veure el model.
func TestArenaDemoMatches(t *testing.T) {
	s := cornerTestServer(t)
	for _, g := range []string{"nfl", "cs2", "lol"} {
		rec := httptest.NewRecorder()
		s.handleArenaMatch(rec, httptest.NewRequest("GET", "/api/arena/match?id=demo-"+g+"&after=0", nil))
		var m arenaMatch
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &m) != nil || m.Game != g || m.Status != "final" || len(m.Events) < 10 {
			t.Fatalf("demo %s: %d %s", g, rec.Code, m.Status)
		}
		rules := 0
		for _, d := range m.Decisions {
			if d.Rule != "" {
				rules++
			}
		}
		if rules == 0 || m.MD[0] == "" || m.MD[1] == "" {
			t.Fatalf("demo %s: les regles i els .md han de ser visibles", g)
		}
		// Les demos no porten moments: el servidor els tria en servir-les.
		checkMoments(t, &m, 3)
		// Cada decisió porta el número de les seves jugades (la demo del LoL es
		// va desar amb els números desquadrats al draft: es refan en servir-la).
		maxEv, maxN := 0, 0
		for _, e := range m.Events {
			maxEv = max(maxEv, e.Decision)
		}
		for _, d := range m.Decisions {
			maxN = max(maxN, d.N)
		}
		if maxEv != maxN {
			t.Fatalf("demo %s: decisions fins a %d, jugades fins a %d", g, maxN, maxEv)
		}
		sit := httptest.NewRecorder()
		s.handleArenaMatch(sit, httptest.NewRequest("GET", "/api/arena/match?id=demo-"+g+"&situation=1&side=0", nil))
		if sit.Code != 200 {
			t.Fatalf("demo %s: situació %d", g, sit.Code)
		}
	}
	if arenaDemo("demo-../../x") != nil {
		t.Fatal("un id estrany no pot sortir del directori")
	}
}

// La demo es narra amb les veus d'aquest ordinador; sense veus, subtítols.
func TestArenaDemoVoice(t *testing.T) {
	s := cornerTestServer(t)
	get := func() arenaMatch {
		rec := httptest.NewRecorder()
		s.handleArenaMatch(rec, httptest.NewRequest("GET", "/api/arena/match?id=demo-cs2&after=0", nil))
		var m arenaMatch
		json.Unmarshal(rec.Body.Bytes(), &m)
		return m
	}
	s.saveVoiceCfg(arenaVoiceCfg{Engine: "off"})
	if m := get(); m.PreviewAudio != "" || (len(m.Lines) > 0 && m.Lines[0].Audio != "") {
		t.Fatalf("sense veus no hi pot haver àudio (la gravació no en porta): %q", m.PreviewAudio)
	}
	tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ID3")) }))
	defer tts.Close()
	s.saveVoiceCfg(arenaVoiceCfg{Engine: "http", URL: tts.URL})
	m := get()
	if m.PreviewAudio == "" || len(m.Lines) == 0 || m.Lines[0].Audio == "" || m.Analysis == nil || m.Analysis.Audio == "" {
		t.Fatalf("amb veus, la demo s'ha de narrar: %+v", m.PreviewAudio)
	}
	waitBackground()
}

// checkMoments: entre min i 6 moments, cadascun amb una decisió que existeix,
// la regla al .md d'aquella banda i la jugada que ho ensenya.
func checkMoments(t *testing.T, m *arenaMatch, min int) {
	t.Helper()
	if m.Analysis == nil || len(m.Analysis.Moments) < min || len(m.Analysis.Moments) > arenaMaxMoments {
		t.Fatalf("%s: moments %+v", m.ID, m.Analysis)
	}
	last := -1
	for _, mo := range m.Analysis.Moments {
		if arenaDecisionOf(m, mo.N, mo.Side) == nil || !ruleInMD(mo.Rule, m.MD[mo.Side]) || mo.What == "" || mo.Why == "" {
			t.Fatalf("%s: moment invàlid %+v", m.ID, mo)
		}
		ok := false
		for _, e := range m.Events {
			if e.N == mo.Event && e.Decision == mo.N {
				ok = true
			}
		}
		if !ok || mo.Event < last {
			t.Fatalf("%s: el moment ha d'anar lligat a una jugada, en ordre: %+v", m.ID, mo)
		}
		last = mo.Event
		if len(strings.Fields(mo.What)) > 21 || len(strings.Fields(mo.Why)) > 26 {
			t.Fatalf("%s: moment massa llarg: %+v", m.ID, mo)
		}
	}
}

// Sense model, els moments surten igualment: jugades que fan marcador amb una
// regla trobada al .md de qui en treu profit, i res de les decisions sense regla.
func TestArenaMomentsFallback(t *testing.T) {
	md := [2]string{"# A\n- Run on first down.\n- Pass deep on 3rd and long.", "# B\n- Blitz on 3rd down."}
	m := &arenaMatch{ID: "x", Status: "final", Names: [2]string{"A", "B"}, MD: md, Winner: 0}
	score := [2]int{}
	for n := 1; n <= 30; n++ {
		m.Decisions = append(m.Decisions,
			arenaDecision{N: n, Side: 0, Why: "they stack the box", Rule: "Run on first down.", RuleFound: n%3 != 0},
			arenaDecision{N: n, Side: 1, Why: "pressure", Rule: "Blitz on 3rd down.", RuleFound: true})
		e := arena.Event{N: n, Text: fmt.Sprintf("Play %d", n), Decision: n}
		if n%5 == 0 {
			score[0] += 7
			e.Highlight = true
		}
		if n == 12 {
			score[1] += 3
		}
		e.Score = score
		m.Events = append(m.Events, e)
	}
	got := arenaFallbackMoments(m, 6)
	if len(got) != 5 {
		t.Fatalf("moments: %+v", got)
	}
	for _, mo := range got {
		d := arenaDecisionOf(m, mo.N, mo.Side)
		if !d.RuleFound || (mo.Side == 0 && mo.N%5 != 0) || (mo.Side == 1 && mo.N != 12) {
			t.Fatalf("moment mal triat: %+v", mo)
		}
		if mo.Why != "they stack the box" && mo.Why != "pressure" {
			t.Fatalf("el perquè ha de sortir de la decisió: %+v", mo)
		}
	}
	// Model que falla: l'anàlisi porta igualment els moments.
	old := arenaModel
	arenaModel = func(*Server, context.Context, string, string, int) (string, error) { return "", fmt.Errorf("down") }
	t.Cleanup(func() { arenaModel = old })
	s := cornerTestServer(t)
	g, _ := arena.Get("nfl")
	a := s.castAnalysis(context.Background(), g, m)
	if a == nil || a.Headline != "" || len(a.Moments) < 3 {
		t.Fatalf("anàlisi sense model: %+v", a)
	}
	// Un moment de model amb una regla inventada o una decisió sense jugada, fora.
	bad := arenaCheckMoments(m, []arenaMoment{{N: 5, Side: 0, Rule: "Always punt on every down.", What: "x", Why: "y"}, {N: 500, Side: 0, Rule: "Run on first down."}, {N: 5, Side: 0, Rule: "Run on first down.", What: "TD", Why: "rule"}})
	if len(bad) != 1 || bad[0].N != 5 || bad[0].Event != 5 {
		t.Fatalf("validació: %+v", bad)
	}
	// Un rival que no es pot veure: els seus moments no surten.
	m.Analysis = a
	arenaHideMoments(m, 1)
	for _, mo := range m.Analysis.Moments {
		if mo.Side == 1 {
			t.Fatal("un moment d'un .md secret no s'ha de mostrar")
		}
	}
}

// Sense cap competició, la retransmissió també porta el guió (si hi ha veu i
// l'explicació de cada joc): si no, la tele d'un partit sol no parlava mai.
func TestArenaStreamScriptWithoutCompetition(t *testing.T) {
	s := cornerTestServer(t)
	rec := httptest.NewRecorder()
	s.handleArenaStream(rec, httptest.NewRequest("GET", "/api/arena/stream", nil))
	var out struct {
		Competition any            `json:"competition"`
		Script      map[string]any `json:"script"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Competition != nil || out.Script == nil || out.Script["explain"] == nil {
		t.Fatalf("cal el guió encara que no hi hagi competició: %s", rec.Body.String())
	}
	if _, ok := out.Script["voice"]; !ok {
		t.Fatal("el guió ha de dir si hi ha veu")
	}
}
