package team

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// scriptRunner replays canned outputs per agent name and counts calls.
type scriptRunner struct {
	mu    sync.Mutex
	outs  map[string][]string
	calls map[string]int
}

func newScriptRunner(outs map[string][]string) *scriptRunner {
	return &scriptRunner{outs: outs, calls: map[string]int{}}
}

func (s *scriptRunner) run(_ context.Context, name, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[name]++
	outs := s.outs[name]
	if len(outs) == 0 {
		return "", fmt.Errorf("no scripted output for %s", name)
	}
	out := outs[0]
	if len(outs) > 1 {
		s.outs[name] = outs[1:]
	}
	return out, nil
}

func (s *scriptRunner) count(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[name]
}

func eventSigs(events []Event) []string {
	sigs := make([]string, 0, len(events))
	for _, e := range events {
		sig := e.Type + "/" + e.Agent
		if e.To != "" {
			sig += ">" + e.To
		}
		sigs = append(sigs, sig)
	}
	sort.Strings(sigs)
	return sigs
}

func collectFlow(task string, spec FlowSpec, r *scriptRunner) (string, []Event, error) {
	var events []Event
	answer, err := RunFlow(context.Background(), task, spec, r.run, func(e Event) {
		events = append(events, e)
	})
	return answer, events, err
}

func approvedJSON(deliverable string) string {
	return `{"approved": true, "feedback": "", "deliverable": ` + strconv.Quote(deliverable) + `}`
}

func rejectedJSON(feedback, deliverable string) string {
	return `{"approved": false, "feedback": ` + strconv.Quote(feedback) + `, "deliverable": ` + strconv.Quote(deliverable) + `}`
}

func TestDefaultFlowIsValid(t *testing.T) {
	if err := ValidateFlow(DefaultFlow()); err != nil {
		t.Fatalf("default flow must validate: %v", err)
	}
}

func TestValidateFlowRejectsBrokenPipelines(t *testing.T) {
	good := DefaultFlow()
	cases := []struct {
		name   string
		mutate func(*FlowSpec)
	}{
		{"empty", func(s *FlowSpec) { s.Nodes = nil }},
		{"blank id", func(s *FlowSpec) { s.Nodes[0].ID = " " }},
		{"dup id", func(s *FlowSpec) { s.Nodes[1].ID = s.Nodes[0].ID }},
		{"unknown role", func(s *FlowSpec) { s.Nodes[1].Role = "oracle" }},
		{"bad mode", func(s *FlowSpec) { s.Nodes[1].Mode = "nap" }},
		{"odd combo needs instruction", func(s *FlowSpec) { s.Nodes[3].Mode = "task" }},
		{"unknown output", func(s *FlowSpec) { s.Output = "ghost" }},
		{"negative rounds", func(s *FlowSpec) { s.MaxRounds = -1 }},
		{"unknown edge endpoint", func(s *FlowSpec) { s.Edges[0].To = "ghost" }},
		{"self loop", func(s *FlowSpec) { s.Edges = append(s.Edges, FlowEdge{From: "builder", To: "builder", Kind: "data"}) }},
		{"bad kind", func(s *FlowSpec) { s.Edges[0].Kind = "telepathy" }},
		{"revision from task node", func(s *FlowSpec) { s.Edges[4] = FlowEdge{From: "builder", To: "researcher", Kind: "revision"} }},
		{"data cycle", func(s *FlowSpec) {
			s.Edges = append(s.Edges, FlowEdge{From: "reviewer", To: "coordinator", Kind: "data"})
		}},
		{"orphan node", func(s *FlowSpec) {
			s.Nodes = append(s.Nodes, FlowNode{ID: "extra", Role: "builder", Mode: "task", Instruction: "do extra"})
		}},
		{"output feeds others", func(s *FlowSpec) {
			s.Edges = append(s.Edges, FlowEdge{From: "reviewer", To: "coordinator", Kind: "data"})
		}},
	}
	for _, tc := range cases {
		spec := good
		spec.Nodes = append([]FlowNode{}, good.Nodes...)
		spec.Edges = append([]FlowEdge{}, good.Edges...)
		tc.mutate(&spec)
		if err := ValidateFlow(spec); err == nil {
			t.Errorf("%s: invalid flow passed validation", tc.name)
		}
	}
	// Revision to work that never reaches the review must fail.
	detached := FlowSpec{
		Nodes: []FlowNode{
			{ID: "r", Role: "reviewer", Mode: "review"},
			{ID: "b", Role: "builder", Mode: "task", Instruction: "build"},
		},
		Edges:  []FlowEdge{{From: "r", To: "b", Kind: "revision"}},
		Output: "r",
	}
	if err := ValidateFlow(detached); err == nil {
		t.Error("revision to non-feeding work must fail validation")
	}
}

