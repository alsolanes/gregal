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
	bashQ := func(cmd string) string { return "{\"command\":" + quote(cmd) + "}" }
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
		{"commit queda a la persona", pol, "git_commit", `{"message":"x"}`, false},
		{"MCP extern queda fora", pol, "mcp_github_x", `{}`, false},
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
