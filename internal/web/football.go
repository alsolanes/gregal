package web

// Partits de futbol americà entre els vents (internal/football). La lliga té
// dues franquícies fetes amb els vents de l'usuari i completades amb
// jugadors d'equip (no surten a la llista de vents). Un partit es juga quart
// a quart: abans de cada quart els dos entrenadors (vents) decideixen el pla
// amb el model, el motor juga el quart i un comentarista (un altre vent)
// comenta les jugades clau. El client el mira «en directe comprimit»: va
// demanant les jugades noves i les anima al seu ritme.
//
// Pressupost: 3 partits al dia; ~15-25 crides al model per partit.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/football"
)

const footballMaxGamesPerDay = 3

type fbTeamView struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Abbr    string            `json:"abbr"`
	Color   string            `json:"color"`
	Color2  string            `json:"color2"`
	Coach   string            `json:"coach"`
	Record  string            `json:"record"`
	Players []football.Player `json:"players"`
}

// fbMatch és un partit desat (football/games/<id>.json).
type fbMatch struct {
	ID       string                    `json:"id"`
	Status   string                    `json:"status"` // pregame, live, final, stopped, failed
	Started  int64                     `json:"started"`
	Finished int64                     `json:"finished,omitempty"`
	Teams    [2]fbTeamView             `json:"teams"`
	Plans    []fbPlanNote              `json:"plans"`
	Plays    []football.Play           `json:"plays"`
	Score    [2]int                    `json:"score"`
	Quarter  int                       `json:"quarter"`
	Summary  string                    `json:"summary,omitempty"`
	MVP      string                    `json:"mvp,omitempty"`
	Comment  string                    `json:"commentator,omitempty"`
	Stats    map[string]*football.Stat `json:"stats,omitempty"`
	Error    string                    `json:"error,omitempty"`
}

type fbPlanNote struct {
	Quarter int    `json:"q"`
	Team    int    `json:"team"`
	Coach   string `json:"coach"`
	Note    string `json:"note"`
}

type fbRun struct {
	mu     sync.Mutex
	m      *fbMatch
	cancel context.CancelFunc
}

var (
	fbMu   sync.Mutex
	fbRuns = map[string]*fbRun{} // partits en marxa, per id
)

func (s *Server) fbDir() string { return filepath.Join(s.windStateDir(), "football") }

func (s *Server) fbWinds() []football.Wind {
	ws, _ := s.winds.List()
	out := make([]football.Wind, 0, len(ws))
	for _, w := range ws {
		out = append(out, football.Wind{ID: w.ID, Name: w.Name, Role: w.Role, Specialty: w.Specialty, Color: w.Color, Style: w.Style})
	}
	return out
}

// Els jocs (la lliga dels vents i l'Arena) van sempre en anglès: textos del
// motor, narradors, informes i visors, sigui quina sigui la llengua de l'app.
func (s *Server) fbLangCode() string { return "en" }

// gameLang és el nom de la llengua dels jocs per als prompts del model.
func gameLang() string { return "English" }

// gameTr tria el text dels jocs (sempre l'anglès; el català queda escrit al
// costat per si mai es vol tornar a fer servir).
func gameTr(_, en string) string { return en }

func (s *Server) fbLoadLeague() *football.League {
	var lg football.League
	raw, err := os.ReadFile(filepath.Join(s.fbDir(), "league.json"))
	if err != nil || json.Unmarshal(raw, &lg) != nil {
		return nil
	}
	return &lg
}

func (s *Server) fbSaveLeague(lg *football.League) {
	_ = os.MkdirAll(s.fbDir(), 0o700)
	raw, _ := json.MarshalIndent(lg, "", " ")
	_ = os.WriteFile(filepath.Join(s.fbDir(), "league.json"), raw, 0o600)
}

// fbLeague torna la lliga, i la crea si encara no n'hi ha (calen 2 vents).
func (s *Server) fbLeague() (*football.League, error) {
	winds := s.fbWinds()
	if len(winds) < 2 {
		return nil, errConst(s.tr("Calen com a mínim dos vents: un entrenador per equip.", "At least two winds are needed: one coach per team."))
	}
	lg := s.fbLoadLeague()
	fresh := football.Ensure(lg, winds, s.fbLangCode(), time.Now().UnixNano()%100000)
	if fresh != lg {
		s.fbSaveLeague(fresh)
	}
	return fresh, nil
}

