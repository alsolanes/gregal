package web

// El Counter-Strike 2 en directe (internal/arena/cs2/live.go): la persona juga
// un jugador del seu equip amb teclat i ratolí a arena-cs2-live.html, els
// altres nou són bots que el navegador mou, i el model només fa el que fa a
// tots els partits: abans de cada ronda llegeix el .md de cada equip i tria
// la compra, el pla i (aquí) el rol de cada bot. El servidor guarda el
// partit (diners, rondes, decisions) i el desa com qualsevol altre, de mena
// «live»: surt a la llista de partits i es pot tornar a mirar al visor, però
// no compta per a l'Elo ni gasta partits de prova.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gregal/internal/arena"
	"gregal/internal/arena/cs2"
)

type arenaLiveRun struct {
	mu     sync.Mutex
	planMu sync.Mutex // un pla alhora (dos clics no fan dues crides al model)
	live   *cs2.Live
	m      *arenaMatch
	sits   []arenaSituation
	ctx    context.Context
	cancel context.CancelFunc
	seen   time.Time
}

var (
	arenaLivesMu sync.Mutex
	arenaLives   = map[string]*arenaLiveRun{}
)

// arenaLiveIdle: un partit en directe que ningú no toca en tanta estona s'atura.
const arenaLiveIdle = 30 * time.Minute

func (s *Server) arenaLiveGet(id string) *arenaLiveRun {
	arenaLivesMu.Lock()
	defer arenaLivesMu.Unlock()
	for k, r := range arenaLives {
		r.mu.Lock()
		idle := time.Since(r.seen) > arenaLiveIdle
		r.mu.Unlock()
		if idle && k != id {
			s.arenaLiveEnd(r, "stopped")
			delete(arenaLives, k)
		}
	}
	return arenaLives[id]
}

// arenaLiveEnd tanca el partit (final o aturat) i el desa. Amb r.mu lliure.
func (s *Server) arenaLiveEnd(r *arenaLiveRun, status string) {
	r.cancel()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m.Status == "final" || r.m.Status == "stopped" {
		return
	}
	r.m.Status, r.m.Finished = status, time.Now().Unix()
	arenaWrite(s.matchPath(r.m.ID), r.m)
	if len(r.sits) > 0 {
		arenaWrite(s.sitPath(r.m.ID), r.sits)
	}
}

type arenaLiveReq struct {
	Action   string          `json:"action"`
	ID       string          `json:"id"`
	MD       string          `json:"md"`
	Display  string          `json:"display"`
	Opponent string          `json:"opponent"`
	Format   string          `json:"format"` // short (MR6) | full (MR12)
	Slot     int             `json:"slot"`
	Roles    []string        `json:"roles"`
	Notes    []string        `json:"notes"`
	Result   json.RawMessage `json:"result"`
}

