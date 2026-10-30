package web

// L'Arena: competició de prompts (internal/arena). Cada jugador (un usuari
// d'aquest servidor) escriu un .md d'estratègia per joc; els dos equips d'un
// partit són idèntics i el mateix model decideix per tots dos llegint cada
// .md. Durant la setmana es fan partits de prova contra bots (estratègies que
// ha escrit el model, de nivells diferents); el dia de la competició
// l'organitzador engega el torneig (grups + eliminatòries) i la pàgina
// arena-stream.html el retransmet. Cada partit oficial mou l'Elo, i cada banda
// rep un informe educatiu: les decisions clau amb la línia del .md que les va
// provocar, i com millorar-lo.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/arena"
	_ "gregal/internal/arena/cs2"
	_ "gregal/internal/arena/lol"
	_ "gregal/internal/arena/nfl"
	"gregal/internal/llm"
)

const (
	arenaMaxMD          = 12000
	arenaPracticePerDay = 6
)

// ---- emmagatzematge ----

func (s *Server) arenaDir() string { return filepath.Join(goalDirFor(s.cfgPath), "arena") }

var arenaMu sync.Mutex // fitxers de l'Arena (entrades, rànquing, competicions)

func arenaRead(path string, v any) bool {
	raw, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(raw, v) == nil
}

func arenaWrite(path string, v any) {
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	raw, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		// Millor quedar-se amb el fitxer d'abans que deixar-ne un de buit.
		log.Printf("arena: no es pot desar %s: %v", filepath.Base(path), err)
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("arena: no es pot desar %s: %v", filepath.Base(path), err)
		return
	}
	// A Windows el canvi de nom falla mentre algú llegeix el fitxer (un visor,
	// la retransmissió): es torna a provar, que perdre el final d'un partit
	// deixaria la competició esperant per sempre.
	for i := 0; ; i++ {
		err := os.Rename(tmp, path)
		if err == nil {
			return
		}
		if i == 40 {
			log.Printf("arena: no es pot desar %s: %v", filepath.Base(path), err)
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func safeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ':' || r == ' ' || r == '.':
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

// arenaEntry és l'estratègia d'un jugador per a un joc.
type arenaEntry struct {
	Owner    string         `json:"owner"`
	Display  string         `json:"display"`
	Game     string         `json:"game"`
	MD       string         `json:"md"`
	Updated  int64          `json:"updated"`
	Bot      bool           `json:"bot,omitempty"`
	Level    string         `json:"level,omitempty"`
	Versions []arenaVersion `json:"versions,omitempty"`
}

type arenaVersion struct {
	At int64  `json:"at"`
	MD string `json:"md"`
}

func (s *Server) entryPath(game, owner string) string {
	return filepath.Join(s.arenaDir(), "entries", safeName(game), safeName(owner)+".json")
}

func (s *Server) arenaEntry(game, owner string) *arenaEntry {
	var e arenaEntry
	if !arenaRead(s.entryPath(game, owner), &e) {
		return nil
	}
	return &e
}

func (s *Server) arenaEntries(game string) []arenaEntry {
	files, _ := filepath.Glob(filepath.Join(s.arenaDir(), "entries", safeName(game), "*.json"))
	var out []arenaEntry
	for _, f := range files {
		var e arenaEntry
		if arenaRead(f, &e) && strings.TrimSpace(e.MD) != "" {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Owner < out[j].Owner })
	return out
}

func (s *Server) saveEntry(e arenaEntry) {
	arenaMu.Lock()
	defer arenaMu.Unlock()
	if old := s.arenaEntry(e.Game, e.Owner); old != nil {
		e.Versions = old.Versions
		if strings.TrimSpace(old.MD) != "" && old.MD != e.MD {
			e.Versions = append(e.Versions, arenaVersion{At: old.Updated, MD: old.MD})
			if len(e.Versions) > 20 {
				e.Versions = e.Versions[len(e.Versions)-20:]
			}
		}
	}
	e.Updated = time.Now().Unix()
	arenaWrite(s.entryPath(e.Game, e.Owner), e)
}

// mdTitle és la primera capçalera del .md (el lema que surt a la retransmissió).
func mdTitle(md string) string {
	for _, l := range strings.Split(md, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "#") {
			return oneLine(strings.TrimSpace(strings.TrimLeft(l, "#")), 80)
		}
	}
	return ""
}

// ---- rànquing (Elo) ----

type arenaRating struct {
	Elo    float64 `json:"elo"`
	Played int     `json:"played"`
	Won    int     `json:"won"`
	Lost   int     `json:"lost"`
	Drawn  int     `json:"drawn"`
}

func (s *Server) ratingsPath() string { return filepath.Join(s.arenaDir(), "ratings.json") }

func (s *Server) arenaRatings() map[string]map[string]*arenaRating {
	r := map[string]map[string]*arenaRating{}
	arenaRead(s.ratingsPath(), &r)
	return r
}

// eloUpdate aplica un resultat (winner -1 = empat) i torna els canvis.
func eloUpdate(a, b float64, winner int) (float64, float64) {
	const k = 32.0
	ea := 1 / (1 + math.Pow(10, (b-a)/400))
	sa := .5
	switch winner {
	case 0:
		sa = 1
	case 1:
		sa = 0
	}
	da := k * (sa - ea)
	return a + da, b - da
}

func (s *Server) arenaRecord(game string, owners [2]string, winner int) {
	arenaMu.Lock()
	defer arenaMu.Unlock()
	all := s.arenaRatings()
	if all[game] == nil {
		all[game] = map[string]*arenaRating{}
	}
	get := func(o string) *arenaRating {
		if all[game][o] == nil {
			all[game][o] = &arenaRating{Elo: 1500}
		}
		return all[game][o]
	}
	a, b := get(owners[0]), get(owners[1])
	a.Elo, b.Elo = eloUpdate(a.Elo, b.Elo, winner)
	a.Played++
	b.Played++
	switch winner {
	case 0:
		a.Won++
		b.Lost++
	case 1:
		b.Won++
		a.Lost++
	default:
		a.Drawn++
		b.Drawn++
	}
	arenaWrite(s.ratingsPath(), all)
}

// ---- partits ----

type arenaDecision struct {
	N        int             `json:"n"`
	Side     int             `json:"side"`
	Short    string          `json:"short"`
	Decision json.RawMessage `json:"decision"`
	Why      string          `json:"why,omitempty"`
	Rule     string          `json:"rule,omitempty"`
	Fallback bool            `json:"fallback,omitempty"`
}

type arenaFeedback struct {
	Scores      map[string]int `json:"scores"` // clarity, coverage, adaptivity, specificity, consistency (1-5)
	Summary     string         `json:"summary"`
	Strengths   []string       `json:"strengths"`
	Mistakes    []arenaMistake `json:"mistakes"`
	Suggestions []string       `json:"suggestions"`
}

type arenaMistake struct {
	Moment string `json:"moment"`
	What   string `json:"what"`
	Issue  string `json:"md_issue"`
	Fix    string `json:"fix"`
}

type arenaMatch struct {
	ID        string            `json:"id"`
	Game      string            `json:"game"`
	Kind      string            `json:"kind"` // practice, official
	Comp      string            `json:"comp,omitempty"`
	Stage     string            `json:"stage,omitempty"`
	Owners    [2]string         `json:"owners"`
	Names     [2]string         `json:"names"`
	Titles    [2]string         `json:"titles"`
	Status    string            `json:"status"` // pregame, live, final, stopped, failed
	Started   int64             `json:"started"`
	Finished  int64             `json:"finished,omitempty"`
	Seed      int64             `json:"seed"`
	Meta      json.RawMessage   `json:"meta,omitempty"`
	Events    []arena.Event     `json:"events"`
	Decisions []arenaDecision   `json:"decisions"`
	Score     [2]int            `json:"score"`
	Winner    int               `json:"winner"`
	Feedback  [2]*arenaFeedback `json:"feedback"`
	// La retransmissió (arena_broadcast.go): prèvia, comentaris i anàlisi.
	Preview      string         `json:"preview,omitempty"`
	PreviewAudio string         `json:"preview_audio,omitempty"`
	Lines        []arenaLine    `json:"lines,omitempty"`
	Analysis     *arenaAnalysis `json:"analysis,omitempty"`
	MD           [2]string      `json:"md,omitempty"`
	Error        string         `json:"error,omitempty"`
}

type arenaRun struct {
	mu     sync.Mutex
	m      *arenaMatch
	cancel context.CancelFunc
}

var (
	arenaRunsMu sync.Mutex
	arenaRuns   = map[string]*arenaRun{}
	arenaQueue  = make(chan struct{}, 1) // un partit alhora (el model no dona per a més)
)

func (s *Server) matchPath(id string) string {
	return filepath.Join(s.arenaDir(), "matches", safeName(id)+".json")
}

func (s *Server) arenaLoadMatch(id string) *arenaMatch {
	arenaRunsMu.Lock()
	r := arenaRuns[id]
	arenaRunsMu.Unlock()
	if r != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		cp := *r.m
		cp.Events = append([]arena.Event(nil), r.m.Events...)
		cp.Decisions = append([]arenaDecision(nil), r.m.Decisions...)
		return &cp
	}
	var m arenaMatch
	if !arenaRead(s.matchPath(id), &m) {
		return nil
	}
	return &m
}

