package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func modelRetall(t *testing.T) Model {
	t.Helper()
	cfg := &config.Config{Language: "ca",
		Mode:      "code",
		Roles:     map[string]config.Role{"chat": {Provider: "p", Model: "m", ContextWindow: 32768}},
		Providers: map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "test")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return mod.(Model)
}

// Sense sostre, vp.SetContent es va fent més car a cada línia i el terminal
// va espès just quan més coses passen.
func TestConversaAcotada(t *testing.T) {
	m := modelRetall(t)
	base := len(m.lines)
	for i := 0; i < MaxLinies+ExcesLinies*3; i++ {
		m.push("línia " + strings.Repeat("x", 10))
	}
	if len(m.lines) > MaxLinies+ExcesLinies {
		t.Fatalf("la conversa no s'ha acotat: %d línies", len(m.lines))
	}
	if m.retallades == 0 {
		t.Fatal("hauria d'haver retallat i dir-ho")
	}
	// Ni una línia de menys: el que hi ha més el que s'ha retallat ha de
	// quadrar amb el que s'ha empès.
	if got := len(m.lines) + m.retallades; got != base+MaxLinies+ExcesLinies*3 {
		t.Fatalf("línies perdudes pel camí: %d vs %d", got, base+MaxLinies+ExcesLinies*3)
	}
}

// Retallar no pot ser silenciós: has de saber que hi havia més amunt i que
// no s'ha perdut.
func TestElRetallEsDiu(t *testing.T) {
	m := modelRetall(t)
	if strings.Contains(m.contingut(), "fora de la pantalla") {
		t.Fatal("sense retallar no ha de dir res")
	}
	for i := 0; i < MaxLinies+ExcesLinies*2; i++ {
		m.push("línia")
	}
	c := m.contingut()
	if !strings.Contains(c, "fora de la pantalla") {
		t.Fatalf("hauria de dir que ha retallat")
	}
	if !strings.Contains(c, "es desa a disc") {
		t.Fatalf("ha de dir que no s'ha perdut: %q", c[:200])
	}
}

// m.streamLine és un índex dins de m.lines: si es retalla i no es mou,
// l'streaming acaba escrivint sobre una línia que no toca.
func TestElRetallMouLaLiniaQueCreix(t *testing.T) {
	m := modelRetall(t)
	for i := 0; i < MaxLinies; i++ {
		m.push("vella")
	}
	m.push("")
	m.streamLine = len(m.lines) - 1
	for i := 0; i < ExcesLinies*2; i++ {
		m.push("nova")
	}
	if m.streamLine < 0 || m.streamLine >= len(m.lines) {
		t.Fatalf("streamLine fora de rang després de retallar: %d de %d", m.streamLine, len(m.lines))
	}
	// I ha de seguir apuntant a la mateixa línia (la buida que vam posar).
	if m.lines[m.streamLine] != "" {
		t.Fatalf("streamLine apunta a una altra línia: %q", m.lines[m.streamLine])
	}
}
