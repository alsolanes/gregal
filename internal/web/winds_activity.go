package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

// L'activitat dels vents en una sola llista: el que han fet en segon pla
// (feines delegades, novetats, runs de rutines, cors i recordatoris que han
// sonat), de la més nova a la més vella. No desa res de nou: llegeix les
// mateixes fonts que /api/winds/tasks, /novetats, /rutines/runs i
// /programats, amb les mateixes regles de qui pot veure què.
//
//	GET /api/winds/activity[?wind=<id>][&before=<ts>][&limit=N] → {items:[…]}

const (
	activityDefaultLimit = 50
	activityMaxLimit     = 200
)

// ActivityItem és una cosa que un vent ha fet.
type ActivityItem struct {
	At     time.Time `json:"at"`
	Wind   string    `json:"wind"`
	Kind   string    `json:"kind"` // task | news | routine | heartbeat | reminder
	Title  string    `json:"title"`
	Status string    `json:"status"`
	Ref    string    `json:"ref"`
}

// parseActivityBefore accepta l'«at» d'un element (RFC 3339) o segons unix.
func parseActivityBefore(raw string) (time.Time, error) {
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(n, 0), nil
	}
	return time.Parse(time.RFC3339Nano, raw)
}

func activityLimit(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return activityDefaultLimit
	}
	return min(n, activityMaxLimit)
}

func (s *Server) handleWindsActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	var before time.Time
	if raw := strings.TrimSpace(q.Get("before")); raw != "" {
		t, err := parseActivityBefore(raw)
		if err != nil {
			http.Error(w, "bad before", http.StatusBadRequest)
			return
		}
		before = t
	}
	items := []ActivityItem{}
	if s.winds != nil {
		list, err := s.winds.List()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		only := strings.TrimSpace(q.Get("wind"))
		byID := map[string]companion.Wind{}
		for _, wd := range list {
			if only == "" || wd.ID == only {
				byID[wd.ID] = wd
			}
		}
		if items, err = s.collectActivity(byID, only); err != nil {
			http.Error(w, "could not read activity", 500)
			return
		}
	}
	sort.SliceStable(items, func(a, b int) bool {
		if !items[a].At.Equal(items[b].At) {
			return items[a].At.After(items[b].At)
		}
		return items[a].Ref < items[b].Ref
	})
	out := []ActivityItem{}
	limit := activityLimit(q.Get("limit"))
	for _, it := range items {
		if len(out) == limit {
			break
		}
		if before.IsZero() || it.At.Before(before) {
			out = append(out, it)
		}
	}
	writeJSON(w, map[string]any{"items": out})
}

// collectActivity ajunta les quatre fonts per als vents de winds.
func (s *Server) collectActivity(winds map[string]companion.Wind, only string) ([]ActivityItem, error) {
	items, err := s.taskActivity(winds, only)
	if err != nil {
		return nil, err
	}
	items = append(items, s.newsActivity(winds)...)
	if s.jobs == nil {
		return items, nil
	}
	runs, err := s.runActivity(winds)
	if err != nil {
		return nil, err
	}
	jobItems, err := s.jobActivity(winds)
	if err != nil {
		return nil, err
	}
	return append(append(items, runs...), jobItems...), nil
}

// taskActivity: les feines delegades. Una feina no porta vent, sinó els noms
// dels seients (Agents); és del primer seient que és un vent d'aquest compte,
// o del vent demanat si hi treballa.
func (s *Server) taskActivity(winds map[string]companion.Wind, only string) ([]ActivityItem, error) {
	records, err := s.windTaskRecords()
	if err != nil {
		return nil, err
	}
	byName := map[string]string{}
	for id, wd := range winds {
		byName[wd.Name] = id
	}
	var items []ActivityItem
	for _, sum := range s.windTaskSummaries(records) {
		seats := make([]string, 0, len(sum.Agents))
		for seat := range sum.Agents {
			seats = append(seats, seat)
		}
		sort.Strings(seats)
		wind := ""
		for _, seat := range seats {
			if id, ok := byName[sum.Agents[seat]]; ok && (wind == "" || id == only) {
				wind = id
			}
		}
		if wind == "" {
			continue
		}
		items = append(items, ActivityItem{At: sum.Created, Wind: wind, Kind: "task",
			Title: oneLine(sum.Task, 160), Status: sum.Status, Ref: sum.ID})
	}
	return items, nil
}

// newsActivity: el que els vents han dit sols, amb la mateixa regla de
// propietari que /api/winds/novetats.
func (s *Server) newsActivity(winds map[string]companion.Wind) []ActivityItem {
	novetatsMu.Lock()
	all := visibleNovetats(s, loadNovetats(s))
	novetatsMu.Unlock()
	var items []ActivityItem
	for _, n := range all {
		if _, ok := winds[n.Wind]; !ok {
			continue
		}
		status := "unread"
		if n.Read {
			status = "read"
		}
		items = append(items, ActivityItem{At: time.Unix(n.Ts, 0), Wind: n.Wind, Kind: "news",
			Title: oneLine(n.Text, 160), Status: status, Ref: n.ID})
	}
	return items
}

// runActivity: els runs de les rutines (els cors i els recordatoris surten
// de jobActivity, un per job, com a /api/winds/programats).
func (s *Server) runActivity(winds map[string]companion.Wind) ([]ActivityItem, error) {
	runs, err := s.jobs.Runs("", 0)
	if err != nil {
		return nil, err
	}
	var items []ActivityItem
	for _, run := range runs {
		if run.Wind == "" || strings.HasPrefix(run.JobID, windJobPrefix) || strings.HasPrefix(run.JobID, remindJobPrefix) {
			continue
		}
		if _, ok := winds[run.Wind]; !ok || !s.canSeeWindRoutine(jobs.Job{Wind: run.Wind, WindOwner: run.WindOwner}) {
			continue
		}
		at := run.FinishedAt
		if at.IsZero() {
			at = run.StartedAt
		}
		items = append(items, ActivityItem{At: at, Wind: run.Wind, Kind: "routine",
			Title: strings.TrimPrefix(strings.TrimSpace(run.JobName), "Responsabilitat · "), Status: run.Status, Ref: run.JobID})
	}
	return items, nil
}

// jobActivity: l'últim cop que ha sonat cada cor i cada recordatori.
func (s *Server) jobActivity(winds map[string]companion.Wind) ([]ActivityItem, error) {
	list, err := s.jobs.List()
	if err != nil {
		return nil, err
	}
	var items []ActivityItem
	for _, j := range list {
		if j.LastAt == nil || !s.canSeeWindJob(j) {
			continue
		}
		wd, ok := winds[automationWind(j)]
		if !ok {
			continue
		}
		it := ActivityItem{At: *j.LastAt, Wind: wd.ID, Status: j.LastStatus, Ref: j.ID}
		switch {
		case strings.HasPrefix(j.ID, remindJobPrefix):
			it.Kind = "reminder"
			it.Title = oneLine(firstNonEmpty(strings.TrimSpace(j.Task), strings.TrimSpace(j.Prompt)), 160)
		case j.Kind == "event" && strings.HasPrefix(j.ID, windJobPrefix):
			it.Kind = "heartbeat"
			it.Title = firstNonEmpty(strings.TrimSpace(wd.Role), wd.Name)
		default:
			continue
		}
		items = append(items, it)
	}
	return items, nil
}
