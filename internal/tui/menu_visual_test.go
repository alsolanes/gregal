package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMenuRendersAsWindow(t *testing.T) {
	mn := &menuState{title: "tria model", items: []menuItem{{text: "primer"}, {text: "segon"}}, idx: 1}
	got := menuLine(mn, 100)
	for _, want := range []string{"╭", "╯", "tria model", "2/2", "segon"} {
		if !strings.Contains(got, want) {
			t.Fatalf("menú sense %q:\n%s", want, got)
		}
	}
}

func TestSettingsMenuIsCentralized(t *testing.T) {
	m := planTestModel(t)
	mn := settingsMenu(&m)
	if mn == nil || mn.title != "configuració" {
		t.Fatal("cal el menú central de configuració")
	}
	if len(mn.items) < 7 {
		t.Fatalf("massa poques opcions de configuració: %d", len(mn.items))
	}
}

func TestMenuEscapeReturnsToParent(t *testing.T) {
	parent := &menuState{title: "configuració"}
	child := &menuState{title: "mode", back: parent}
	m := planTestModel(t)
	m.menu = child
	m.menuKeys("esc")
	if m.menu != parent {
		t.Fatal("Esc ha de tornar al menú pare")
	}
}

// Canviar un permís no tanca el menú: es torna a la llista amb el valor
// nou, el cursor on era, i Esc segueix tornant al menú pare. Abans cada
// canvi et deixava a la conversa i calia reobrir /settings tres cops per
// canviar tres eines.
func TestElMenuDePermisosEsQuedaObert(t *testing.T) {
	m := planTestModel(t)
	m.cfgPath = filepath.Join(t.TempDir(), "config.yaml")
	// Save exigeix els rols obligatoris.
	for _, rol := range []string{"code", "reviewer", "summary", "title"} {
		m.cfg.Roles[rol] = m.cfg.Roles["chat"]
	}
	pare := &menuState{title: "configuració"}
	m.menu = permissionsMenu(&m)
	m.menu.back = pare
	// Baixa fins a bash i entra-hi.
	for i, it := range m.menu.items {
		if it.val == "bash" {
			m.menu.idx = i
		}
	}
	fila := m.menu.idx
	m.menuKeys("enter")
	if m.menu == nil || m.menu.title != "permís: bash" {
		t.Fatalf("Enter sobre bash ha d'obrir el submenú: %+v", m.menu)
	}
	for i, it := range m.menu.items {
		if it.val == "allow" {
			m.menu.idx = i
		}
	}
	m.menuKeys("enter")
	if m.menu == nil || m.menu.title != "permisos d'eines" {
		t.Fatalf("després de triar, la llista de permisos ha de seguir oberta: %+v", m.menu)
	}
	if m.menu.idx != fila || !strings.Contains(m.menu.items[fila].text, "allow") {
		t.Fatalf("el cursor ha de ser a bash i la fila dir allow: idx=%d %q", m.menu.idx, m.menu.items[fila].text)
	}
	if m.cfg.Permissions.Tools["bash"] != "allow" {
		t.Fatal("el permís s'ha de desar al config")
	}
	m.menuKeys("esc")
	if m.menu != pare {
		t.Fatalf("Esc ha de tornar al menú pare, no a la versió vella de la llista: %+v", m.menu)
	}
}
