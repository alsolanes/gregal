package team

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// The runner learns which seat it serves (role, node, subagent) from the
// context: that is how the web gives each role its own tools.
func TestRunnerSeesItsNode(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]FlowNode{}
	runner := func(ctx context.Context, name, _ string) (string, error) {
		node, ok := NodeFrom(ctx)
		if !ok {
			t.Errorf("%s: no node in context", name)
		}
		mu.Lock()
		seen[node.ID] = node
		mu.Unlock()
		if node.Role == "reviewer" {
			return `{"approved":true,"feedback":"","deliverable":"final"}`, nil
		}
		return "out", nil
	}
	if _, err := Run(context.Background(), "task", runner, nil); err != nil {
		t.Fatal(err)
	}
	for _, role := range Roles {
		if seen[role].Role != role {
			t.Fatalf("classic run: %s seat=%+v", role, seen[role])
		}
	}
	spec := FlowSpec{
		Nodes:  []FlowNode{{ID: "scout", Role: "researcher", Agent: "my-scout"}, {ID: "judge", Role: "reviewer", Mode: "review"}},
		Edges:  []FlowEdge{{From: "scout", To: "judge", Kind: "data"}},
		Output: "judge",
	}
	seen = map[string]FlowNode{}
	if _, err := RunFlow(context.Background(), "task", spec, runner, nil); err != nil {
		t.Fatal(err)
	}
	if seen["scout"].Agent != "my-scout" || seen["judge"].Mode != "review" {
		t.Fatalf("flow seats=%+v", seen)
	}
	raw, _ := json.Marshal(spec.Nodes[0])
	var back FlowNode
	if json.Unmarshal(raw, &back) != nil || back.Agent != "my-scout" {
		t.Fatalf("agent field must round-trip: %s", raw)
	}
}
