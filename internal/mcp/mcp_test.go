package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeServer és un servidor MCP mínim (initialize, tools/list, tools/call).
const fakeServer = `import sys, json
def send(o): sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()
tools = [{"name": "eco", "description": "Torna el text rebut",
          "inputSchema": {"type": "object", "properties": {"text": {"type": "string"}}}}]
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    try: req = json.loads(line)
    except Exception: continue
    rid, m = req.get("id"), req.get("method")
    if m == "initialize":
        send({"jsonrpc": "2.0", "id": rid, "result": {"protocolVersion": "2024-11-05", "capabilities": {}, "serverInfo": {"name": "fals", "version": "0"}}})
    elif m == "tools/list":
        send({"jsonrpc": "2.0", "id": rid, "result": {"tools": tools}})
    elif m == "tools/call":
        txt = req["params"]["arguments"].get("text", "")
        send({"jsonrpc": "2.0", "id": rid, "result": {"content": [{"type": "text", "text": "ECO:" + str(txt)}]}})
    elif rid is not None:
        send({"jsonrpc": "2.0", "id": rid, "error": {"code": -32601, "message": "nope"}})
`

func writeFake(t *testing.T) string {
	t.Helper()
	// LookPath no basta: a Windows hi ha un python3.exe de mentida (l'àlies
	// d'execució de la Store) que existeix i només et diu que instal·lis
	// Python. Comprovem que de debò executa alguna cosa.
	if err := exec.Command("python3", "-c", "print(1)").Run(); err != nil {
		t.Skip("sense python3 utilitzable")
	}
	p := filepath.Join(t.TempDir(), "fals.py")
	if err := os.WriteFile(p, []byte(fakeServer), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDialListCall(t *testing.T) {
	p := writeFake(t)
	c, err := Dial("prova", Server{Command: "python3", Args: []string{p}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	tools, err := c.Tools()
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Spec.Name != "mcp_prova_eco" {
		t.Fatalf("tools=%+v", tools)
	}
	out, err := tools[0].Call(`{"text":"hola"}`)
	if err != nil || out != "ECO:hola" {
		t.Fatalf("call=%q err=%v", out, err)
	}
}

func TestSetupRegisters(t *testing.T) {
	p := writeFake(t)
	m := Setup(map[string]Server{"prova": {Command: "python3", Args: []string{p}}})
	defer m.Close()
	if len(m.Errors) != 0 {
		t.Fatalf("errors=%v", m.Errors)
	}
	if !strings.Contains(m.Summary(), "1 eina") {
		t.Fatalf("summary=%q", m.Summary())
	}
	if len(m.SpecsOf()) != 1 {
		t.Fatalf("specs=%v", m.SpecsOf())
	}
}

func TestSetupError(t *testing.T) {
	m := Setup(map[string]Server{"trencat": {Command: "/no/existeix"}})
	defer m.Close()
	if len(m.Errors) != 1 {
		t.Fatalf("errors=%v", m.Errors)
	}
	if !strings.Contains(m.Summary(), "1 error") {
		t.Fatalf("summary=%q", m.Summary())
	}
}
