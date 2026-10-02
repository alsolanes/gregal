package agent

import (
	"fmt"
	"testing"
)

// Idèntica 3 cops = doom (comportament de sempre).
func TestDoomExacteATres(t *testing.T) {
	var d DoomTracker
	sig := Sig("bash", `{"command":"ls /tmp"}`)
	if d.Note(sig) || d.Note(sig) {
		t.Fatal("les 2 primeres passen")
	}
	if !d.Note(sig) {
		t.Fatal("la 3a idèntica és doom")
	}
}

// Variant amb comptador (repro8 → repro11, cossos diferents): doom a la 4a.
func TestDoomDifusComptador(t *testing.T) {
	var d DoomTracker
	cap := "import { makePlayer, startGame, loadLevel } from './engine.js';\nconst cfg = "
	for i := 8; i <= 11; i++ {
		body := cap + fmt.Sprintf("{ nivell: %d, llavor: 'abc%ddef' };\nconsole.log('prova %d');", i, i*7, i)
		sig := Sig("bash", fmt.Sprintf(`{"command":"cd /proj && cat > repro%d.mjs <<'EOF'\n%s"}`, i, body))
		foc := d.Note(sig)
		if i < 11 && foc {
			t.Fatalf("la %da variant encara passa", i-7)
		}
		if i == 11 && !foc {
			t.Fatal("la 4a variant de plantilla ha de ser doom")
		}
	}
}

// Alternant eines no hi ha bucle.
func TestDoomAlternatNoDispara(t *testing.T) {
	var d DoomTracker
	a := Sig("bash", `{"command":"ls /tmp"}`)
	b := Sig("glob", `{"pattern":"*.go"}`)
	for i := 0; i < 6; i++ {
		if d.Note(a) || d.Note(b) {
			t.Fatalf("alternant no és doom (iteració %d)", i)
		}
	}
}

// Eines diferents amb mateixa forma no es confonen.
func TestDoomEinaDiferentNoDispara(t *testing.T) {
	var d DoomTracker
	for i := 0; i < 5; i++ {
		if d.Note(Sig("bash", fmt.Sprintf(`{"command":"prova %d"}`, i))) {
			t.Fatal("bash sol no fa doom")
		}
		if d.Note(Sig("glob", fmt.Sprintf(`{"pattern":"%d*.go"}`, i))) {
			t.Fatal("canviar d'eina trenca la ratxa")
		}
	}
}

// Quatre write de fitxers diferents a la mateixa carpeta no són cap bucle
// (vist en viu: el tercer quedava bloquejat perquè Sig agrupa per
// directori). Reescriure el MATEIX fitxer amb el mateix contingut, sí.
func TestDoomEscripturesDeFitxersDiferents(t *testing.T) {
	var d DoomTracker
	for _, f := range []string{"go.mod", "main.go", "calc.go", "calc_test.go"} {
		if d.Note(DoomSig("write", fmt.Sprintf(`{"path":"calc/%s","content":"package main // %s"}`, f, f))) {
			t.Fatalf("write de %s marcat com a repetit", f)
		}
	}
	var d2 DoomTracker
	same := DoomSig("write", `{"path":"calc/main.go","content":"package main"}`)
	d2.Note(same)
	d2.Note(same)
	if !d2.Note(same) {
		t.Fatal("el mateix fitxer amb el mateix contingut 3 cops sí que és bucle")
	}
	if DoomSig("bash", `{"command":"ls"}`) != Sig("bash", `{"command":"ls"}`) {
		t.Fatal("per a bash, DoomSig i Sig coincideixen")
	}
}

// Mateixa sortida exacta 3 cops = repetició (NoteOut).
func TestDoomMateixaSortidaATres(t *testing.T) {
	var d DoomTracker
	out := "llegit /tmp/x.txt\n1|hola\n"
	if d.NoteOut(out) || d.NoteOut(out) {
		t.Fatal("les 2 primeres passen")
	}
	if !d.NoteOut(out) {
		t.Fatal("la 3a idèntica és repetició")
	}
}

// Sortides buides no compten: moltes eines no tornen res.
func TestDoomSortidaBuidaNoDispara(t *testing.T) {
	var d DoomTracker
	for i := 0; i < 5; i++ {
		if d.NoteOut("(sense sortida)") {
			t.Fatal("les buides no fan doom")
		}
		if d.NoteOut("") {
			t.Fatal("les buides no fan doom")
		}
	}
}

// Sortida nova trenca la ratxa de sortides.
func TestDoomSortidaNovaTrencaRatxa(t *testing.T) {
	var d DoomTracker
	d.NoteOut("A")
	d.NoteOut("A")
	if d.NoteOut("B") {
		t.Fatal("una sortida nova no és repetició")
	}
	if d.NoteOut("B") {
		t.Fatal("2 cops encara passen")
	}
	if !d.NoteOut("B") {
		t.Fatal("3 cops de B sí")
	}
}

// SemblaFracas: marques de test en vermell sí, paraules soltes no.
func TestSemblaFracas(t *testing.T) {
	si := []string{
		"Traceback (most recent call last):\n  File \"x.py\"\nAssertionError: 1 != 2",
		"Ran 2 tests\nFAILED (failures=1)",
		"go: command not found",
		"ERROR: bloc no trobat a calc.py",
	}
	for _, s := range si {
		if !SemblaFracas(s) {
			t.Fatalf("hauria de semblar fracàs: %q", s)
		}
	}
	no := []string{
		"llegit fitxer sense errors",
		"TOT BÉ",
		"2 passed in 0.3s",
	}
	for _, s := range no {
		if SemblaFracas(s) {
			t.Fatalf("no hauria de semblar fracàs: %q", s)
		}
	}
}

// TascaDeReparacio: verbs de reparació sí, preguntes no.
func TestTascaDeReparacio(t *testing.T) {
	si := []string{
		"Arregla el bug de calc.py",
		"Fes que passin els tests",
		"fix the failing build",
	}
	for _, s := range si {
		if !TascaDeReparacio(s) {
			t.Fatalf("hauria de ser reparació: %q", s)
		}
	}
	no := []string{
		"Per què falla el test?",
		"Quants fitxers hi ha?",
		"Explica què fa calc.py",
	}
	for _, s := range no {
		if TascaDeReparacio(s) {
			t.Fatalf("no hauria de ser reparació: %q", s)
		}
	}
}