// handleArenaLive: POST {action: start|plan|result|stop|roles, …}; GET ?id= l'estat.
func (s *Server) handleArenaLive(w http.ResponseWriter, r *http.Request) {
	me, admin := s.arenaUser(r)
	if me == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	game, _ := arena.Get("cs2")
	if r.Method != http.MethodPost {
		run := s.arenaLiveGet(r.URL.Query().Get("id"))
		if run == nil {
			http.Error(w, "no such live match", http.StatusNotFound)
			return
		}
		run.mu.Lock()
		defer run.mu.Unlock()
		writeJSON(w, map[string]any{"id": run.m.ID, "meta": run.m.Meta, "names": run.m.Names, "score": run.m.Score, "status": run.m.Status,
			"human": run.live.HumanID(), "team": run.live.HumanTeam(), "setup": run.live.Setup()})
		return
	}
	var req arenaLiveReq
	// El resultat porta les pistes dels deu jugadors: més gran que una petició normal.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Action == "start" {
		s.arenaLiveStart(w, me, game, req)
		return
	}
	if req.Action == "roles" {
		s.arenaLiveRoles(w, r, me, game, req)
		return
	}
	run := s.arenaLiveGet(req.ID)
	if run == nil {
		http.Error(w, "no such live match", http.StatusNotFound)
		return
	}
	run.mu.Lock()
	run.seen = time.Now()
	own := run.m.Owners[0] == me
	run.mu.Unlock()
	if !own && !admin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	switch req.Action {
	case "plan":
		s.arenaLivePlan(w, run, game)
	case "result":
		var res cs2.LiveResult
		if json.Unmarshal(req.Result, &res) != nil {
			http.Error(w, "bad result", http.StatusBadRequest)
			return
		}
		run.mu.Lock()
		ev, err := run.live.Finish(res)
		if err != nil {
			run.mu.Unlock()
			writeJSONStatus(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		m := run.m
		m.Events = append(m.Events, ev...)
		mt := run.live.Match()
		m.Score = mt.Score()
		done := run.live.Done()
		if done {
			m.Winner = mt.Winner()
		}
		arenaWrite(s.matchPath(m.ID), m)
		arenaWrite(s.sitPath(m.ID), run.sits)
		score, winner := m.Score, m.Winner
		run.mu.Unlock()
		if done {
			s.arenaLiveEnd(run, "final")
			arenaLivesMu.Lock()
			delete(arenaLives, req.ID)
			arenaLivesMu.Unlock()
		}
		writeJSON(w, map[string]any{"score": score, "done": done, "winner": winner, "events": ev})
	case "stop":
		s.arenaLiveEnd(run, "stopped")
		arenaLivesMu.Lock()
		delete(arenaLives, req.ID)
		arenaLivesMu.Unlock()
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
}

func (s *Server) arenaLiveStart(w http.ResponseWriter, me string, game arena.Game, req arenaLiveReq) {
	mine := s.arenaEntry(game.ID(), me)
	md := strings.TrimSpace(req.MD)
	if md == "" && mine != nil {
		md = mine.MD
	}
	if md == "" {
		md = game.Starter()
	}
	if len(md) > arenaMaxMD {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("The .md is too long (max %d characters).", arenaMaxMD)})
		return
	}
	if mine == nil || mine.MD != md || (req.Display != "" && req.Display != mine.Display) {
		s.saveEntry(arenaEntry{Owner: me, Display: s.arenaDisplay(me, req.Display, mine), Game: game.ID(), MD: md})
		mine = s.arenaEntry(game.ID(), me)
	}
	// El rival: un bot o l'estratègia d'un altre jugador (el seu .md no es mostra).
	var opp *arenaEntry
	if req.Opponent != "" && req.Opponent != me {
		opp = s.arenaEntry(game.ID(), req.Opponent)
	}
	if opp == nil {
		opp = s.arenaNextBot(game, me)
	}
	if opp == nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "No opponent available."})
		return
	}
	half := 6
	if req.Format == "full" {
		half = 12
	}
	model := s.arenaModelRef()
	now := time.Now()
	id := fmt.Sprintf("cs2live-%s-%04x", now.Format("20060102-150405"), now.UnixNano()&0xffff)
	seed := int64(now.UnixNano() % 1000000)
	names := [2]string{mine.Display, opp.Display}
	live := cs2.NewLive(seed, s.fbLangCode(), names, half, 0, req.Slot)
	live.SetRoles(0, req.Roles, req.Notes)
	meta, _ := json.Marshal(live.Match().Meta())
	m := &arenaMatch{ID: id, Game: game.ID(), Kind: "live", Stage: "Live", Status: "live", Started: now.Unix(), Seed: seed, Winner: -1,
		Owners: [2]string{me, opp.Owner}, Names: names, Titles: [2]string{mdTitle(md), mdTitle(opp.MD)}, MD: [2]string{md, opp.MD}, Model: model, Meta: meta}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), arenaPinKey{}, model))
	run := &arenaLiveRun{live: live, m: m, ctx: ctx, cancel: cancel, seen: now}
	arenaLivesMu.Lock()
	arenaLives[id] = run
	arenaLivesMu.Unlock()
	arenaWrite(s.matchPath(id), m)
	writeJSON(w, map[string]any{"id": id, "meta": json.RawMessage(meta), "names": names, "human": live.HumanID(), "team": 0, "half": half})
}

