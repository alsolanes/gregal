package agent

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// En autònom no es pregunta res: el que abans esperava la persona (el
// perillós) ara passa sol, i només el prohibit continua prohibit.
func TestPoliticaAutonomaNoPreguntaMai(t *testing.T) {
	dir := t.TempDir()
	p := &Policy{ProjectDir: dir}
	bash := func(c string) string { b, _ := json.Marshal(map[string]string{"command": c}); return string(b) }
	file := func(path string) string { b, _ := json.Marshal(map[string]string{"path": path, "content": "x"}); return string(b) }
	for _, tc := range []struct {
		name, tool, args, want string
	}{
		{"write inside", "write", file(filepath.Join(dir, "site", "index.html")), "allow"},
		{"write outside", "write", file(filepath.Join(filepath.Dir(dir), "elsewhere.txt")), "allow"},
		{"tests", "bash", bash("python -m unittest discover -s tests -v 2>&1 | tail -20"), "allow"},
		{"npm install local", "bash", bash("npm install && npm run build"), "allow"},
		{"rm build dir", "bash", bash("rm -rf dist node_modules"), "allow"},
		{"mkdir and cd absolute", "bash", bash("cd \"" + filepath.ToSlash(dir) + "\" && mkdir -p site && ls 2>/dev/null"), "allow"},
		{"rm everything", "bash", bash("rm -rf ."), "allow"},
		{"git push", "bash", bash("git push origin main"), "allow"},
		{"git reset hard", "bash", bash("git reset --hard HEAD~1"), "allow"},
		{"global install", "bash", bash("npm install -g typescript"), "allow"},
		{"kill", "bash", bash("taskkill /F /IM node.exe"), "allow"},
		{"write outside via redirect", "bash", bash("echo hi > ../outside.txt"), "allow"},
		{"hard deny stays", "bash", bash("sudo rm -rf /"), "deny"},
		{"web fetch", "web_fetch", `{"url":"https://example.com"}`, "allow"},
		{"browser click", "browser", `{"action":"click","target":"1"}`, "allow"},
		{"browser eval", "browser", `{"action":"eval","js":"1"}`, "allow"},
		{"question disabled", "question", `{"query":"which?"}`, "deny"},
		{"commit asks", "git_commit", `{"message":"x"}`, "allow"},
		{"mcp asks", "mcp_srv_tool", `{}`, "allow"},
	} {
		if got, reason := p.Decide(ModeAutonomous, tc.tool, tc.args); got != tc.want {
			t.Errorf("%s: got %s (%s), want %s", tc.name, got, reason, tc.want)
		}
	}
	// Un «ask» del config tampoc fa preguntar; un «deny» sí que mana.
	p.Tools = map[string]string{"bash": "ask"}
	if got, _ := p.Decide(ModeAutonomous, "bash", bash("ls")); got != "allow" {
		t.Errorf("config ask in autonomous: %s, want allow", got)
	}
	p.Tools = map[string]string{"bash": "deny"}
	if got, _ := p.Decide(ModeAutonomous, "bash", bash("ls")); got != "deny" {
		t.Errorf("config deny ignored in autonomous: %s", got)
	}
}