// arenaModel és la crida al model per decidir (els tests la substitueixen).
// Temperatura baixa: el mateix .md ha de donar la mateixa decisió.
var arenaModel = func(s *Server, ctx context.Context, system, user string, maxTokens int) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	// El model de la conversa, no el dels vents: l'Arena en fa moltes crides
	// seguides i ha de ser el ràpid i fiable que la persona ja fa servir.
	p, role := s.roleRef()
	s.mu.Unlock()
	if client == nil || cfg == nil {
		return "", errConst("no model configured")
	}
	msgs := []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	// Sense raonament: cada decisió ha de ser ràpida (un partit de l'NFL en té
	// més de cent per banda) i el .md ja és el raonament.
	text, _, _, err := client.ChatStreamWithToolsFO(llm.WithThink(ctx, "no"), cfg.PrimTarget(p, role), cfg.FallbackTarget(role), msgs, 0.2, maxTokens, nil, func(string) {}, nil, nil)
	return text, err
}

func arenaSystem(g arena.Game, md string) string {
	return "You are the head coach of a team in an AI strategy competition. You must play by the RULES and follow your team's STRATEGY, written by your human player. Follow it faithfully; when it does not cover a case, decide in its spirit with sound judgement.\n\nRULES:\n" + g.Rules() +
		"\n\nYOUR TEAM'S STRATEGY:\n<<<\n" + md + "\n>>>\n\n" +
		`Answer with JSON only: {"decision": <the decision JSON exactly as described>, "why": "one short sentence", "rule": "the exact line of the strategy you applied, copied word for word, or \"\" if none"}`
}

// arenaDecide demana la decisió d'una banda.
func (s *Server) arenaDecide(ctx context.Context, g arena.Game, md string, p arena.Prompt) (arena.Answer, bool) {
	user := p.Situation + "\nDECISION FORMAT:\n" + p.Schema
	for try := 0; try < 2; try++ {
		raw, err := arenaModel(s, ctx, arenaSystem(g, md), user, 500)
		if err != nil {
			if ctx.Err() != nil {
				return arena.Answer{}, true
			}
			continue
		}
		a := arena.ParseAnswer(raw)
		if len(a.Decision) > 0 {
			return a, false
		}
	}
	return arena.Answer{}, true
}

// arenaStart prepara un partit i el posa a la cua. Torna l'id.
func (s *Server) arenaStart(game arena.Game, kind, comp, stage string, e [2]arenaEntry) string {
	now := time.Now()
	id := fmt.Sprintf("%s-%s-%04x", game.ID(), now.Format("20060102-150405"), now.UnixNano()&0xffff)
	seed := int64(now.UnixNano() % 1000000)
	if comp != "" {
		h := sha256.Sum256([]byte(comp + e[0].Owner + e[1].Owner + stage))
		seed = int64(h[0])<<16 | int64(h[1])<<8 | int64(h[2])
	}
	m := &arenaMatch{ID: id, Game: game.ID(), Kind: kind, Comp: comp, Stage: stage, Status: "pregame", Started: now.Unix(), Seed: seed, Winner: -1,
		Owners: [2]string{e[0].Owner, e[1].Owner}, Names: [2]string{e[0].Display, e[1].Display},
		Titles: [2]string{mdTitle(e[0].MD), mdTitle(e[1].MD)}, MD: [2]string{e[0].MD, e[1].MD}}
	ctx, cancel := context.WithCancel(context.Background())
	run := &arenaRun{m: m, cancel: cancel}
	arenaRunsMu.Lock()
	arenaRuns[id] = run
	arenaRunsMu.Unlock()
	arenaWrite(s.matchPath(id), m)
	goBackground(func() { s.arenaPlay(ctx, run, game) })
	return id
}

