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

// arenaCompactPath: els partits (i el que va veure el model) es desen sense
// sagnar: són els fitxers grans (centenars de KB) i ningú no els edita a mà.
// Els vells, sagnats, es continuen llegint igual.
func arenaCompactPath(path string) bool { return filepath.Base(filepath.Dir(path)) == "matches" }

func arenaWrite(path string, v any) {
	var raw []byte
	var err error
	if arenaCompactPath(path) {
		raw, err = json.Marshal(v)
	} else {
		raw, err = json.MarshalIndent(v, "", " ")
	}
	if err != nil {
		// Millor quedar-se amb el fitxer d'abans que deixar-ne un de buit.
		log.Printf("arena: no es pot desar %s: %v", filepath.Base(path), err)
		return
	}
	arenaWriteBytes(path, raw)
}

// arenaPathMu: un sol escriptor alhora per fitxer (el .tmp és compartit).
var arenaPathMu sync.Map

// arenaWriteBytes desa un JSON ja preparat. Es pot cridar sense cap altre
// candau: qui desa un partit el serialitza amb run.mu i l'escriu fora, perquè
// el canvi de nom de Windows pot trigar fins a un segon.
func arenaWriteBytes(path string, raw []byte) {
	mu, _ := arenaPathMu.LoadOrStore(path, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	defer arenaSumForget(path)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
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
	// RuleFound: la regla que el model diu que ha aplicat és de debò al .md
	// (comprovat pel servidor, no pel model).
	RuleFound bool `json:"rule_found,omitempty"`
	// El cost de la decisió: temps (ms, amb els reintents), crides al model,
	// tokens d'entrada / sortida / d'entrada servits de la cache del
	// proveïdor, i si s'ha reaprofitat una resposta idèntica (memo).
	MS     int64 `json:"ms,omitempty"`
	Calls  int   `json:"calls,omitempty"`
	PT     int   `json:"pt,omitempty"`
	CT     int   `json:"ct,omitempty"`
	Cached int   `json:"cached,omitempty"`
	Memo   bool  `json:"memo,omitempty"`
}

// arenaStats és el que ha costat un partit: crides, tokens i temps.
type arenaStats struct {
	Decisions  int   `json:"decisions"`
	Calls      int   `json:"calls"` // crides al model per decidir (amb els reintents)
	Retries    int   `json:"retries,omitempty"`
	Fallbacks  int   `json:"fallbacks,omitempty"`
	Memo       int   `json:"memo,omitempty"` // decisions reaprofitades sense cridar el model
	Prompt     int   `json:"prompt_tokens"`
	Completion int   `json:"completion_tokens"`
	Cached     int   `json:"cached_tokens"`
	ModelMS    int64 `json:"model_ms"` // suma del temps de decisió de les dues bandes
	QueueMS    int64 `json:"queue_ms,omitempty"`
	WallMS     int64 `json:"wall_ms"` // del xut inicial al final (sense l'informe del coach)
	// Totes les crides del partit: decisions, narradors, anàlisi i informes.
	AllCalls      int `json:"all_calls"`
	AllPrompt     int `json:"all_prompt_tokens"`
	AllCompletion int `json:"all_completion_tokens"`
	AllCached     int `json:"all_cached_tokens"`
}

// ruleNorm deixa un text en minúscules, sense markdown ni puntuació.
func ruleNorm(t string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(t) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// ruleInMD: la regla citada surt al .md, literal o gairebé (el model de
// vegades retalla o canvia un signe): o és un tros del .md, o el 80% de les
// seves paraules són a una mateixa línia.
func ruleInMD(rule, md string) bool {
	r := ruleNorm(rule)
	if len(r) < 6 {
		return false
	}
	if strings.Contains(ruleNorm(md), r) {
		return true
	}
	words := strings.Fields(r)
	if len(words) < 3 {
		return false
	}
	for _, line := range strings.Split(md, "\n") {
		l := " " + ruleNorm(line) + " "
		if len(l) < 8 {
			continue
		}
		hit := 0
		for _, w := range words {
			if strings.Contains(l, " "+w+" ") {
				hit++
			}
		}
		if float64(hit) >= .8*float64(len(words)) {
			return true
		}
	}
	return false
}

// arenaFollowed: quina part de les decisions de cada banda aplica una regla
// que és de debò al seu .md.
func arenaFollowed(m *arenaMatch) [2]float64 {
	var n, ok [2]int
	for _, d := range m.Decisions {
		if d.Side < 0 || d.Side > 1 {
			continue
		}
		n[d.Side]++
		if d.RuleFound {
			ok[d.Side]++
		}
	}
	var out [2]float64
	for i := range out {
		if n[i] > 0 {
			out[i] = float64(ok[i]) / float64(n[i])
		}
	}
	return out
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
	Model     string            `json:"model,omitempty"` // el model que hi va jugar
	// La retransmissió (arena_broadcast.go): prèvia, comentaris i anàlisi.
	Preview      string         `json:"preview,omitempty"`
	PreviewAudio string         `json:"preview_audio,omitempty"`
	Lines        []arenaLine    `json:"lines,omitempty"`
	Analysis     *arenaAnalysis `json:"analysis,omitempty"`
	MD           [2]string      `json:"md,omitempty"`
	Error        string         `json:"error,omitempty"`
	// Hints: el format de decisió porta les ajudes de cada crida (contra què
	// guanya i perd). Per defecte, a les pràctiques sí i als oficials no.
	Hints bool        `json:"hints,omitempty"`
	Stats *arenaStats `json:"stats,omitempty"`
	// Queue: la posició a la cua mentre espera torn (pregame), no es desa.
	Queue int `json:"queue,omitempty"`
}

type arenaRun struct {
	mu     sync.Mutex
	m      *arenaMatch
	cancel context.CancelFunc
	sits   []arenaSituation // el que va veure el model, decisió per decisió
}

var (
	arenaRunsMu sync.Mutex
	arenaRuns   = map[string]*arenaRun{}
)

// ---- torns (quants partits alhora) ----

// arenaSlotCfg: quants partits es juguen alhora. Els oficials i les
// pràctiques tenen torns separats: una pràctica no espera que s'acabi tota
// una competició. Es desa a settings.json com {"slots": {...}}.
type arenaSlotCfg struct {
	Official int `json:"official"`
	Practice int `json:"practice"`
}

const (
	arenaDefaultOfficialSlots = 1
	arenaDefaultPracticeSlots = 2
	arenaMaxSlots             = 8
)

func (s *Server) arenaSlots() arenaSlotCfg {
	var st struct {
		Slots arenaSlotCfg `json:"slots"`
	}
	arenaRead(filepath.Join(s.arenaDir(), "settings.json"), &st)
	c := st.Slots
	if c.Official <= 0 {
		c.Official = arenaDefaultOfficialSlots
	}
	if c.Practice <= 0 {
		c.Practice = arenaDefaultPracticeSlots
	}
	c.Official, c.Practice = min(c.Official, arenaMaxSlots), min(c.Practice, arenaMaxSlots)
	return c
}

func (s *Server) saveArenaSlots(c arenaSlotCfg) arenaSlotCfg {
	c.Official = max(1, min(c.Official, arenaMaxSlots))
	c.Practice = max(1, min(c.Practice, arenaMaxSlots))
	arenaMu.Lock()
	defer arenaMu.Unlock()
	path := filepath.Join(s.arenaDir(), "settings.json")
	raw := map[string]json.RawMessage{}
	arenaRead(path, &raw)
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(c)
	raw["slots"] = b
	arenaWrite(path, raw)
	arenaQ.wake(s.arenaSlotLimit)
	return c
}

func (s *Server) arenaSlotLimit(class string) int {
	c := s.arenaSlots()
	if class == "official" {
		return c.Official
	}
	return c.Practice
}

func arenaClass(kind string) string {
	if kind == "official" {
		return "official"
	}
	return "practice"
}

// arenaSlotQueue reparteix els torns per classe, per ordre d'arribada.
type arenaSlotQueue struct {
	mu      sync.Mutex
	running map[string]int
	waiting []*arenaWaiter
}

type arenaWaiter struct {
	id, class string
	ready     chan struct{}
}

var arenaQ = &arenaSlotQueue{running: map[string]int{}}

// grantLocked dona torn als que esperen mentre n'hi hagi de lliures.
func (q *arenaSlotQueue) grantLocked(limit func(string) int) {
	lim := map[string]int{}
	for i := 0; i < len(q.waiting); {
		w := q.waiting[i]
		l, ok := lim[w.class]
		if !ok {
			l = max(1, limit(w.class))
			lim[w.class] = l
		}
		if q.running[w.class] < l {
			q.running[w.class]++
			close(w.ready)
			q.waiting = append(q.waiting[:i], q.waiting[i+1:]...)
			continue
		}
		i++
	}
}

func (q *arenaSlotQueue) wake(limit func(string) int) {
	q.mu.Lock()
	q.grantLocked(limit)
	q.mu.Unlock()
}

// acquire espera torn; false si el context s'acaba abans.
func (q *arenaSlotQueue) acquire(ctx context.Context, id, class string, limit func(string) int) bool {
	w := &arenaWaiter{id: id, class: class, ready: make(chan struct{})}
	q.mu.Lock()
	q.waiting = append(q.waiting, w)
	q.grantLocked(limit)
	q.mu.Unlock()
	select {
	case <-w.ready:
		return true
	case <-ctx.Done():
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-w.ready: // ha arribat el torn alhora: es torna
		q.running[class]--
		q.grantLocked(limit)
	default:
		for i, x := range q.waiting {
			if x == w {
				q.waiting = append(q.waiting[:i], q.waiting[i+1:]...)
				break
			}
		}
	}
	return false
}

func (q *arenaSlotQueue) release(class string, limit func(string) int) {
	q.mu.Lock()
	q.running[class]--
	q.grantLocked(limit)
	q.mu.Unlock()
}

// position: 1 = el següent a jugar de la seva classe; 0 = no espera.
func (q *arenaSlotQueue) position(id string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	class, n := "", 0
	for _, w := range q.waiting {
		if w.id == id {
			class = w.class
			break
		}
	}
	if class == "" {
		return 0
	}
	for _, w := range q.waiting {
		if w.class == class {
			n++
		}
		if w.id == id {
			return n
		}
	}
	return 0
}

// arenaSituation: el que el model va veure en una decisió (la situació i el
// format), per poder comprovar per què va decidir el que va decidir.
type arenaSituation struct {
	N         int    `json:"n"`
	Side      int    `json:"side"`
	Situation string `json:"situation"`
	Schema    string `json:"schema"`
}

func (s *Server) sitPath(id string) string {
	return filepath.Join(s.arenaDir(), "matches", safeName(id)+".sit")
}

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
		if r.m.Stats != nil {
			st := *r.m.Stats
			cp.Stats = &st
		}
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
	fallback := cfg.FallbackTarget(role)
	// Un partit juga tot amb el mateix model (el que s'hi va fixar en
	// començar), i sense model de reserva: si una banda canviés de model a
	// mitges, el partit ja no seria igual per a tots dos.
	if ref, _ := ctx.Value(arenaPinKey{}).(string); ref != "" {
		if pp, mm, ok := splitModel(ref); ok {
			if prov, ok := cfg.Providers[pp]; ok {
				p, role.Provider, role.Model = prov, pp, mm
			}
		}
		fallback = nil
	}
	msgs := []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	// Sense raonament: cada decisió ha de ser ràpida (un partit de l'NFL en té
	// més de cent per banda) i el .md ja és el raonament.
	text, _, _, err := client.ChatStreamWithToolsFO(llm.WithThink(ctx, "no"), cfg.PrimTarget(p, role), fallback, msgs, 0.2, maxTokens, nil, func(string) {}, nil, nil)
	return text, err
}

// arenaRetryWait: l'espera base entre intents (els tests la posen a zero).
var arenaRetryWait = 1500 * time.Millisecond

// arenaPinKey porta al context el model fixat d'un partit («provider/model»).
type arenaPinKey struct{}

// arenaModelRef és el model que la conversa fa servir ara, «provider/model».
func (s *Server) arenaModelRef() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		return ""
	}
	_, r := s.roleRef()
	if r.Provider == "" || r.Model == "" {
		return ""
	}
	return r.Provider + "/" + r.Model
}

