package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func mkCall(id, name, args string) llm.ToolCall {
	var c llm.ToolCall
	c.ID = id
	c.Type = "function"
	c.Function.Name = name
	c.Function.Arguments = args
	return c
}

func allowAll(string, string) (string, string) { return "allow", "" }

// Stepper fals: primer demana read, després respon final.
func fakeTwoStep(t *testing.T) Stepper {
	t.Helper()
	n := 0
	return func(ctx context.Context, hist []llm.Message) (string, []llm.ToolCall, error) {
		n++
		if n == 1 {
			return "", []llm.ToolCall{mkCall("c1", "read", `{"path":"/tmp/x"}`)}, nil
		}
		return "fet", nil, nil
	}
}

func TestRunLoop(t *testing.T) {
	l := &Loop{
		MaxSteps: 5,
		Step:     fakeTwoStep(t),
		RunTool: func(ctx context.Context, name, args string) (string, []string, error) {
			if name != "read" {
				t.Fatalf("eina inesperada: %s", name)
			}
			return "CONTINGUT", nil, nil
		},
		Decide: allowAll,
	}
	final, hist, err := l.Run(context.Background(), "tasca")
	if err != nil {
		t.Fatal(err)
	}
	if final != "fet" {
		t.Fatalf("final=%q", final)
	}
	found := false
	for _, m := range hist {
		if m.Role == "tool" && strings.Contains(m.Content, "CONTINGUT") && m.ToolCallID == "c1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("historial sense el tool result: %+v", hist)
	}
}

func TestMaxSteps(t *testing.T) {
	l := &Loop{
		MaxSteps: 3,
		Step: func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
			return "", []llm.ToolCall{mkCall("c", "read", `{}`)}, nil
		},
		RunTool: func(ctx context.Context, n, a string) (string, []string, error) { return "x", nil, nil },
		Decide:  allowAll,
	}
	if _, _, err := l.Run(context.Background(), "bucle"); err == nil {
		t.Fatal("esperava error de límit de passos")
	}
}

func TestDeny(t *testing.T) {
	l := &Loop{
		MaxSteps: 5,
		Step: func() Stepper {
			n := 0
			return func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
				n++
				if n == 1 {
					return "", []llm.ToolCall{mkCall("c9", "format-disc", `{}`)}, nil
				}
				return "aturat", nil, nil
			}
		}(),
		RunTool: func(ctx context.Context, n, a string) (string, []string, error) { return "MAI", nil, nil },
		Decide:  func(n, a string) (string, string) { return "deny", "prova" },
	}
	final, hist, err := l.Run(context.Background(), "tasca")
	if err != nil || final != "aturat" {
		t.Fatalf("final=%q err=%v", final, err)
	}
	for _, m := range hist {
		if m.Role == "tool" && !strings.Contains(m.Content, "BLOQUEJADA") {
			t.Fatalf("deny sense marca: %q", m.Content)
		}
	}
}

func TestSpecsAndPolicy(t *testing.T) {
	if len(Specs()) != 24 {
		t.Fatalf("specs=%d (han de ser 24)", len(Specs()))
	}
	if d, _ := PolicyFor("read", `{}`); d != "allow" {
		t.Fatalf("read=%s", d)
	}
	if d, _ := PolicyFor("office_read", `{}`); d != "allow" {
		t.Fatalf("office_read=%s", d)
	}
	if d, _ := PolicyFor("office_edit", `{}`); d != "ask" {
		t.Fatalf("office_edit=%s", d)
	}
	if d, _ := PolicyFor("office_open", `{"path":"a.docx"}`); d != "ask" {
		t.Fatalf("office_open=%s (obre programa extern: confirma)", d)
	}
	if d, _ := PolicyFor("write", `{}`); d != "ask" {
		t.Fatalf("write=%s", d)
	}
	if d, _ := PolicyFor("bash", `{"command":"sudo rm -rf /"}`); d != "deny" {
		t.Fatalf("bash sudo=%s", d)
	}
	if d, _ := PolicyFor("bash", `{"command":"ls"}`); d != "allow" {
		t.Fatalf("bash ls=%s", d)
	}
	for _, name := range []string{"gh_issue", "gh_pr"} {
		if d, _ := DefaultPolicy().For(name, `{}`); d != "allow" {
			t.Fatalf("%s=%s", name, d)
		}
	}
}