func (s *Server) arenaPlay(ctx context.Context, run *arenaRun, game arena.Game) {
	defer func() {
		run.cancel()
		run.mu.Lock()
		arenaWrite(s.matchPath(run.m.ID), run.m)
		run.mu.Unlock()
		arenaRunsMu.Lock()
		delete(arenaRuns, run.m.ID)
		arenaRunsMu.Unlock()
	}()
	// Un partit alhora: els altres esperen el seu torn.
	select {
	case arenaQueue <- struct{}{}:
	case <-ctx.Done():
		s.arenaFinish(run, "stopped", "")
		return
	}
	defer func() { <-arenaQueue }()
	m := run.m
	lang := s.fbLangCode()
	match := game.NewMatch(m.Seed, lang, m.Names)
	meta, _ := json.Marshal(match.Meta())
	run.mu.Lock()
	m.Meta, m.Status = meta, "live"
	run.mu.Unlock()
	md := m.MD
	// La prèvia de l'analista, abans del xut inicial.
	if pv := s.castPreview(ctx, game, m); pv != "" {
		audio := s.voiceLater(pv, "analyst", lang)
		run.mu.Lock()
		m.Preview, m.PreviewAudio = pv, audio
		run.mu.Unlock()
	}
	// El narrador comenta en segon pla, per trams de jugades clau, sense
	// frenar el partit.
	castCh := make(chan []arena.Event, 64)
	castDone := make(chan struct{})
	go func() {
		defer close(castDone)
		for batch := range castCh {
			run.mu.Lock()
			names, decs := m.Names, append([]arenaDecision(nil), m.Decisions...)
			run.mu.Unlock()
			lines := s.castLines(ctx, game, names, batch, decs)
			run.mu.Lock()
			m.Lines = append(m.Lines, lines...)
			run.mu.Unlock()
		}
	}()
	var pendingHL []arena.Event
	castClosed := false
	closeCast := func() {
		if !castClosed {
			castClosed = true
			close(castCh)
		}
	}
	defer closeCast() // un partit aturat a mig fer també allibera el narrador
	steps := 0
	for {
		if ctx.Err() != nil {
			s.arenaFinish(run, "stopped", "")
			return
		}
		p, done := match.Next()
		if done {
			break
		}
		steps++
		if steps > 2000 {
			s.arenaFinish(run, "failed", "too many decision points")
			return
		}
		var ans [2]arena.Answer
		var fb [2]bool
		var wg sync.WaitGroup
		for side := 0; side < 2; side++ {
			wg.Add(1)
			go func(side int) {
				defer wg.Done()
				if strings.TrimSpace(p[side].Schema) == "" {
					return // aquesta banda no decideix ara (p. ex. torn de l'altre al draft)
				}
				ans[side], fb[side] = s.arenaDecide(ctx, game, md[side], p[side])
			}(side)
		}
		wg.Wait()
		events := match.Apply([2]json.RawMessage{ans[0].Decision, ans[1].Decision})
		run.mu.Lock()
		dn := len(m.Decisions)/2 + 1
		for side := 0; side < 2; side++ {
			if strings.TrimSpace(p[side].Schema) == "" {
				continue
			}
			m.Decisions = append(m.Decisions, arenaDecision{N: dn, Side: side, Short: p[side].Short, Decision: ans[side].Decision,
				Why: oneLine(ans[side].Why, 240), Rule: oneLine(ans[side].Rule, 240), Fallback: fb[side]})
		}
		m.Events = append(m.Events, events...)
		for _, e := range events {
			if e.Highlight {
				pendingHL = append(pendingHL, e)
			}
		}
		if len(pendingHL) >= 3 {
			castCh <- pendingHL
			pendingHL = nil
		}
		m.Score = match.Score()
		if steps%4 == 0 {
			arenaWrite(s.matchPath(m.ID), m)
		}
		run.mu.Unlock()
	}
	if len(pendingHL) > 0 {
		castCh <- pendingHL
	}
	closeCast()
	<-castDone
	run.mu.Lock()
	m.Score, m.Winner = match.Score(), match.Winner()
	run.mu.Unlock()
	if a := s.castAnalysis(ctx, game, m); a != nil {
		run.mu.Lock()
		m.Analysis = a
		run.mu.Unlock()
	}
	if m.Kind == "official" {
		s.arenaRecord(m.Game, m.Owners, m.Winner)
	}
	s.arenaFinish(run, "final", "")
	// L'informe educatiu de cada banda (després: el partit ja es pot mirar).
	for side := 0; side < 2; side++ {
		if f := s.arenaFeedback(ctx, game, run, side); f != nil {
			run.mu.Lock()
			m.Feedback[side] = f
			run.mu.Unlock()
		}
	}
	log.Printf("arena: %s %s %d-%d", m.Game, m.ID, m.Score[0], m.Score[1])
}

func (s *Server) arenaFinish(run *arenaRun, status, why string) {
	run.mu.Lock()
	run.m.Status, run.m.Error, run.m.Finished = status, why, time.Now().Unix()
	arenaWrite(s.matchPath(run.m.ID), run.m)
	run.mu.Unlock()
}

// arenaFeedback: el que ha anat bé i malament, lligat al .md.
func (s *Server) arenaFeedback(ctx context.Context, game arena.Game, run *arenaRun, side int) *arenaFeedback {
	run.mu.Lock()
	m := run.m
	md := m.MD[side]
	result := "a draw"
	switch m.Winner {
	case side:
		result = "a WIN"
	case 1 - side:
		result = "a LOSS"
	}
	var hl []string
	for _, e := range m.Events {
		if e.Highlight {
			hl = append(hl, fmt.Sprintf("[d%d] %s (score %d-%d)", e.Decision, e.Text, e.Score[0], e.Score[1]))
		}
	}
	if len(hl) > 30 {
		hl = hl[len(hl)-30:]
	}
	var decs []string
	for _, d := range m.Decisions {
		if d.Side != side {
			continue
		}
		decs = append(decs, fmt.Sprintf("d%d %s → %s | why: %s | rule: %s", d.N, d.Short, oneLine(string(d.Decision), 120), d.Why, d.Rule))
	}
	if len(decs) > 60 {
		// Les primeres, les del final i una mostra del mig.
		keep := append(append([]string{}, decs[:20]...), decs[len(decs)/2-10:len(decs)/2+10]...)
		decs = append(keep, decs[len(decs)-20:]...)
	}
	names, score := m.Names, m.Score
	run.mu.Unlock()
	lang := s.cornerLang()
	system := "You are a prompt-engineering coach. You review a player's strategy .md after an AI competition match and teach them how to write better instructions. Be concrete, kind and honest. Reply with JSON only."
	user := fmt.Sprintf("GAME RULES:\n%s\n\nTHE PLAYER'S STRATEGY (.md) for team %s:\n<<<\n%s\n>>>\n\nRESULT: %s, final score %s %d - %s %d.\n\nKEY EVENTS (dN = decision number):\n%s\n\nTHIS TEAM'S DECISIONS (situation → decision | why | quoted rule):\n%s\n\n",
		game.Rules(), names[side], md, result, names[0], score[0], names[1], score[1], strings.Join(hl, "\n"), strings.Join(decs, "\n")) +
		"Score the strategy 1-5 on: clarity (unambiguous, well structured), coverage (handles the situations that came up), adaptivity (reacts to the opponent's tendencies and the game state), specificity (concrete if/then rules and examples), consistency (no contradictions). " +
		"summary = 2-3 sentences. strengths = up to 3. mistakes = up to 4 key moments where the .md caused or failed to prevent a bad decision: moment (decision number and situation), what happened, md_issue (what in the .md was missing, vague or wrong; quote it), fix (a concrete rewritten rule). suggestions = up to 4 general prompt-writing lessons. Write all text in " + lang + ".\n" +
		`JSON: {"scores":{"clarity":3,"coverage":3,"adaptivity":3,"specificity":3,"consistency":3},"summary":"…","strengths":["…"],"mistakes":[{"moment":"…","what":"…","md_issue":"…","fix":"…"}],"suggestions":["…"]}`
	raw, err := arenaModel(s, ctx, system, user, 2000)
	if err != nil {
		return nil
	}
	var f arenaFeedback
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &f) != nil {
		return nil
	}
	return &f
}

