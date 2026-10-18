package mcpimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (home, cwd string) {
	home, cwd = t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	jcwd, _ := jsonPath(cwd)
	write(t, filepath.Join(home, ".claude.json"), `{
  "mcpServers": {
    "github": {"type":"http","url":"https://api.example.com/mcp?api_key=SECRET123","headers":{"Authorization":"Bearer ghp_SECRETTOKEN"}},
    "fs": {"command":"npx","args":["-y","@mcp/fs","--token","TOPSECRET","/data"],"env":{"API_KEY":"sk-SECRET","MODE":"ro"}}
  },
  "projects": {"`+jcwd+`": {"mcpServers": {"local db": {"command":"db-mcp"}}}}
}`)
	write(t, filepath.Join(cwd, ".mcp.json"), `{"mcpServers":{"fs":{"command":"other"}}}`)
	write(t, filepath.Join(home, ".codex", "config.toml"), `
model = "gpt-5"  # not ours

[mcp_servers.context7]
command = "npx"
args = [
  "-y",   # comment
  "@upstash/context7-mcp",
]
env = { TOKEN = "tok-SECRET", PLAIN = "x" }

[mcp_servers.remote]
url = "https://mcp.example.com/sse"
bearer_token_env_var = "REMOTE_TOKEN"

[mcp_servers.remote.http_headers]
"X-Team" = "blue"

[mcp_servers.off]
command = "off-mcp"
enabled = false

[mcp_servers.withenv]
command = "x"
[mcp_servers.withenv.env]
SECRET_A = "aaa"
`)
	write(t, filepath.Join(home, ".config", "opencode", "opencode.json"), `{
  // comment
  "mcp": {
    "sentry": {"type":"remote","url":"https://mcp.sentry.dev/mcp","oauth":{"clientId":"cid","clientSecret":"csecret","scope":"a b"},},
    "loc": {"type":"local","command":["uvx","srv","--flag"],"environment":{"K":"v"}},
    "dis": {"type":"local","command":["x"],"enabled":false}
  }
}`)
	return home, cwd
}

