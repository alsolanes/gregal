package web

// Gregal Wrapped: resum setmanal o mensual de l'ús, calculat només amb el
// que ja hi ha al disc (registre durable d'events i converses desades). No
// s'inventa res: el que no es pot saber (tokens sense events d'ús, cost
// sense preu) es diu com a desconegut.

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/session"
)

type statsCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type statsDay struct {
	Date  string `json:"date"` // AAAA-MM-DD, hora local
	Turns int    `json:"turns"`
	Tools int    `json:"tools"`
}

type statsTokens struct {
	Known     bool     `json:"known"`
	Up        int      `json:"up"`
	Down      int      `json:"down"`
	Estimated bool     `json:"estimated"`
	CostUSD   *float64 `json:"cost_usd"`
}

type statsStubborn struct {
	RunID    int       `json:"run_id"`
	Session  string    `json:"session"`
	Command  string    `json:"command"`
	Failures int       `json:"failures"`
	At       time.Time `json:"at"`
}

type statsReport struct {
	Range              string         `json:"range"`
	From               time.Time      `json:"from"`
	To                 time.Time      `json:"to"`
	Sessions           int            `json:"sessions"`
	SavedSessions      int            `json:"saved_sessions"`
	Turns              int            `json:"turns"`
	ToolCalls          int            `json:"tool_calls"`
	ToolsByKind        map[string]int `json:"tools_by_kind"`
	TopTools           []statsCount   `json:"top_tools"`
	FilesChanged       int            `json:"files_changed"`
	TopFiles           []statsCount   `json:"top_files"`
	CheckRuns          int            `json:"check_runs"`
	ChecksFailed       int            `json:"checks_failed"`
	TestsTurnedGreen   int            `json:"tests_turned_green"`
	StubbornBug        *statsStubborn `json:"stubborn_bug"`
	Tokens             statsTokens    `json:"tokens"`
	FavoriteModel      string         `json:"favorite_model"`
	FavoriteModelTurns int            `json:"favorite_model_turns"`
	ActiveDays         int            `json:"active_days"`
	LongestStreak      int            `json:"longest_streak_days"`
	CurrentStreak      int            `json:"current_streak_days"`
	Days               []statsDay     `json:"days"`
}

// statsToolKind agrupa les eines per família: és el que es pot llegir
// d'un cop d'ull («molt de shell, poca edició»).
func statsToolKind(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "read" || n == "read_image" || n == "glob" || n == "todoread" || strings.HasPrefix(n, "office_read") || n == "office_open":
		return "read"
	case n == "grep" || n == "symbols" || n == "search" || n == "repomap":
		return "search"
	case n == "write" || n == "edit" || n == "patch" || n == "multi_edit" || n == "todowrite" || n == "office_edit" || n == "office_create":
		return "edit"
	case strings.HasPrefix(n, "bash") || n == "exec" || n == "git_commit":
		return "shell"
	case strings.HasPrefix(n, "web_") || n == "browser" || n == "preview" || strings.HasPrefix(n, "gh_"):
		return "web"
	case n == "delegate" || n == "skill":
		return "delegate"
	case strings.HasPrefix(n, "mcp_"):
		return "mcp"
	}
	return "other"
}

// statsRange torna l'inici (mitjanit local) i el nombre de dies del rang.
func statsRange(rng string, now time.Time) (time.Time, int) {
	days := 7
	if rng == "month" {
		days = 30
	}
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	return midnight.AddDate(0, 0, -(days - 1)), days
}

func statsNormCmd(cmd string) string {
	return strings.ToLower(strings.Join(strings.Fields(cmd), " "))
}

