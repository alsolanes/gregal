package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gregal/internal/events"
	"gregal/internal/session"
)

func statsEv(t *testing.T, at time.Time, sess string, run int, kind string, payload any) events.Event {
	t.Helper()
	e := events.Event{At: at, SessionID: sess, RunID: run, Kind: kind}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		e.Payload = raw
	}
	return e
}

func TestComputeStatsAggregates(t *testing.T) {
	loc := time.FixedZone("x", 2*3600)
	now := time.Date(2026, 10, 13, 18, 0, 0, 0, loc)
	day := func(back int, h int) time.Time { return time.Date(2026, 10, 13-back, h, 0, 0, 0, loc) }
	call := func(at time.Time, sess string, run int, id, name, args string) events.Event {
		return statsEv(t, at, sess, run, "tool_call", map[string]string{"id": id, "name": name, "args": args})
	}
	res := func(at time.Time, sess string, run int, id, name, out string) events.Event {
		return statsEv(t, at, sess, run, "tool_result", map[string]string{"id": id, "name": name, "output": out})
	}
	evs := []events.Event{
		// Fora del rang setmanal (fa 20 dies): compta per a les ratxes i el mes.
		statsEv(t, day(20, 10), "old", 1, "run_started", nil),
		// Run 2 (fa 2 dies): edita, falla dues vegades els tests i acaba verd.
		statsEv(t, day(2, 9), "s1", 2, "run_started", nil),
		call(day(2, 9), "s1", 2, "c1", "read", `{"path":"a.go"}`),
		res(day(2, 9), "s1", 2, "c1", "read", "package a"),
		call(day(2, 9), "s1", 2, "c2", "edit", `{"path":"a.go","old":"x","new":"y"}`),
		res(day(2, 9), "s1", 2, "c2", "edit", "edit aplicat a a.go"),
		call(day(2, 9), "s1", 2, "c3", "bash", `{"command":"go test ./..."}`),
		res(day(2, 9), "s1", 2, "c3", "bash", "ERROR: exit: exit status 1\nFAIL"),
		call(day(2, 9), "s1", 2, "c4", "bash", `{"command":"go  test ./..."}`),
		res(day(2, 9), "s1", 2, "c4", "bash", "ERROR: exit: exit status 1\nFAIL"),
		call(day(2, 9), "s1", 2, "c5", "write", `{"path":"b.go","content":"x"}`),
		res(day(2, 9), "s1", 2, "c5", "write", "ERROR: denied"),
		call(day(2, 10), "s1", 2, "c6", "bash", `{"command":"go test ./..."}`),
		res(day(2, 10), "s1", 2, "c6", "bash", "ok"),
		statsEv(t, day(2, 10), "s1", 2, "usage", map[string]any{"model": "deepseek", "up_tokens": 100, "down_tokens": 40, "cost_usd": 0.01}),
		statsEv(t, day(2, 10), "s1", 2, "run_completed", nil),
		// Run 3 (ahir, una altra conversa): checkpoint vermell i després verd.
		statsEv(t, day(1, 11), "s2", 3, "run_started", nil),
		statsEv(t, day(1, 11), "s2", 3, "autonomous_checkpoint", map[string]any{"checks": []map[string]any{{"command": "npm test", "code": 1}}}),
		statsEv(t, day(1, 12), "s2", 3, "autonomous_checkpoint", map[string]any{"checks": []map[string]any{{"command": "npm test", "code": 0}}}),
		statsEv(t, day(1, 12), "s2", 3, "usage", map[string]any{"model": "deepseek", "up_tokens": 10, "down_tokens": 5}),
		// Avui: una altra crida i un model diferent.
		statsEv(t, day(0, 8), "s2", 4, "run_started", nil),
		call(day(0, 8), "s2", 4, "c1", "grep", `{"pattern":"x"}`),
		statsEv(t, day(0, 8), "s2", 4, "usage", map[string]any{"model": "qwen", "up_tokens": 1, "down_tokens": 1}),
		statsEv(t, day(0, 8), "s2", 4, "retro_message", map[string]any{"key": "run-4"}),
	}
	saved := []session.Info{{Name: "a", SavedAt: day(1, 9)}, {Name: "b", SavedAt: day(40, 9)}}
	rep := computeStats(evs, saved, "week", now)

	if rep.Range != "week" || len(rep.Days) != 7 || rep.Days[6].Date != "2026-10-13" {
		t.Fatalf("range/days: %+v", rep.Days)
	}
	if rep.Sessions != 2 || rep.SavedSessions != 1 || rep.Turns != 3 {
		t.Fatalf("sessions=%d saved=%d turns=%d", rep.Sessions, rep.SavedSessions, rep.Turns)
	}
	if rep.ToolCalls != 7 || rep.ToolsByKind["shell"] != 3 || rep.ToolsByKind["edit"] != 2 || rep.ToolsByKind["read"] != 1 || rep.ToolsByKind["search"] != 1 {
		t.Fatalf("tools=%d kinds=%v", rep.ToolCalls, rep.ToolsByKind)
	}
	if rep.TopTools[0].Name != "bash" || rep.TopTools[0].Count != 3 {
		t.Fatalf("top tools: %+v", rep.TopTools)
	}
	// La write fallida no compta com a fitxer canviat.
	if rep.FilesChanged != 1 || rep.TopFiles[0].Name != "a.go" {
		t.Fatalf("files=%d %+v", rep.FilesChanged, rep.TopFiles)
	}
	if rep.CheckRuns != 5 || rep.ChecksFailed != 3 || rep.TestsTurnedGreen != 2 {
		t.Fatalf("checks=%d failed=%d green=%d", rep.CheckRuns, rep.ChecksFailed, rep.TestsTurnedGreen)
	}
	if b := rep.StubbornBug; b == nil || b.RunID != 2 || b.Failures != 2 || b.Command != "go test ./..." || b.Session != "s1" {
		t.Fatalf("stubborn: %+v", rep.StubbornBug)
	}
	if !rep.Tokens.Known || rep.Tokens.Up != 111 || rep.Tokens.Down != 46 || rep.Tokens.CostUSD == nil || *rep.Tokens.CostUSD != 0.01 {
		t.Fatalf("tokens: %+v", rep.Tokens)
	}
	if rep.FavoriteModel != "deepseek" || rep.FavoriteModelTurns != 2 {
		t.Fatalf("favorite: %s %d", rep.FavoriteModel, rep.FavoriteModelTurns)
	}
	if rep.ActiveDays != 3 || rep.LongestStreak != 3 || rep.CurrentStreak != 3 {
		t.Fatalf("days=%d longest=%d current=%d", rep.ActiveDays, rep.LongestStreak, rep.CurrentStreak)
	}
	if rep.Days[4].Turns != 1 || rep.Days[4].Tools != 6 {
		t.Fatalf("per-day: %+v", rep.Days[4])
	}

	month := computeStats(evs, saved, "month", now)
	if month.Turns != 4 || len(month.Days) != 30 || month.ActiveDays != 4 {
		t.Fatalf("month: turns=%d days=%d active=%d", month.Turns, len(month.Days), month.ActiveDays)
	}
}

