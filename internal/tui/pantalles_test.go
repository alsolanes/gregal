package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Pantalles simulades, compartides pels fitxers dorats (golden_test.go) i
// pel bolcat de revisió (TestZZDumpVisual). Tot el que hi surt és
// determinista: ni hora, ni sessions de debò, ni claus del disc.

// temaGolden és el tema amb què es munten les pantalles: New() aplica el
// del config, així que el tema es passa per aquí i no amb SetTema.
var temaGolden = "fosc"

// modelGolden és un model net a una mida donada: sense sessions desades
// (GREGAL_DATA_DIR a un temporal), sense claus (GREGAL_AUTH a un
// temporal), branca fixa, projecte fix i el tema de temaGolden.
func modelGolden(t *testing.T, ample, alt int) Model {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_AUTH", filepath.Join(t.TempDir(), "auth.yaml"))
	old := gitBranchFn
	t.Cleanup(func() { gitBranchFn = old })
	gitBranchFn = func(string) string { return "main" }
	cfg := &config.Config{Language: "ca",
		Mode:  "code",
		Theme: temaGolden,
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "model-prova", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "manual"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "vtest")
	m.cwd = "C:/Users/usera/tui-agent"
	mod, _ := m.Update(tea.WindowSizeMsg{Width: ample, Height: alt})
	m = mod.(Model)
	m.branch = "main"
	return m
}

// omplePantallaTorn deixa el model amb un torn sencer ja fet: la
// instrucció, sis eines (una que falla), un diff, una checklist i la
// resposta en markdown. És la pantalla de referència del disseny.
func omplePantallaTorn(t *testing.T, m *Model) {
	t.Helper()
	m.treuBenvinguda()
	m.push(userLine("Afegeix un camp Timeout al config i propaga'l al client"))
	m.pushToolCall("read", `{"path":"internal/config/config.go"}`)
	m.pushToolResult("read", "llegit internal/config/config.go\n1\tpackage config\n2\t\n3\timport (\n4\t\t\"fmt\"\n5\t)\n6\t\n7\ttype Config struct {\n8\t\tProviders map[string]Provider\n9\t}", false)
	m.pushToolCall("grep", `{"pattern":"Timeout","dir":"internal"}`)
	m.pushToolResult("grep", "internal/llm/client.go:146: hc: &http.Client{Timeout: 180 * time.Second}\ninternal/tools/tools.go:34: DefaultTimeout = 120 * time.Second", false)
	m.pushToolCall("web_fetch", `{"url":"https://go.dev/blog/loopvar-preview"}`)
	m.pushToolResult("web_fetch", "# Fixing For Loops in Go 1.22\nhttps://go.dev/blog/loopvar-preview · 1256 paraules · contingut principal\n\nGo 1.21 includes…", false)
	m.pushToolCall("edit", `{"path":"internal/config/config.go","old_string":"type Config struct {","new_string":"type Config struct {\n\tTimeout int"}`)
	m.pushToolResult("edit", "edit aplicat a internal/config/config.go", false)
	m.push(dinsRail(diffCos("internal/config/config.go", "type Config struct {\n\tProviders map[string]Provider\n}\n", "type Config struct {\n\tTimeout int `yaml:\"timeout\"`\n\tProviders map[string]Provider\n}\n", 40)))
	m.pushToolCall("bash", `{"command":"go test ./internal/config/"}`)
	m.pushToolResult("bash", "ERROR: exit status 1\n# gregal/internal/config\ninternal/config/config.go:9:2: undefined: Timeout", true)
	m.pushToolCall("todowrite", `{"items":[{"title":"a","status":"done"},{"title":"b","status":"working"}]}`)
	m.pushToolResult("todowrite", "todos 1/2:\n✓ a\n◌ b", false)
	tools.TodoSet([]tools.TodoItem{{Title: "Llegir config.go", Status: "done"}, {Title: "Afegir el camp Timeout", Status: "working"}, {Title: "Propagar-lo al client", Status: "pending"}, {Title: "Tests", Status: "pending"}})
	t.Cleanup(tools.TodoClear)
	m.push(assistantMD("He afegit el camp `Timeout` a `Config`. Ara toca propagar-lo:\n\n1. `internal/llm/client.go`: llegir `cfg.Timeout`\n2. Tests a `config_test.go`\n\n```go\nfunc New(timeout time.Duration) *Client {\n\treturn &Client{hc: &http.Client{Timeout: timeout}}\n}\n```\n\n**Nota**: el valor per defecte segueix sent 180 s.", m.vp.Width))
	// El resum, fix: runSummaryLine mira el rellotge i «42s» podria ser
	// «43s» en una màquina lenta.
	m.push(faintStyle.Render("resum del torn · 6 eines · 1 fitxer · 1 error · 42s"))
	m.status = "llest"
	m.refresh()
	m.vp.GotoBottom()
}

// pendingDeMostra és una aprovació d'edit amb el seu diff.
func pendingDeMostra() *pendingOp {
	return &pendingOp{
		desc:    "edit internal/config/config.go",
		preview: diffCos("internal/config/config.go", "type Config struct {\n\tProviders map[string]Provider\n}\n", "type Config struct {\n\tTimeout int `yaml:\"timeout\"`\n\tProviders map[string]Provider\n}\n", 40),
	}
}

// preguntaDeMostra és una pregunta del model amb tres opcions.
func preguntaDeMostra() *questionPending {
	return &questionPending{
		query:   "Quin format vols per al fitxer de sortida?",
		options: []tools.QuestionOption{{Label: "JSON", Description: "una línia per registre"}, {Label: "CSV"}, {Label: "YAML", Description: "llegible a mà"}},
	}
}

// pantalles són les sis vistes que es protegeixen amb fitxers dorats.
var pantalles = []struct {
	nom   string
	munta func(t *testing.T, m *Model)
}{
	{"benvinguda", func(t *testing.T, m *Model) {}},
	{"torn", omplePantallaTorn},
	{"aprovacio", func(t *testing.T, m *Model) { omplePantallaTorn(t, m); m.pending = pendingDeMostra() }},
	{"selector", func(t *testing.T, m *Model) { omplePantallaTorn(t, m); m.menu = settingsMenu(m) }},
	{"pregunta", func(t *testing.T, m *Model) { omplePantallaTorn(t, m); m.pendingQ = preguntaDeMostra() }},
	{"cockpit", func(t *testing.T, m *Model) { omplePantallaTorn(t, m); m.showTodos = true }},
}

// TestZZDumpVisual escriu la pantalla del torn a GREGAL_DUMP per revisar
// el disseny sense arrencar el programa. Salta sense la variable.
func TestZZDumpVisual(t *testing.T) {
	dest := os.Getenv("GREGAL_DUMP")
	if dest == "" {
		t.Skip()
	}
	m := modelGolden(t, 120, 44)
	omplePantallaTorn(t, &m)
	os.WriteFile(dest, []byte(m.View()+"\n=====LINES=====\n"+strings.Join(m.lines, "\n")), 0o644)
}