// arenaReview: revisió del .md sense jugar (durant la setmana).
func (s *Server) arenaReview(ctx context.Context, game arena.Game, md string) (*arenaFeedback, error) {
	lang := s.cornerLang()
	system := "You are a prompt-engineering coach reviewing a strategy .md for an AI competition. The same LLM will read this .md at every decision point together with the game situation. Teach the player how to write better instructions. Reply with JSON only."
	user := "GAME RULES:\n" + game.Rules() + "\n\nDECISION FORMAT EXAMPLES come from the game; the .md must help choose among them.\n\nTHE STRATEGY (.md):\n<<<\n" + md + "\n>>>\n\n" +
		"Score it 1-5 on clarity, coverage, adaptivity, specificity, consistency. summary = 2-3 sentences. strengths = up to 3. mistakes = up to 4 weak spots (moment = the kind of situation, what = what the model would probably do, md_issue = quote or describe the problem, fix = a concrete rewritten rule). suggestions = up to 4 prompt-writing lessons. Write in " + lang + ".\n" +
		`JSON: {"scores":{"clarity":3,"coverage":3,"adaptivity":3,"specificity":3,"consistency":3},"summary":"…","strengths":["…"],"mistakes":[{"moment":"…","what":"…","md_issue":"…","fix":"…"}],"suggestions":["…"]}`
	raw, err := arenaModel(s, ctx, system, user, 2000)
	if err != nil {
		return nil, err
	}
	var f arenaFeedback
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &f) != nil {
		return nil, errConst("the review did not answer in JSON")
	}
	return &f, nil
}

// ---- bots ----

var arenaBotStyles = []struct{ ID, Name, Level, Brief string }{
	{"bot:rookie", "Bot Rookie", "rookie", "a beginner: a short, vague strategy of 4-6 generic lines (e.g. 'play aggressive', 'be smart'), with no if/then rules and no reading of the opponent"},
	{"bot:aggro", "Bot Aggro", "solid", "an all-out aggressive coach: clear structure and a strong identity (maximum pressure and risk), some situational rules, little adaptation to the opponent"},
	{"bot:turtle", "Bot Turtle", "solid", "a conservative, low-risk coach: clear rules to protect leads and avoid mistakes, plays the percentages, adapts only a little"},
	{"bot:analyst", "Bot Analyst", "expert", "an expert: a well-structured strategy with priorities, explicit if/then rules for every common situation, reads the opponent's tendencies shown in the situation and counters them, avoids being predictable, includes 2 short examples"},
}

// arenaSeedBots fa que el model escrigui els .md dels bots d'un joc.
func (s *Server) arenaSeedBots(ctx context.Context, game arena.Game) error {
	for _, b := range arenaBotStyles {
		system := "You write strategy .md files for an AI competition. The same LLM reads the .md at every decision point together with the game situation and must decide. Write ONLY the .md (Markdown), in English, no preamble."
		user := "GAME RULES:\n" + game.Rules() + "\n\nWrite the strategy of " + b.Brief + ". Start with a level-1 heading that is the strategy's name and motto. Keep it under 6000 characters."
		raw, err := arenaModel(s, ctx, system, user, 3000)
		if err != nil {
			return err
		}
		md := strings.TrimSpace(raw)
		md = strings.TrimPrefix(strings.TrimPrefix(md, "```markdown"), "```md")
		md = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(md, "```"), "```"))
		if len(md) > arenaMaxMD {
			md = md[:arenaMaxMD]
		}
		s.saveEntry(arenaEntry{Owner: b.ID, Display: b.Name, Game: game.ID(), MD: md, Bot: true, Level: b.Level})
	}
	return nil
}

// ---- competicions (grups + eliminatòries) ----

type arenaSlot struct {
	Game    string    `json:"game"`
	Stage   string    `json:"stage"` // "Group A", "Semifinal 1", "Final"…
	Phase   string    `json:"phase"` // group, playoff
	A       string    `json:"a"`
	B       string    `json:"b"`
	Match   string    `json:"match,omitempty"`
	Winner  int       `json:"winner"`
	Score   [2]int    `json:"score"`
	Done    bool      `json:"done"`
	Pending bool      `json:"pending,omitempty"` // eliminatòria que espera rivals
	From    [2]string `json:"from,omitempty"`    // «A1», «B2», «W:Semifinal 1»
}

type arenaComp struct {
	ID       string                  `json:"id"`
	Title    string                  `json:"title"`
	Games    []string                `json:"games"`
	Status   string                  `json:"status"` // ready, running, done, stopped
	Created  int64                   `json:"created"`
	Started  int64                   `json:"started,omitempty"`
	Entries  map[string][]arenaEntry `json:"entries"` // per joc, la foto de les estratègies
	Groups   map[string][][]string   `json:"groups"`  // per joc, els grups (owners)
	Schedule []arenaSlot             `json:"schedule"`
	Current  int                     `json:"current"`
	Champion map[string]string       `json:"champion,omitempty"`
}

func (s *Server) compPath(id string) string {
	return filepath.Join(s.arenaDir(), "competitions", safeName(id)+".json")
}

func (s *Server) arenaComp(id string) *arenaComp {
	var c arenaComp
	if !arenaRead(s.compPath(id), &c) {
		return nil
	}
	return &c
}

func (s *Server) arenaComps() []arenaComp {
	files, _ := filepath.Glob(filepath.Join(s.arenaDir(), "competitions", "*.json"))
	var out []arenaComp
	for _, f := range files {
		var c arenaComp
		if arenaRead(f, &c) {
			c.Entries = nil
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created > out[j].Created })
	return out
}

// arenaResume torna a posar en marxa el que una aturada del servidor (una
// actualització, un reinici) va deixar a mitges: els partits que anaven es
// donen per aturats i les competicions en marxa continuen pel partit que
// tocava, que es torna a jugar des del principi.
func (s *Server) arenaResume(ctx context.Context) {
	files, _ := filepath.Glob(filepath.Join(s.arenaDir(), "matches", "*.json"))
	for _, f := range files {
		var m arenaMatch
		if !arenaRead(f, &m) || (m.Status != "live" && m.Status != "pregame") {
			continue
		}
		arenaRunsMu.Lock()
		running := arenaRuns[m.ID] != nil
		arenaRunsMu.Unlock()
		if running {
			continue
		}
		m.Status = "stopped"
		arenaMu.Lock()
		arenaWrite(f, &m)
		arenaMu.Unlock()
	}
	for _, c := range s.arenaComps() {
		if c.Status != "running" {
			continue
		}
		arenaCompMu.Lock()
		if _, have := arenaCompCancel[c.ID]; have {
			arenaCompMu.Unlock()
			continue
		}
		cctx, cancel := context.WithCancel(ctx)
		arenaCompCancel[c.ID] = cancel
		arenaCompMu.Unlock()
		id := c.ID
		log.Printf("arena: la competició %s continua", id)
		goBackground(func() { s.arenaRunComp(cctx, id) })
	}
}

// arenaGroups reparteix els jugadors en grups de 3-5 (serpentina per Elo).
func arenaGroups(owners []string, elo map[string]float64) [][]string {
	sort.SliceStable(owners, func(i, j int) bool { return elo[owners[i]] > elo[owners[j]] })
	n := len(owners)
	g := 1
	if n >= 6 {
		g = (n + 3) / 4
	}
	groups := make([][]string, g)
	for i, o := range owners {
		row := i / g
		k := i % g
		if row%2 == 1 {
			k = g - 1 - k
		}
		groups[k] = append(groups[k], o)
	}
	return groups
}