// arenaSystem és la part fixa de cada decisió d'una banda (regles, .md i com
// respondre): sempre idèntica i al principi, perquè la cache de prefixos
// dels proveïdors l'aprofiti partit rere partit.
func arenaSystem(g arena.Game, md string) string {
	return "You are the head coach of a team in an AI strategy competition. You must play by the RULES and follow your team's STRATEGY, written by your human player. Follow it faithfully; when it does not cover a case, decide in its spirit with sound judgement.\n\nRULES:\n" + g.Rules() +
		"\n\nYOUR TEAM'S STRATEGY:\n<<<\n" + md + "\n>>>\n\n" +
		`Answer with JSON only, no other text: {"decision": <the decision JSON exactly as described>, "why": "one short sentence, max 20 words", "rule": "the exact line of the strategy you applied, copied word for word, or \"\" if none"}`
}

// arenaUserPrompt: primer el format (canvia poc: atac/defensa, T/CT) i al
// final la situació, que canvia a cada decisió.
func arenaUserPrompt(p arena.Prompt) string {
	return "DECISION FORMAT:\n" + p.Schema + "\n\nSITUATION:\n" + p.Situation
}

// arenaDecisionTokens: prou per a la decisió, el why (≤ 20 paraules) i la
// regla citada. Si una resposta arriba tallada, el reintent en té més.
const (
	arenaDecisionTokens = 250
	arenaRetryTokens    = 400
)