func TestDefaultFlowMatchesClassicRun(t *testing.T) {
	outs := map[string][]string{
		"coordinator": {"plan v1"},
		"researcher":  {"research v1"},
		"builder":     {"draft v1"},
		"reviewer":    {approvedJSON("final v1")},
	}
	classicRunner := newScriptRunner(outs)
	var classicEvents []Event
	classicAnswer, err := Run(context.Background(), "task", classicRunner.run, func(e Event) {
		classicEvents = append(classicEvents, e)
	})
	if err != nil {
		t.Fatalf("classic run: %v", err)
	}
	flowRunner := newScriptRunner(map[string][]string{
		"coordinator": {"plan v1"},
		"researcher":  {"research v1"},
		"builder":     {"draft v1"},
		"reviewer":    {approvedJSON("final v1")},
	})
	flowAnswer, flowEvents, err := collectFlow("task", DefaultFlow(), flowRunner)
	if err != nil {
		t.Fatalf("flow run: %v", err)
	}
	if flowAnswer != classicAnswer {
		t.Fatalf("answer differs: flow=%q classic=%q", flowAnswer, classicAnswer)
	}
	classicSigs, flowSigs := eventSigs(classicEvents), eventSigs(flowEvents)
	if strings.Join(classicSigs, "|") != strings.Join(flowSigs, "|") {
		t.Fatalf("event mismatch:\nclassic: %v\nflow:    %v", classicSigs, flowSigs)
	}
	for _, role := range Roles {
		if got := flowRunner.count(role); got != 1 {
			t.Fatalf("%s ran %d times, want 1", role, got)
		}
	}
}

func TestFlowRevisionRoundsThenFinalizes(t *testing.T) {
	r := newScriptRunner(map[string][]string{
		"coordinator": {"plan"},
		"researcher":  {"research"},
		"builder":     {"draft v1", "draft v2", "draft v3"},
		"reviewer": {
			rejectedJSON("fix it", "draft v1?"),
			rejectedJSON("fix it more", "draft v2?"),
			rejectedJSON("still off", "draft v3?"),
		},
	})
	spec := DefaultFlow()
	spec.MaxRounds = 2
	answer, events, err := collectFlow("task", spec, r)
	if err != nil {
		t.Fatalf("flow run: %v", err)
	}
	if r.count("builder") != 3 || r.count("reviewer") != 3 {
		t.Fatalf("builder=%d reviewer=%d, want 3 each (initial + 2 rounds)", r.count("builder"), r.count("reviewer"))
	}
	if !strings.Contains(answer, "Outstanding review feedback") || !strings.Contains(answer, "still off") {
		t.Fatalf("final answer must carry outstanding feedback: %q", answer)
	}
	discussions := 0
	for _, e := range events {
		if e.Type == "discussion" && e.To == "builder" {
			discussions++
		}
	}
	if discussions != 2 {
		t.Fatalf("discussions=%d, want 2", discussions)
	}
}

func TestFlowGateModeFinalizesWithoutRework(t *testing.T) {
	r := newScriptRunner(map[string][]string{
		"coordinator": {"plan"},
		"researcher":  {"research"},
		"builder":     {"draft v1"},
		"reviewer":    {rejectedJSON("nope", "draft v1?")},
	})
	spec := DefaultFlow()
	spec.MaxRounds = 0
	answer, _, err := collectFlow("task", spec, r)
	if err != nil {
		t.Fatalf("flow run: %v", err)
	}
	if r.count("builder") != 1 || r.count("reviewer") != 1 {
		t.Fatalf("gate mode must not rework: builder=%d reviewer=%d", r.count("builder"), r.count("reviewer"))
	}
	if !strings.Contains(answer, "Outstanding review feedback") {
		t.Fatalf("gate rejection must finalize with feedback: %q", answer)
	}
}

func TestFlowCustomChainSkipsResearch(t *testing.T) {
	r := newScriptRunner(map[string][]string{
		"coordinator": {"plan"},
		"builder":     {"draft"},
		"reviewer":    {approvedJSON("ship it")},
	})
	spec := FlowSpec{
		Nodes: []FlowNode{
			{ID: "coordinator", Role: "coordinator", Mode: "task", Instruction: "plan it"},
			{ID: "builder", Role: "builder", Mode: "task", Instruction: "build it"},
			{ID: "reviewer", Role: "reviewer", Mode: "review"},
		},
		Edges: []FlowEdge{
			{From: "coordinator", To: "builder", Kind: "data"},
			{From: "builder", To: "reviewer", Kind: "data"},
		},
		Output:    "reviewer",
		MaxRounds: 1,
	}
	answer, _, err := collectFlow("task", spec, r)
	if err != nil {
		t.Fatalf("flow run: %v", err)
	}
	if answer != "ship it" {
		t.Fatalf("answer=%q", answer)
	}
	if got := r.count("researcher"); got != 0 {
		t.Fatalf("skipped role ran %d times", got)
	}
}

func TestFlowHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := newScriptRunner(map[string][]string{"coordinator": {"plan"}})
	_, _, err := collectFlowWithCtx(ctx, "task", DefaultFlow(), r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}

func collectFlowWithCtx(ctx context.Context, task string, spec FlowSpec, r *scriptRunner) (string, []Event, error) {
	var events []Event
	answer, err := RunFlow(ctx, task, spec, r.run, func(e Event) {
		events = append(events, e)
	})
	return answer, events, err
}
