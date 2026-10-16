package agent

import (
	"fmt"
	"testing"
)

func TestReadOnlyMCPPolicy(t *testing.T) {
	SetReadOnlyMCPTools([]string{"mcp_demo_query"})
	t.Cleanup(func() { SetReadOnlyMCPTools(nil) })
	p := &Policy{}
	for _, mode := range []string{ModeCode, ModeChat, ModeInspect, ModeGoal, ModeAutonomous} {
		if d, why := p.Decide(mode, "mcp_demo_query", `{"table":"orders"}`); d != "allow" {
			t.Errorf("%s: read-only MCP tool = %s (%s), want allow", mode, d, why)
		}
	}
	if d, _ := p.Decide(ModeCode, "mcp_other_write", `{}`); d != "ask" {
		t.Errorf("other MCP tool in code = %s, want ask", d)
	}
	if d, _ := p.Decide(ModeChat, "mcp_other_write", `{}`); d != "deny" {
		t.Errorf("other MCP tool in chat = %s, want deny", d)
	}
	if d, _ := p.Decide(ModeAutonomous, "mcp_other_write", `{}`); d != "allow" {
		t.Errorf("other MCP tool in autonomous = %s, want allow (autonomous never asks)", d)
	}
	// Un permissions.tools explícit continua manant.
	deny := &Policy{Tools: map[string]string{"mcp_demo_query": "deny"}}
	if d, _ := deny.Decide(ModeCode, "mcp_demo_query", `{}`); d != "deny" {
		t.Errorf("config deny ignored: %s", d)
	}
	SetReadOnlyMCPTools(nil)
	if d, _ := p.Decide(ModeCode, "mcp_demo_query", `{}`); d != "ask" {
		t.Errorf("after reset = %s, want ask", d)
	}
}

// Consultes MCP diferents seguides no són un bucle; la mateixa, sí.
func TestDoomMCPDistinctQueries(t *testing.T) {
	d := &DoomTracker{}
	base := `{"table":"orders","select":["month(order_date) as month","sum(revenue) as revenue"],"filters":[{"col":"status","op":"=","value":"completed"}],"group_by":["%s"]}`
	for _, g := range []string{"month(order_date)", "products.category", "regions.name", "customers.segment", "channel"} {
		if d.Note(DoomSig("mcp_demo_query", fmt.Sprintf(base, g))) {
			t.Fatalf("distinct query %s flagged as repeated", g)
		}
	}
	same := DoomSig("mcp_demo_query", fmt.Sprintf(base, "channel"))
	d.Note(same)
	if !d.Note(same) {
		t.Error("third identical MCP call not flagged")
	}
}