// arenaDecStats és el cost d'una decisió.
type arenaDecStats struct {
	MS                      int64
	Calls, Retries          int
	Prompt, Completion, Hit int
	Memo                    bool
}

// arenaDecide demana la decisió d'una banda.
func (s *Server) arenaDecide(ctx context.Context, g arena.Game, md string, p arena.Prompt) (arena.Answer, bool) {
	a, fb, _ := s.arenaDecideStats(ctx, g, md, p, false)
	return a, fb
}

// arenaDecideStats decideix i torna el que ha costat. memo: es pot
// reaprofitar la resposta d'un prompt idèntic (només a les pràctiques).
func (s *Server) arenaDecideStats(ctx context.Context, g arena.Game, md string, p arena.Prompt, memo bool) (ans arena.Answer, fallback bool, st arenaDecStats) {
	start := time.Now()
	system, user := arenaSystem(g, md), arenaUserPrompt(p)
	var key [32]byte
	if memo {
		pin, _ := ctx.Value(arenaPinKey{}).(string)
		key = sha256.Sum256([]byte(pin + "\x00" + system + "\x00" + user))
		if a, ok := arenaMemoGet(key); ok {
			st.Memo = true
			st.MS = time.Since(start).Milliseconds()
			return a, false, st
		}
	}
	meter := &llm.UsageMeter{}
	cctx := llm.WithUsage(ctx, meter)
	defer func() {
		u, _, _ := meter.Total()
		st.Prompt, st.Completion, st.Hit = u.PromptTokens, u.CompletionTokens, u.PromptTokensDetails.CachedTokens
		st.MS = time.Since(start).Milliseconds()
	}()
	ask, maxTok := user, arenaDecisionTokens
	var lastValid arena.Answer // JSON bo però amb un valor que no és a la llista
	// Tres intents. Si el proveïdor falla, s'espera (creixent) i es torna a
	// demanar igual; si la resposta no serveix, es torna a demanar de seguida
	// dient-li què no anava bé.
	for try := 0; try < 3; try++ {
		if try > 0 {
			st.Retries++
		}
		st.Calls++
		raw, err := arenaModel(s, cctx, system, ask, maxTok)
		if err != nil {
			if ctx.Err() != nil {
				return arena.Answer{}, true, st
			}
			select {
			case <-ctx.Done():
				return arena.Answer{}, true, st
			case <-time.After(time.Duration((try+1)*(try+1)) * arenaRetryWait):
			}
			continue
		}
		a := arena.ParseAnswer(raw)
		if len(a.Decision) == 0 {
			ask, maxTok = user+arenaFixup(raw, "it was not valid JSON (or it was cut off)"), arenaRetryTokens
			continue
		}
		if problem := arenaCheckDecision(a.Decision, p.Schema); problem != "" {
			lastValid = a
			ask, maxTok = user+arenaFixup(raw, problem), arenaRetryTokens
			continue
		}
		if memo {
			arenaMemoPut(key, a)
		}
		return a, false, st
	}
	if len(lastValid.Decision) > 0 {
		// El joc ja posa el valor per defecte als camps que no entén.
		return lastValid, false, st
	}
	return arena.Answer{}, true, st
}

// arenaFixup: el missatge que es posa al final quan una resposta no serveix
// (el principi del prompt queda igual i la cache el reaprofita).
func arenaFixup(prev, problem string) string {
	return "\n\nYOUR PREVIOUS ANSWER:\n" + clip(strings.TrimSpace(prev), 600) + "\nIt was rejected: " + problem +
		`. Answer again with ONLY the JSON {"decision": ..., "why": ..., "rule": ...}, no other text.`
}

var arenaOptRe = regexp.MustCompile(`"([A-Za-z0-9_]+)"\s*:\s*"([^"]*\|[^"]*)"`)

// arenaSchemaOptions: els valors permesos de cada camp, de la primera línia
// del format ({"buy":"eco|half|force|full", …}).
func arenaSchemaOptions(schema string) map[string][]string {
	line := schema
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	out := map[string][]string{}
	for _, m := range arenaOptRe.FindAllStringSubmatch(line, -1) {
		out[m[1]] = strings.Split(m[2], "|")
	}
	return out
}

// arenaCheckDecision diu què no va d'una decisió ("" si està bé): un valor
// que no és a la llista del format. Els camps que falten no compten (el joc
// hi posa el valor per defecte).
func arenaCheckDecision(dec json.RawMessage, schema string) string {
	opts := arenaSchemaOptions(schema)
	if len(opts) == 0 {
		return ""
	}
	var obj map[string]any
	if json.Unmarshal(dec, &obj) != nil {
		return "the decision must be a JSON object with the fields of the DECISION FORMAT"
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, ok := obj[k]
		if !ok || v == nil {
			continue
		}
		allowed := opts[k]
		sv, isStr := v.(string)
		if !isStr {
			return fmt.Sprintf("%q must be one of %s", k, strings.Join(allowed, "|"))
		}
		sv = strings.ToLower(strings.TrimSpace(sv))
		found := false
		for _, a := range allowed {
			if sv == a {
				found = true
				break
			}
		}
		if !found {
			return fmt.Sprintf("value %q is not allowed for %q (allowed: %s)", v, k, strings.Join(allowed, "|"))
		}
	}
	return ""
}

// La memòria de respostes de les pràctiques: el mateix model, el mateix .md
// i la mateixa situació byte a byte (la ronda de pistoles del CS2, les
// primeres tries del LoL) donen la mateixa decisió sense tornar a cridar.
var (
	arenaMemoMu sync.Mutex
	arenaMemo   = map[[32]byte]arena.Answer{}
)

const arenaMemoMax = 4096

func arenaMemoGet(k [32]byte) (arena.Answer, bool) {
	arenaMemoMu.Lock()
	defer arenaMemoMu.Unlock()
	a, ok := arenaMemo[k]
	return a, ok
}