// arenaBuildSchedule fa el calendari d'un joc: tots contra tots als grups i
// eliminatòria amb els dos primers de cada grup.
func arenaBuildSchedule(game string, groups [][]string) []arenaSlot {
	var out []arenaSlot
	letters := "ABCDEFGH"
	for gi, grp := range groups {
		for i := 0; i < len(grp); i++ {
			for j := i + 1; j < len(grp); j++ {
				out = append(out, arenaSlot{Game: game, Stage: "Group " + string(letters[gi]), Phase: "group", A: grp[i], B: grp[j], Winner: -1})
			}
		}
	}
	switch {
	case len(groups) == 1 && len(groups[0]) >= 3:
		out = append(out, arenaSlot{Game: game, Stage: "Final", Phase: "playoff", Pending: true, Winner: -1, From: [2]string{"A1", "A2"}})
	case len(groups) == 2:
		out = append(out,
			arenaSlot{Game: game, Stage: "Semifinal 1", Phase: "playoff", Pending: true, Winner: -1, From: [2]string{"A1", "B2"}},
			arenaSlot{Game: game, Stage: "Semifinal 2", Phase: "playoff", Pending: true, Winner: -1, From: [2]string{"B1", "A2"}},
			arenaSlot{Game: game, Stage: "Final", Phase: "playoff", Pending: true, Winner: -1, From: [2]string{"W:Semifinal 1", "W:Semifinal 2"}})
	case len(groups) >= 3:
		// Quarts amb els dos primers de cada grup (fins a 8), i després semis i final.
		var seeds []string
		for gi := range groups {
			seeds = append(seeds, string(letters[gi])+"1")
		}
		for gi := range groups {
			seeds = append(seeds, string(letters[gi])+"2")
		}
		for len(seeds) < 8 {
			seeds = append(seeds, "")
		}
		pairs := [][2]int{{0, 7}, {3, 4}, {1, 6}, {2, 5}}
		for i, p := range pairs {
			out = append(out, arenaSlot{Game: game, Stage: fmt.Sprintf("Quarterfinal %d", i+1), Phase: "playoff", Pending: true, Winner: -1, From: [2]string{seeds[p[0]], seeds[p[1]]}})
		}
		out = append(out,
			arenaSlot{Game: game, Stage: "Semifinal 1", Phase: "playoff", Pending: true, Winner: -1, From: [2]string{"W:Quarterfinal 1", "W:Quarterfinal 2"}},
			arenaSlot{Game: game, Stage: "Semifinal 2", Phase: "playoff", Pending: true, Winner: -1, From: [2]string{"W:Quarterfinal 3", "W:Quarterfinal 4"}},
			arenaSlot{Game: game, Stage: "Final", Phase: "playoff", Pending: true, Winner: -1, From: [2]string{"W:Semifinal 1", "W:Semifinal 2"}})
	}
	return out
}

type standing struct {
	Owner         string `json:"owner"`
	Played        int    `json:"played"`
	Won           int    `json:"won"`
	Drawn         int    `json:"drawn"`
	Lost          int    `json:"lost"`
	PointsFor     int    `json:"pf"`
	PointsAgainst int    `json:"pa"`
	Pts           int    `json:"pts"`
}

// arenaStandings calcula la classificació d'un grup (3 per victòria, 1 per empat).
func arenaStandings(slots []arenaSlot, game, stage string, members []string) []standing {
	m := map[string]*standing{}
	for _, o := range members {
		m[o] = &standing{Owner: o}
	}
	for _, sl := range slots {
		if sl.Game != game || sl.Stage != stage || !sl.Done {
			continue
		}
		a, b := m[sl.A], m[sl.B]
		if a == nil || b == nil {
			continue
		}
		a.Played++
		b.Played++
		a.PointsFor += sl.Score[0]
		a.PointsAgainst += sl.Score[1]
		b.PointsFor += sl.Score[1]
		b.PointsAgainst += sl.Score[0]
		switch sl.Winner {
		case 0:
			a.Won++
			b.Lost++
			a.Pts += 3
		case 1:
			b.Won++
			a.Lost++
			b.Pts += 3
		default:
			a.Drawn++
			b.Drawn++
			a.Pts++
			b.Pts++
		}
	}
	out := make([]standing, 0, len(members))
	for _, o := range members {
		out = append(out, *m[o])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pts != out[j].Pts {
			return out[i].Pts > out[j].Pts
		}
		di, dj := out[i].PointsFor-out[i].PointsAgainst, out[j].PointsFor-out[j].PointsAgainst
		if di != dj {
			return di > dj
		}
		return out[i].PointsFor > out[j].PointsFor
	})
	return out
}

// resolveSlot omple els rivals d'una eliminatòria quan ja se saben.
func (c *arenaComp) resolveSlot(i int) bool {
	sl := &c.Schedule[i]
	if !sl.Pending {
		return true
	}
	var who [2]string
	for k, from := range sl.From {
		switch {
		case from == "":
			who[k] = ""
		case strings.HasPrefix(from, "W:"):
			stage := strings.TrimPrefix(from, "W:")
			found := false
			for _, x := range c.Schedule {
				if x.Game == sl.Game && x.Stage == stage && x.Done {
					found = true
					if x.Winner == 1 {
						who[k] = x.B
					} else {
						who[k] = x.A
					}
				}
			}
			if !found {
				return false
			}
		default:
			gi := int(from[0] - 'A')
			pos, _ := strconv.Atoi(from[1:])
			groups := c.Groups[sl.Game]
			if gi >= len(groups) {
				who[k] = ""
				continue
			}
			stage := "Group " + string(from[0])
			for _, x := range c.Schedule {
				if x.Game == sl.Game && x.Stage == stage && !x.Done {
					return false
				}
			}
			st := arenaStandings(c.Schedule, sl.Game, stage, groups[gi])
			if pos-1 < len(st) {
				who[k] = st[pos-1].Owner
			}
		}
	}
	sl.A, sl.B, sl.Pending = who[0], who[1], false
	// Un rival que no hi és: passa l'altre sense jugar.
	if sl.A == "" || sl.B == "" {
		sl.Done = true
		if sl.A == "" {
			sl.Winner = 1
		} else {
			sl.Winner = 0
		}
	}
	return true
}

var arenaCompMu sync.Mutex
var arenaCompCancel = map[string]context.CancelFunc{}

