package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPendingApprovalWithArrows(t *testing.T) {
	ran := false
	m := Model{pending: &pendingOp{desc: "prova", run: func(ctx context.Context) (string, error) {
		ran = true
		return "ok", nil
	}}}

	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mod.(Model)
	if m.confirmIdx != 1 {
		t.Fatalf("dreta hauria de seleccionar denega: %d", m.confirmIdx)
	}
	mod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mod.(Model)
	if cmd != nil || m.pending != nil || ran {
		t.Fatal("enter sobre denega ha de cancel·lar sense executar")
	}

	m.pending = &pendingOp{desc: "prova", run: func(ctx context.Context) (string, error) {
		ran = true
		return "ok", nil
	}}
	m.confirmIdx = 1
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = mod.(Model)
	mod, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mod.(Model)
	if cmd == nil || m.pending != nil {
		t.Fatal("enter sobre permet ha de retornar l'execució")
	}
	_ = cmd()
	if !ran {
		t.Fatal("l'acció aprovada no s'ha executat")
	}
}

func TestAprovacioNoEsDenegaSola(t *testing.T) {
	// El compte enrere de 120 s venia de la web. En un terminal, deixar
	// la pantalla i tornar amb l'edicio rebutjada es pitjor que esperar:
	// l'aprovacio ara espera indefinidament.
	corregut := false
	m := Model{pending: &pendingOp{desc: "prova", run: func(ctx context.Context) (string, error) {
		corregut = true
		return "no hauria de correr", nil
	}}}
	for i := 0; i < 5; i++ {
		mod, _ := m.Update(streamTickMsg{})
		m = mod.(Model)
	}
	if m.pending == nil {
		t.Fatal("l'aprovacio no pot desapareixer sola amb els ticks")
	}
	if corregut {
		t.Fatal("no s'ha d'executar res sense resposta de l'usuari")
	}
	for _, l := range m.lines {
		if strings.Contains(l, "denegat") {
			t.Fatalf("cap avis de denegacio automatica: %q", l)
		}
	}
}

func TestEscTancaLAprovacio(t *testing.T) {
	m := Model{pending: &pendingOp{desc: "prova", run: func(ctx context.Context) (string, error) {
		return "", nil
	}}}
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mod.(Model)
	if m.pending != nil {
		t.Fatal("Esc ha de tancar l'aprovacio")
	}
}

// TestAprovacioNoEsDisparaEscrivint: si el diàleg s'obre mentre l'usuari
// escriu, la lletra següent («s» de «dos», Enter) no aprova l'eina. Passat
// l'instant de protecció, les dreceres funcionen com sempre.
func TestAprovacioNoEsDisparaEscrivint(t *testing.T) {
	saved := approvalGuard
	approvalGuard = time.Hour
	t.Cleanup(func() { approvalGuard = saved })
	ran := false
	op := &pendingOp{desc: "prova", run: func(ctx context.Context) (string, error) { ran = true; return "ok", nil }}
	m := Model{pending: op, pendingSeen: op, pendingAt: time.Now()}
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("s")}, {Type: tea.KeyRunes, Runes: []rune("a")}, {Type: tea.KeyEnter}} {
		mod, cmd := m.Update(k)
		m = mod.(Model)
		if cmd != nil || m.pending == nil || ran {
			t.Fatalf("%q ha aprovat un diàleg acabat d'obrir", k.String())
		}
	}
	approvalGuard = 0
	mod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = mod.(Model)
	if cmd == nil || m.pending != nil {
		t.Fatal("passada la protecció, «s» ha d'aprovar")
	}
}