func fbTeamOf(t *football.Team) fbTeamView {
	return fbTeamView{ID: t.ID, Name: t.Name, Abbr: t.Abbr, Color: t.Color, Color2: t.Color2, Coach: t.Coach,
		Record: fmt.Sprintf("%d-%d%s", t.Wins, t.Losses, map[bool]string{true: fmt.Sprintf("-%d", t.Ties), false: ""}[t.Ties > 0]), Players: t.Players}
}

func (s *Server) fbSave(m *fbMatch) {
	dir := filepath.Join(s.fbDir(), "games")
	_ = os.MkdirAll(dir, 0o700)
	raw, _ := json.Marshal(m)
	_ = os.WriteFile(filepath.Join(dir, m.ID+".json"), raw, 0o600)
}

func (s *Server) fbLoad(id string) *fbMatch {
	if strings.ContainsAny(id, `/\.`) || id == "" {
		return nil
	}
	fbMu.Lock()
	r := fbRuns[id]
	fbMu.Unlock()
	if r != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		cp := *r.m
		cp.Plays = append([]football.Play(nil), r.m.Plays...)
		cp.Plans = append([]fbPlanNote(nil), r.m.Plans...)
		return &cp
	}
	raw, err := os.ReadFile(filepath.Join(s.fbDir(), "games", id+".json"))
	if err != nil {
		return nil
	}
	var m fbMatch
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return &m
}

func (s *Server) fbList() []fbMatch {
	matches, _ := filepath.Glob(filepath.Join(s.fbDir(), "games", "*.json"))
	var out []fbMatch
	for _, f := range matches {
		m := s.fbLoad(strings.TrimSuffix(filepath.Base(f), ".json"))
		if m == nil {
			continue
		}
		m.Plays, m.Stats = nil, nil
		for i := range m.Teams {
			m.Teams[i].Players = nil
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started > out[j].Started })
	return out
}

// ---- model ----

// footballModel és la crida al model (els tests la substitueixen).
var footballModel = func(s *Server, ctx context.Context, system, user string, maxTokens int) (string, error) {
	return cornerModel(s, ctx, system, user, maxTokens)
}

func (s *Server) fbRoster(t *football.Team) string {
	var b strings.Builder
	for _, p := range t.Players {
		if p.Pos == football.OL || p.Pos == football.DL {
			continue
		}
		fmt.Fprintf(&b, "%s #%d %s (%s, overall %d)\n", p.ID, p.Number, p.Name, p.Pos, p.Overall())
	}
	return b.String()
}

// fbCoachPlan: l'entrenador decideix el pla del quart.
func (s *Server) fbCoachPlan(ctx context.Context, coach companion.Wind, mine, theirs *football.Team, g *football.Game, team int, quarter int) football.Plan {
	lang := gameLang()
	system := companion.WindIdentity(coach) + coach.Persona + "\n\nYOU ARE THE HEAD COACH of " + mine.Name + " in an American football (NFL rules) game against " + theirs.Name + ". Reply with JSON only, no prose, no code fences."
	var sit string
	if quarter == 1 {
		sit = "It is the opening kickoff."
	} else {
		sit = fmt.Sprintf("Start of quarter %d. Score: %s %d - %s %d.", quarter, mine.Name, g.Score[team], theirs.Name, g.Score[1-team])
		if quarter >= 5 {
			sit = "OVERTIME. " + sit
		}
	}
	user := sit + "\n\nYOUR PLAYERS (skill positions):\n" + s.fbRoster(mine) + "\nTHEIR PLAYERS:\n" + s.fbRoster(theirs) +
		"\nSet your game plan for this quarter. pass_rate 0.3-0.8 (league average 0.58), deep_shots 0.05-0.4 (0.18), aggression 0-1 (going for it on 4th down and 2-point tries), blitz 0.05-0.6 (0.25), tempo 0-1 (no-huddle), focus = the id of one player to feature (or \"\"), note = one short coach line in " + lang + " for the broadcast, in your own voice.\n" +
		`JSON: {"pass_rate":0.58,"deep_shots":0.18,"aggression":0.35,"blitz":0.25,"tempo":0.3,"focus":"","note":"…"}`
	raw, err := footballModel(s, ctx, system, user, 600)
	p := football.DefaultPlan()
	if err != nil {
		return p
	}
	if obj := firstJSONObject(raw); obj != "" {
		_ = json.Unmarshal([]byte(obj), &p)
	}
	return p
}

