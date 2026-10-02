package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// omplir deixa la conversa amb prou línies perquè hi hagi on pujar.
func omplir(m *Model, n int) {
	for i := range n {
		m.push(fmt.Sprintf("línia %d de la conversa", i))
	}
	m.refresh()
}

// La barra d'estat anuncia «Pg↑↓ scroll» des de sempre i les tecles no
// estaven implementades enlloc: llegir el que havia passat amunt era
// impossible. Aquest test és el contracte.
func TestPageUpDownFanScroll(t *testing.T) {
	m := planTestModel(t)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mod.(Model)
	omplir(&m, 200)
	if !m.vp.AtBottom() {
		t.Fatal("després d'omplir hauríem de ser al final")
	}

	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = mod.(Model)
	if m.vp.AtBottom() {
		t.Fatal("PageUp no ha mogut res")
	}
	dalt := m.vp.YOffset

	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = mod.(Model)
	if m.vp.YOffset <= dalt {
		t.Fatalf("PageDown havia de baixar: %d → %d", dalt, m.vp.YOffset)
	}

	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	m = mod.(Model)
	if m.vp.YOffset != 0 {
		t.Fatalf("ctrl+home ha d'anar a dalt de tot, YOffset=%d", m.vp.YOffset)
	}
}

// Amb l'autocomplete obert el viewport encongeix. Abans, qualsevol canvi
// d'alçada forçava GotoBottom a View(), de manera que cada tecla et tornava
// al final i no es podia llegir res amunt mentre escrivies.
func TestEscriureNoEtBaixaAlFinal(t *testing.T) {
	m := planTestModel(t)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mod.(Model)
	omplir(&m, 200)

	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = mod.(Model)
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = mod.(Model)
	abans := m.vp.YOffset
	vistaAbans := m.View()

	// Escriure "/mo" obre el popup de suggeriments (canvia l'alçada).
	for _, r := range "/mo" {
		mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mod.(Model)
	}
	if len(m.suggestions()) == 0 {
		t.Fatal("«/mo» havia d'obrir suggeriments")
	}
	if m.vp.YOffset != abans {
		t.Fatalf("escriure ha mogut la lectura: %d → %d", abans, m.vp.YOffset)
	}
	// I el que es pinta segueix sent el tros que llegíem, no el final.
	if strings.Contains(m.View(), "línia 199 de la conversa") && !strings.Contains(vistaAbans, "línia 199 de la conversa") {
		t.Fatal("la vista ha saltat al final en obrir el popup")
	}
}

// Quan ja ets al final, la sortida nova t'hi manté (i el popup no ho trenca).
func TestAlFinalEsSegueixElFinal(t *testing.T) {
	m := planTestModel(t)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mod.(Model)
	omplir(&m, 50)
	m.push("l'última de totes")
	m.refresh()
	if !strings.Contains(m.View(), "l'última de totes") {
		t.Fatal("al final, la línia nova s'ha de veure")
	}
}