func arenaMemoPut(k [32]byte, a arena.Answer) {
	arenaMemoMu.Lock()
	defer arenaMemoMu.Unlock()
	if len(arenaMemo) >= arenaMemoMax {
		clear(arenaMemo)
	}
	arenaMemo[k] = a
}

// arenaStart prepara un partit i el posa a la cua. Torna l'id.
// Les pràctiques porten les ajudes del format; els oficials, les de la competició.
func (s *Server) arenaStart(game arena.Game, kind, comp, stage string, e [2]arenaEntry) string {
	return s.arenaStartWith(game, kind, comp, stage, e, "", kind != "official")
}

// arenaStartWith: amb el model fixat (el de la competició); buit, el d'ara.
func (s *Server) arenaStartWith(game arena.Game, kind, comp, stage string, e [2]arenaEntry, model string, hints bool) string {
	if model == "" {
		model = s.arenaModelRef()
	}
	now := time.Now()
	id := fmt.Sprintf("%s-%s-%04x", game.ID(), now.Format("20060102-150405"), now.UnixNano()&0xffff)
	seed := int64(now.UnixNano() % 1000000)
	if comp != "" {
		h := sha256.Sum256([]byte(comp + e[0].Owner + e[1].Owner + stage))
		seed = int64(h[0])<<16 | int64(h[1])<<8 | int64(h[2])
	}
	m := &arenaMatch{ID: id, Game: game.ID(), Kind: kind, Comp: comp, Stage: stage, Status: "pregame", Started: now.Unix(), Seed: seed, Winner: -1,
		Owners: [2]string{e[0].Owner, e[1].Owner}, Names: [2]string{e[0].Display, e[1].Display},
		Titles: [2]string{mdTitle(e[0].MD), mdTitle(e[1].MD)}, MD: [2]string{e[0].MD, e[1].MD}, Model: model, Hints: hints}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), arenaPinKey{}, model))
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
		s.arenaSaveRun(run, true)
		arenaRunsMu.Lock()
		delete(arenaRuns, run.m.ID)
		arenaRunsMu.Unlock()
	}()
	// Els partits esperen torn: els oficials i les pràctiques per separat.
	class := arenaClass(run.m.Kind)
	queued := time.Now()
	if !arenaQ.acquire(ctx, run.m.ID, class, s.arenaSlotLimit) {
		s.arenaFinish(run, "stopped", "")
		return
	}
	release := sync.OnceFunc(func() { arenaQ.release(class, s.arenaSlotLimit) })
	defer release()
	// Tot el que el model gasta en aquest partit (decisions, narradors,
	// anàlisi, informes) suma aquí.
	meter := &llm.UsageMeter{}
	ctx = llm.WithUsage(ctx, meter)
	m := run.m
	lang := s.fbLangCode()
	match := game.NewMatch(m.Seed, lang, m.Names)
	if h, ok := match.(interface{ SetHints(bool) }); ok {
		h.SetHints(m.Hints)
	}
	meta, _ := json.Marshal(match.Meta())
	kickoff := time.Now()
	run.mu.Lock()
	m.Meta, m.Status = meta, "live"
	m.Stats = &arenaStats{QueueMS: kickoff.Sub(queued).Milliseconds()}
	run.mu.Unlock()
	syncStats := func() { // amb run.mu
		u, calls, _ := meter.Total()
		st := m.Stats
		st.WallMS = time.Since(kickoff).Milliseconds()
		st.AllCalls, st.AllPrompt, st.AllCompletion, st.AllCached = calls, u.PromptTokens, u.CompletionTokens, u.PromptTokensDetails.CachedTokens
	}
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
	memo := m.Kind == "practice"
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
		var cost [2]arenaDecStats
		var wg sync.WaitGroup
		for side := 0; side < 2; side++ {
			wg.Add(1)
			go func(side int) {
				defer wg.Done()
				if strings.TrimSpace(p[side].Schema) == "" {
					return // aquesta banda no decideix ara (p. ex. torn de l'altre al draft)
				}
				ans[side], fb[side], cost[side] = s.arenaDecideStats(ctx, game, md[side], p[side], memo)
			}(side)
		}
		wg.Wait()
		events := match.Apply([2]json.RawMessage{ans[0].Decision, ans[1].Decision})
		var raw, sits []byte
		run.mu.Lock()
		dn := steps // el mateix número que porten les jugades (al draft del LoL, de vegades només decideix un equip)
		for side := 0; side < 2; side++ {
			if strings.TrimSpace(p[side].Schema) == "" {
				continue
			}
			c := cost[side]
			m.Decisions = append(m.Decisions, arenaDecision{N: dn, Side: side, Short: p[side].Short, Decision: ans[side].Decision,
				Why: oneLine(ans[side].Why, 240), Rule: oneLine(ans[side].Rule, 240), Fallback: fb[side],
				RuleFound: !fb[side] && ruleInMD(ans[side].Rule, md[side]), MS: c.MS, Calls: c.Calls, PT: c.Prompt, CT: c.Completion, Cached: c.Hit, Memo: c.Memo})
			run.sits = append(run.sits, arenaSituation{N: dn, Side: side, Situation: clip(p[side].Situation, 4000), Schema: clip(p[side].Schema, 3000)})
			st := m.Stats
			st.Decisions++
			st.Calls += c.Calls
			st.Retries += c.Retries
			st.Prompt += c.Prompt
			st.Completion += c.Completion
			st.Cached += c.Hit
			st.ModelMS += c.MS
			if fb[side] {
				st.Fallbacks++
			}
			if c.Memo {
				st.Memo++
			}
		}
		m.Events = append(m.Events, events...)
		m.Score = match.Score()
		syncStats()
		// Es desa cada 4 passos: aquí només se serialitza (amb el candau) i
		// s'escriu fora, perquè els visors no s'esperin pel disc. El que va
		// veure el model només es demana del partit en marxa a la memòria:
		// al disc n'hi ha prou de tant en tant.
		if steps%4 == 0 {
			raw, _ = json.Marshal(m)
			if steps%40 == 0 {
				sits, _ = json.Marshal(run.sits)
			}
		}
		run.mu.Unlock()
		for _, e := range events {
			if e.Highlight {
				pendingHL = append(pendingHL, e)
			}
		}
		// Mai bloquejar per al narrador: si va endarrerit, les jugades
		// s'acumulen per al tram següent (les més noves, com a molt 30).
		if len(pendingHL) >= 3 {
			select {
			case castCh <- pendingHL:
				pendingHL = nil
			default:
				if n := len(pendingHL); n > 30 {
					pendingHL = append([]arena.Event(nil), pendingHL[n-30:]...)
				}
			}
		}
		if raw != nil {
			arenaWriteBytes(s.matchPath(m.ID), raw)
		}
		if sits != nil {
			arenaWriteBytes(s.sitPath(m.ID), sits)
		}
	}
	if len(pendingHL) > 0 {
		castCh <- pendingHL // sense cap candau: el narrador pot treballar
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
	run.mu.Lock()
	syncStats()
	run.mu.Unlock()
	// Un partit oficial on el model no ha pogut decidir massa vegades per una
	// banda (més del 10%) no diu res dels .md: no compta i es torna a jugar.
	if m.Kind == "official" {
		if side, rate := arenaFallbackRate(m); rate > arenaMaxFallback {
			s.arenaFinish(run, "failed", fmt.Sprintf("too many default decisions for side %d (%.0f%%)", side, rate*100))
			log.Printf("arena: %s %s es repetirà: %.0f%% de decisions per defecte", m.Game, m.ID, rate*100)
			return
		}
		s.arenaRecord(m.Game, m.Owners, m.Winner)
	}
	s.arenaFinish(run, "final", "")
	// El torn queda lliure: l'informe no fa esperar el partit següent.
	release()
	// L'informe educatiu de cada banda (després: el partit ja es pot mirar).
	// Els bots de les pràctiques no en tenen: ningú el llegiria.
	for side := 0; side < 2; side++ {
		if m.Kind == "practice" && strings.HasPrefix(m.Owners[side], "bot:") {
			continue
		}
		if f := s.arenaFeedback(ctx, game, run, side); f != nil {
			run.mu.Lock()
			m.Feedback[side] = f
			run.mu.Unlock()
		}
	}
	run.mu.Lock()
	u, calls, _ := meter.Total()
	m.Stats.AllCalls, m.Stats.AllPrompt, m.Stats.AllCompletion, m.Stats.AllCached = calls, u.PromptTokens, u.CompletionTokens, u.PromptTokensDetails.CachedTokens
	run.mu.Unlock()
	log.Printf("arena: %s %s %d-%d", m.Game, m.ID, m.Score[0], m.Score[1])
}