// fbCommentary: el comentarista diu una frase per a cada jugada clau del quart.
func (s *Server) fbCommentary(ctx context.Context, caster companion.Wind, m *fbMatch, plays []football.Play) map[int]string {
	hl := football.Highlights(plays)
	if len(hl) == 0 {
		return nil
	}
	if len(hl) > 12 {
		hl = hl[len(hl)-12:]
	}
	var b strings.Builder
	for _, p := range hl {
		fmt.Fprintf(&b, "%d | Q%d %d:%02d | %s %d - %s %d | %s\n", p.N, p.Quarter, p.Clock/60, p.Clock%60, m.Teams[0].Abbr, p.Score[0], m.Teams[1].Abbr, p.Score[1], p.Text)
	}
	system := companion.WindIdentity(caster) + caster.Persona + "\n\nYOU ARE THE TV PLAY-BY-PLAY COMMENTATOR of an American football game between " + m.Teams[0].Name + " and " + m.Teams[1].Name + ". Reply with JSON only."
	user := "Key plays of this stretch (n | time | score after | what happened):\n" + b.String() +
		"\nWrite one exciting broadcast line for each play, in " + gameLang() + ", in your own voice, max 25 words, no hashtags. Never invent stats or names.\n" +
		`JSON: {"lines":[{"n":12,"text":"…"}]}`
	raw, err := footballModel(s, ctx, system, user, 1500)
	if err != nil {
		return nil
	}
	var out struct {
		Lines []struct {
			N    int    `json:"n"`
			Text string `json:"text"`
		} `json:"lines"`
	}
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &out) != nil {
		return nil
	}
	res := map[int]string{}
	for _, l := range out.Lines {
		res[l.N] = oneLine(l.Text, 220)
	}
	return res
}

// fbRecap: crònica final i el millor jugador.
func (s *Server) fbRecap(ctx context.Context, caster companion.Wind, m *fbMatch, g *football.Game) (string, string) {
	var b strings.Builder
	for t := 0; t < 2; t++ {
		l := g.BoxLeaders(t)
		for _, pid := range []string{l.Passer, l.Rusher, l.Receiver, l.Defender} {
			if pid == "" {
				continue
			}
			st := g.Stats[pid]
			fmt.Fprintf(&b, "%s (%s): pass %d/%d %d yds %d TD %d INT; rush %d-%d %d TD; rec %d-%d %d TD; tackles %d sacks %d INT %d\n",
				fbName(m, pid), m.Teams[t].Abbr, st.PassComp, st.PassAtt, st.PassYds, st.PassTD, st.Ints, st.RushAtt, st.RushYds, st.RushTD, st.Rec, st.RecYds, st.RecTD, st.Tackles, st.Sacks, st.DefInt)
		}
	}
	system := companion.WindIdentity(caster) + caster.Persona + "\n\nYOU ARE THE COMMENTATOR. Write the final recap of the game. Reply with JSON only."
	user := fmt.Sprintf("Final: %s %d - %s %d.\nStandout players:\n%s\nKey plays:\n", m.Teams[0].Name, g.Score[0], m.Teams[1].Name, g.Score[1], b.String())
	for _, p := range football.Highlights(g.Plays) {
		user += fmt.Sprintf("Q%d %s\n", p.Quarter, p.Text)
	}
	user += "\nrecap = 3-4 lively sentences in " + gameLang() + "; mvp = the id of the player of the game (from the list above).\n" + `JSON: {"recap":"…","mvp":"id"}`
	raw, err := footballModel(s, ctx, system, user, 900)
	if err != nil {
		return "", ""
	}
	var out struct {
		Recap string `json:"recap"`
		MVP   string `json:"mvp"`
	}
	if obj := firstJSONObject(raw); obj != "" {
		_ = json.Unmarshal([]byte(obj), &out)
	}
	return strings.TrimSpace(out.Recap), strings.TrimSpace(out.MVP)
}

