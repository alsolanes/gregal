package agent

import "testing"

// Commits i PRs: «ask» per defecte, denegats en els modes de només lectura,
// i un permissions.tools explícit mana (com a la resta d'eines).
func TestGitToolsPolicy(t *testing.T) {
	p := DefaultPolicy()
	for _, name := range []string{"git_commit", "gh_pr_create"} {
		if d, _ := p.Decide(ModeCode, name, `{}`); d != "ask" {
			t.Fatalf("%s en code=%s, volia ask", name, d)
		}
		if d, _ := p.Decide(ModeAutonomous, name, `{}`); d != "allow" {
			t.Fatalf("%s en autònom=%s, volia allow (l'autònom no pregunta)", name, d)
		}
		for _, m := range []string{ModeChat, ModeInspect, ModeGoal} {
			if d, _ := p.Decide(m, name, `{}`); d != "deny" {
				t.Fatalf("%s en %s=%s, volia deny", name, m, d)
			}
		}
	}
	allow := &Policy{Tools: map[string]string{"git_commit": "allow"}}
	if d, _ := allow.Decide(ModeCode, "git_commit", `{}`); d != "allow" {
		t.Fatalf("override allow=%s", d)
	}
	if d, _ := allow.Decide(ModeChat, "git_commit", `{}`); d != "deny" {
		t.Fatalf("override no salta el mode xat: %s", d)
	}
}
