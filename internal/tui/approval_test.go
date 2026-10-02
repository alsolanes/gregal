package tui

import (
	"context"
	"strings"
	"testing"

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