func statsTop(m map[string]int, n int) []statsCount {
	out := make([]statsCount, 0, len(m))
	for k, v := range m {
		out = append(out, statsCount{Name: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// statsStreaks calcula la ratxa més llarga i l'actual (acabant avui o
// ahir: un dia sense obrir-lo encara no l'ha trencada) sobre tots els dies.
func statsStreaks(days map[string]bool, now time.Time) (longest, current int) {
	if len(days) == 0 {
		return 0, 0
	}
	keys := make([]string, 0, len(days))
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	run := 0
	var prev time.Time
	for i, k := range keys {
		d, err := time.ParseInLocation("2006-01-02", k, now.Location())
		if err != nil {
			continue
		}
		if i > 0 && d.Sub(prev) <= 25*time.Hour && d.After(prev) {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
		prev = d
	}
	y, m, dd := now.Date()
	day := time.Date(y, m, dd, 0, 0, 0, 0, now.Location())
	if !days[day.Format("2006-01-02")] {
		day = day.AddDate(0, 0, -1)
	}
	for days[day.Format("2006-01-02")] {
		current++
		day = day.AddDate(0, 0, -1)
	}
	return longest, current
}

// computeStats és la part pura de /api/stats: rep tots els events de
// l'usuari (de totes les converses, en ordre d'id) i les converses desades.
func computeStats(evs []events.Event, saved []session.Info, rng string, now time.Time) statsReport {
	if rng != "month" {
		rng = "week"
	}
	from, nDays := statsRange(rng, now)
	rep := statsReport{Range: rng, From: from, To: now, ToolsByKind: map[string]int{}}
	loc := now.Location()
	dayKey := func(t time.Time) string { return t.In(loc).Format("2006-01-02") }
	perDay := map[string]*statsDay{}
	for i := 0; i < nDays; i++ {
		k := from.AddDate(0, 0, i).Format("2006-01-02")
		perDay[k] = &statsDay{Date: k}
		rep.Days = append(rep.Days, statsDay{Date: k})
	}
	inRange := func(t time.Time) bool { return !t.Before(from) && !t.After(now) }

	allDays := map[string]bool{}
	sessions := map[string]bool{}
	turns := map[[2]any]bool{}
	tools := map[string]int{}
	files := map[string]int{}
	models := map[string]int{}
	delegateModels := map[string]int{}
	type pending struct {
		name, cmd, session string
		paths              []string
		run                int
	}
	calls := map[string]pending{}
	// Seqüència de checks per conversa i ordre: verd = passa després d'un
	// o més errors. lastFail compta els errors pendents de cada ordre.
	failStreak := map[string]int{}
	type runCmd struct {
		run int
		cmd string
	}
	runFails := map[runCmd]int{}
	check := func(e events.Event, cmd string, ok bool) {
		key := e.SessionID + "\x00" + statsNormCmd(cmd)
		counted := inRange(e.At)
		if counted {
			rep.CheckRuns++
		}
		if !ok {
			failStreak[key]++
			if counted {
				rep.ChecksFailed++
			}
			if e.RunID > 0 {
				runFails[runCmd{e.RunID, key}]++
			}
			return
		}
		if failStreak[key] > 0 && counted {
			rep.TestsTurnedGreen++
			if n := runFails[runCmd{e.RunID, key}]; e.RunID > 0 && n > 0 && (rep.StubbornBug == nil || n > rep.StubbornBug.Failures) {
				rep.StubbornBug = &statsStubborn{RunID: e.RunID, Session: e.SessionID, Command: strings.Join(strings.Fields(cmd), " "), Failures: n, At: e.At}
			}
		}
		failStreak[key] = 0
		delete(runFails, runCmd{e.RunID, key})
	}
	var up, down int
	var cost float64
	costKnown, usageSeen := false, false

	for _, e := range evs {
		if strings.HasPrefix(e.Kind, "retro_") {
			continue
		}
		if !e.At.IsZero() {
			allDays[dayKey(e.At)] = true
		}
		counted := inRange(e.At)
		if counted && e.SessionID != "" {
			sessions[e.SessionID] = true
		}
		var p map[string]json.RawMessage
		if len(e.Payload) > 0 {
			_ = json.Unmarshal(e.Payload, &p)
		}
		str := func(k string) string {
			var v string
			if raw, ok := p[k]; ok {
				_ = json.Unmarshal(raw, &v)
			}
			return v
		}
		switch e.Kind {
		case "run_started":
			if counted && e.RunID > 0 {
				k := [2]any{e.SessionID, e.RunID}
				if !turns[k] {
					turns[k] = true
					if d := perDay[dayKey(e.At)]; d != nil {
						d.Turns++
					}
				}
			}
		case "tool_call":
			name := str("name")
			if name == "" {
				continue
			}
			if counted {
				rep.ToolCalls++
				tools[name]++
				rep.ToolsByKind[statsToolKind(name)]++
				if d := perDay[dayKey(e.At)]; d != nil {
					d.Tools++
				}
			}
			args := retroArgs(p["args"])
			pc := pending{name: strings.ToLower(name), session: e.SessionID, run: e.RunID}
			switch pc.name {
			case "write", "edit", "patch", "multi_edit":
				if path := retroArg(args, "path", "file", "file_path"); path != "" {
					pc.paths = append(pc.paths, path)
				}
				if list, ok := args["paths"].([]any); ok {
					for _, x := range list {
						if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
							pc.paths = append(pc.paths, strings.TrimSpace(s))
						}
					}
				}
			case "bash", "bash_background":
				pc.cmd = retroArg(args, "command", "cmd")
			}
			if id := str("id"); id != "" {
				calls[e.SessionID+"\x00"+id] = pc
			}
		case "tool_result":
			id := str("id")
			pc, ok := calls[e.SessionID+"\x00"+id]
			if !ok {
				continue
			}
			delete(calls, e.SessionID+"\x00"+id)
			okRes := retroToolOK(str("output"))
			if okRes && counted {
				for _, path := range pc.paths {
					files[strings.ReplaceAll(path, "\\", "/")]++
				}
			}
			if pc.cmd != "" && retroIsTest(pc.cmd) {
				check(e, pc.cmd, okRes)
			}
		case "autonomous_checkpoint":
			var list []struct {
				Command string   `json:"command"`
				Code    *float64 `json:"code"`
			}
			if raw, ok := p["checks"]; ok {
				_ = json.Unmarshal(raw, &list)
			}
			for _, c := range list {
				if strings.TrimSpace(c.Command) == "" || c.Code == nil {
					continue
				}
				check(e, c.Command, *c.Code == 0)
			}
		case "usage":
			if !counted {
				continue
			}
			usageSeen = true
			if m := str("model"); m != "" {
				models[m]++
			}
			var u struct {
				Up   int      `json:"up_tokens"`
				Down int      `json:"down_tokens"`
				Cost *float64 `json:"cost_usd"`
			}
			_ = json.Unmarshal(e.Payload, &u)
			up += u.Up
			down += u.Down
			if u.Cost != nil {
				cost += *u.Cost
				costKnown = true
			}
		case "delegate":
			if m := str("model"); m != "" && counted && str("status") == "completed" {
				delegateModels[m]++
			}
		}
	}

	rep.Sessions = len(sessions)
	rep.Turns = len(turns)
	for _, s := range saved {
		if inRange(s.SavedAt) {
			rep.SavedSessions++
		}
	}
	rep.TopTools = statsTop(tools, 6)
	rep.FilesChanged = len(files)
	rep.TopFiles = statsTop(files, 5)
	if usageSeen {
		rep.Tokens = statsTokens{Known: true, Up: up, Down: down, Estimated: true}
		if costKnown {
			c := cost
			rep.Tokens.CostUSD = &c
		}
	}
	fav := models
	if len(fav) == 0 {
		fav = delegateModels
	}
	if top := statsTop(fav, 1); len(top) == 1 {
		rep.FavoriteModel, rep.FavoriteModelTurns = top[0].Name, top[0].Count
	}
	for i := range rep.Days {
		if d := perDay[rep.Days[i].Date]; d != nil {
			rep.Days[i] = *d
		}
		if allDays[rep.Days[i].Date] {
			rep.ActiveDays++
		}
	}
	rep.LongestStreak, rep.CurrentStreak = statsStreaks(allDays, now)
	return rep
}

// statsUserEvents llegeix tots els events durables de l'usuari (totes les
// converses). Topall generós però finit: un registre enorme no penja la
// petició.
func (s *Server) statsUserEvents() []events.Event {
	h := s.Hub()
	if h == nil || h.eventStore == nil {
		return nil
	}
	var out []events.Event
	after := uint64(0)
	for i := 0; i < 400; i++ {
		got, err := h.eventStore.AfterUser(after, 500, s.usuari())
		if err != nil || len(got) == 0 {
			break
		}
		out = append(out, got...)
		after = got[len(got)-1].ID
		if len(got) < 500 {
			break
		}
	}
	return out
}

// handleStats: GET /api/stats?range=week|month.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "només GET", http.StatusMethodNotAllowed)
		return
	}
	rng := strings.TrimSpace(r.URL.Query().Get("range"))
	if rng == "" {
		rng = "week"
	}
	if rng != "week" && rng != "month" {
		http.Error(w, "range ha de ser week o month", http.StatusBadRequest)
		return
	}
	saved, _ := session.List(s.sessDir())
	writeJSON(w, computeStats(s.statsUserEvents(), saved, rng, time.Now()))
}

// recordUsage deixa al registre durable què ha gastat el torn (tokens
// estimats amb agent.HistCost i, si el config té preu, el cost). És la
// font de «tokens», «cost» i «model preferit» de /api/stats.
func (s *Server) recordUsage(role config.Role, up, down int, usd float64, priced bool) {
	if s == nil || s.hub == nil || (up == 0 && down == 0) {
		return
	}
	p := map[string]any{"model": role.Model, "provider": role.Provider, "up_tokens": up, "down_tokens": down, "estimated": true}
	if priced {
		p["cost_usd"] = usd
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	s.mu.Lock()
	runID := 0
	if s.active != nil {
		runID = s.active.ID
	}
	s.mu.Unlock()
	_ = s.hub.appendEvent(events.Event{SessionID: s.id, User: s.usuari(), RunID: runID, Kind: "usage", Text: role.Model, Payload: raw})
}
