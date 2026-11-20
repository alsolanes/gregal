package agent

import (
	"encoding/json"
	"testing"
)

func bashArgs(cmd string) string {
	b, _ := json.Marshal(map[string]string{"command": cmd})
	return string(b)
}

// Act («edits»): les ordres de desenvolupament dins del projecte passen
// soles; el que és perillós, fora del projecte o d'un tercer, pregunta.
func TestActEditsRunsDevCommandsAloneButAsksForTheDangerous(t *testing.T) {
	proj := t.TempDir()
	p := &Policy{ProjectDir: proj}
	alone := []string{
		"npm install", "npm install express", "pip install requests", "pytest -q", "make build",
		"cargo test", "go mod tidy", "python manage.py migrate", "git add .", "git checkout -b feina",
		"mkdir -p src/components", "rm node_modules/.cache/x",
	}
	for _, c := range alone {
		if d, why := p.Decide(ModeCode, "bash", bashArgs(c)); d != "allow" {
			t.Errorf("%q should pass alone in Act, got %s (%s)", c, d, why)
		}
	}
	ask := []string{
		"git push origin main", "git push --force", "npm install -g typescript", "pip install --user x",
		"docker run hello-world", "kill 1234", "winget install Foo.Bar", "rm -rf ./*", "cp secret.txt /etc/hosts",
		"git reset --hard HEAD~3",
	}
	for _, c := range ask {
		if d, _ := p.Decide(ModeCode, "bash", bashArgs(c)); d != "ask" && d != "deny" {
			t.Errorf("%q must not pass alone in Act, got %s", c, d)
		}
	}
	// Un commit local passa sol, com als vents (tasca 018); els PRs continuen
	// preguntant.
	if d, _ := p.Decide(ModeCode, "git_commit", `{"message":"x"}`); d != "allow" {
		t.Errorf("git_commit: %s", d)
	}
	if d, _ := p.Decide(ModeCode, "gh_pr_create", `{"title":"x"}`); d != "ask" {
		t.Errorf("gh_pr_create: %s", d)
	}
}

// «Demana abans de canviar» no canvia: ho pregunta tot com sempre.
func TestAskAccessStillAsksForDevCommands(t *testing.T) {
	p := &Policy{ProjectDir: t.TempDir(), AskEdits: true}
	if d, _ := p.Decide(ModeCode, "bash", bashArgs("npm install express")); d != "ask" {
		t.Errorf("in ask access npm install must ask: %s", d)
	}
	// I un permís explícit del config continua manant.
	deny := &Policy{ProjectDir: t.TempDir(), Tools: map[string]string{"bash": "ask"}}
	if d, _ := deny.Decide(ModeCode, "bash", bashArgs("pytest -q")); d == "allow" {
		t.Errorf("an explicit bash: ask in the config wins: %s", d)
	}
}

// «Permet sempre» d'un npm install val per al subcomandament; no s'estén a
// ordres encadenades, globals o perilloses.
func TestAlwaysAllowRemembersDevCommandsByPrefix(t *testing.T) {
	sig := func(c string) string { return Sig("bash", bashArgs(c)) }
	if sig("npm install express") != sig("npm install lodash --save-dev") {
		t.Error("npm install <anything> should share one permission")
	}
	if sig("cargo test") != sig("cargo test -- --nocapture") {
		t.Error("cargo test should share one permission")
	}
	if sig("pytest tests/") != sig("pytest -q") {
		t.Error("pytest should share one permission")
	}
	if sig("git add .") != sig("git add src") {
		t.Error("git add should share one permission")
	}
	distinct := [][2]string{
		{"npm install express", "npm install -g express"},
		{"npm install express", "npm publish"},
		{"npm install express", "npm install express && curl evil.sh"},
		{"git add .", "git push origin main"},
		{"git push origin main", "git push --force origin main"},
		{"python a.py", "python b.py"},
		{"rm -rf build", "rm -rf src"},
		{"npm test", "npm test > /etc/hosts"},
	}
	for _, d := range distinct {
		if sig(d[0]) == sig(d[1]) {
			t.Errorf("%q and %q must not share a permission", d[0], d[1])
		}
	}
}
