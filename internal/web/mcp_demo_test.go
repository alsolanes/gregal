package web

import (
	"gregal/internal/mcp"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPWebDemoPreset(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "gregal-fake.exe")
	prev := demoExecutable
	demoExecutable = func() (string, error) { return fake, nil }
	t.Cleanup(func() { demoExecutable = prev })
	s, h := mcpWebServer(t)
	w := doReq(h, "POST", "/api/mcp/servers", `{"preset":"demo"}`, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"saved":true`) {
		t.Fatalf("preset: %d %s", w.Code, w.Body.String())
	}
	sc := s.cfg.MCP["demo"]
	if !sc.ReadOnly || len(sc.Args) != 2 || sc.Args[0] != "mcp" || sc.Args[1] != "demo" || sc.Command != "gregal" {
		t.Fatalf("demo server config: %+v", sc)
	}
	// «gregal» es resol al binari actual, i un camí antic que ja no existeix
	// (app portable a una altra carpeta temporal) també.
	if got := mcp.ResolveCommand(sc.Command, sc.Args); got != fake {
		t.Fatalf("resolve gregal: %q", got)
	}
	if got := mcp.ResolveCommand(`C:\gone\TempKold\gregal.exe`, sc.Args); got != fake {
		t.Fatalf("stale demo path not repaired: %q", got)
	}
	raw, _ := os.ReadFile(s.cfgPath)
	if !strings.Contains(string(raw), "readonly: true") {
		t.Fatalf("readonly not saved: %s", raw)
	}
	if !strings.Contains(w.Body.String(), `"readonly":true`) {
		t.Errorf("status lacks readonly: %s", w.Body.String())
	}
	if w := doReq(h, "POST", "/api/mcp/servers", `{"name":"x","preset":"other"}`, true); w.Code != 400 {
		t.Errorf("unknown preset: %d", w.Code)
	}
	if w := doReq(h, "POST", "/api/mcp/servers", `{"preset":"demo"}`, false); w.Code != 401 {
		t.Errorf("without token: %d", w.Code)
	}
}
