package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func scrollTestModel(t *testing.T) Model {
	t.Helper()
	old := gitBranchFn
	t.Cleanup(func() { gitBranchFn = old })
	gitBranchFn = func(string) string { return "main" }
	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "m", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "test")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mod.(Model)
	m.lines = nil
	for i := 1; i <= 100; i++ {
		m.lines = append(m.lines, fmt.Sprintf("línia %03d", i))
	}
	m.refresh()
	return m
}

func pressScrollKey(m Model, key tea.KeyType) Model {
	mod, _ := m.Update(tea.KeyMsg{Type: key})
	return mod.(Model)
}

func TestViewportScrollKeyboard(t *testing.T) {
	m := scrollTestModel(t)
	bottom := m.vp.YOffset
	if bottom == 0 || !m.vp.AtBottom() {
		t.Fatalf("precondició: viewport no és al final: offset=%d", bottom)
	}

	m = pressScrollKey(m, tea.KeyPgUp)
	if m.vp.YOffset >= bottom {
		t.Fatalf("PageUp no ha pujat: abans=%d després=%d", bottom, m.vp.YOffset)
	}
	pageOffset := m.vp.YOffset

	m = pressScrollKey(m, tea.KeyCtrlU)
	if m.vp.YOffset >= pageOffset {
		t.Fatalf("Ctrl+U no ha pujat mitja pàgina: abans=%d després=%d", pageOffset, m.vp.YOffset)
	}

	m = pressScrollKey(m, tea.KeyCtrlHome)
	if !m.vp.AtTop() {
		t.Fatalf("Ctrl+Home no ha anat a dalt: offset=%d", m.vp.YOffset)
	}
	m = pressScrollKey(m, tea.KeyCtrlEnd)
	if !m.vp.AtBottom() {
		t.Fatalf("Ctrl+End no ha anat al final: offset=%d", m.vp.YOffset)
	}
}

func TestViewportScrollMouseWheel(t *testing.T) {
	m := scrollTestModel(t)
	bottom := m.vp.YOffset
	mod, _ := m.Update(tea.MouseMsg{X: 4, Y: 8, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = mod.(Model)
	if m.vp.YOffset >= bottom {
		t.Fatalf("la roda del ratolí no ha pujat: abans=%d després=%d", bottom, m.vp.YOffset)
	}
}

func TestNewOutputDoesNotStealManualScroll(t *testing.T) {
	m := scrollTestModel(t)
	m.vp.PageUp()
	offset := m.vp.YOffset
	m.push("sortida nova")
	if m.vp.YOffset != offset {
		t.Fatalf("nova sortida ha robat el scroll: abans=%d després=%d", offset, m.vp.YOffset)
	}

	m.vp.GotoBottom()
	oldBottom := m.vp.YOffset
	m.push("una altra sortida")
	if !m.vp.AtBottom() || m.vp.YOffset <= oldBottom {
		t.Fatalf("al final no ha seguit la sortida: abans=%d després=%d", oldBottom, m.vp.YOffset)
	}
}