// arenaSaveRun desa el partit (i, si cal, el que va veure el model): es
// serialitza amb run.mu i s'escriu sense.
func (s *Server) arenaSaveRun(run *arenaRun, sits bool) {
	run.mu.Lock()
	raw, err := json.Marshal(run.m)
	var sraw []byte
	if sits && len(run.sits) > 0 {
		sraw, _ = json.Marshal(run.sits)
	}
	id := run.m.ID
	run.mu.Unlock()
	if sraw != nil {
		arenaWriteBytes(s.sitPath(id), sraw)
	}
	if err != nil {
		log.Printf("arena: no es pot desar %s: %v", id, err)
		return
	}
	arenaWriteBytes(s.matchPath(id), raw)
}

const arenaMaxFallback = 0.10

// arenaFallbackRate torna la banda amb més decisions per defecte i la proporció.
func arenaFallbackRate(m *arenaMatch) (int, float64) {
	var n, fb [2]int
	for _, d := range m.Decisions {
		if d.Side < 0 || d.Side > 1 {
			continue
		}
		n[d.Side]++
		if d.Fallback {
			fb[d.Side]++
		}
	}
	worst, rate := 0, 0.0
	for side := 0; side < 2; side++ {
		if n[side] > 0 {
			if r := float64(fb[side]) / float64(n[side]); r > rate {
				worst, rate = side, r
			}
		}
	}
	return worst, rate
}

func (s *Server) arenaFinish(run *arenaRun, status, why string) {
	run.mu.Lock()
	run.m.Status, run.m.Error, run.m.Finished = status, why, time.Now().Unix()
	run.mu.Unlock()
	s.arenaSaveRun(run, true)
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
	lang := gameLang()
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
	lang := gameLang()
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
	// Les eliminatòries es juguen al millor de 3, canviant de costat:
	// Series són les victòries de cada banda i Matches tots els partits.
	Series  [2]int   `json:"series,omitempty"`
	Matches []string `json:"matches,omitempty"`
	Replays int      `json:"replays,omitempty"`
}

type arenaComp struct {
	ID       string                  `json:"id"`
	Title    string                  `json:"title"`
	Games    []string                `json:"games"`
	Status   string                  `json:"status"`          // ready, running, done, stopped
	Model    string                  `json:"model,omitempty"` // fixat en començar
	Created  int64                   `json:"created"`
	Started  int64                   `json:"started,omitempty"`
	Entries  map[string][]arenaEntry `json:"entries"` // per joc, la foto de les estratègies
	Groups   map[string][][]string   `json:"groups"`  // per joc, els grups (owners)
	Schedule []arenaSlot             `json:"schedule"`
	Current  int                     `json:"current"`
	Champion map[string]string       `json:"champion,omitempty"`
	// Hints: el format de decisió dels partits porta les ajudes de cada crida
	// (contra què guanya i perd). Per defecte no: el .md ha de fer la feina.
	Hints bool `json:"hints,omitempty"`
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
		// Els grups juguen un partit per parella; les eliminatòries, al millor
		// de 3, canviant de costat a cada partit. Cada repetició (un empat a
		// les eliminatòries, un partit que no compta) porta una llavor nova.
		e := [2]arenaEntry{find(sl.A), find(sl.B)}
		gameNo := len(sl.Matches) + 1
		stage := sl.Stage
		if sl.Phase == "playoff" {
			stage = fmt.Sprintf("%s · Game %d", sl.Stage, gameNo)
		}
		if sl.Replays > 0 {
			stage += fmt.Sprintf(" · replay %d", sl.Replays)
		}
		flip := sl.Phase == "playoff" && gameNo%2 == 0
		if flip {
			e[0], e[1] = e[1], e[0]
		}
		mid := s.arenaStartWith(game, "official", c.ID, stage, e, c.Model, c.Hints)
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
			if m == nil || (m.Status != "final" && m.Status != "stopped" && m.Status != "failed") {
				time.Sleep(2 * time.Second)
				continue
			}
			// El resultat, vist des de A i B del calendari.
			w, sc := m.Winner, m.Score
			if flip {
				sc = [2]int{sc[1], sc[0]}
				if w >= 0 {
					w = 1 - w
				}
			}
			arenaMu.Lock()
			if c2 := s.arenaComp(id); c2 != nil {
				slot := &c2.Schedule[next]
				switch {
				case m.Status == "stopped" || (m.Status == "failed" && (slot.Replays < arenaMaxReplays || m.Winner < 0 && slot.Replays < 5)):
					// No compta: es torna a jugar (amb un límit, si és per les
					// decisions per defecte; un error de veritat, sempre).
					slot.Replays++
					slot.Match = ""
				case slot.Phase != "playoff":
					slot.Matches = append(slot.Matches, mid)
					slot.Done, slot.Winner, slot.Score, slot.Replays = true, w, sc, 0
				default:
					slot.Matches = append(slot.Matches, mid)
					if w >= 0 {
						slot.Series[w]++
					}
					slot.Score, slot.Replays = slot.Series, 0
					if slot.Series[0] >= 2 || slot.Series[1] >= 2 || len(slot.Matches) >= 5 {
						slot.Done = true
						slot.Winner = 0
						if slot.Series[1] > slot.Series[0] {
							slot.Winner = 1
						}
					} else {
						slot.Match = ""
					}
				}
				arenaWrite(s.compPath(id), c2)
			}
			arenaMu.Unlock()
			break
		}
	}
}

