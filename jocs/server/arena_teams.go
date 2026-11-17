package server

// Teams (departments, offices, practices) for a company competition, and the
// learning report for the organiser.
//
//   - The organiser lists the teams (settings.json {"teams": {"list": [...]}});
//     with no list the team mode is off and nothing about teams is shown.
//   - Each person picks their team from the list; the organiser can set
//     anyone's. Stored as settings.json {"teams": {"members": {owner: team}}}.
//   - Team standings rank by average XP (the same formula as the Prompt level,
//     jocs-levels.js), so a big team does not win by size alone.
//   - The report (GET /api/arena/report, organiser only) is a CSV with one row
//     per person: what they practised and how their prompting improved.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type arenaTeamsCfg struct {
	List    []string          `json:"list"`
	Members map[string]string `json:"members,omitempty"`
}

const (
	arenaMaxTeams    = 40
	arenaTeamNameMax = 40
)

func (s *Server) arenaTeams() arenaTeamsCfg {
	var st struct {
		Teams arenaTeamsCfg `json:"teams"`
	}
	arenaRead(filepath.Join(s.arenaDir(), "settings.json"), &st)
	if st.Teams.Members == nil {
		st.Teams.Members = map[string]string{}
	}
	if st.Teams.List == nil {
		st.Teams.List = []string{}
	}
	return st.Teams
}

func (s *Server) saveArenaTeams(c arenaTeamsCfg) {
	arenaMu.Lock()
	defer arenaMu.Unlock()
	path := filepath.Join(s.arenaDir(), "settings.json")
	raw := map[string]json.RawMessage{}
	arenaRead(path, &raw)
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(c)
	raw["teams"] = b
	arenaWrite(path, raw)
}

// cleanTeamList: trimmed, clipped, no duplicates (case-insensitive), in order.
func cleanTeamList(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(t)
		if r := []rune(t); len(r) > arenaTeamNameMax {
			t = string(r[:arenaTeamNameMax])
		}
		k := strings.ToLower(t)
		if t == "" || seen[k] || len(out) >= arenaMaxTeams {
			continue
		}
		seen[k] = true
		out = append(out, t)
	}
	return out
}

// setArenaTeamList replaces the list; members of a removed team lose it.
func (s *Server) setArenaTeamList(list []string) arenaTeamsCfg {
	c := s.arenaTeams()
	c.List = cleanTeamList(list)
	ok := map[string]bool{}
	for _, t := range c.List {
		ok[t] = true
	}
	for o, t := range c.Members {
		if !ok[t] {
			delete(c.Members, o)
		}
	}
	s.saveArenaTeams(c)
	return c
}