func fbName(m *fbMatch, pid string) string {
	for _, t := range m.Teams {
		for _, p := range t.Players {
			if p.ID == pid {
				return p.Name
			}
		}
	}
	return pid
}

// ---- partit ----

func (s *Server) fbGamesToday() int {
	today := time.Now().Format("2006-01-02")
	n := 0
	for _, m := range s.fbList() {
		if time.Unix(m.Started, 0).Format("2006-01-02") == today {
			n++
		}
	}
	return n
}

// fbStart engega un partit nou en segon pla i en torna l'id.
func (s *Server) fbStart() (string, error) {
	fbMu.Lock()
	for _, r := range fbRuns {
		r.mu.Lock()
		running := r.m.Status == "pregame" || r.m.Status == "live"
		r.mu.Unlock()
		if running {
			fbMu.Unlock()
			return "", errConst(s.tr("Ja s'està jugant un partit.", "A game is already being played."))
		}
	}
	fbMu.Unlock()
	if s.fbGamesToday() >= footballMaxGamesPerDay {
		return "", errConst(s.tr("Avui ja s'han jugat prou partits (3 al dia).", "Enough games for today (3 a day)."))
	}
	lg, err := s.fbLeague()
	if err != nil {
		return "", err
	}
	now := time.Now()
	id := fmt.Sprintf("%s-%04x", now.Format("20060102-150405"), now.UnixNano()&0xffff)
	m := &fbMatch{ID: id, Status: "pregame", Started: time.Now().Unix(), Quarter: 1,
		Teams: [2]fbTeamView{fbTeamOf(lg.Teams[0]), fbTeamOf(lg.Teams[1])}}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	run := &fbRun{m: m, cancel: cancel}
	fbMu.Lock()
	fbRuns[id] = run
	fbMu.Unlock()
	s.fbSave(m)
	goBackground(func() { s.fbPlay(ctx, run, lg) })
	return id, nil
}

