package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/team"
)

func workshopSnapshotForTest(s *Server) *workshopActivity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.workshopSnapshotLocked()
}

func TestWorkshopActivityGenerationRetainsCompletionAndIgnoresStaleCallbacks(t *testing.T) {
	s := hubTestServer(t)
	first := s.beginWorkshop("first task", "main-model", 8)
	s.updateWorkshop(first, "tool", 2, 8, "read", "reading source")
	s.noteDelegate(first, agent.DelegateEvent{
		ID: "child-first", Task: "inspect file", Status: agent.DelegateWorking,
		Model: "child-model", Step: 1, MaxSteps: 12, Tool: "read",
		StartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	s.finishWorkshop(first, nil)

	completed := workshopSnapshotForTest(s)
	if completed == nil || completed.State != "completed" || completed.Task != "first task" {
		t.Fatalf("completed activity was not retained: %+v", completed)
	}
	if len(completed.Agents) != 1 || completed.Agents[0].Status != agent.DelegateCancelled {
		t.Fatalf("unfinished child should be closed when the turn completes: %+v", completed.Agents)
	}

	second := s.beginWorkshop("second task", "next-model", 4)
	if second == first {
		t.Fatalf("generation did not advance: first=%d second=%d", first, second)
	}
	s.updateWorkshop(first, "failed", 99, 99, "stale-tool", "stale update")
	s.noteDelegate(first, agent.DelegateEvent{ID: "stale-child", Status: agent.DelegateWorking})
	s.finishWorkshop(first, fmt.Errorf("stale failure"))

	current := workshopSnapshotForTest(s)
	if current == nil || current.State != "thinking" || current.Task != "second task" ||
		current.Model != "next-model" || current.Limit != 4 || len(current.Agents) != 0 {
		t.Fatalf("old generation changed the current activity: %+v", current)
	}
}

func TestWorkshopActivityAgentHistoryIsBounded(t *testing.T) {
	s := hubTestServer(t)
	generation := s.beginWorkshop("bounded history", "main-model", 6)
	for i := 0; i < maxWorkshopAgents+9; i++ {
		s.noteDelegate(generation, agent.DelegateEvent{
			ID: fmt.Sprintf("child-%02d", i), Task: "inspect", Status: agent.DelegateCompleted,
			Model: "child-model", Step: 1, MaxSteps: 12,
			StartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		})
	}

	snapshot := workshopSnapshotForTest(s)
	if snapshot == nil || len(snapshot.Agents) != maxWorkshopAgents {
		t.Fatalf("agent history length=%d, want %d", len(snapshot.Agents), maxWorkshopAgents)
	}
	if snapshot.Agents[0].ID != "child-09" || snapshot.Agents[len(snapshot.Agents)-1].ID != fmt.Sprintf("child-%02d", maxWorkshopAgents+8) {
		t.Fatalf("history did not retain the newest terminal agents: first=%q last=%q", snapshot.Agents[0].ID, snapshot.Agents[len(snapshot.Agents)-1].ID)
	}
	if snapshot.Evicted != 9 {
		t.Fatalf("evicted=%d, want 9 dropped children reported", snapshot.Evicted)
	}
}

func TestWorkshopActivityIsScopedToSession(t *testing.T) {
	h := hubTestServer(t).Hub()
	first := h.Session("workshop-one")
	second := h.Session("workshop-two")

	first.beginWorkshop("first session task", "model-one", 3)
	second.beginWorkshop("second session task", "model-two", 5)

	firstSnapshot := workshopSnapshotForTest(first)
	secondSnapshot := workshopSnapshotForTest(second)
	if firstSnapshot == nil || firstSnapshot.Task != "first session task" || firstSnapshot.Model != "model-one" {
		t.Fatalf("first session activity=%+v", firstSnapshot)
	}
	if secondSnapshot == nil || secondSnapshot.Task != "second session task" || secondSnapshot.Model != "model-two" {
		t.Fatalf("second session activity=%+v", secondSnapshot)
	}
}

type workshopProviderMessage struct {
	Role       string `json:"role"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

type workshopProviderRequest struct {
	Model    string                    `json:"model"`
	Stream   bool                      `json:"stream"`
	Messages []workshopProviderMessage `json:"messages"`
}

func workshopFunctionCall(id, name, arguments string) map[string]any {
	return map[string]any{
		"id": id, "type": "function",
		"function": map[string]string{"name": name, "arguments": arguments},
	}
}

func workshopProviderResponse(w http.ResponseWriter, stream bool, content string, calls ...map[string]any) {
	message := map[string]any{"role": "assistant", "content": content}
	if len(calls) > 0 {
		message["tool_calls"] = calls
	}
	if !stream {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": message, "finish_reason": map[bool]string{true: "tool_calls", false: "stop"}[len(calls) > 0]}},
		})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	delta := map[string]any{}
	if content != "" {
		delta["content"] = content
	}
	if len(calls) > 0 {
		delta["tool_calls"] = calls
	}
	finish := "stop"
	if len(calls) > 0 {
		finish = "tool_calls"
	}
	for _, event := range []any{
		map[string]any{"choices": []any{map[string]any{"delta": delta, "finish_reason": finish}}},
	} {
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}

func workshopLiveActivity(t *testing.T, handler http.Handler, sessionID string) *workshopActivity {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/live?session="+sessionID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live sessions returned %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Sessions []struct {
			ID       string            `json:"id"`
			Activity *workshopActivity `json:"activity"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode live sessions: %v", err)
	}
	for _, session := range got.Sessions {
		if session.ID == sessionID {
			return session.Activity
		}
	}
	t.Fatalf("session %q missing from live sessions: %s", sessionID, rec.Body.String())
	return nil
}

func TestWorkshopActivityTracksDelegatesThroughLiveSessionsHTTP(t *testing.T) {
	childReady := make(chan struct{}, 2)
	providerCalls := make(chan string, 10000)
	releaseChildren := make(chan struct{})
	var childrenReleased bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request workshopProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		providerCalls <- fmt.Sprintf("model=%s stream=%t path=%s", request.Model, request.Stream, r.URL.Path)
		lastRole, lastToolCall := "", ""
		for _, message := range request.Messages {
			lastRole, lastToolCall = message.Role, message.ToolCallID
		}
		switch request.Model {
		case "parent-model":
			plain := false
			for _, message := range request.Messages {
				if message.Role == "user" && strings.Contains(message.Content, "plain turn") {
					plain = true
				}
			}
			if plain {
				workshopProviderResponse(w, request.Stream, "plain turn finished without delegating")
				return
			}
			delegated := false
			for _, message := range request.Messages {
				// tool results never carry "name" on the wire (strict
				// providers reject it), so match the echoed tool_call_id.
				if message.Role == "tool" && message.ToolCallID == "parent-delegate" {
					delegated = true
				}
			}
			if delegated {
				workshopProviderResponse(w, request.Stream, "parent finished")
				return
			}
			args, _ := json.Marshal(map[string]any{"tasks": []any{
				map[string]string{"prompt": "inspect first child"},
				map[string]string{"prompt": "inspect second child"},
			}})
			workshopProviderResponse(w, request.Stream, "", workshopFunctionCall("parent-delegate", "delegate", string(args)))
		case "child-model":
			if lastRole == "tool" && lastToolCall == "child-read" {
				// Non-blocking: a duplicate barrier hit must not wedge the
				// fixture handler; later assertions catch real loops.
				select {
				case childReady <- struct{}{}:
				default:
				}
				select {
				case <-releaseChildren:
				case <-time.After(8 * time.Second):
					http.Error(w, "test child wait timed out", http.StatusGatewayTimeout)
					return
				}
				workshopProviderResponse(w, request.Stream, "child finished after reading")
				return
			}
			args, _ := json.Marshal(map[string]string{"path": "workshop-fixture.txt"})
			workshopProviderResponse(w, request.Stream, "", workshopFunctionCall("child-read", "read", string(args)))
		default:
			http.Error(w, "unexpected model: "+request.Model, http.StatusBadRequest)
		}
	}))
	defer provider.Close()

	server := hubTestServer(t)
	server.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL}
	server.cfg.Roles["code"] = config.Role{Provider: "p", Model: "parent-model", ContextWindow: 32768, MaxTokens: 1024}
	server.cfg.Roles["delegate"] = config.Role{Provider: "p", Model: "child-model", ContextWindow: 32768, MaxTokens: 1024}
	server.cfg.Permissions.Tools = map[string]string{"delegate": "allow", "read": "allow"}
	// The fixture listens on 127.0.0.1, which counts as a one-slot local
	// server; this test needs both children alive at once.
	server.cfg.Agent.ParallelSubagents = 2
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "workshop-fixture.txt"), []byte("child-read-marker\n"), 0600); err != nil {
		t.Fatal(err)
	}
	server.mu.Lock()
	server.cwd = workspace
	server.policy.ProjectDir = workspace
	server.policy.Tools = server.cfg.Permissions.Tools
	server.mode = agent.ModeCode
	server.mu.Unlock()

	hub := server.Hub()
	const sessionID = "workshop-integration"
	_ = hub.Session(sessionID)
	handler := hub.Handler()
	turnDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"delegate twice and inspect the fixture"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gregal-Session", sessionID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		turnDone <- rec
	}()
	defer func() {
		if !childrenReleased {
			close(releaseChildren)
		}
		select {
		case <-turnDone:
		case <-time.After(2 * time.Second):
		}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-childReady:
		case <-time.After(6 * time.Second):
			var calls []string
			for len(providerCalls) > 0 {
				calls = append(calls, <-providerCalls)
			}
			select {
			case result := <-turnDone:
				t.Fatalf("only %d children reached the post-read barrier; provider calls=%v; agent=%d %s", i, calls, result.Code, result.Body.String())
			default:
				t.Fatalf("only %d children reached the post-read barrier; provider calls=%v", i, calls)
			}
		}
	}

	var active *workshopActivity
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		active = workshopLiveActivity(t, handler, sessionID)
		if active != nil && len(active.Agents) == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if active == nil || active.State == "completed" || len(active.Agents) != 2 {
		t.Fatalf("live activity did not expose both active children: %+v", active)
	}
	seen := map[string]bool{}
	for _, child := range active.Agents {
		if child.ID == "" || seen[child.ID] {
			t.Fatalf("child ids must be distinct: %+v", active.Agents)
		}
		seen[child.ID] = true
		if child.Status != agent.DelegateWorking || child.Model != "child-model" ||
			child.Step < 1 || child.MaxSteps != 12 || child.Tool != "read" ||
			!strings.Contains(child.Output, "child-read-marker") {
			t.Errorf("child progress did not show the read before completion: %+v", child)
		}
	}

	close(releaseChildren)
	childrenReleased = true
	var turn *httptest.ResponseRecorder
	select {
	case turn = <-turnDone:
	case <-time.After(6 * time.Second):
		t.Fatal("parent turn did not finish after releasing the children")
	}
	if turn.Code != http.StatusOK {
		t.Fatalf("agent endpoint returned %d: %s", turn.Code, turn.Body.String())
	}
	completed := workshopLiveActivity(t, handler, sessionID)
	if completed == nil || completed.State != "completed" || len(completed.Agents) != 2 {
		t.Fatalf("completed workshop snapshot was not retained: %+v", completed)
	}
	for _, child := range completed.Agents {
		if child.Status != agent.DelegateCompleted || !strings.Contains(child.Output, "child finished after reading") {
			t.Errorf("completed child missing final output: %+v", child)
		}
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"start a second plain turn"}`))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("X-Gregal-Session", sessionID)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second agent endpoint returned %d: %s", secondRec.Code, secondRec.Body.String())
	}
	reset := workshopLiveActivity(t, handler, sessionID)
	if reset == nil || reset.State != "completed" || reset.Task != "start a second plain turn" || len(reset.Agents) != 0 {
		t.Fatalf("new turn did not reset the retained child activity: %+v", reset)
	}
}

func TestWorkshopActivityTracksTurnUsageAndCost(t *testing.T) {
	s := hubTestServer(t)
	agent.SetPrices(map[string]agent.Price{"main-model": {In: 1, Out: 2}}) // $ per milió de tokens
	defer agent.SetPrices(nil)
	generation := s.beginWorkshop("metered turn", "main-model", 6)
	meter := &llm.UsageMeter{}
	s.noteUsage(generation, meter, "p")
	s.noteUsage(generation, nil, "p")
	if snap := workshopSnapshotForTest(s); snap.Calls != 0 || snap.TokensSource != "" || snap.CostUSD != nil {
		t.Fatalf("empty meter must stay silent: %+v", snap)
	}
	meter.Add(llm.Usage{PromptTokens: 1000000, CompletionTokens: 500000, TotalTokens: 1500000})
	s.noteUsage(generation, meter, "p")
	snap := workshopSnapshotForTest(s)
	if snap.Calls != 1 || snap.UpTokens != 1000000 || snap.DownTokens != 500000 || snap.TokensSource != "api" {
		t.Fatalf("turn usage not recorded: %+v", snap)
	}
	if snap.CostUSD == nil || *snap.CostUSD != 2 {
		t.Fatalf("turn cost not priced: %+v", snap)
	}
	meter.Add(llm.Usage{})
	s.noteUsage(generation, meter, "p")
	if snap := workshopSnapshotForTest(s); snap.Calls != 2 || snap.TokensSource != "mixed" {
		t.Fatalf("partially declared usage must read mixed: %+v", snap)
	}
	s.noteUsage(generation+99, meter, "p")
	if snap := workshopSnapshotForTest(s); snap.Calls != 2 {
		t.Fatalf("stale generation changed the activity: %+v", snap)
	}
}

func TestWorkshopActivityKeepsBoundedTurnHistory(t *testing.T) {
	s := hubTestServer(t)
	generation := s.beginWorkshop("timeline turn", "main-model", 6)
	for i := 1; i <= maxWorkshopHistory+15; i++ {
		state := "thinking"
		if i%2 == 0 {
			state = "tools"
		}
		s.updateWorkshop(generation, state, i, 60, "read", "reading")
	}
	snap := workshopSnapshotForTest(s)
	if snap == nil || len(snap.History) != maxWorkshopHistory {
		t.Fatalf("history length=%d, want %d", len(snap.History), maxWorkshopHistory)
	}
	if snap.Trimmed != 15 {
		t.Fatalf("trimmed=%d, want 15 dropped rungs reported", snap.Trimmed)
	}
	if snap.History[0].Step != 16 || snap.History[len(snap.History)-1].Step != maxWorkshopHistory+15 {
		t.Fatalf("history did not retain the newest steps: first=%+v last=%+v", snap.History[0], snap.History[len(snap.History)-1])
	}
	for _, entry := range snap.History {
		if entry.At.IsZero() || (entry.State != "thinking" && entry.State != "tools") {
			t.Fatalf("history entry missing state or time: %+v", entry)
		}
	}
	s.finishWorkshop(generation, nil)
	if snap := workshopSnapshotForTest(s); len(snap.History) != maxWorkshopHistory {
		t.Fatalf("completed turn lost its timeline: %+v", snap)
	}
	next := s.beginWorkshop("fresh turn", "main-model", 4)
	s.updateWorkshop(next, "thinking", 1, 4, "", "")
	if snap := workshopSnapshotForTest(s); len(snap.History) != 1 || snap.History[0].Step != 1 {
		t.Fatalf("new turn did not reset the timeline: %+v", snap)
	}
}

func TestNoteTeamEventMapsPipelineToRoom(t *testing.T) {
	s := hubTestServer(t)
	generation := s.beginWorkshop("team task", "team-model", 0)
	s.noteTeamEvent(generation, team.Event{Type: "working", Agent: "coordinator", Activity: "plan"}, "")
	if snap := workshopSnapshotForTest(s); snap.State != "thinking" || snap.Kind != "" {
		t.Fatalf("coordinator working should read thinking without kind yet: %+v", snap)
	}
	s.noteTeamEvent(generation, team.Event{Type: "working", Agent: "builder", Activity: "draft"}, "")
	s.noteTeamEvent(generation, team.Event{Type: "completed", Agent: "builder", Activity: "draft", Output: "draft v1"}, "builder-model")
	s.noteTeamEvent(generation, team.Event{Type: "handoff", Agent: "builder", To: "reviewer"}, "")
	s.noteTeamEvent(generation, team.Event{Type: "working", Agent: "reviewer", Activity: "review"}, "")
	snap := workshopSnapshotForTest(s)
	if snap.State != "reviewing" {
		t.Fatalf("reviewer working should read reviewing: %+v", snap)
	}
	if len(snap.Agents) != 3 {
		t.Fatalf("room must track started roles: %+v", snap.Agents)
	}
	var builder *agent.DelegateEvent
	for i := range snap.Agents {
		if snap.Agents[i].ID == "builder" {
			builder = &snap.Agents[i]
		}
	}
	if builder == nil || builder.Status != agent.DelegateCompleted {
		t.Fatalf("completed role must persist as room agent: %+v", snap.Agents)
	}
	if builder.Output != "draft v1" || builder.Model != "builder-model" {
		t.Fatalf("room agent keeps output and model: %+v", builder)
	}
	if len(snap.History) < 4 {
		t.Fatalf("handoffs must leave timeline rungs: %+v", snap.History)
	}
	s.finishWorkshop(generation, nil)
	if snap := workshopSnapshotForTest(s); snap.State != "completed" {
		t.Fatalf("done run must read completed: %+v", snap)
	}
}

func TestFinishTeamErrorNeverStoresProviderURLs(t *testing.T) {
	s := hubTestServer(t)
	generation := s.beginWorkshop("team task", "team-model", 0)
	s.finishWorkshop(generation, fmt.Errorf("provider https://models.example.net/v1?key=S3CR3T: HTTP 502"))
	snap := workshopSnapshotForTest(s)
	if snap.State != "failed" {
		t.Fatalf("provider error must read failed: %+v", snap)
	}
	for _, secret := range []string{"S3CR3T", "models.example.net"} {
		if strings.Contains(snap.Message, secret) {
			t.Fatalf("room message leaks provider details: %q", snap.Message)
		}
	}
}

func TestStopRunFiresTeamCancel(t *testing.T) {
	s := hubTestServer(t)
	fired := false
	s.mu.Lock()
	s.teamCancel = func() { fired = true }
	s.mu.Unlock()
	w := httptest.NewRecorder()
	s.handleCancel(w, httptest.NewRequest("POST", "/api/agent/cancel", nil))
	if !fired {
		t.Fatal("stopRun must fire the team cancel slot")
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out["cancelled"] != true {
		t.Fatalf("cancel must report stopped: %s", w.Body.String())
	}
}

func TestLiveMarksFlowingSessionsBusy(t *testing.T) {
	h := hubTestServer(t).Hub()
	s := h.Session("flowing")
	s.mu.Lock()
	s.flowRunning = true
	s.mu.Unlock()
	w := httptest.NewRecorder()
	h.handleLiveSessions(w, httptest.NewRequest("GET", "/api/sessions/live", nil))
	var live struct {
		Sessions []map[string]any `json:"sessions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &live); err != nil {
		t.Fatal(err)
	}
	for _, item := range live.Sessions {
		if item["id"] == "flowing" && item["busy"] != true {
			t.Fatalf("team run must read busy: %v", item)
		}
	}
}