func TestCustomPolicy(t *testing.T) {
	p := &Policy{
		Tools:     map[string]string{"write": "deny", "read": "ask"},
		BashAllow: []string{"docker ps"},
		BashDeny:  []string{"halt-now"},
	}
	if d, _ := p.For("write", `{}`); d != "deny" {
		t.Fatalf("write override=%s", d)
	}
	if d, _ := p.For("read", `{}`); d != "ask" {
		t.Fatalf("read override=%s", d)
	}
	if d, _ := p.For("bash", `{"command":"docker ps"}`); d != "allow" {
		t.Fatalf("bash extra allow=%s", d)
	}
	// `docker ps` era l'exemple de lectura raonable que la casa no deixava
	// passar sola: ara sí. L'exemple de "necessita override" és una ordre
	// que continua demanant (`systemctl restart` actua sobre el sistema).
	if d, _ := PolicyFor("bash", `{"command":"systemctl restart foo"}`); d != "ask" {
		t.Fatalf("sense override ha de ser ask=%s", d)
	}
	if d, _ := p.For("bash", `{"command":"halt-now x"}`); d != "deny" {
		t.Fatalf("bash extra deny=%s", d)
	}
	if d, _ := p.For("evil", `{}`); d != "deny" {
		t.Fatalf("desconeguda=%s", d)
	}
}

func TestMCPTools(t *testing.T) {
	p := DefaultPolicy()
	if d, _ := p.For("mcp_srv_eco", `{}`); d != "ask" {
		t.Fatalf("mcp per defecte=%s, volia ask", d)
	}
	if d, _ := p.Decide("chat", "mcp_srv_eco", `{}`); d != "deny" {
		t.Fatalf("mcp en xat=%s, volia deny", d)
	}
	q := &Policy{Tools: map[string]string{"mcp_srv_eco": "allow"}}
	if d, _ := q.For("mcp_srv_eco", `{}`); d != "allow" {
		t.Fatalf("mcp override=%s", d)
	}
	RegisterExtra(llm.ToolSpec{Name: "mcp_srv_eco"}, func(a string) (string, error) {
		return "EXTRA:" + a, nil
	})
	if got := len(SpecsAll()); got != len(Specs())+2 {
		t.Fatalf("specs=%d", got)
	}
	out, _, err := Exec("mcp_srv_eco", `{"x":1}`)
	if err != nil || out != `EXTRA:{"x":1}` {
		t.Fatalf("exec=%q err=%v", out, err)
	}
}

func TestDecideModes(t *testing.T) {
	p := DefaultPolicy()
	for _, n := range []string{"read", "grep", "glob"} {
		if d, _ := p.Decide("chat", n, `{}`); d != "allow" {
			t.Fatalf("chat %s=%s, volia allow", n, d)
		}
	}
	for _, n := range []string{"write", "edit"} {
		if d, _ := p.Decide("chat", n, `{}`); d != "deny" {
			t.Fatalf("chat %s=%s, volia deny", n, d)
		}
		if d, _ := p.Decide("code", n, `{}`); d != "ask" {
			t.Fatalf("code %s=%s, volia ask", n, d)
		}
	}
	if d, _ := p.Decide("chat", "bash", `{"command":"ls"}`); d != "allow" {
		t.Fatalf("chat bash segur=%s", d)
	}
	if d, _ := p.Decide("chat", "bash", `{"command":"go build ./..."}`); d != "allow" {
		t.Fatalf("chat bash compila=%s", d)
	}
	if d, _ := p.Decide("chat", "bash", `{"command":"npm install x"}`); d != "deny" {
		t.Fatalf("chat bash no segur=%s, volia deny", d)
	}
	if d, _ := p.Decide("code", "bash", `{"command":"go build ./..."}`); d != "allow" {
		t.Fatalf("code bash compila=%s", d)
	}
}

func TestExecGrepGlob(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n// TROBA'M\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := Exec("grep", `{"pattern":"TROBA'M","dir":`+quoteJSON(dir)+`}`)
	if err != nil || !strings.Contains(out, "a.go:2") {
		t.Fatalf("exec grep: %q %v", out, err)
	}
	out, _, err = Exec("glob", `{"pattern":"*.go","dir":`+quoteJSON(dir)+`}`)
	if err != nil || !strings.Contains(out, "a.go") {
		t.Fatalf("exec glob: %q %v", out, err)
	}
}