// arenaLivePlan: el model decideix la ronda per als dos equips (alhora) i el
// motor la prepara. Si ja n'hi ha una de preparada, torna aquella.
func (s *Server) arenaLivePlan(w http.ResponseWriter, run *arenaLiveRun, game arena.Game) {
	run.planMu.Lock()
	defer run.planMu.Unlock()
	run.mu.Lock()
	if st := run.live.Setup(); st != nil {
		run.mu.Unlock()
		writeJSON(w, map[string]any{"setup": st, "decisions": s.arenaLiveDecs(run, st.Round)})
		return
	}
	if run.live.Done() {
		run.mu.Unlock()
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "match over"})
		return
	}
	p, _ := run.live.Prompts()
	md := run.m.MD
	run.mu.Unlock()
	ctx, cancel := context.WithTimeout(run.ctx, 2*time.Minute)
	defer cancel()
	var ans [2]arena.Answer
	var fb [2]bool
	var wg sync.WaitGroup
	for side := 0; side < 2; side++ {
		wg.Add(1)
		go func(side int) {
			defer wg.Done()
			ans[side], fb[side] = s.arenaDecide(ctx, game, md[side], p[side])
		}(side)
	}
	wg.Wait()
	if run.ctx.Err() != nil {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "stopped"})
		return
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	st, err := run.live.Start([2]json.RawMessage{ans[0].Decision, ans[1].Decision})
	if err != nil {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	for side := 0; side < 2; side++ {
		run.m.Decisions = append(run.m.Decisions, arenaDecision{N: st.Round, Side: side, Short: p[side].Short, Decision: ans[side].Decision,
			Why: oneLine(ans[side].Why, 240), Rule: oneLine(ans[side].Rule, 240), Fallback: fb[side], RuleFound: !fb[side] && ruleInMD(ans[side].Rule, md[side])})
		run.sits = append(run.sits, arenaSituation{N: st.Round, Side: side, Situation: clip(p[side].Situation, 4000), Schema: clip(p[side].Schema, 3000)})
	}
	writeJSON(w, map[string]any{"setup": st, "decisions": s.arenaLiveDecs(run, st.Round)})
}

// arenaLiveDecs: les decisions d'aquesta ronda que la persona pot veure: la
// del seu equip sencera; de la del rival, només el que se'n veu a la ronda.
func (s *Server) arenaLiveDecs(run *arenaLiveRun, round int) []map[string]any {
	var out []map[string]any
	for _, d := range run.m.Decisions {
		if d.N != round {
			continue
		}
		if d.Side == run.live.HumanTeam() {
			out = append(out, map[string]any{"side": d.Side, "decision": d.Decision, "why": d.Why, "rule": d.Rule, "fallback": d.Fallback})
		}
	}
	return out
}

// arenaLiveRoles: el model proposa un rol i una instrucció per a cada bot de
// l'equip, a partir del .md.
func (s *Server) arenaLiveRoles(w http.ResponseWriter, r *http.Request, me string, game arena.Game, req arenaLiveReq) {
	md := strings.TrimSpace(req.MD)
	if md == "" {
		if e := s.arenaEntry(game.ID(), me); e != nil {
			md = e.MD
		}
	}
	if md == "" {
		md = game.Starter()
	}
	slot := max(0, min(4, req.Slot))
	pos := []string{"IGL", "AWPer", "Entry", "Support", "Lurker"}
	system := "You are the head coach of a Counter-Strike 2 team in an AI strategy game. Your human player plays one position with keyboard and mouse; the other four are bots. Give each bot a role and one short instruction in the spirit of the team's STRATEGY. Reply with JSON only.\n\nRULES:\n" + game.Rules() + "\n\nTEAM STRATEGY:\n<<<\n" + clip(md, arenaMaxMD) + "\n>>>"
	user := fmt.Sprintf("The human plays position %d (%s). Bot roles: entry (first in, takes duels), support (follows the entry, flashes and trades), lurker (plays apart, flanks late; as CT holds a lonely spot), awper (holds long angles, buys the AWP), anchor (holds the site, does not rotate early), rifler (standard), follow (stays with the human player, covers them and trades their duels: use it when the strategy says to follow, stick with or cover the player). For each position 0-4 except the human's, one role and one instruction (max 15 words) that covers both T and CT.\n", slot, pos[slot]) +
		`JSON: {"players":[{"slot":0,"role":"rifler","note":"…"}]}`
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	raw, err := arenaModel(s, ctx, system, user, 700)
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	var out struct {
		Players []struct {
			Slot int    `json:"slot"`
			Role string `json:"role"`
			Note string `json:"note"`
		} `json:"players"`
	}
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &out) != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": "the coach did not answer in JSON"})
		return
	}
	roles, notes := make([]string, 5), make([]string, 5)
	for _, p := range out.Players {
		if p.Slot >= 0 && p.Slot < 5 && p.Slot != slot {
			roles[p.Slot], notes[p.Slot] = strings.ToLower(strings.TrimSpace(p.Role)), oneLine(p.Note, 140)
		}
	}
	writeJSON(w, map[string]any{"roles": roles, "notes": notes})
}
