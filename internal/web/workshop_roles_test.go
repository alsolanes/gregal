package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

func call(name, args string) llm.ToolCall {
	c := llm.ToolCall{ID: name}
	c.Function.Name, c.Function.Arguments = name, args
	return c
}

func TestClassifyCallRoles(t *testing.T) {
	cases := []struct {
		name, args, role, kind, target string
	}{
		{"todowrite", `{}`, roleCoordinator, "plan", ""},
		{"read", `{"path":"src/stats.py"}`, roleResearcher, "read", "src/stats.py"},
		{"grep", `{"pattern":"def mean"}`, roleResearcher, "search", "def mean"},
		{"web_fetch", `{"url":"https://www.wikipedia.org/wiki/Mean"}`, roleResearcher, "web", "wikipedia.org"},
		{"web_search", `{"query":"python statistics"}`, roleResearcher, "web", "python statistics"},
		{"edit", `{"path":"src/stats.py","old_string":"a","new_string":"b"}`, roleBuilder, "edit", "src/stats.py"},
		{"patch", `{"path":"a.go"}`, roleBuilder, "edit", "a.go"},
		{"git_commit", `{"message":"x"}`, roleBuilder, "commit", ""},
		{"bash", `{"command":"python -m pytest -q"}`, roleTester, "test", "python -m pytest -q"},
		{"bash", `{"command":"go test ./..."}`, roleTester, "test", "go test ./..."},
		{"bash", `{"command":"npm run lint"}`, roleTester, "test", "npm run lint"},
		{"bash", `{"command":"ls -la src"}`, roleResearcher, "search", "ls -la src"},
		{"bash", `{"command":"mkdir out && cp a b"}`, roleBuilder, "run", "mkdir out && cp a b"},
		{"delegate", `{"tasks":[{"prompt":"a"},{"prompt":"b"}]}`, roleCoordinator, "delegate", ""},
		{"mcp_jira_search_issues", `{}`, roleResearcher, "search", "mcp_jira_search_issues"},
	}
	for _, c := range cases {
		got := classifyCall(c.name, c.args)
		if got.Role != c.role || got.Kind != c.kind || got.Target != c.target {
			t.Errorf("%s %s: got role=%q kind=%q target=%q", c.name, c.args, got.Role, got.Kind, got.Target)
		}
	}
	if got := classifyCall("bash_output", `{"id":"1"}`); got.Role != "" {
		t.Errorf("bash_output must inherit the current role, got %q", got.Role)
	}
}

func TestClassifyCallsPicksDominantRole(t *testing.T) {
	info := classifyCalls([]llm.ToolCall{call("read", `{"path":"a.py"}`), call("read", `{"path":"b.py"}`), call("read", `{"path":"a.py"}`)})
	if info.Role != roleResearcher || info.N != 2 || len(info.Files) != 2 {
		t.Fatalf("parallel reads: %+v", info)
	}
	info = classifyCalls([]llm.ToolCall{call("read", `{"path":"a.py"}`), call("edit", `{"path":"b.py"}`), call("edit", `{"path":"c.py"}`)})
	if info.Role != roleBuilder || info.Kind != "edit" || info.N != 3 {
		t.Fatalf("mixed read+edit must be the builder: %+v", info)
	}
	info = classifyCalls([]llm.ToolCall{call("edit", `{"path":"b.py"}`), call("bash", `{"command":"pytest"}`)})
	if info.Role != roleTester || info.Command != "pytest" {
		t.Fatalf("edit+test must be the tester: %+v", info)
	}
}

func TestLaneResult(t *testing.T) {
	tester := laneInfo{Role: roleTester}
	if laneResult(tester, []agent.Execucio{{Sortida: "3 passed in 0.1s"}}) != "ok" {
		t.Fatal("green tests must be ok")
	}
	if laneResult(tester, []agent.Execucio{{Sortida: "ERROR: exit status 1\nFAILED test_x"}}) != "fail" {
		t.Fatal("red tests must fail")
	}
	if laneResult(laneInfo{Role: roleResearcher}, []agent.Execucio{{Sortida: "ERROR: no matches"}}) != "ok" {
		t.Fatal("an empty search is not a failure")
	}
}

