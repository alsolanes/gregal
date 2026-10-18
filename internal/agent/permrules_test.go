package agent

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"gregal/internal/config"
)

func rulesPolicy(t *testing.T, y string) config.PermissionsCfg {
	t.Helper()
	var p config.PermissionsCfg
	if err := yaml.Unmarshal([]byte(y), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGlobMatch(t *testing.T) {
	for _, tc := range []struct {
		pat, s string
		want   bool
	}{
		{"git *", "git status", true},
		{"git *", "git", false},
		{"git status", "git status -s", false},
		{"*", "", true},
		{"docs/**", "docs/a/b.md", true},
		{"docs/**", "src/docs/a.md", false},
		{"*.env", ".env", true},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
	} {
		if got := globMatch(tc.pat, tc.s); got != tc.want {
			t.Errorf("globMatch(%q,%q)=%v", tc.pat, tc.s, got)
		}
	}
}

func TestBashGlobRules(t *testing.T) {
	cfg := rulesPolicy(t, `
tools:
  bash:
    "git *": ask
    "git status*": allow
    "npm test*": allow
    "rm *": deny
`)
	p := PolicyFromConfig(&config.Config{Permissions: cfg}, "")
	for _, tc := range []struct{ cmd, want string }{
		{"git status -s", "allow"},
		{"git push origin x", "ask"},
		{"git log", "ask"},
		{"npm test", "allow"},
		{"npm test && make deploy", "ask"}, // second stage has no rule
		{"git status > ~/.bashrc", "ask"},  // redirect hides a write: allow is not granted
		{"npm test $(whoami)", "ask"},      // substitution
		{"rm -f x", "deny"},
		{"git status; rm -f x", "deny"},
		{"sudo git status", "deny"}, // hard denial of the classifier stays
	} {
		args := `{"command":` + jsonQuote(tc.cmd) + `}`
		got, _ := p.Decide(ModeCode, "bash", args)
		if got != tc.want {
			t.Errorf("%q: got %s want %s", tc.cmd, got, tc.want)
		}
	}
}

func TestBashAllowRuleNeverBeatsHardDenial(t *testing.T) {
	p := PolicyFromConfig(&config.Config{Permissions: rulesPolicy(t, "rules:\n  bash:\n    \"*\": allow\n")}, "")
	if d, _ := p.Decide(ModeCode, "bash", `{"command":"rm -rf /"}`); d != "deny" {
		t.Fatalf("hard denial relaxed: %s", d)
	}
	if d, _ := p.Decide(ModeAutonomous, "bash", `{"command":"sudo ls"}`); d != "deny" {
		t.Fatalf("hard denial relaxed in autonomous: %s", d)
	}
}

func TestPathRulesAndAutonomous(t *testing.T) {
	dir := t.TempDir()
	cfg := rulesPolicy(t, `
rules:
  edit:
    "docs/**": allow
    "*.env": deny
    "src/**": ask
`)
	p := PolicyFromConfig(&config.Config{Permissions: cfg}, dir)
	if d, _ := p.Decide(ModeCode, "edit", `{"path":"docs/a.md"}`); d != "allow" {
		t.Errorf("docs: %s", d)
	}
	if d, _ := p.Decide(ModeCode, "edit", `{"path":"config/prod.env"}`); d != "deny" {
		t.Errorf("env: %s", d)
	}
	// Inside the project writes pass by default; a rule can turn that into ask.
	if d, _ := p.Decide(ModeCode, "edit", `{"path":"src/x.go"}`); d != "ask" {
		t.Errorf("src ask: %s", d)
	}
	// Autonomous never asks, but a deny rule still denies.
	if d, _ := p.Decide(ModeAutonomous, "edit", `{"path":"src/x.go"}`); d != "allow" {
		t.Errorf("autonomous ask must pass: %s", d)
	}
	if d, _ := p.Decide(ModeAutonomous, "edit", `{"path":"a/b.env"}`); d != "deny" {
		t.Errorf("autonomous deny rule: %s", d)
	}
	// Read-only modes cannot be widened by an allow rule.
	if d, _ := p.Decide(ModeInspect, "edit", `{"path":"docs/a.md"}`); d != "deny" {
		t.Errorf("inspect: %s", d)
	}
	// Absolute paths inside the project match relative patterns.
	if d, _ := p.Decide(ModeCode, "edit", `{"path":`+jsonQuote(dir+"/x/a.env")+`}`); d != "deny" {
		t.Errorf("abs env: %s", d)
	}
}

func TestAutonomousBashAskRuleStillRuns(t *testing.T) {
	p := PolicyFromConfig(&config.Config{Permissions: rulesPolicy(t, "rules:\n  bash:\n    \"git *\": ask\n")}, "")
	if d, _ := p.Decide(ModeAutonomous, "bash", `{"command":"git log"}`); d != "allow" {
		t.Fatalf("autonomous never asks: %s", d)
	}
	if d, _ := p.For("bash", `{"command":"git log"}`); d != "ask" {
		t.Fatalf("For: %s", d)
	}
}

func TestSubagentPermissions(t *testing.T) {
	cfg := rulesPolicy(t, `
tools:
  bash: ask
rules:
  edit:
    "docs/**": allow
subagents:
  explorer:
    tools:
      bash:
        "git log*": allow
      edit: deny
  reviewer:
    rules:
      edit:
        "docs/secret/**": deny
`)
	root := PolicyFromConfig(&config.Config{Permissions: cfg}, "")
	if d, _ := root.For("edit", `{"path":"docs/a.md"}`); d != "allow" {
		t.Fatalf("root edit: %s", d)
	}
	ex := subagentPolicy(cfg, "explorer")
	if d, _ := ex.For("edit", `{"path":"docs/a.md"}`); d != "deny" {
		t.Fatalf("explorer tool-level deny must replace general rules: %s", d)
	}
	if d, _ := ex.For("bash", `{"command":"git log -3"}`); d != "allow" {
		t.Fatalf("explorer bash: %s", d)
	}
	if d, _ := ex.For("bash", `{"command":"go build"}`); d != "ask" {
		t.Fatalf("explorer bash fallback: %s", d)
	}
	rv := subagentPolicy(cfg, "reviewer")
	if d, _ := rv.For("edit", `{"path":"docs/secret/k.md"}`); d != "deny" {
		t.Fatalf("reviewer: %s", d)
	}
	if d, _ := rv.For("edit", `{"path":"docs/a.md"}`); d != "allow" {
		t.Fatalf("reviewer inherits: %s", d)
	}
	// The general policy is not mutated by building a subagent's.
	if d, _ := root.For("edit", `{"path":"docs/secret/k.md"}`); d != "allow" {
		t.Fatalf("root mutated: %s", d)
	}
}

func TestPermissionsYAMLRoundTripAndValidate(t *testing.T) {
	cfg := rulesPolicy(t, "tools:\n  read: allow\n  bash:\n    \"git *\": ask\n    \"git push*\": deny\n")
	if cfg.Tools["read"] != "allow" || len(cfg.Tools) != 1 {
		t.Fatalf("tools: %v", cfg.Tools)
	}
	r := cfg.Rules["bash"]
	if len(r) != 2 || r[0].Pattern != "git *" || r[1].Decision != "deny" {
		t.Fatalf("order not kept: %v", r)
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back config.PermissionsCfg
	if err := yaml.Unmarshal(out, &back); err != nil || len(back.Rules["bash"]) != 2 || back.Rules["bash"][1].Pattern != "git push*" {
		t.Fatalf("round trip: %v %s", err, out)
	}
}

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
