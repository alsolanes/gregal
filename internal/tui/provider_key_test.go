package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"

	tea "github.com/charmbracelet/bubbletea"
)

func keyTestModel(t *testing.T) Model {
	t.Helper()
	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat":     {Provider: "p", Model: "model-prova", ContextWindow: 32768},
			"code":     {Provider: "p", Model: "model-prova", ContextWindow: 32768},
			"reviewer": {Provider: "p", Model: "model-prova", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: "http://127.0.0.1"}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "manual"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	m := New(cfg, path, llm.New(), "vtest")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	return mod.(Model)
}

// Triar un provider sense clau arma la captura (prompt), no falla.
func TestKeyPromptIfMissing(t *testing.T) {
	m := keyTestModel(t)
	if got := m.keyPromptIfMissing("p"); got == "" {
		t.Fatal("hauria de demanar clau per provider sense clau")
	}
	if m.pendingKeyFor != "p" {
		t.Fatalf("pendingKeyFor=%q", m.pendingKeyFor)
	}
	m.cfg.Providers["amb"] = config.Provider{BaseURL: "http://x", APIKey: "sk-1"}
	if got := m.keyPromptIfMissing("amb"); got != "" {
		t.Fatalf("amb clau no ha de demanar: %q", got)
	}
}

// applyProviderKey activa la clau i la desa al magatzem propi, mai al
// config (que és un fitxer que la gent edita i comparteix).
func TestApplyProviderKey(t *testing.T) {
	auth := filepath.Join(t.TempDir(), "auth.yaml")
	t.Setenv("GREGAL_AUTH", auth)
	m := keyTestModel(t)
	msg := m.applyProviderKey("p", "sk-prova-123")
	if m.cfg.Providers["p"].APIKey != "sk-prova-123" {
		t.Fatalf("clau no activa: %q", msg)
	}
	raw, _ := os.ReadFile(m.cfgPath)
	if strings.Contains(string(raw), "sk-prova-123") {
		t.Fatal("el secret literal no pot quedar al config")
	}
	desat, err := os.ReadFile(auth)
	if err != nil {
		t.Fatalf("la clau s'ha de desar al magatzem: %v", err)
	}
	if !strings.Contains(string(desat), "sk-prova-123") {
		t.Fatalf("magatzem sense la clau: %q", desat)
	}
	if config.LoadAuth()["p"] != "sk-prova-123" {
		t.Fatal("el magatzem s'ha de poder tornar a llegir")
	}
}

// /provider key <nom> sense clau arma la captura en comptes d'error.
func TestProviderKeyInteractiu(t *testing.T) {
	m := keyTestModel(t)
	out := m.providerCmd("key p")
	if m.pendingKeyFor != "p" {
		t.Fatalf("hauria d'armar captura: %q", out)
	}
}

// /permissiu commuta (off per defecte).
func TestPermissiuToggle(t *testing.T) {
	m := keyTestModel(t)
	if m.permissive {
		t.Fatal("per defecte desactivat")
	}
	mod, _ := m.runCommand("/permissiu")
	m = mod.(Model)
	if !m.permissive {
		t.Fatal("/permissiu no activa")
	}
	mod, _ = m.runCommand("/permissiu off")
	m = mod.(Model)
	if m.permissive {
		t.Fatal("/permissiu off no desactiva")
	}
	mod, _ = m.runCommand("/permissiu n'importe-quoi")
	m = mod.(Model)
	if m.permissive {
		t.Fatal("arg invàlid no pot activar")
	}
}

// /key o /apikey amb nom de provider arma la captura de secret.
func TestKeyCommand(t *testing.T) {
	m := keyTestModel(t)
	mod, _ := m.runCommand("/key p")
	m = mod.(Model)
	if m.pendingKeyFor != "p" {
		t.Fatalf("pendingKeyFor=%q, volia p", m.pendingKeyFor)
	}

	// Amb clau directa (/key p sk-direct-123)
	mod, _ = m.runCommand("/key p sk-direct-123")
	m = mod.(Model)
	if m.cfg.Providers["p"].APIKey != "sk-direct-123" {
		t.Fatalf("clau no assignada: %q", m.cfg.Providers["p"].APIKey)
	}

	// Amb /apikey aplicat al provider del rol actiu
	mod, _ = m.runCommand("/apikey sk-active-456")
	m = mod.(Model)
	if m.cfg.Providers["p"].APIKey != "sk-active-456" {
		t.Fatalf("clau no assignada al provider actiu: %q", m.cfg.Providers["p"].APIKey)
	}
}

// settingsMenu conté opció per a clau d'API, tema i connectors.
func TestSettingsMenuOptions(t *testing.T) {
	m := keyTestModel(t)
	sMenu := settingsMenu(&m)
	hasKey, hasTheme, hasConnectors := false, false, false
	for _, it := range sMenu.items {
		if it.val == "apikey" {
			hasKey = true
		}
		if it.val == "theme" {
			hasTheme = true
		}
		if it.val == "connectors" {
			hasConnectors = true
		}
	}
	if !hasKey {
		t.Fatal("settingsMenu ha de tenir l'opció apikey")
	}
	if !hasTheme {
		t.Fatal("settingsMenu ha de tenir l'opció theme")
	}
	if !hasConnectors {
		t.Fatal("settingsMenu ha de tenir l'opció connectors")
	}

	// Triar apikey obre providerKeyMenu
	_, next := sMenu.run(&m, "apikey")
	if next == nil || !strings.Contains(next.title, "clau d'API") {
		t.Fatalf("apikey ha d'obrir providerKeyMenu: %v", next)
	}

	// Triar theme obre themeMenu i permet canviar tema
	_, tMenu := sMenu.run(&m, "theme")
	if tMenu == nil || !strings.Contains(tMenu.title, "tema") {
		t.Fatalf("theme ha d'obrir themeMenu: %v", tMenu)
	}
	tMenu.run(&m, "clar")
	if m.cfg.Tema() != "clar" {
		t.Fatalf("tema=%q, volia clar", m.cfg.Tema())
	}
}