func TestWorkshopLanesHandoffsAndUsage(t *testing.T) {
	s := goalTestServer(t)
	gen := s.beginWorkshop("task", "m", 10)
	meter := &llm.UsageMeter{}
	lane, ho := s.beginLane(gen, 1, classifyCall("read", `{"path":"a.py"}`), "")
	if lane.Seq == 0 || ho == nil || ho.From != roleCoordinator || ho.To != roleResearcher || ho.Kind != "read" {
		t.Fatalf("first read must hand off coordinator→researcher: lane=%+v ho=%+v", lane, ho)
	}
	if _, again := s.beginLane(gen, 2, classifyCall("grep", `{"pattern":"x"}`), ""); again != nil {
		t.Fatal("same role: no handoff")
	}
	edit, ho := s.beginLane(gen, 3, classifyCalls([]llm.ToolCall{call("edit", `{"path":"a.py"}`), call("edit", `{"path":"b.py"}`)}), "")
	if ho == nil || ho.To != roleBuilder || ho.Kind != "edit" || ho.N != 2 {
		t.Fatalf("edit handoff: %+v", ho)
	}
	done, ok := s.endLane(gen, edit.Seq, "ok")
	if !ok || !done.Done || done.Result != "ok" {
		t.Fatalf("endLane: %+v", done)
	}
	if _, ok := s.endLane(gen+1, edit.Seq, "ok"); ok {
		t.Fatal("stale generation must be ignored")
	}
	s.noteUsage(gen, meter, "p")
	s.finishWorkshop(gen, nil)

	s.mu.Lock()
	snap := s.workshopSnapshotLocked()
	s.mu.Unlock()
	raw, _ := json.Marshal(snap)
	var got struct {
		Role     string            `json:"role"`
		Lanes    []workshopLane    `json:"lanes"`
		Handoffs []workshopHandoff `json:"handoffs"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Role != roleCoordinator || len(got.Lanes) != 3 {
		t.Fatalf("snapshot role=%q lanes=%d", got.Role, len(got.Lanes))
	}
	for _, l := range got.Lanes {
		if !l.Done {
			t.Fatalf("finish must close every lane: %+v", l)
		}
	}
	last := got.Handoffs[len(got.Handoffs)-1]
	if last.From != roleBuilder || last.To != roleCoordinator || last.Kind != "report" {
		t.Fatalf("completed turn must hand back to the coordinator: %+v", last)
	}
}

func TestWorkshopLanesAreBounded(t *testing.T) {
	s := goalTestServer(t)
	gen := s.beginWorkshop("task", "m", 10)
	for i := 0; i < maxWorkshopLanes+15; i++ {
		name := "read"
		if i%2 == 1 {
			name = "edit"
		}
		s.beginLane(gen, i, classifyCall(name, `{"path":"a.py"}`), "")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.activity.Lanes) != maxWorkshopLanes || s.activity.LanesTrimmed != 15 || len(s.activity.Handoffs) != maxWorkshopHandoffs {
		t.Fatalf("lanes=%d trimmed=%d handoffs=%d", len(s.activity.Lanes), s.activity.LanesTrimmed, len(s.activity.Handoffs))
	}
}

// Torn complet amb model fals: el scout (fill real) investiga primer, el
// seu resum arriba al coordinador, i la sala rep carrils i relleus
// (researcher → coordinator → builder → tester).
func TestConduirTornRunsScoutAndEmitsLanes(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "stats.py"), []byte("def mean(xs):\n    return sum(xs)/len(xs)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var coordinatorSteps atomic.Int32
	var briefSeen atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "SUBAGENT scout") {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"stats.py:1 defines mean(); add describe() below it"},"finish_reason":"stop"}]}`)
			return
		}
		for _, m := range req.Messages {
			if m.Role == "user" && strings.Contains(m.Content, "RESEARCHER BRIEF") && strings.Contains(m.Content, "add describe() below it") {
				briefSeen.Store(true)
			}
		}
		switch coordinatorSteps.Add(1) {
		case 1:
			args, _ := json.Marshal(map[string]string{"path": "stats.py", "old_string": "def mean(xs):", "new_string": "def describe(xs):\n    return {}\n\n\ndef mean(xs):"})
			fmt.Fprintf(w, `{"choices":[{"message":{"tool_calls":[{"id":"e1","type":"function","function":{"name":"edit","arguments":%q}}]},"finish_reason":"tool_calls"}]}`, string(args))
		case 2:
			fmt.Fprint(w, `{"choices":[{"message":{"tool_calls":[{"id":"t1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"go test ./...\"}"}}]},"finish_reason":"tool_calls"}]}`)
		default:
			fmt.Fprint(w, `{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}]}`)
		}
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL}}
	s.permissive = true
	role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}
	task := "add a describe() function to stats.py with tests"
	gen := s.beginWorkshop(task, "m", 6)
	ctx := context.WithValue(context.Background(), workshopRunKey{}, gen)
	var mu sync.Mutex
	var kinds []string
	var handoffs []string
	scoutAgent := ""
	reply, _, err := s.conduirTorn(ctx, task, agent.ModeCode, "code", s.cfg.Providers["p"], role, 6, ws, nil, func(kind string, data any) {
		mu.Lock()
		defer mu.Unlock()
		switch kind {
		case "lane", "handoff":
			kinds = append(kinds, kind)
			if kind == "handoff" {
				m := data.(map[string]any)
				handoffs = append(handoffs, fmt.Sprintf("%s>%s:%s", m["from"], m["to"], m["kind"]))
			}
		case "delegate":
			if ev, ok := data.(agent.DelegateEvent); ok && ev.Agent != "" {
				scoutAgent = ev.Agent
			}
		}
	})
	if err != nil || reply != "done" {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
	if !briefSeen.Load() {
		t.Fatal("the scout's brief must reach the coordinator")
	}
	if scoutAgent != agent.ScoutAgentName {
		t.Fatalf("scout must surface as a real delegate child, got %q", scoutAgent)
	}
	got := strings.Join(handoffs, " ")
	want := "coordinator>researcher:read researcher>coordinator:brief coordinator>builder:edit builder>tester:test"
	if got != want {
		t.Fatalf("handoffs = %q, want %q", got, want)
	}
	s.mu.Lock()
	lanes := append([]workshopLane{}, s.activity.Lanes...)
	agents := append([]agent.DelegateEvent{}, s.activity.Agents...)
	s.mu.Unlock()
	if len(lanes) != 3 || lanes[0].Kind != "scout" || lanes[0].Agent == "" || lanes[1].Role != roleBuilder || lanes[2].Role != roleTester {
		t.Fatalf("lanes: %+v", lanes)
	}
	if len(agents) != 1 || agents[0].ID != lanes[0].Agent || agents[0].Status != agent.DelegateCompleted {
		t.Fatalf("scout child in the room: %+v", agents)
	}
}

func TestConduirTornSkipsScoutWhenOff(t *testing.T) {
	var scoutCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "SUBAGENT scout") {
			scoutCalls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}]}`)
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL}}
	s.cfg.Agent.Scout = "off"
	role := config.Role{Provider: "p", Model: "m", MaxTokens: 4096, ContextWindow: 32768}
	if _, _, err := s.conduirTorn(context.Background(), "add a describe() function to stats.py with tests", agent.ModeCode, "code", s.cfg.Providers["p"], role, 3, t.TempDir(), nil, func(string, any) {}); err != nil {
		t.Fatal(err)
	}
	if scoutCalls.Load() != 0 {
		t.Fatal("agent.scout: off must not run the scout")
	}
}