// arenaRunComp juga tota la competició, partit rere partit.
func (s *Server) arenaRunComp(ctx context.Context, id string) {
	for {
		if ctx.Err() != nil {
			return
		}
		arenaMu.Lock()
		c := s.arenaComp(id)
		arenaMu.Unlock()
		if c == nil || c.Status != "running" {
			return
		}
		next := -1
		for i := range c.Schedule {
			if c.Schedule[i].Done {
				continue
			}
			if !c.resolveSlot(i) {
				continue
			}
			if c.Schedule[i].Done {
				continue
			}
			next = i
			break
		}
		if next < 0 {
			// Potser falta resoldre eliminatòries buides; si no queda res, s'ha acabat.
			pending := false
			for i := range c.Schedule {
				if !c.Schedule[i].Done {
					pending = true
					c.resolveSlot(i)
				}
			}
			if !pending {
				c.Status = "done"
				c.Champion = map[string]string{}
				for _, sl := range c.Schedule {
					if sl.Stage == "Final" && sl.Done {
						c.Champion[sl.Game] = map[bool]string{true: sl.B, false: sl.A}[sl.Winner == 1]
					}
				}
				for _, g := range c.Games {
					if c.Champion[g] == "" && len(c.Groups[g]) == 1 {
						st := arenaStandings(c.Schedule, g, "Group A", c.Groups[g][0])
						if len(st) > 0 {
							c.Champion[g] = st[0].Owner
						}
					}
				}
			}
			arenaMu.Lock()
			arenaWrite(s.compPath(id), c)
			arenaMu.Unlock()
			if !pending {
				return
			}
			time.Sleep(time.Second)
			continue
		}
		sl := &c.Schedule[next]
		game, ok := arena.Get(sl.Game)
		if !ok {
			sl.Done = true
			continue
		}
		find := func(owner string) arenaEntry {
			for _, e := range c.Entries[sl.Game] {
				if e.Owner == owner {
					return e
				}
			}
			return arenaEntry{Owner: owner, Display: owner}
		}
		// Costat per sorteig determinista (els grups juguen un sol partit).
		a, b := find(sl.A), find(sl.B)
		mid := s.arenaStart(game, "official", c.ID, sl.Stage, [2]arenaEntry{a, b})
		sl.Match = mid
		c.Current = next
		arenaMu.Lock()
		arenaWrite(s.compPath(id), c)
		arenaMu.Unlock()
		// Espera el resultat.
		for {
			if ctx.Err() != nil {
				arenaRunsMu.Lock()
				if r := arenaRuns[mid]; r != nil {
					r.cancel()
				}
				arenaRunsMu.Unlock()
				return
			}
			m := s.arenaLoadMatch(mid)
			if m != nil && (m.Status == "final" || m.Status == "stopped" || m.Status == "failed") {
				arenaMu.Lock()
				c2 := s.arenaComp(id)
				if c2 != nil {
					c2.Schedule[next].Done = m.Status == "final"
					c2.Schedule[next].Winner = m.Winner
					c2.Schedule[next].Score = m.Score
					if m.Status != "final" {
						// Un partit que ha fallat es torna a jugar.
						c2.Schedule[next].Match = ""
					}
					arenaWrite(s.compPath(id), c2)
				}
				arenaMu.Unlock()
				// Eliminatòria empatada: es repeteix (cal un guanyador).
				if m.Status == "final" && c2 != nil && c2.Schedule[next].Phase == "playoff" && m.Winner < 0 {
					arenaMu.Lock()
					c3 := s.arenaComp(id)
					c3.Schedule[next].Done, c3.Schedule[next].Match = false, ""
					arenaWrite(s.compPath(id), c3)
					arenaMu.Unlock()
				}
				break
			}
			time.Sleep(2 * time.Second)
		}
	}
}

// ---- API ----

func (s *Server) arenaUser(r *http.Request) (name string, admin bool) {
	if isSpectator(r) {
		return "", false
	}
	u := userFrom(r)
	if u == nil {
		return "local", true
	}
	if u.Tothom {
		return "local", u.Admin
	}
	return u.Name, u.Admin
}

func (s *Server) arenaGamesJSON() []map[string]string {
	lang := s.fbLangCode()
	var out []map[string]string
	for _, g := range arena.Games() {
		out = append(out, map[string]string{"id": g.ID(), "title": g.Title(lang), "starter": g.Starter(), "rules": g.Rules()})
	}
	return out
}