// setArenaTeam puts owner in team ("" = no team). The team must be listed.
func (s *Server) setArenaTeam(owner, team string) error {
	c := s.arenaTeams()
	team = strings.TrimSpace(team)
	if team != "" {
		found := false
		for _, t := range c.List {
			if t == team {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unknown team")
		}
		c.Members[owner] = team
	} else {
		delete(c.Members, owner)
	}
	s.saveArenaTeams(c)
	return nil
}

// arenaXP: the same as xpOf in jocs-levels.js.
func arenaXP(p *arenaPlayer) int {
	if p == nil {
		return 0
	}
	xp := float64(p.Reviewed*10 + p.BotsBeaten*20 + p.Official[0]*30)
	if p.Reviewed > 0 {
		xp += float64(p.Prompt) * 2
	}
	return int(math.Round(xp))
}

type arenaTeamRow struct {
	Name        string  `json:"name"`
	Rank        int     `json:"rank"`
	Members     int     `json:"members"`
	Active      int     `json:"active"` // members who played at least once
	XP          int     `json:"xp"`     // average per member
	Prompt      int     `json:"prompt_score"`
	Improvement float64 `json:"improvement"` // average coach gain, 0-100 points
	VsVague     arenaWL `json:"vs_vague"`
	Official    int     `json:"official_wins"`
	BotsBeaten  int     `json:"bots_beaten"`
	Top         string  `json:"top,omitempty"` // display name of the best member
}

// arenaTeamTable sets each player's team and returns the team standings.
func arenaTeamTable(players []*arenaPlayer, c arenaTeamsCfg) []arenaTeamRow {
	if len(c.List) == 0 {
		return nil
	}
	rows := map[string]*arenaTeamRow{}
	order := []*arenaTeamRow{}
	for _, t := range c.List {
		r := &arenaTeamRow{Name: t}
		rows[t] = r
		order = append(order, r)
	}
	type acc struct {
		xp, prompt, gain float64
		promptN, gainN   int
		bestXP           int
		best             string
	}
	accs := map[string]*acc{}
	for _, p := range players {
		t := c.Members[p.Owner]
		r := rows[t]
		if r == nil {
			continue
		}
		p.Team = t
		a := accs[t]
		if a == nil {
			a = &acc{bestXP: -1}
			accs[t] = a
		}
		r.Members++
		if p.Played > 0 {
			r.Active++
		}
		xp := arenaXP(p)
		a.xp += float64(xp)
		if xp > a.bestXP {
			a.bestXP, a.best = xp, p.Display
		}
		if p.Reviewed > 0 {
			a.prompt += float64(p.Prompt)
			a.promptN++
		}
		if p.Reviewed >= 2 {
			a.gain += (p.CoachLast - p.CoachFirst) * 4
			a.gainN++
		}
		if wl := p.VsBots["rookie"]; wl != nil {
			r.VsVague.Played += wl.Played
			r.VsVague.Won += wl.Won
		}
		r.Official += p.Official[0]
		r.BotsBeaten += p.BotsBeaten
	}
	for t, a := range accs {
		r := rows[t]
		r.XP = int(math.Round(a.xp / float64(r.Members)))
		if a.promptN > 0 {
			r.Prompt = int(math.Round(a.prompt / float64(a.promptN)))
		}
		if a.gainN > 0 {
			r.Improvement = math.Round(10*a.gain/float64(a.gainN)) / 10
		}
		if a.bestXP > 0 {
			r.Top = a.best
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if a.XP != b.XP {
			return a.XP > b.XP
		}
		if a.Prompt != b.Prompt {
			return a.Prompt > b.Prompt
		}
		return a.Active > b.Active
	})
	out := make([]arenaTeamRow, len(order))
	for i, r := range order {
		r.Rank = i + 1
		out[i] = *r
	}
	return out
}

// ---- the learning report ----

var arenaReportHeader = []string{
	"player", "account", "team", "xp", "matches", "won", "lost", "drawn",
	"coach_reviews", "prompt_score", "first_coach", "latest_coach", "coach_gain",
	"clarity", "specificity", "coverage", "adaptivity", "consistency",
	"vs_vague_won", "vs_vague_played", "bots_beaten", "official_won", "official_lost",
	"md_versions", "first_version_win_rate", "latest_version_win_rate", "last_active",
}

func arenaReportRows(players []*arenaPlayer) [][]string {
	rows := [][]string{arenaReportHeader}
	f1 := func(v float64) string { return fmt.Sprintf("%.1f", v) }
	rate := func(v arenaMDVersion) string {
		if v.Played == 0 {
			return ""
		}
		return fmt.Sprintf("%.0f", 100*float64(v.Won)/float64(v.Played))
	}
	for _, p := range players {
		first, last, gain := "", "", ""
		if p.Reviewed > 0 {
			first, last = f1(p.CoachFirst*4), f1(p.CoachLast*4)
			if p.Reviewed >= 2 {
				gain = f1((p.CoachLast - p.CoachFirst) * 4)
			}
		}
		pr := func(k string) string {
			if p.Principles == nil {
				return ""
			}
			return f1(p.Principles[k])
		}
		vv := arenaWL{}
		if wl := p.VsBots["rookie"]; wl != nil {
			vv = *wl
		}
		// The game where the person iterated most: their before/after.
		var vs []arenaMDVersion
		for _, g := range p.Games {
			if len(g.Versions) > len(vs) {
				vs = g.Versions
			}
		}
		nv, fr, lr := "0", "", ""
		if len(vs) > 0 {
			nv, fr, lr = fmt.Sprint(vs[len(vs)-1].N), rate(vs[0]), rate(vs[len(vs)-1])
		}
		lastActive := ""
		if p.Last > 0 {
			lastActive = time.Unix(p.Last, 0).UTC().Format("2006-01-02")
		}
		prompt := ""
		if p.Reviewed > 0 {
			prompt = fmt.Sprint(p.Prompt)
		}
		rows = append(rows, []string{
			p.Display, p.Owner, p.Team, fmt.Sprint(arenaXP(p)), fmt.Sprint(p.Played), fmt.Sprint(p.Won), fmt.Sprint(p.Lost), fmt.Sprint(p.Drawn),
			fmt.Sprint(p.Reviewed), prompt, first, last, gain,
			pr("clarity"), pr("specificity"), pr("coverage"), pr("adaptivity"), pr("consistency"),
			fmt.Sprint(vv.Won), fmt.Sprint(vv.Played), fmt.Sprint(p.BotsBeaten), fmt.Sprint(p.Official[0]), fmt.Sprint(p.Official[1]),
			nv, fr, lr, lastActive,
		})
	}
	return rows
}

// csvSafe keeps a spreadsheet from running a cell as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		if _, err := fmt.Sscanf(v, "%g", new(float64)); err != nil {
			return "'" + v
		}
	}
	return v
}

// handleArenaReport: GET → the learning report as CSV (organiser only).
func (s *Server) handleArenaReport(w http.ResponseWriter, r *http.Request) {
	if _, admin := s.arenaUser(r); !admin || isSpectator(r) {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	v := s.arenaSeasonFor("")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="prompt-cup-report-%s.csv"`, time.Now().Format("2006-01-02")))
	cw := csv.NewWriter(w)
	for _, row := range arenaReportRows(v.Players) {
		for i := range row {
			row[i] = csvSafe(row[i])
		}
		_ = cw.Write(row)
	}
	cw.Flush()
}
