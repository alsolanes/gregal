package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestViewHasVisualProjectAndModeShell(t *testing.T) {
	old := gitBranchFn
	defer func() { gitBranchFn = old }()
	gitBranchFn = func(string) string { return "main" }

	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "model-prova", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "manual"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "vtest")
	m.cwd = filepath.Join(string(filepath.Separator), "tmp", "projecte-espectacular")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	view := mod.(Model).View()
	// El mode a la barra i al peu del composer, el projecte i la branca a
	// la capçalera, el model al peu, i l'ajuda a ? (les pistes han sortit de
	// la barra).
	for _, want := range []string{"CODE", "projecte-espectacular", "main", "model-prova", "? ajuda"} {
		if !strings.Contains(view, want) {
			t.Fatalf("la shell visual no mostra %q:\n%s", want, view)
		}
	}
}

func TestInitialRoleFollowsMode(t *testing.T) {
	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat":  {Provider: "p", Model: "chat"},
			"code":  {Provider: "p", Model: "code"},
			"think": {Provider: "p", Model: "think"},
		},
	}
	if got := initialRole(cfg); got != "code" {
		t.Fatalf("mode code ha d'arrencar amb code, no %q", got)
	}
	cfg.Mode = "inspect"
	if got := initialRole(cfg); got != "think" {
		t.Fatalf("mode inspect ha d'arrencar amb think, no %q", got)
	}
	delete(cfg.Roles, "code")
	delete(cfg.Roles, "think")
	if got := initialRole(cfg); got != "chat" {
		t.Fatalf("fallback sense code/think = %q", got)
	}
}

func TestToolTimelineHasReadableHierarchy(t *testing.T) {
	call := toolCallLine("write", `{"path":"main.go"}`)
	result := toolResultLine("write", "escrit main.go", false)
	if !strings.Contains(call, "▸ write main.go") {
		t.Fatalf("crida d'eina poc clara: %q", call)
	}
	if !strings.Contains(result, "✓ write") || !strings.Contains(result, "escrit main.go") {
		t.Fatalf("resultat d'eina poc clar: %q", result)
	}
	fail := toolResultLine("write", "ERROR: x", true)
	if !strings.Contains(fail, "✗ write") {
		t.Fatalf("error d'eina poc clar: %q", fail)
	}
}
