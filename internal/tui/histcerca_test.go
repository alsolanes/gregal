package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func modelAmbHistorial(t *testing.T, entrades ...string) Model {
	t.Helper()
	m := modelDeBarra(t, 100)
	m.histEntries = entrades
	return m
}

// Amb ↑/↓ es recorre l'historial una a una: per trobar «aquella ordre
// llarga de fa dos dies» has de picar la fletxa vint vegades. Ctrl+R és la
// drecera que ja tens als dits de la shell.
func TestLaCercaAlHistorialTrobaPerTrossos(t *testing.T) {
	m := modelAmbHistorial(t, "fes els tests", "arregla el graf", "fes el build de windows", "mira el diff")
	m.obreCercaHist()
	if m.histCerca == nil {
		t.Fatal("no s'ha obert")
	}
	for _, r := range "build" {
		m.tecleaCercaHist(string(r), []rune{r})
	}
	if got := m.input.Value(); got != "fes el build de windows" {
		t.Errorf("volem la que porta «build», tenim %q", got)
	}
	// Enter la deixa al camp SENSE enviar-la: una ordre repescada que
	// s'executa sense mirar-la és com s'esborren coses.
	m.tecleaCercaHist("enter", nil)
	if m.histCerca != nil {
		t.Error("Enter ha de tancar la cerca")
	}
	if got := m.input.Value(); got != "fes el build de windows" {
		t.Errorf("Enter ha de deixar-la al camp: %q", got)
	}
}

// Ctrl+R repetit va a la coincidència anterior, com al bash.
func TestCtrlRRepetitVaALAnterior(t *testing.T) {
	m := modelAmbHistorial(t, "fes A", "fes B", "fes C")
	m.obreCercaHist()
	for _, r := range "fes" {
		m.tecleaCercaHist(string(r), []rune{r})
	}
	if got := m.input.Value(); got != "fes C" {
		t.Errorf("la primera ha de ser la més recent: %q", got)
	}
	m.tecleaCercaHist("ctrl+r", nil)
	if got := m.input.Value(); got != "fes B" {
		t.Errorf("Ctrl+R ha d'anar a l'anterior: %q", got)
	}
	m.tecleaCercaHist("ctrl+r", nil)
	if got := m.input.Value(); got != "fes A" {
		t.Errorf("i a l'anterior: %q", got)
	}
	// Al final de tot es queda, no dona la volta: donar la volta en
	// silenci fa passar de llarg el que buscaves.
	m.tecleaCercaHist("ctrl+r", nil)
	if got := m.input.Value(); got != "fes A" {
		t.Errorf("no ha de donar la volta: %q", got)
	}
}

// Esc deixa el camp com estava: obrir la cerca per equivocació no pot
// menjar-se el que estaves escrivint.
func TestEscDeixaElCampComEstava(t *testing.T) {
	m := modelAmbHistorial(t, "una cosa vella")
	m.input.SetValue("el que estava escrivint")
	m.obreCercaHist()
	m.tecleaCercaHist("esc", nil)
	if m.histCerca != nil {
		t.Error("Esc ha de tancar la cerca")
	}
	if got := m.input.Value(); got != "el que estava escrivint" {
		t.Errorf("Esc ha de tornar el que hi havia: %q", got)
	}
}

// Sense coincidències s'ha de veure, no quedar-se mut.
func TestQuanNoHiHaCoincidenciaHoDiu(t *testing.T) {
	m := modelAmbHistorial(t, "fes els tests")
	m.obreCercaHist()
	for _, r := range "zzz" {
		m.tecleaCercaHist(string(r), []rune{r})
	}
	if !strings.Contains(m.etiquetaCercaHist(), T("cerca.cap")) {
		t.Errorf("ha de dir que no n'hi ha cap: %q", m.etiquetaCercaHist())
	}
}

// Mentre la cerca és oberta, les tecles són seves: escriure-hi no pot
// disparar les dreceres de sota (una «t» obrint els todos, posem).
func TestLaCercaEsMenjaLesTeclesMentreEsOberta(t *testing.T) {
	m := modelAmbHistorial(t, "fes els tests")
	m.showTodos = false
	m.obreCercaHist()
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	mm := mod.(Model)
	if mm.histCerca == nil {
		t.Fatal("la cerca s'ha tancat sola")
	}
	if mm.histCerca.consulta != "t" {
		t.Errorf("la lletra havia d'anar a la consulta: %q", mm.histCerca.consulta)
	}
}
