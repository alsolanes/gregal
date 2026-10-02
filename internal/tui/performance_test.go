package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// View s'executa per cada tecla. Mai pot fer I/O ni crear processos.
func TestViewDoesNotRefreshGitBranch(t *testing.T) {
	old := gitBranchFn
	defer func() { gitBranchFn = old }()
	calls := 0
	gitBranchFn = func(string) string {
		calls++
		return "main"
	}

	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "m", ContextWindow: 32768},
		},
		Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "test")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mod.(Model)
	for range 50 {
		mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		m = mod.(Model)
		_ = m.View()
	}
	for range 50 {
		mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = mod.(Model)
		_ = m.View()
	}
	// Bracketed paste ha d'arribar com un únic missatge, sense cap I/O extra.
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enganxat d'una tirada"), Paste: true})
	m = mod.(Model)
	_ = m.View()
	if calls != 1 {
		t.Fatalf("git branch s'ha consultat %d cops; en volem 1 a New i 0 en escriure/esborrar/render", calls)
	}
}

// Pintar la conversa passa a cada tecla. Amb un transcript llarg, el que
// costi aquí es nota escrivint: és la diferència entre un TUI viu i un que
// va espès.
func BenchmarkViewConversaLlarga(b *testing.B) {
	cfg := &config.Config{Language: "ca",
		Mode:      "code",
		Roles:     map[string]config.Role{"chat": {Provider: "p", Model: "m", ContextWindow: 32768}},
		Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "bench")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	m = mod.(Model)
	for i := 0; i < 400; i++ {
		m.push(userLine("una pregunta de l'usuari que ocupa una línia sencera"))
		m.push(assistantLine("una resposta de l'agent, amb prou text per ocupar una línia i escaig del terminal"))
		m.push(toolCallLine("read", `{"path":"internal/agent/loop.go"}`))
	}
	m.refresh()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// I escriure: cada tecla és un Update + un View.
func BenchmarkEscriureAmbConversaLlarga(b *testing.B) {
	cfg := &config.Config{Language: "ca",
		Mode:      "code",
		Roles:     map[string]config.Role{"chat": {Provider: "p", Model: "m", ContextWindow: 32768}},
		Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "bench")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	m = mod.(Model)
	for i := 0; i < 400; i++ {
		m.push(assistantLine("una resposta de l'agent amb prou text per ocupar una línia del terminal"))
	}
	m.refresh()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		m = mod.(Model)
		_ = m.View()
	}
}

// push() crida refresh(), que fa strings.Join de TOTA la conversa. Durant
// l'streaming la línia de la resposta es reescriu a cada token, o sigui
// que això passa desenes de vegades per segon amb el transcript sencer.
func benchPush(b *testing.B, linies int) {
	cfg := &config.Config{Language: "ca",
		Mode:      "code",
		Roles:     map[string]config.Role{"chat": {Provider: "p", Model: "m", ContextWindow: 32768}},
		Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "bench")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	m = mod.(Model)
	for i := 0; i < linies; i++ {
		m.lines = append(m.lines, assistantLine("una resposta de l'agent amb prou text per ocupar una línia del terminal"))
	}
	m.refresh()
	m.push("")
	sl := len(m.lines) - 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// El que passa a cada token: reescriu l'última línia i repinta.
		m.lines[sl] = assistantLine("resposta que creix token a token…")
		m.refresh()
	}
}

func BenchmarkStreamAmb500Linies(b *testing.B)   { benchPush(b, 500) }
func BenchmarkStreamAmb5000Linies(b *testing.B)  { benchPush(b, 5000) }
func BenchmarkStreamAmb20000Linies(b *testing.B) { benchPush(b, 20000) }

// On va el temps de debò: muntar el text o donar-lo al viewport?
func BenchmarkNomesJoin(b *testing.B) {
	linies := make([]string, 20000)
	for i := range linies {
		linies[i] = assistantLine("una resposta de l'agent amb prou text per ocupar una línia del terminal")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strings.Join(linies, "\n")
	}
}

func BenchmarkNomesSetContent(b *testing.B) {
	linies := make([]string, 20000)
	for i := range linies {
		linies[i] = assistantLine("una resposta de l'agent amb prou text per ocupar una línia del terminal")
	}
	text := strings.Join(linies, "\n")
	vp := viewport.New(110, 30)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vp.SetContent(text)
	}
}

// Amb el sostre, una sessió que no para de créixer no es va tornant
// espessa: el pitjor cas queda fix.
func BenchmarkStreamSessioLlarguissima(b *testing.B) {
	cfg := &config.Config{Language: "ca",
		Mode:      "code",
		Roles:     map[string]config.Role{"chat": {Provider: "p", Model: "m", ContextWindow: 32768}},
		Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "bench")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	m = mod.(Model)
	// 40.000 línies empeses com a la vida real, passant pel sostre.
	for i := 0; i < 40000; i++ {
		m.push(assistantLine("una resposta de l'agent amb prou text per ocupar una línia del terminal"))
	}
	sl := len(m.lines) - 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.lines[sl] = assistantLine("resposta que creix token a token…")
		m.refresh()
	}
}
