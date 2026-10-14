package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func carregaAmb(t *testing.T, extra string) (*Config, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(strings.Replace(defaultYAML, "hooks:\n  post_edit: \"\"\n", extra, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(p)
	return c, err
}

func TestHooksCicleDeVidaEsCarreguen(t *testing.T) {
	c, err := carregaAmb(t, `hooks:
  post_edit: "gofmt -w {file}"
  stop_max_continues: 2
  pre_tool:
    - command: ./guard.sh
      matcher: "bash|write"
      timeout_s: 5
  stop:
    - command: ./check.sh
`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Hooks.PostEdit == "" || len(c.Hooks.PreTool) != 1 || c.Hooks.PreTool[0].Matcher != "bash|write" || c.Hooks.PreTool[0].TimeoutS != 5 || len(c.Hooks.Stop) != 1 || c.Hooks.StopMaxContinues != 2 {
		t.Fatalf("hooks: %+v", c.Hooks)
	}
}

func TestHooksInvalids(t *testing.T) {
	for _, bad := range []string{
		"hooks:\n  pre_tool:\n    - command: x\n      matcher: \"(\"\n",
		"hooks:\n  stop:\n    - matcher: x\n",
		"hooks:\n  user_prompt:\n    - command: x\n      timeout_s: 9999\n",
	} {
		if _, err := carregaAmb(t, bad); err == nil {
			t.Fatalf("hauria de fallar: %s", bad)
		}
	}
}

func TestValidaMCPRemot(t *testing.T) {
	if err := ValidaMCP("a", MCPServerCfg{URL: "https://mcp.example.com/mcp"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidaMCP("a", MCPServerCfg{URL: "ftp://x"}); err == nil {
		t.Fatal("url no http")
	}
	if err := ValidaMCP("a", MCPServerCfg{Transport: "sse"}); err == nil {
		t.Fatal("sse sense url")
	}
	if err := ValidaMCP("a", MCPServerCfg{Transport: "carrier-pigeon", Command: "x"}); err == nil {
		t.Fatal("transport desconegut")
	}
	if err := ValidaMCP("a", MCPServerCfg{Command: "npx"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidaMCP("a", MCPServerCfg{}); err == nil {
		t.Fatal("ni command ni url")
	}
}