// handleArena: GET l'estat (jocs, les meves estratègies, rànquing, bots,
// competicions, partits recents); POST {action: save|practice|review|seed_bots}.
func (s *Server) handleArena(w http.ResponseWriter, r *http.Request) {
	me, admin := s.arenaUser(r)
	if r.Method == http.MethodPost {
		var req struct {
			Action   string   `json:"action"`
			Game     string   `json:"game"`
			MD       string   `json:"md"`
			Display  string   `json:"display"`
			Opponent string   `json:"opponent"`
			Games    []string `json:"games"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		game, ok := arena.Get(req.Game)
		if !ok && req.Action != "seed_bots" {
			http.Error(w, "unknown game", http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "save":
			md := strings.TrimSpace(req.MD)
			if len(md) > arenaMaxMD {
				writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": s.tr(fmt.Sprintf("El .md és massa llarg (màxim %d caràcters).", arenaMaxMD), fmt.Sprintf("The .md is too long (max %d characters).", arenaMaxMD))})
				return
			}
			s.saveEntry(arenaEntry{Owner: me, Display: s.arenaDisplay(me, req.Display, s.arenaEntry(game.ID(), me)), Game: game.ID(), MD: md})
			writeJSON(w, map[string]bool{"ok": true})
			return
		case "practice":
			// Un sol clic: desa el .md que s'envia (o la plantilla, si encara
			// no n'hi ha cap) i tria el rival que toca si no se n'ha triat.
			if s.arenaPracticeToday(me) >= arenaPracticePerDay && !admin {
				writeJSONStatus(w, http.StatusTooManyRequests, map[string]string{"error": s.tr("Avui ja has fet prou partits de prova.", "Enough practice games for today.")})
				return
			}
			mine := s.arenaEntry(game.ID(), me)
			md := strings.TrimSpace(req.MD)
			if md == "" && (mine == nil || strings.TrimSpace(mine.MD) == "") {
				md = game.Starter()
			}
			if len(md) > arenaMaxMD {
				writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": s.tr(fmt.Sprintf("El .md és massa llarg (màxim %d caràcters).", arenaMaxMD), fmt.Sprintf("The .md is too long (max %d characters).", arenaMaxMD))})
				return
			}
			if md != "" && (mine == nil || mine.MD != md || (req.Display != "" && req.Display != mine.Display)) {
				s.saveEntry(arenaEntry{Owner: me, Display: s.arenaDisplay(me, req.Display, mine), Game: game.ID(), MD: md})
				mine = s.arenaEntry(game.ID(), me)
			}
			var opp *arenaEntry
			if req.Opponent != "" {
				opp = s.arenaEntry(game.ID(), req.Opponent)
			}
			if opp == nil || !opp.Bot {
				opp = s.arenaNextBot(game, me)
			}
			e := [2]arenaEntry{*mine, *opp}
			if rand.Intn(2) == 1 {
				e[0], e[1] = e[1], e[0]
			}
			writeJSON(w, map[string]string{"id": s.arenaStart(game, "practice", "", "Practice", e)})
			return
		case "review":
			md := strings.TrimSpace(req.MD)
			if md == "" {
				if mine := s.arenaEntry(game.ID(), me); mine != nil {
					md = mine.MD
				}
			}
			if md == "" {
				http.Error(w, "empty", http.StatusBadRequest)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
			defer cancel()
			f, err := s.arenaReview(ctx, game, md)
			if err != nil {
				writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, f)
			return
		case "seed_bots":
			if !admin {
				http.Error(w, "admin only", http.StatusForbidden)
				return
			}
			ids := req.Games
			if len(ids) == 0 {
				for _, g := range arena.Games() {
					ids = append(ids, g.ID())
				}
			}
			goBackground(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
				defer cancel()
				for _, id := range ids {
					if g, ok := arena.Get(id); ok {
						if err := s.arenaSeedBots(ctx, g); err != nil {
							log.Printf("arena: bots %s: %v", id, err)
						}
					}
				}
			})
			writeJSON(w, map[string]bool{"ok": true})
			return
		}
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	// GET
	s.arenaAutoSeed()
	ratings := s.arenaRatings()
	next := map[string]any{}
	mine := map[string]any{}
	bots := map[string][]map[string]any{}
	board := map[string][]map[string]any{}
	for _, g := range arena.Games() {
		if e := s.arenaEntry(g.ID(), me); e != nil {
			mine[g.ID()] = map[string]any{"md": e.MD, "display": e.Display, "updated": e.Updated, "versions": len(e.Versions)}
		}
		displays := map[string]string{}
		for _, e := range s.arenaEntries(g.ID()) {
			displays[e.Owner] = e.Display
			if e.Bot {
				bots[g.ID()] = append(bots[g.ID()], map[string]any{"owner": e.Owner, "display": e.Display, "level": e.Level, "md": e.MD})
			}
		}
		for owner, r := range ratings[g.ID()] {
			d := displays[owner]
			if d == "" {
				d = owner
			}
			board[g.ID()] = append(board[g.ID()], map[string]any{"owner": owner, "display": d, "elo": math.Round(r.Elo), "played": r.Played, "won": r.Won, "lost": r.Lost, "drawn": r.Drawn, "bot": strings.HasPrefix(owner, "bot:")})
		}
		sort.Slice(board[g.ID()], func(i, j int) bool { return board[g.ID()][i]["elo"].(float64) > board[g.ID()][j]["elo"].(float64) })
		if b := s.arenaNextBot(g, me); b != nil {
			next[g.ID()] = map[string]any{"owner": b.Owner, "display": b.Display, "level": b.Level}
		}
	}
	writeJSON(w, map[string]any{"me": me, "display": s.arenaDisplay(me, "", nil), "admin": admin, "games": s.arenaGamesJSON(), "mine": mine, "bots": bots, "next": next, "seeding": arenaSeedingNow(),
		"leaderboard": board, "competitions": s.arenaComps(), "matches": s.arenaRecent(me), "practice_left": arenaPracticePerDay - s.arenaPracticeToday(me)})
}

// arenaDisplay: el nom que es veu als partits. Sense comptes, «Jo».
func (s *Server) arenaDisplay(me, want string, prev *arenaEntry) string {
	if d := oneLine(want, 40); d != "" {
		return d
	}
	if prev != nil && prev.Display != "" && prev.Display != "local" {
		return prev.Display
	}
	if me == "local" || me == "" {
		return s.tr("Jo", "Me")
	}
	return me
}

// arenaNextBot tria el rival de pràctica: el primer bot (de més fàcil a més
// difícil) que encara no has guanyat. Sense bots escrits, la plantilla.
func (s *Server) arenaNextBot(game arena.Game, me string) *arenaEntry {
	beaten := map[string]bool{}
	for _, m := range s.arenaMatchList() {
		if m.Game != game.ID() || m.Status != "final" || m.Winner < 0 {
			continue
		}
		if m.Owners[m.Winner] == me {
			beaten[m.Owners[1-m.Winner]] = true
		}
	}
	var last *arenaEntry
	for _, b := range arenaBotStyles {
		e := s.arenaEntry(game.ID(), b.ID)
		if e == nil || !e.Bot {
			continue
		}
		last = e
		if !beaten[b.ID] {
			return e
		}
	}
	if last != nil {
		return last
	}
	return &arenaEntry{Owner: "bot:template", Display: s.tr("Bot Plantilla", "Bot Template"), Game: game.ID(), Bot: true, Level: "rookie", MD: game.Starter()}
}

var (
	arenaSeedMu    sync.Mutex
	arenaSeedTried = map[string]bool{}
	arenaSeedBusy  = map[string]bool{}
)

func arenaSeedingNow() []string {
	arenaSeedMu.Lock()
	defer arenaSeedMu.Unlock()
	var out []string
	for g, b := range arenaSeedBusy {
		if b {
			out = append(out, g)
		}
	}
	sort.Strings(out)
	return out
}

// arenaAutoSeed escriu els bots d'un joc la primera vegada que algú entra a
// l'Arena (un cop per joc i per arrencada): ningú ha de prémer cap botó.
func (s *Server) arenaAutoSeed() {
	s.mu.Lock()
	ready := s.client != nil && s.cfg != nil
	s.mu.Unlock()
	if !ready {
		return
	}
	for _, g := range arena.Games() {
		hasBots := false
		for _, e := range s.arenaEntries(g.ID()) {
			if e.Bot {
				hasBots = true
				break
			}
		}
		arenaSeedMu.Lock()
		key := s.arenaDir() + "|" + g.ID()
		if hasBots || arenaSeedTried[key] {
			arenaSeedMu.Unlock()
			continue
		}
		arenaSeedTried[key], arenaSeedBusy[g.ID()] = true, true
		arenaSeedMu.Unlock()
		game := g
		goBackground(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			if err := s.arenaSeedBots(ctx, game); err != nil {
				log.Printf("arena: bots %s: %v", game.ID(), err)
			}
			arenaSeedMu.Lock()
			arenaSeedBusy[game.ID()] = false
			arenaSeedMu.Unlock()
		})
	}
}

func (s *Server) arenaMatchList() []arenaMatch {
	files, _ := filepath.Glob(filepath.Join(s.arenaDir(), "matches", "*.json"))
	var out []arenaMatch
	for _, f := range files {
		var m arenaMatch
		if !arenaRead(f, &m) {
			continue
		}
		if live := s.arenaLoadMatch(m.ID); live != nil {
			m = *live
		}
		m.Events, m.Decisions, m.Meta, m.MD, m.Feedback = nil, nil, nil, [2]string{}, [2]*arenaFeedback{}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started > out[j].Started })
	return out
}

func (s *Server) arenaRecent(me string) []arenaMatch {
	var out []arenaMatch
	for _, m := range s.arenaMatchList() {
		if m.Kind == "official" || m.Owners[0] == me || m.Owners[1] == me {
			out = append(out, m)
		}
		if len(out) >= 30 {
			break
		}
	}
	return out
}

func (s *Server) arenaPracticeToday(me string) int {
	today := time.Now().Format("2006-01-02")
	n := 0
	for _, m := range s.arenaMatchList() {
		if m.Kind == "practice" && (m.Owners[0] == me || m.Owners[1] == me) && time.Unix(m.Started, 0).Format("2006-01-02") == today {
			n++
		}
	}
	return n
}

// handleArenaMatch: GET ?id=&after=N; POST ?id= {action: stop}.
func (s *Server) handleArenaMatch(w http.ResponseWriter, r *http.Request) {
	me, admin := s.arenaUser(r)
	id := r.URL.Query().Get("id")
	m := s.arenaLoadMatch(id)
	if m == nil {
		http.Error(w, "no such match", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		if !admin && m.Owners[0] != me && m.Owners[1] != me {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		arenaRunsMu.Lock()
		if run := arenaRuns[id]; run != nil {
			run.cancel()
		}
		arenaRunsMu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	var ev []arena.Event
	for _, e := range m.Events {
		if e.N > after {
			ev = append(ev, e)
		}
	}
	m.Events = ev
	if after > 0 {
		m.Meta = nil
	}
	// El .md dels rivals no es mostra fins que el partit és oficial i acabat
	// (o és un bot): durant la setmana cadascú guarda el seu secret.
	for side := 0; side < 2; side++ {
		own := m.Owners[side] == me
		if !own && !strings.HasPrefix(m.Owners[side], "bot:") && !(m.Kind == "official" && m.Status == "final") {
			m.MD[side] = ""
			m.Feedback[side] = nil
			for i := range m.Decisions {
				if m.Decisions[i].Side == side {
					m.Decisions[i].Rule = ""
				}
			}
		}
	}
	writeJSON(w, m)
}

// handleArenaComp: GET ?id= (o la llista); POST {action: create|start|stop, …}.
func (s *Server) handleArenaComp(w http.ResponseWriter, r *http.Request) {
	_, admin := s.arenaUser(r)
	if r.Method == http.MethodPost {
		if !admin {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Action string   `json:"action"`
			ID     string   `json:"id"`
			Title  string   `json:"title"`
			Games  []string `json:"games"`
			Bots   bool     `json:"bots"` // els bots també hi juguen (proves)
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		switch req.Action {
		case "spectator":
			writeJSON(w, map[string]string{"key": s.arenaSpectatorKey(req.Title == "reset")})
			return
		case "create":
			c := arenaComp{ID: "comp-" + time.Now().Format("20060102-150405"), Title: oneLine(req.Title, 80), Status: "ready", Created: time.Now().Unix(),
				Entries: map[string][]arenaEntry{}, Groups: map[string][][]string{}}
			if c.Title == "" {
				c.Title = s.tr("Competició del ", "Competition of ") + time.Now().Format("2006-01-02")
			}
			ratings := s.arenaRatings()
			games := req.Games
			if len(games) == 0 {
				for _, g := range arena.Games() {
					games = append(games, g.ID())
				}
			}
			for _, gid := range games {
				if _, ok := arena.Get(gid); !ok {
					continue
				}
				var owners []string
				elo := map[string]float64{}
				for _, e := range s.arenaEntries(gid) {
					if e.Bot && !req.Bots {
						continue
					}
					e.Versions = nil
					c.Entries[gid] = append(c.Entries[gid], e)
					owners = append(owners, e.Owner)
					elo[e.Owner] = 1500
					if r := ratings[gid][e.Owner]; r != nil {
						elo[e.Owner] = r.Elo
					}
				}
				if len(owners) < 2 {
					continue
				}
				c.Games = append(c.Games, gid)
				c.Groups[gid] = arenaGroups(owners, elo)
				c.Schedule = append(c.Schedule, arenaBuildSchedule(gid, c.Groups[gid])...)
			}
			if len(c.Games) == 0 {
				writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": s.tr("Cal que almenys dos jugadors tinguin un .md en algun joc.", "At least two players need a .md for some game.")})
				return
			}
			arenaMu.Lock()
			arenaWrite(s.compPath(c.ID), c)
			arenaMu.Unlock()
			writeJSON(w, map[string]string{"id": c.ID})
			return
		case "start":
			arenaMu.Lock()
			c := s.arenaComp(req.ID)
			if c == nil || c.Status == "running" || c.Status == "done" {
				arenaMu.Unlock()
				http.Error(w, "cannot start", http.StatusConflict)
				return
			}
			c.Status, c.Started = "running", time.Now().Unix()
			arenaWrite(s.compPath(c.ID), c)
			arenaMu.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			arenaCompMu.Lock()
			arenaCompCancel[c.ID] = cancel
			arenaCompMu.Unlock()
			goBackground(func() { s.arenaRunComp(ctx, c.ID) })
			writeJSON(w, map[string]bool{"ok": true})
			return
		case "stop":
			arenaCompMu.Lock()
			if cancel := arenaCompCancel[req.ID]; cancel != nil {
				cancel()
				delete(arenaCompCancel, req.ID)
			}
			arenaCompMu.Unlock()
			arenaMu.Lock()
			if c := s.arenaComp(req.ID); c != nil && c.Status == "running" {
				c.Status = "stopped"
				arenaWrite(s.compPath(c.ID), c)
			}
			arenaMu.Unlock()
			writeJSON(w, map[string]bool{"ok": true})
			return
		}
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]any{"competitions": s.arenaComps()})
		return
	}
	arenaMu.Lock()
	c := s.arenaComp(id)
	arenaMu.Unlock()
	if c == nil {
		http.Error(w, "no such competition", http.StatusNotFound)
		return
	}
	writeJSON(w, s.compView(c))
}

// compView: la competició amb classificacions i noms, sense els .md (fins al final).
func (s *Server) compView(c *arenaComp) map[string]any {
	names := map[string]string{}
	titles := map[string]string{}
	for _, list := range c.Entries {
		for _, e := range list {
			names[e.Owner] = e.Display
			titles[e.Owner] = mdTitle(e.MD)
		}
	}
	tables := map[string][]map[string]any{}
	letters := "ABCDEFGH"
	for _, g := range c.Games {
		for gi, grp := range c.Groups[g] {
			stage := "Group " + string(letters[gi])
			var rows []standing
			rows = arenaStandings(c.Schedule, g, stage, grp)
			tables[g] = append(tables[g], map[string]any{"stage": stage, "rows": rows})
		}
	}
	reveal := map[string][]arenaEntry{}
	if c.Status == "done" {
		reveal = c.Entries // en acabar, tothom pot aprendre dels .md dels altres
	}
	return map[string]any{"id": c.ID, "title": c.Title, "games": c.Games, "status": c.Status, "created": c.Created, "started": c.Started,
		"schedule": c.Schedule, "current": c.Current, "champion": c.Champion, "names": names, "titles": titles, "tables": tables, "entries": reveal}
}

// handleArenaStream: el que la retransmissió necessita (competició en marxa,
// partit actual i següent, classificacions).
func (s *Server) handleArenaStream(w http.ResponseWriter, r *http.Request) {
	comps := s.arenaComps()
	var cur *arenaComp
	for i := range comps {
		if comps[i].Status == "running" {
			cur = &comps[i]
			break
		}
	}
	if cur == nil && len(comps) > 0 {
		cur = &comps[0]
	}
	if cur == nil {
		writeJSON(w, map[string]any{"competition": nil})
		return
	}
	arenaMu.Lock()
	c := s.arenaComp(cur.ID)
	arenaMu.Unlock()
	if c == nil {
		writeJSON(w, map[string]any{"competition": nil})
		return
	}
	writeJSON(w, map[string]any{"competition": s.compView(c), "script": s.streamScript(c)})
}

var arenaGameRe = regexp.MustCompile(`^[a-z0-9]+$`)

// handleArenaStarter: la plantilla .md d'un joc (text pla, per descarregar-la).
func (s *Server) handleArenaStarter(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("game")
	g, ok := arena.Get(id)
	if !ok || !arenaGameRe.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`-strategy.md"`)
	_, _ = w.Write([]byte(g.Starter()))
}