func (s *Server) fbPlay(ctx context.Context, run *fbRun, lg *football.League) {
	defer func() {
		run.cancel()
		run.mu.Lock()
		s.fbSave(run.m)
		run.mu.Unlock()
		fbMu.Lock()
		delete(fbRuns, run.m.ID)
		fbMu.Unlock()
	}()
	winds, _ := s.winds.List()
	coach := [2]companion.Wind{windByID(winds, lg.Teams[0].Coach), windByID(winds, lg.Teams[1].Coach)}
	caster := coach[0]
	for _, w := range winds {
		if w.ID != coach[0].ID && w.ID != coach[1].ID {
			caster = w
			break
		}
	}
	run.mu.Lock()
	run.m.Comment = caster.Name
	run.mu.Unlock()
	g := football.NewGame(lg.Teams[0], lg.Teams[1], time.Now().UnixNano(), s.fbLangCode())
	for !g.Finished() {
		if ctx.Err() != nil {
			s.fbEnd(run, "stopped", "")
			return
		}
		q := g.Quarter
		// Els dos entrenadors decideixen alhora.
		var wg sync.WaitGroup
		plans := [2]football.Plan{}
		for t := 0; t < 2; t++ {
			wg.Add(1)
			go func(t int) {
				defer wg.Done()
				plans[t] = s.fbCoachPlan(ctx, coach[t], lg.Teams[t], lg.Teams[1-t], g, t, q)
			}(t)
		}
		wg.Wait()
		if ctx.Err() != nil {
			s.fbEnd(run, "stopped", "")
			return
		}
		for t := 0; t < 2; t++ {
			g.SetPlan(t, plans[t])
		}
		run.mu.Lock()
		for t := 0; t < 2; t++ {
			if n := strings.TrimSpace(plans[t].Note); n != "" {
				run.m.Plans = append(run.m.Plans, fbPlanNote{Quarter: q, Team: t, Coach: coach[t].Name, Note: oneLine(n, 240)})
			}
		}
		run.mu.Unlock()
		plays := append([]football.Play(nil), g.SimQuarter()...)
		if lines := s.fbCommentary(ctx, caster, run.m, plays); lines != nil {
			for i := range plays {
				if c, ok := lines[plays[i].N]; ok {
					plays[i].Comment = c
				}
			}
		}
		run.mu.Lock()
		run.m.Status = "live"
		run.m.Plays = append(run.m.Plays, plays...)
		run.m.Score, run.m.Quarter = g.Score, g.Quarter
		s.fbSave(run.m)
		run.mu.Unlock()
	}
	recap, mvp := s.fbRecap(ctx, caster, run.m, g)
	// Rècord de la lliga.
	switch {
	case g.Score[0] > g.Score[1]:
		lg.Teams[0].Wins++
		lg.Teams[1].Losses++
	case g.Score[1] > g.Score[0]:
		lg.Teams[1].Wins++
		lg.Teams[0].Losses++
	default:
		lg.Teams[0].Ties++
		lg.Teams[1].Ties++
	}
	lg.Games++
	s.fbSaveLeague(lg)
	run.mu.Lock()
	run.m.Summary, run.m.MVP, run.m.Stats = recap, mvp, g.Stats
	run.m.Teams[0].Record, run.m.Teams[1].Record = fbTeamOf(lg.Teams[0]).Record, fbTeamOf(lg.Teams[1]).Record
	run.mu.Unlock()
	s.fbEnd(run, "final", "")
	win := 0
	if g.Score[1] > g.Score[0] {
		win = 1
	}
	text := s.tr(fmt.Sprintf("Final: %s %d - %s %d.", lg.Teams[0].Name, g.Score[0], lg.Teams[1].Name, g.Score[1]),
		fmt.Sprintf("Final: %s %d - %s %d.", lg.Teams[0].Name, g.Score[0], lg.Teams[1].Name, g.Score[1]))
	recordNovetat(s, coach[win], "football", text)
	log.Printf("football: %s %d - %d", run.m.ID, g.Score[0], g.Score[1])
}

func (s *Server) fbEnd(run *fbRun, status, why string) {
	run.mu.Lock()
	run.m.Status, run.m.Finished, run.m.Error = status, time.Now().Unix(), why
	s.fbSave(run.m)
	run.mu.Unlock()
}

// ---- API ----

// handleFootball: GET lliga i partits; POST {action: play|reset}.
func (s *Server) handleFootball(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Action string `json:"action"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		switch req.Action {
		case "play":
			id, err := s.fbStart()
			if err != nil {
				writeJSONStatus(w, http.StatusConflict, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]string{"id": id})
			return
		case "reset":
			_ = os.Remove(filepath.Join(s.fbDir(), "league.json"))
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
	}
	lg, err := s.fbLeague()
	out := map[string]any{"games": s.fbList(), "max_per_day": footballMaxGamesPerDay, "today": s.fbGamesToday()}
	if err != nil {
		out["error"] = err.Error()
	} else {
		out["teams"] = []fbTeamView{fbTeamOf(lg.Teams[0]), fbTeamOf(lg.Teams[1])}
	}
	writeJSON(w, out)
}

// handleFootballMatch: GET ?id=&after=N (jugades noves); POST ?id= {action: stop}.
func (s *Server) handleFootballMatch(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if r.Method == http.MethodPost {
		fbMu.Lock()
		run := fbRuns[id]
		fbMu.Unlock()
		if run != nil {
			run.cancel()
		}
		writeJSON(w, map[string]bool{"ok": run != nil})
		return
	}
	m := s.fbLoad(id)
	if m == nil {
		http.Error(w, "no such game", http.StatusNotFound)
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	var plays []football.Play
	for _, p := range m.Plays {
		if p.N > after {
			plays = append(plays, p)
		}
	}
	m.Plays = plays
	if after > 0 {
		// Els equips ja els té qui pregunta: només el que canvia.
		for i := range m.Teams {
			m.Teams[i].Players = nil
		}
	}
	writeJSON(w, m)
}
