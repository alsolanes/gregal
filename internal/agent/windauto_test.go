package agent

import (
	"path/filepath"
	"testing"
)

// L'ordre del xat del vent aprova sola la feina d'aquí (escriure i executar
// dins del projecte, sense res destructiu). Tot «deny» continua denegat i
// el config explícit mana.
func TestWindAutoAllow(t *testing.T) {
	proj := t.TempDir()
	pol := &Policy{ProjectDir: proj}
	dins := filepath.Join(proj, "sub", "f.txt")
	bashQ := func(cmd string) string { return "{\"command\":" + quoteJSON(cmd) + "}" }
	casos := []struct {
		nom  string
		pol  *Policy
		eina string
		args string
		vol  bool
	}{
		{"escriu dins", pol, "write", `{"path":` + quote(dins) + `,"content":"x"}`, true},
		{"escriu fora", pol, "write", `{"path":"/tmp/fora-vents.txt","content":"x"}`, false},
		{"escriu sense projecte", &Policy{}, "write", `{"path":"f.txt","content":"x"}`, false},
		{"bash compila dins", pol, "bash", bashQ("go build ./..."), true},
		{"bash crea dir dins", pol, "bash", bashQ("mkdir -p sub"), true},
		{"bash curl internet", pol, "bash", bashQ("curl https://example.com/x"), false},
		{"bash sudo", pol, "bash", bashQ("sudo rm -rf /tmp/x"), false},
		{"bash perillosa", pol, "bash", bashQ("git push"), false},
		{"bash escriu fora", pol, "bash", bashQ("echo x > /tmp/fora-vents.txt"), false},
		{"bash fons segur", pol, "bash_background", bashQ("go test ./..."), true},
		// Un heredoc és el contingut del fitxer: camins com '/api' o un
		// «kill()» de JavaScript no són ordres (el joc del vent es quedava
		// sense escriure). Però un heredoc que va a una shell es mira sencer.
		{"heredoc amb codi js", pol, "bash", bashQ("cat > joc.js <<'EOF'\nfetch('/api/x'); // http://x/y\nfunction kill(){}\nEOF"), true},
		{"heredoc python", pol, "bash", bashQ("python - <<'PY'\nopen('/etc/hosts').read()\nPY"), true},
		{"heredoc a bash", pol, "bash", bashQ("bash <<'EOF'\nrm -rf ..\nEOF"), false},
		{"heredoc a fora", pol, "bash", bashQ("cat > /tmp/fora-vents.txt <<EOF\nhola\nEOF"), false},
		{"bash_kill propi", pol, "bash_kill", `{"id":"p1"}`, true},
		{"node -e amb codi", pol, "bash", bashQ(`node -e "fetch('http://127.0.0.1:1/health').then(r=>{if(r.status>399)process.exit(1)})"`), true},
		{"python -c amb camí", pol, "bash", bashQ(`python -c 'print(open("/etc/hosts").read()[:1])'`), true},
		{"node -e i escriu fora", pol, "bash", bashQ(`node -e "1" > /tmp/fora-vents.txt`), false},
		{"app restart", pol, "app", `{"action":"restart","name":"x"}`, true},
		{"app start sense dir", pol, "app", `{"action":"start","name":"x","command":"node server.js"}`, true},
		{"commit local dins del projecte", pol, "git_commit", `{"message":"x"}`, true},
		{"commit sense projecte no", &Policy{}, "git_commit", `{"message":"x"}`, false},
		{"MCP extern queda fora", pol, "mcp_github_x", `{}`, false},
		{"amb poders de servidor res no atura el torn", &Policy{ProjectDir: proj, ServerPowers: true}, "mcp_github_x", `{}`, true},
		{"sense poders una eina fora de la llista queda fora", pol, "send_file", `{"path":"/x"}`, false},
		{"config explícit bash=ask mana", &Policy{ProjectDir: proj, Tools: map[string]string{"bash": "ask"}}, "bash", bashQ("go build ./..."), false},
		{"config explícit write=ask mana", &Policy{ProjectDir: proj, Tools: map[string]string{"write": "ask"}}, "write", `{"path":` + quote(dins) + `,"content":"x"}`, false},
		// Documents i símbols: la feina d'aquí també els ha de poder fer.
		{"office_edit dins", pol, "office_edit", `{"path":` + quote(dins) + `,"op":"replace"}`, true},
		{"office_create fora", pol, "office_create", `{"path":"/tmp/fora.docx","content":"x"}`, false},
		{"pdf_create dins", pol, "pdf_create", `{"path":` + quote(dins) + `}`, true},
		{"lsp consulta", pol, "lsp", `{"operation":"references","path":` + quote(dins) + `}`, true},
		{"lsp rename dins", pol, "lsp", `{"operation":"rename","path":` + quote(dins) + `,"new_name":"y"}`, true},
		{"lsp rename fora", pol, "lsp", `{"operation":"rename","path":"/tmp/fora.go","new_name":"y"}`, false},
		{"nota de sessió", pol, "knowledge_note", `{"text":"x"}`, true},
		{"nota global queda a la persona", pol, "knowledge_note", `{"text":"x","collection":"zqc-cap-colleccio"}`, false},
	}
	for _, c := range casos {
		t.Run(c.nom, func(t *testing.T) {
			if got := c.pol.WindAutoAllow(c.eina, c.args); got != c.vol {
				t.Fatalf("WindAutoAllow(%s) = %v, vol %v", c.nom, got, c.vol)
			}
		})
	}
}

func quote(s string) string {
	b := []byte{'"'}
	for _, r := range s {
		if r == '"' || r == '\\' {
			b = append(b, '\\')
		}
		b = append(b, string(r)...)
	}
	return string(append(b, '"'))
}
