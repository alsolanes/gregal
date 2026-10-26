package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/llm"
	"gregal/internal/session"
)

// TestResumeMenuAmagaVentsIAparta: les converses dels Vents no surten al
// selector, i les d'una carpeta que ja no existeix van al final marcades;
// triar-ne una ho explica sense tancar el selector.
func TestResumeMenuAmagaVentsIAparta(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := session.DefaultDir()
	viva := t.TempDir()
	desa := func(name, ws string, at time.Time) {
		t.Helper()
		if _, err := session.SaveSession(dir, session.Session{Name: name, Title: name, Role: "code", Workspace: ws, Convo: []llm.Message{{Role: "user", Content: "hola"}}}); err != nil {
			t.Fatal(err)
		}
	}
	desa("rotllana-colla", "", time.Now())
	desa("company-xaloc", "", time.Now())
	desa("sessio-perduda", filepath.Join(viva, "ja-no-hi-es"), time.Now())
	desa("sessio-bona", viva, time.Now())
	mn := resumeMenu()
	if mn == nil {
		t.Fatal("hi havia d'haver selector")
	}
	var texts []string
	for _, it := range mn.items {
		texts = append(texts, it.text)
	}
	joined := strings.Join(texts, "\n")
	if strings.Contains(joined, "rotllana-") || strings.Contains(joined, "company-") {
		t.Fatalf("les converses dels Vents no hi han de sortir:\n%s", joined)
	}
	last := mn.items[len(mn.items)-1]
	if !strings.HasPrefix(last.text, "⚠") || !strings.Contains(last.text, "sessio-perduda") {
		t.Fatalf("la de la carpeta perduda ha d'anar al final, marcada:\n%s", joined)
	}
	m := Model{menu: mn}
	mn.idx = len(mn.items) - 1
	m.menuKeys("enter")
	if m.menu != mn {
		t.Fatal("triar una conversa perduda no ha de tancar el selector")
	}
	m.menuKeys("esc")
	if m.menu != nil {
		t.Fatal("Esc ha de tancar el selector, no tornar-hi")
	}
}

// TestMenuFiltraAccentsIPagina: «ç» filtra, i PgDn/End/Home salten pàgines.
func TestMenuFiltraAccentsIPagina(t *testing.T) {
	var items []menuItem
	for i := 0; i < 30; i++ {
		items = append(items, menuItem{text: "fila", val: "x"})
	}
	items = append(items, menuItem{text: "barça", val: "b"})
	m := Model{menu: &menuState{items: items}}
	m.menuKeys("pgdown")
	if m.menu.idx != menuWin {
		t.Fatalf("PgDn: %d", m.menu.idx)
	}
	m.menuKeys("end")
	if m.menu.idx != len(items)-1 {
		t.Fatalf("End: %d", m.menu.idx)
	}
	m.menuKeys("home")
	if m.menu.idx != 0 {
		t.Fatalf("Home: %d", m.menu.idx)
	}
	for _, k := range []string{"b", "a", "r", "ç"} {
		m.menuKeys(k)
	}
	if vis := m.menu.visibles(); len(vis) != 1 || vis[0].val != "b" {
		t.Fatalf("el filtre «barç» havia de deixar només «barça»: %+v", vis)
	}
}

// TestNomSessioCurtNomesDates: només els noms de sessió amb data es retallen.
func TestNomSessioCurtNomesDates(t *testing.T) {
	if got := nomSessioCurt("sessio-20260918-112808-312"); got != "18/09 11:28" {
		t.Fatalf("data: %q", got)
	}
	if got := nomSessioCurt("rotllana-em-recomaneu-un-llibre"); got != "rotllana-em-recomaneu-un-llibre" {
		t.Fatalf("un nom que no és data no s'ha de tocar: %q", got)
	}
}

// TestCutBytesNoTrencaCaracters: retallar mai deixa mig caràcter.
func TestCutBytesNoTrencaCaracters(t *testing.T) {
	got := shortArgs(strings.Repeat("à", 10), 5)
	if strings.ContainsRune(got, '�') || got != "àà…" {
		t.Fatalf("retall trencat: %q", got)
	}
}

// TestCtrlCSenseFeinaAvisaAbansDeSortir: sense res en marxa, el primer Ctrl+C
// avisa i conserva el text; el segon, poc després, surt.
func TestCtrlCSenseFeinaAvisaAbansDeSortir(t *testing.T) {
	m := planTestModel(t)
	m.input.SetValue("esborrany a mig escriure")
	mod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = mod.(Model)
	if cmd != nil || m.input.Value() != "esborrany a mig escriure" {
		t.Fatal("el primer Ctrl+C no ha de sortir ni perdre l'esborrany")
	}
	if _, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatal("el segon Ctrl+C ha de sortir")
	}
}
