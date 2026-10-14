package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/mcp"
)

// TestMain deixa que el binari de test faci de `gregal mcp demo` quan el
// test de sota el llança com a servidor stdio.
func TestMain(m *testing.M) {
	if os.Getenv("GREGAL_TEST_MCP_DEMO") == "1" {
		runMCPDemo()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestMCPDemoEndToEnd: el gestor MCP connecta el servidor de demo pel
// procés real, en registra les eines com a només lectura i les crida.
func TestMCPDemoEndToEnd(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	m := mcp.Setup(map[string]mcp.Server{"demo": {Command: exe, Env: map[string]string{"GREGAL_TEST_MCP_DEMO": "1"}, ReadOnly: true}})
	t.Logf("connect + initialize + tools/list: %s", time.Since(t0))
	defer func() {
		m.Remove("demo")
		m.Close()
	}()
	if errs := m.ErrorsSnapshot(); len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	names := map[string]bool{}
	for _, s := range m.SpecsOf() {
		names[s.Name] = true
	}
	for _, want := range []string{"mcp_demo_list_tables", "mcp_demo_describe_table", "mcp_demo_query"} {
		if !names[want] {
			t.Fatalf("missing %s in %v", want, names)
		}
		if !agent.IsReadOnlyMCP(want) {
			t.Errorf("%s not marked read-only", want)
		}
		if d, why := (&agent.Policy{}).Decide(agent.ModeCode, want, `{}`); d != "allow" {
			t.Errorf("%s policy = %s (%s)", want, d, why)
		}
	}
	out, err := m.CallTool(context.Background(), "demo", "query", `{"table":"orders","select":["year(order_date) as year","count(*) as orders"],"group_by":["year(order_date)"]}`)
	if err != nil || !strings.Contains(out, `"columns":["year","orders"]`) || !strings.Contains(out, `"2025"`) {
		t.Fatalf("query: %s %v", out, err)
	}
	if len(m.Prompts()) != 2 {
		t.Errorf("prompts: %v", m.Prompts())
	}
	readme, err := m.ResourceRead(context.Background(), "demo", "demo://schema")
	if err != nil || !strings.Contains(readme, "marketing_spend") {
		t.Errorf("schema resource: %v", err)
	}
	m.Remove("demo")
	if agent.IsReadOnlyMCP("mcp_demo_query") {
		t.Error("read-only flag survives removal")
	}
}