func TestComputeStatsEmptyIsHonest(t *testing.T) {
	rep := computeStats(nil, nil, "bogus", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	if rep.Range != "week" || rep.Tokens.Known || rep.Tokens.CostUSD != nil || rep.StubbornBug != nil || rep.FavoriteModel != "" || rep.CurrentStreak != 0 {
		t.Fatalf("empty report must not invent data: %+v", rep)
	}
}

func TestStatsStreakGapAndYesterday(t *testing.T) {
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	days := map[string]bool{"2026-03-01": true, "2026-03-02": true, "2026-03-03": true, "2026-03-04": true, "2026-03-08": true, "2026-03-09": true}
	longest, current := statsStreaks(days, now)
	if longest != 4 || current != 2 {
		t.Fatalf("longest=%d current=%d", longest, current)
	}
}

func TestStatsAndReplayEndpoints(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	s.Hub()
	now := time.Now()
	for _, e := range []events.Event{
		statsEv(t, now, s.id, 7, "run_started", nil),
		statsEv(t, now, s.id, 7, "tool_call", map[string]string{"id": "c1", "name": "bash", "args": `{"command":"go test ./..."}`}),
		statsEv(t, now, s.id, 7, "token", nil),
		statsEv(t, now, s.id, 7, "tool_result", map[string]string{"id": "c1", "name": "bash", "output": "ok"}),
		statsEv(t, now, s.id, 7, "delegate", map[string]any{"id": "d1", "task": "t", "status": "working"}),
		statsEv(t, now, s.id, 7, "run_completed", nil),
		statsEv(t, now, s.id, 8, "run_started", nil),
		statsEv(t, now, "other", 9, "run_started", nil),
	} {
		e.User = s.usuari()
		if _, err := s.Hub().eventStore.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	rec := get("/api/stats?range=week")
	var rep statsReport
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &rep) != nil || rep.Turns != 3 || rep.ToolCalls != 1 || rep.CheckRuns != 1 {
		t.Fatalf("stats: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get("/api/stats?range=year"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad range must be 400, got %d", rec.Code)
	}

	rec = get("/api/replay")
	var list struct {
		Runs []replayRun `json:"runs"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Runs) != 2 {
		t.Fatalf("runs: %d %s", rec.Code, rec.Body.String())
	}
	if r := list.Runs[1]; r.RunID != 7 || r.State != "completed" || r.Tools != 1 || r.Agents != 1 || list.Runs[0].State != "running" {
		t.Fatalf("run summary: %+v", list.Runs)
	}
	rec = get("/api/replay?run_id=7")
	var one struct {
		Events []events.Event `json:"events"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &one) != nil || len(one.Events) != 5 {
		t.Fatalf("replay events (tokens must be skipped): %d %s", rec.Code, rec.Body.String())
	}
	if rec := get("/api/replay?run_id=9"); rec.Code != http.StatusNotFound {
		t.Fatalf("a run of another conversation must be 404, got %d", rec.Code)
	}
	if rec := get("/api/replay?run_id=x"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad run id must be 400, got %d", rec.Code)
	}
}
