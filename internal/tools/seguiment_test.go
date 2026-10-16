package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nouSeguiment() *Seguiment { return &Seguiment{fitxers: map[string]*estatFitxer{}} }

const goTabs = "package a\n\nfunc F() {\n\tg()\n}\n"

// P9: una edició aproximada sobre un fitxer que el model no ha llegit no
// s'aplica; l'exacta sí.
func TestEdicioAproximadaSenseLecturaEsRebutja(t *testing.T) {
	s := nouSeguiment()
	p := fitxer(t, goTabs)
	_, err := s.Edit(p, "func F() {\n    g()\n}", "func F() {\n    h()\n}")
	if err == nil || !strings.Contains(err.Error(), "no l'has llegit") || !strings.Contains(err.Error(), "read") {
		t.Fatalf("havia de demanar llegir-lo: %v", err)
	}
	if raw, _ := os.ReadFile(p); string(raw) != goTabs {
		t.Fatalf("no s'havia de tocar: %q", raw)
	}
	if _, err := s.Edit(p, "\tg()", "\th()"); err != nil {
		t.Fatalf("exacte i únic s'aplica sense llegir: %v", err)
	}
}

func TestEdicioAproximadaDespresDeLlegirSAplica(t *testing.T) {
	s := nouSeguiment()
	p := fitxer(t, goTabs)
	if _, err := s.Read(p, 0, 0); err != nil {
		t.Fatal(err)
	}
	nota, err := s.Edit(p, "func F() {\n    g()\n}", "func F() {\n    h()\n}")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nota, nivIndentacio) {
		t.Errorf("%q", nota)
	}
	// I la segona edició sobre el que ha deixat la primera també: el model
	// sap què hi ha perquè ho ha escrit ell.
	if _, err := s.Edit(p, "func F() {\n    h()\n}", "func F() {\n    k()\n}"); err != nil {
		t.Fatalf("després d'una edició pròpia el fitxer és al dia: %v", err)
	}
}

func TestFitxerCanviatPerUnAltreDespresDeLlegirEsRebutja(t *testing.T) {
	s := nouSeguiment()
	p := fitxer(t, goTabs)
	if _, err := s.Read(p, 0, 0); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte(goTabs+"\nfunc Z() {}\n"), 0o644) // l'usuari, un altre agent
	_, err := s.Edit(p, "func F() {\n    g()\n}", "x")
	if err == nil || !strings.Contains(err.Error(), "ha canviat") {
		t.Fatalf("havia de dir que ha canviat: %v", err)
	}
	_, err = s.Patch(p, []PatchOp{{AfterLine: 2, New: "// x"}})
	if err == nil || !strings.Contains(err.Error(), "after_line") {
		t.Fatalf("after_line sobre un fitxer desfasat és a cegues: %v", err)
	}
	// Coneix (després d'un write propi o del hook) el torna a donar per bo.
	s.Coneix(p)
	if _, err := s.Edit(p, "func F() {\n    g()\n}", "func F() {\n    h()\n}"); err != nil {
		t.Fatal(err)
	}
}

func fitxerLlarg(t *testing.T, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "línia número %d amb una mica de text\n", i)
	}
	return fitxer(t, b.String())
}

// P10: rellegir el mateix rang sense canvis no repeteix el contingut.
func TestRelecturaSenseCanvis(t *testing.T) {
	s := nouSeguiment()
	p := fitxerLlarg(t, 50)
	s.NouPas()
	primer, _ := s.Read(p, 0, 0)
	if strings.Contains(primer, "sense canvis") {
		t.Fatal("la primera lectura ha de ser sencera")
	}
	s.NouPas()
	s.NouPas()
	segon, _ := s.Read(p, 0, 0)
	if !strings.Contains(segon, "sense canvis") || !strings.Contains(segon, "fa 2 crides") {
		t.Fatalf("la segona ha de dir sense canvis: %q", segon)
	}
	// Un subrang del que ja ha llegit, també.
	s.NouPas()
	if sub, _ := s.Read(p, 10, 5); !strings.Contains(sub, "sense canvis") && len(sub) >= minRelectura {
		t.Errorf("subrang: %q", sub)
	}
	// Hi torna just després del «sense canvis»: no la té, se li dona.
	s.NouPas()
	tercer, _ := s.Read(p, 0, 0)
	if tercer != primer {
		t.Fatalf("la tercera ha de ser sencera: %q", tercer)
	}
}

func TestRelecturaDespresDeCanviOLluny(t *testing.T) {
	s := nouSeguiment()
	p := fitxerLlarg(t, 50)
	s.Read(p, 0, 0)
	s.NouPas()
	if _, err := s.Edit(p, "línia número 7 amb", "línia SET amb"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Read(p, 0, 0); strings.Contains(r, "sense canvis") || !strings.Contains(r, "línia SET") {
		t.Fatalf("després d'editar s'ha de tornar a donar: %q", r)
	}
	for i := 0; i <= finestraRelectura; i++ {
		s.NouPas()
	}
	if r, _ := s.Read(p, 0, 0); strings.Contains(r, "sense canvis") {
		t.Fatal("fora de la finestra la lectura pot haver estat podada: s'ha de donar sencera")
	}
}

func TestReadTallaPerLiniaIDiuOnSeguir(t *testing.T) {
	p := fitxerLlarg(t, 1000)
	out, err := Read(p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > MaxOutputChars+200 || !strings.Contains(out, "continua amb offset=") {
		t.Fatalf("len %d, final %q", len(out), out[len(out)-120:])
	}
	cos := out[:strings.LastIndex(out, "…(retallat")]
	if !strings.HasSuffix(cos, " text\n") {
		t.Errorf("ha de tallar per una línia sencera: %q", cos[len(cos)-60:])
	}
}

// P5: cap i cua, amb la sortida sencera desada.
func TestRetallaCapICuaDesaLaSortida(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 1; i <= 2000; i++ {
		fmt.Fprintf(&b, "ok   paquet/%d\n", i)
	}
	b.WriteString("--- FAIL: TestImportant\n    error al final\n")
	full := b.String()
	out := retallaCapICua(full, MaxOutputChars, func(s string) string { return desaSortida(dir, "bash", s) })
	if len(out) > MaxOutputChars+400 {
		t.Errorf("massa llarg: %d", len(out))
	}
	if !strings.HasPrefix(out, "ok   paquet/1\n") || !strings.Contains(out, "--- FAIL: TestImportant") {
		t.Errorf("ha de guardar el cap i la cua (on hi ha l'error)")
	}
	i := strings.Index(out, ".gregal/out/")
	if i < 0 {
		t.Fatalf("ha de dir on és la sortida sencera: %q", out)
	}
	rel := out[i:]
	rel = rel[:strings.Index(rel, ".txt")+4]
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil || string(raw) != full {
		t.Fatalf("la sortida desada ha de ser sencera: %v", err)
	}
	if gi, _ := os.ReadFile(filepath.Join(dir, ".gregal", "out", ".gitignore")); string(gi) != "*\n" {
		t.Errorf(".gitignore: %q", gi)
	}
	if curt := retallaCapICua("hola", 100, nil); curt != "hola" {
		t.Errorf("%q", curt)
	}
}