// arenaMaxReplays: vegades que es repeteix un partit que no compta abans
// d'acceptar-ne el resultat (sense moure l'Elo).
const arenaMaxReplays = 2

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
			At       int64    `json:"at"`       // version
			Official int      `json:"official"` // slots
			Practice int      `json:"practice"` // slots
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		if req.Action == "slots" {
			// Quants partits alhora: oficials i pràctiques per separat.
			if !admin {
				http.Error(w, "admin only", http.StatusForbidden)
				return
			}
			writeJSON(w, s.saveArenaSlots(arenaSlotCfg{Official: req.Official, Practice: req.Practice}))
			return
		}
		game, ok := arena.Get(req.Game)
		if !ok && req.Action != "seed_bots" {
			http.Error(w, "unknown game", http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "version":
			// El text d'una versió del meu .md (de la llista «history»).
			if e := s.arenaEntry(game.ID(), me); e != nil {
				if e.Updated == req.At {
					writeJSON(w, map[string]any{"at": e.Updated, "md": e.MD, "current": true})
					return
				}
				for _, v := range e.Versions {
					if v.At == req.At {
						writeJSON(w, map[string]any{"at": v.At, "md": v.MD})
						return
					}
				}
			}
			http.Error(w, "no such version", http.StatusNotFound)
			return
		case "save":
			md := strings.TrimSpace(req.MD)
			if len(md) > arenaMaxMD {
				writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": gameTr(fmt.Sprintf("El .md és massa llarg (màxim %d caràcters).", arenaMaxMD), fmt.Sprintf("The .md is too long (max %d characters).", arenaMaxMD))})
				return
			}
			s.saveEntry(arenaEntry{Owner: me, Display: s.arenaDisplay(me, req.Display, s.arenaEntry(game.ID(), me)), Game: game.ID(), MD: md})
			writeJSON(w, map[string]bool{"ok": true})
			return
		case "practice":
			// Un sol clic: desa el .md que s'envia (o la plantilla, si encara
			// no n'hi ha cap) i tria el rival que toca si no se n'ha triat.
			if s.arenaPracticeToday(me) >= arenaPracticePerDay && !admin {
				writeJSONStatus(w, http.StatusTooManyRequests, map[string]string{"error": gameTr("Avui ja has fet prou partits de prova.", "Enough practice games for today.")})
				return
			}
			mine := s.arenaEntry(game.ID(), me)
			md := strings.TrimSpace(req.MD)
			if md == "" && (mine == nil || strings.TrimSpace(mine.MD) == "") {
				md = game.Starter()
			}
			if len(md) > arenaMaxMD {
				writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": gameTr(fmt.Sprintf("El .md és massa llarg (màxim %d caràcters).", arenaMaxMD), fmt.Sprintf("The .md is too long (max %d characters).", arenaMaxMD))})
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
	s.voiceAutoInstall("en")
	ratings := s.arenaRatings()
	// Una sola passada pels partits (de la cache): recents, progrés,
	// pràctiques d'avui i bots guanyats.
	sums := s.arenaSummaries()
	beaten := arenaBeaten(sums, me)
	next := map[string]any{}
	mine := map[string]any{}
	bots := map[string][]map[string]any{}
	board := map[string][]map[string]any{}
	for _, g := range arena.Games() {
		if e := s.arenaEntry(g.ID(), me); e != nil {
			mine[g.ID()] = map[string]any{"md": e.MD, "display": e.Display, "updated": e.Updated, "versions": len(e.Versions), "history": arenaHistory(e)}
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
		if b := s.arenaNextBotFrom(g, me, beaten); b != nil {
			next[g.ID()] = map[string]any{"owner": b.Owner, "display": b.Display, "level": b.Level}
		}
	}
	writeJSON(w, map[string]any{"me": me, "display": s.arenaDisplay(me, "", nil), "progress": arenaProgressOf(sums, me), "admin": admin, "games": s.arenaGamesJSON(), "mine": mine, "bots": bots, "next": next, "seeding": arenaSeedingNow(),
		"leaderboard": board, "competitions": s.arenaComps(), "matches": arenaRecentOf(sums, me), "practice_left": arenaPracticePerDay - arenaPracticeTodayOf(sums, me),
		"slots": s.arenaSlots()})
}

// arenaHistory: les versions del .md (la més antiga primer i l'actual al
// final): quan, un hash curt i la mida. El text d'una versió es demana amb
// POST {action: "version", game, at}.
func arenaHistory(e *arenaEntry) []map[string]any {
	out := make([]map[string]any, 0, len(e.Versions)+1)
	add := func(at int64, md string, current bool) {
		h := sha256.Sum256([]byte(md))
		row := map[string]any{"at": at, "hash": fmt.Sprintf("%x", h[:4]), "size": len(md)}
		if current {
			row["current"] = true
		}
		out = append(out, row)
	}
	for _, v := range e.Versions {
		add(v.At, v.MD, false)
	}
	add(e.Updated, e.MD, true)
	return out
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
		return gameTr("Jo", "Me")
	}
	return me
}

// arenaNextBot tria el rival de pràctica: el primer bot (de més fàcil a més
// difícil) que encara no has guanyat. Sense bots escrits, la plantilla.
func (s *Server) arenaNextBot(game arena.Game, me string) *arenaEntry {
	return s.arenaNextBotFrom(game, me, arenaBeaten(s.arenaSummaries(), me))
}

// arenaBeaten: per joc, els rivals que me ja ha guanyat.
func arenaBeaten(sums []arenaSum, me string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, x := range sums {
		m := &x.M
		if m.Status != "final" || m.Winner < 0 || m.Owners[m.Winner] != me {
			continue
		}
		if out[m.Game] == nil {
			out[m.Game] = map[string]bool{}
		}
		out[m.Game][m.Owners[1-m.Winner]] = true
	}
	return out
}

func (s *Server) arenaNextBotFrom(game arena.Game, me string, beatenAll map[string]map[string]bool) *arenaEntry {
	beaten := beatenAll[game.ID()]
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
	return &arenaEntry{Owner: "bot:template", Display: gameTr("Bot Plantilla", "Bot Template"), Game: game.ID(), Bot: true, Level: "rookie", MD: game.Starter()}
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

// ---- resums dels partits (cache) ----

// arenaSum és la capçalera d'un partit (sense jugades, decisions, .md ni
// comentaris) i la nota del coach de cada banda: el que la pàgina de l'Arena
// necessita de tots els partits. Es guarda en memòria per fitxer, amb la
// data i la mida: GET /api/arena ja no llegeix ni analitza cada partit
// sencer a cada petició (amb 117 partits, 31 MB, trigava 2 s).
type arenaSum struct {
	M     arenaMatch
	Coach [2]int
}

type arenaSumEntry struct {
	mod, size int64
	sum       arenaSum
}

var (
	arenaSumMu sync.Mutex
	arenaSums  = map[string]*arenaSumEntry{}
)

func arenaSumForget(path string) {
	if !strings.HasSuffix(path, ".json") {
		return
	}
	arenaSumMu.Lock()
	delete(arenaSums, path)
	arenaSumMu.Unlock()
}

// arenaSkip llegeix un camp JSON sense guardar-lo.
type arenaSkip struct{}

func (*arenaSkip) UnmarshalJSON([]byte) error { return nil }

// arenaFileHead és un fitxer de partit llegit només pel que fa falta: els
// camps de fora tapen els de l'arenaMatch de dins.
type arenaFileHead struct {
	*arenaMatch
	Events       arenaSkip `json:"events"`
	Decisions    arenaSkip `json:"decisions"`
	Meta         arenaSkip `json:"meta"`
	Lines        arenaSkip `json:"lines"`
	Analysis     arenaSkip `json:"analysis"`
	MD           arenaSkip `json:"md"`
	Preview      arenaSkip `json:"preview"`
	PreviewAudio arenaSkip `json:"preview_audio"`
	Feedback     [2]*struct {
		Scores map[string]int `json:"scores"`
	} `json:"feedback"`
}

func coachOf(scores map[string]int) int {
	n := 0
	for _, v := range scores {
		n += v
	}
	return n
}

// arenaHeadOf: la capçalera d'un partit en memòria (amb el candau del run).
func arenaHeadOf(m *arenaMatch) arenaSum {
	h := *m
	var sum arenaSum
	for i, f := range m.Feedback {
		if f != nil {
			sum.Coach[i] = coachOf(f.Scores)
		}
	}
	h.Events, h.Decisions, h.Meta, h.Lines, h.Analysis = nil, nil, nil, nil, nil
	h.MD, h.Feedback, h.Preview, h.PreviewAudio = [2]string{}, [2]*arenaFeedback{}, "", ""
	if m.Stats != nil {
		st := *m.Stats
		h.Stats = &st
	}
	sum.M = h
	return sum
}

func arenaReadHead(path string) (arenaSum, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return arenaSum{}, false
	}
	var m arenaMatch
	h := arenaFileHead{arenaMatch: &m}
	if json.Unmarshal(raw, &h) != nil {
		return arenaSum{}, false
	}
	sum := arenaSum{M: m}
	for i, f := range h.Feedback {
		if f != nil {
			sum.Coach[i] = coachOf(f.Scores)
		}
	}
	return sum, true
}

// arenaSummaries: totes les capçaleres, de la més nova a la més antiga. Els
// partits en marxa surten tal com són ara mateix (i, si esperen torn, amb la
// posició a la cua).
func (s *Server) arenaSummaries() []arenaSum {
	dir := filepath.Join(s.arenaDir(), "matches")
	ents, _ := os.ReadDir(dir)
	out := make([]arenaSum, 0, len(ents))
	seen := make(map[string]bool, len(ents))
	for _, de := range ents {
		name := de.Name()
		if de.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, name)
		mod, size := info.ModTime().UnixNano(), info.Size()
		arenaSumMu.Lock()
		c := arenaSums[path]
		arenaSumMu.Unlock()
		if c == nil || c.mod != mod || c.size != size {
			sum, ok := arenaReadHead(path)
			if !ok {
				continue
			}
			c = &arenaSumEntry{mod: mod, size: size, sum: sum}
			arenaSumMu.Lock()
			arenaSums[path] = c
			arenaSumMu.Unlock()
		}
		out = append(out, c.sum)
		seen[c.sum.M.ID] = true
	}
	arenaRunsMu.Lock()
	runs := make([]*arenaRun, 0, len(arenaRuns))
	for _, r := range arenaRuns {
		runs = append(runs, r)
	}
	arenaRunsMu.Unlock()
	live := map[string]arenaSum{}
	for _, r := range runs {
		r.mu.Lock()
		h := arenaHeadOf(r.m)
		r.mu.Unlock()
		if h.M.Status == "pregame" {
			h.M.Queue = arenaQ.position(h.M.ID)
		}
		live[h.M.ID] = h
	}
	for i := range out {
		if h, ok := live[out[i].M.ID]; ok {
			out[i] = h
			delete(live, h.M.ID)
		}
	}
	for _, h := range live {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].M.Started > out[j].M.Started })
	return out
}

