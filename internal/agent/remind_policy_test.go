package agent

import "testing"

// remind (l'eina d'avisos dels vents) no és nativa ni MCP: la política d'un
// torn interactiu no la pot tractar com a desconeguda.
func TestPolicyAllowsServerRemind(t *testing.T) {
	var p *Policy
	if d, why := p.For("remind", `{"wind":"x","at":"+1h","text":"hola"}`); d != "allow" {
		t.Fatalf("remind = %s (%s)", d, why)
	}
	if d, _ := p.For("una_eina_inventada", `{}`); d != "deny" {
		t.Fatalf("una eina desconeguda s'ha de denegar, no %s", d)
	}
}
