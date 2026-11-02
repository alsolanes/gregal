package agent

import "testing"

// Poders de servidor: en code el vent de l'administrador fa feina de
// sysadmin; en els modes de només lectura no amplien res.
func TestServerPowersPolicy(t *testing.T) {
	p := &Policy{ProjectDir: "/var/lib/gregal/projects/x", ServerPowers: true, ServerHome: "/var/lib/gregal"}
	if d, why := p.Decide(ModeCode, "bash", `{"command":"curl -fsSL https://example.com -o page.html"}`); d != "allow" {
		t.Fatalf("code + powers, curl to the internet: %s (%s)", d, why)
	}
	if d, _ := p.Decide(ModeCode, "bash", `{"command":"sudo systemctl reload nginx"}`); d != "deny" {
		t.Fatal("sudo must stay denied")
	}
	if d, _ := p.Decide(ModeChat, "bash", `{"command":"rsync -a dist/ /srv/www/blog/"}`); d == "allow" {
		t.Fatal("chat mode must stay read-only even with server powers")
	}
	if d, _ := p.Decide(ModeCode, "write", `{"path":"/srv/www/blog/index.html","content":"x"}`); d != "allow" {
		t.Fatalf("write to a served site: %s", d)
	}
	if d, _ := p.Decide(ModeCode, "write", `{"path":"/etc/caddy/Caddyfile","content":"x"}`); d == "allow" {
		t.Fatal("critical paths stay protected")
	}
	if !p.WindAutoAllow("tunnel", `{"action":"start","target":"3000"}`) {
		t.Fatal("a wind with powers can open a tunnel")
	}
	if !p.WindAutoAllow("bash", `{"command":"git clone https://github.com/x/y && cd y && npm ci"}`) {
		t.Fatal("a wind with powers runs normal sysadmin commands")
	}

	// La plantilla del config porta write/edit: ask (per al xat principal):
	// amb poders no atura el vent; un deny explícit, sí.
	asked := &Policy{ProjectDir: "/p", ServerPowers: true, ServerHome: "/var/lib/gregal", Tools: map[string]string{"write": "ask", "edit": "deny"}}
	if !asked.WindAutoAllow("write", `{"path":"/srv/www/blog/index.html"}`) {
		t.Fatal("a config 'ask' must not stop a wind with server powers")
	}
	if asked.WindAutoAllow("edit", `{"path":"/srv/www/blog/index.html"}`) {
		t.Fatal("a config 'deny' still wins")
	}

	plain := &Policy{ProjectDir: "/var/lib/gregal/projects/x"}
	if plain.WindAutoAllow("tunnel", `{"action":"start","target":"3000"}`) {
		t.Fatal("without powers a wind cannot open a tunnel")
	}
	if !plain.WindAutoAllow("tunnel", `{"action":"list"}`) {
		t.Fatal("listing tunnels is read-only")
	}
	if d, _ := plain.Decide(ModeCode, "bash", `{"command":"curl https://example.com"}`); d != "deny" {
		t.Fatal("without powers curl to the internet stays denied")
	}
	if plain.WindAutoAllow("write", `{"path":"/srv/www/blog/index.html"}`) {
		t.Fatal("without powers writes outside the project are not auto-allowed")
	}
}
