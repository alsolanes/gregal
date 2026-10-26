package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"gregal/internal/config"
)

// TestFilaDelRailContinuaDinsDelRail: una fila d'eina massa llarga segueix
// amb la vora; les files que ja hi caben no es toquen.
func TestFilaDelRailContinuaDinsDelRail(t *testing.T) {
	fila := toolRailStyle.Render("│ ") + "✓ web_fetch https://go.dev/blog/loopvar-preview · Fixing For Loops in Go 1.22 · contingut principal"
	out := strings.Split(ansi.Strip(embolcallaFila(fila, 60)), "\n")
	if len(out) < 2 {
		t.Fatalf("havia d'embolcallar: %q", out)
	}
	for i, l := range out {
		if !strings.HasPrefix(l, "│ ") || ansi.StringWidth(l) > 60 {
			t.Fatalf("fila %d fora del rail o massa ampla: %q", i, l)
		}
	}
	diff := toolRailStyle.Render("│ ") + "  +  Timeout int\n" + toolRailStyle.Render("│ ") + "     Providers"
	if got := embolcallaFila(diff, 60); got != diff {
		t.Fatalf("un bloc curt de diverses files no s'ha de tocar: %q", ansi.Strip(got))
	}
}

func TestVisualRecordaIRefaAmbAltraAmplada(t *testing.T) {
	m := Model{wrapMemo: &wrapMemo{}, lines: []string{strings.Repeat("paraula ", 20)}}
	m.vp.Width = 40
	a := m.visual()
	if a != m.visual() {
		t.Fatal("la mateixa amplada ha de donar el mateix")
	}
	m.vp.Width = 100
	if b := m.visual(); strings.Count(b, "\n") >= strings.Count(a, "\n") {
		t.Fatal("amb més amplada calen menys files: el record no s'ha invalidat")
	}
}

func TestMenuConfigTextosNets(t *testing.T) {
	SetIdioma("en")
	defer SetIdioma("ca")
	if got := clauVisible(config.Provider{}); got != "no key" {
		t.Fatalf("clau buida en anglès: %q", got)
	}
	if got := clauVisible(config.Provider{APIKey: "sk-x"}); got != "saved" {
		t.Fatalf("clau desada: %q", got)
	}
	if got := revisorText("manual", config.Role{}); strings.Contains(got, "(/)") {
		t.Fatalf("revisor sense rol: %q", got)
	}
	if got := providersText(1); strings.Contains(got, "endpoints") {
		t.Fatalf("singular: %q", got)
	}
}