// arenaProgress: els teus partits acabats per joc (del més antic al més nou):
// resultat, marcador, rival i la nota del coach (0-25), per veure si el .md
// millora d'un partit a l'altre.
func (s *Server) arenaProgress(me string) map[string][]map[string]any {
	return arenaProgressOf(s.arenaSummaries(), me)
}

func arenaProgressOf(sums []arenaSum, me string) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for i := len(sums) - 1; i >= 0; i-- { // del més antic al més nou
		m := &sums[i].M
		if m.Status != "final" {
			continue
		}
		side := -1
		for i, o := range m.Owners {
			if o == me {
				side = i
			}
		}
		if side < 0 {
			continue
		}
		res := "draw"
		if m.Winner == side {
			res = "win"
		} else if m.Winner == 1-side {
			res = "loss"
		}
		out[m.Game] = append(out[m.Game], map[string]any{"id": m.ID, "started": m.Started, "kind": m.Kind, "opponent": m.Names[1-side],
			"result": res, "score": [2]int{m.Score[side], m.Score[1-side]}, "coach": sums[i].Coach[side]})
	}
	for g := range out {
		sort.SliceStable(out[g], func(i, j int) bool { return out[g][i]["started"].(int64) < out[g][j]["started"].(int64) })
		if n := len(out[g]); n > 20 {
			out[g] = out[g][n-20:]
		}
	}
	return out
}