func jsonPath(p string) (string, error) {
	return strings.ReplaceAll(p, `\`, `\\`), nil
}

func byName(list []Found, n string) (Found, bool) {
	for _, f := range list {
		if f.Name == n {
			return f, true
		}
	}
	return Found{}, false
}

func TestDiscoverAllSources(t *testing.T) {
	home, cwd := fixture(t)
	res := Discover(home, cwd)
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	gh, ok := byName(res.Found, "github")
	if !ok || gh.Cfg.Transport != "http" || gh.Cfg.Headers["Authorization"] == "" || gh.Origin != ClaudeCode {
		t.Fatalf("github: %+v", gh)
	}
	if _, ok := byName(res.Found, "local-db"); !ok {
		t.Fatal("project-scoped server of ~/.claude.json missing (or its name was not cleaned)")
	}
	c7, ok := byName(res.Found, "context7")
	if !ok || c7.Cfg.Command != "npx" || len(c7.Cfg.Args) != 2 || c7.Cfg.Env["PLAIN"] != "x" {
		t.Fatalf("codex stdio: %+v", c7)
	}
	rm, ok := byName(res.Found, "remote")
	if !ok || rm.Cfg.URL == "" || rm.Cfg.Headers["Authorization"] != "Bearer ${REMOTE_TOKEN}" || rm.Cfg.Headers["X-Team"] != "blue" {
		t.Fatalf("codex remote: %+v", rm)
	}
	if off, _ := byName(res.Found, "off"); !off.Disabled {
		t.Fatal("codex enabled=false not read")
	}
	sn, ok := byName(res.Found, "sentry")
	if !ok || sn.Cfg.OAuth == nil || sn.Cfg.OAuth.ClientID != "cid" || len(sn.Cfg.OAuth.Scopes) != 2 || len(sn.Notes) == 0 {
		t.Fatalf("opencode remote: %+v", sn)
	}
	lc, ok := byName(res.Found, "loc")
	if !ok || lc.Cfg.Command != "uvx" || len(lc.Cfg.Args) != 2 || lc.Cfg.Env["K"] != "v" {
		t.Fatalf("opencode local: %+v", lc)
	}
}

func TestFromFilterAndPriority(t *testing.T) {
	home, cwd := fixture(t)
	only := Discover(home, cwd, CodexOnly()...)
	for _, f := range only.Found {
		if f.Origin != Codex {
			t.Fatalf("filter leaked %s", f.Origin)
		}
	}
	// "fs" is defined in the project .mcp.json and in ~/.claude.json: the project wins.
	plan := MakePlan(nil, Discover(home, cwd).Found)
	fs, _ := byName(plan.Add, "fs")
	if fs.Cfg.Command != "other" {
		t.Fatalf("project definition must win, got %+v", fs.Cfg)
	}
	if len(plan.Duplicate) != 1 {
		t.Fatalf("duplicates: %+v", plan.Duplicate)
	}
}

func CodexOnly() []string { return []string{Codex} }

func TestPlanAgainstExisting(t *testing.T) {
	home, cwd := fixture(t)
	existing := map[string]config.MCPServerCfg{
		"context7": {Command: "npx", Args: []string{"-y", "@upstash/context7-mcp"}, Env: map[string]string{"TOKEN": "tok-SECRET", "PLAIN": "x"}},
		"loc":      {Command: "different"},
	}
	plan := MakePlan(existing, Discover(home, cwd).Found)
	if _, ok := byName(plan.Present, "context7"); !ok {
		t.Fatal("identical server must be reported as present")
	}
	if _, ok := byName(plan.Conflicts, "loc"); !ok {
		t.Fatal("same name, different definition must be a conflict")
	}
	if _, ok := byName(plan.Disabled, "off"); !ok {
		t.Fatal("disabled must not be added")
	}
	if _, ok := byName(plan.Add, "off"); ok {
		t.Fatal("disabled was added")
	}
	cfg := &config.Config{MCP: existing}
	added := Apply(cfg, plan, nil)
	if len(added) == 0 || cfg.MCP["loc"].Command != "different" {
		t.Fatalf("apply: %v / conflict overwritten: %+v", added, cfg.MCP["loc"])
	}
	for n, s := range cfg.MCP {
		if err := config.ValidaMCP(n, s); err != nil {
			t.Fatalf("%s: %v", n, err)
		}
	}
	cfg2 := &config.Config{}
	if got := Apply(cfg2, plan, []string{"sentry"}); len(got) != 1 || len(cfg2.MCP) != 1 {
		t.Fatalf("only filter: %v", got)
	}
}

func TestReportNeverShowsSecrets(t *testing.T) {
	home, cwd := fixture(t)
	res := Discover(home, cwd)
	out := Report(res, MakePlan(nil, res.Found))
	for _, secret := range []string{"SECRET123", "ghp_SECRETTOKEN", "TOPSECRET", "sk-SECRET", "tok-SECRET", "csecret", "aaa"} {
		if strings.Contains(out, secret) {
			t.Errorf("report leaks %q:\n%s", secret, out)
		}
	}
	for _, want := range []string{"API_KEY=***", "${REMOTE_TOKEN}", "--token ***", "github", "context7"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

func TestBrokenFileIsReportedNotFatal(t *testing.T) {
	home, cwd := t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	write(t, filepath.Join(home, ".claude.json"), `{not json`)
	write(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.a]\ncommand = \"x\"\n")
	res := Discover(home, cwd)
	if len(res.Errors) != 1 || len(res.Found) != 1 {
		t.Fatalf("%+v", res)
	}
}

func TestTOMLBasics(t *testing.T) {
	root := parseTOML("a = 1\n[x.\"y z\"]\nk = 'lit'\nl = [\"a\", \"b\"]\nt = { u = \"v\", w = true }\n")
	x := root["x"].(tomlTable)["y z"].(tomlTable)
	if x["k"] != "lit" || len(tomlStrings(x["l"])) != 2 || tomlStringMap(x["t"])["u"] != "v" {
		t.Fatalf("%v", root)
	}
}