// arenaMatchList: les capçaleres de tots els partits, del més nou al més antic.
func (s *Server) arenaMatchList() []arenaMatch {
	sums := s.arenaSummaries()
	out := make([]arenaMatch, len(sums))
	for i := range sums {
		out[i] = sums[i].M
	}
	return out
}

func (s *Server) arenaRecent(me string) []arenaMatch { return arenaRecentOf(s.arenaSummaries(), me) }

func arenaRecentOf(sums []arenaSum, me string) []arenaMatch {
	out := []arenaMatch{}
	for i := range sums {
		m := sums[i].M
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
	return arenaPracticeTodayOf(s.arenaSummaries(), me)
}

func arenaPracticeTodayOf(sums []arenaSum, me string) int {
	today := time.Now().Format("2006-01-02")
	n := 0
	for i := range sums {
		m := &sums[i].M
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
		if m = arenaDemo(id); m != nil {
			s.arenaDemoVoice(m)
		}
	}
	if m == nil {
		http.Error(w, "no such match", http.StatusNotFound)
		return
	}
	renum := arenaRenumber(m) // partits antics del LoL: el número de cada decisió
	for i, n := range renum {
		m.Decisions[i].N = n
	}
	arenaWithMoments(m) // partits antics i de mostra: els moments del servidor
	if r.Method == http.MethodPost {
		if arenaDemoGame(id) != "" {
			http.Error(w, "demo", http.StatusBadRequest)
			return
		}
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
	// ?situation=N&side=S: el que el model va veure en aquella decisió.
	if q := r.URL.Query().Get("situation"); q != "" {
		n, _ := strconv.Atoi(q)
		side, _ := strconv.Atoi(r.URL.Query().Get("side"))
		var sits []arenaSituation
		arenaRunsMu.Lock()
		run := arenaRuns[id]
		arenaRunsMu.Unlock()
		if run != nil {
			run.mu.Lock()
			sits = append(sits, run.sits...)
			run.mu.Unlock()
		} else if !arenaRead(s.sitPath(id), &sits) {
			sits = arenaDemoSits(id)
		}
		if len(renum) == len(sits) {
			for i := range sits {
				sits[i].N = renum[i]
			}
		}
		for _, x := range sits {
			if x.N == n && x.Side == side {
				writeJSON(w, x)
				return
			}
		}
		http.Error(w, "not recorded", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	after, _ := strconv.Atoi(q.Get("after"))
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
	if m.Status == "pregame" {
		m.Queue = arenaQ.position(m.ID)
	}
	// El .md dels rivals no es mostra fins que el partit és oficial i acabat
	// (o és un bot): durant la setmana cadascú guarda el seu secret.
	for side := 0; side < 2; side++ {
		own := m.Owners[side] == me
		public := strings.HasPrefix(m.Owners[side], "bot:") || strings.HasPrefix(m.Owners[side], "demo:")
		if !own && !public && !(m.Kind == "official" && m.Status == "final") {
			m.MD[side] = ""
			m.Feedback[side] = nil
			arenaHideMoments(m, side)
			for i := range m.Decisions {
				if m.Decisions[i].Side == side {
					m.Decisions[i].Rule = ""
				}
			}
		}
	}
	followed := arenaFollowed(m)
	// Les consultes lleugeres: dafter=K torna només les decisions a partir de
	// la K (el visor ja en té K), lafter=K el mateix amb els comentaris, i
	// els .md només van amb after=0. Sense paràmetres, tot com sempre.
	out := struct {
		*arenaMatch
		Followed       [2]float64 `json:"followed"`
		MD             *[2]string `json:"md,omitempty"`
		DecisionsTotal *int       `json:"decisions_total,omitempty"`
		LinesTotal     *int       `json:"lines_total,omitempty"`
	}{arenaMatch: m, Followed: followed}
	if after <= 0 {
		out.MD = &m.MD
	}
	if v := q.Get("dafter"); v != "" {
		k, _ := strconv.Atoi(v)
		n := len(m.Decisions)
		m.Decisions = m.Decisions[min(max(k, 0), n):]
		out.DecisionsTotal = &n
	}
	if v := q.Get("lafter"); v != "" {
		k, _ := strconv.Atoi(v)
		n := len(m.Lines)
		m.Lines = m.Lines[min(max(k, 0), n):]
		out.LinesTotal = &n
	}
	writeJSON(w, out)
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
			Bots   bool     `json:"bots"`  // els bots també hi juguen (proves)
			Hints  bool     `json:"hints"` // el format de decisió porta les ajudes de cada crida
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
				Entries: map[string][]arenaEntry{}, Groups: map[string][][]string{}, Hints: req.Hints}
			if c.Title == "" {
				c.Title = gameTr("Competició del ", "Competition of ") + time.Now().Format("2006-01-02")
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
				writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": gameTr("Cal que almenys dos jugadors tinguin un .md en algun joc.", "At least two players need a .md for some game.")})
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
			if c.Model == "" {
				c.Model = s.arenaModelRef() // tota la competició, amb el mateix model
			}
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
		"schedule": c.Schedule, "current": c.Current, "champion": c.Champion, "names": names, "titles": titles, "tables": tables, "entries": reveal, "hints": c.Hints}
}

// handleArenaStream: el que la retransmissió necessita (competició en marxa,
// partit actual i següent, classificacions).
func (s *Server) handleArenaStream(w http.ResponseWriter, r *http.Request) {
	s.voiceAutoInstall("en")
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
	// Sense competició (un partit sol, una demo) també cal el guió: diu si hi
	// ha veu i porta l'explicació de cada joc. Sense això la tele no parlava.
	noComp := func() {
		all := &arenaComp{Title: "Gregal Arena"}
		for _, g := range arena.Games() {
			all.Games = append(all.Games, g.ID())
		}
		writeJSON(w, map[string]any{"competition": nil, "script": s.streamScript(all)})
	}
	if cur == nil {
		noComp()
		return
	}
	arenaMu.Lock()
	c := s.arenaComp(cur.ID)
	arenaMu.Unlock()
	if c == nil {
		noComp()
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
