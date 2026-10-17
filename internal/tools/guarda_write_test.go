package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write sobre un fitxer existent: s'ha d'haver llegit i no pot haver
// canviat. Un fitxer nou, o un que el model mateix ha escrit, passa.
func TestGuardaEscriptura(t *testing.T) {
	dir := t.TempDir()
	nou := filepath.Join(dir, "nou.txt")
	s := nouSeguiment()
	if err := s.GuardaEscriptura(nou); err != nil {
		t.Fatalf("fitxer nou: %v", err)
	}
	vell := filepath.Join(dir, "vell.go")
	os.WriteFile(vell, []byte("package a\n"), 0o644)
	if err := s.GuardaEscriptura(vell); err == nil || !strings.Contains(err.Error(), "no l'has llegit") {
		t.Fatalf("sense llegir: %v", err)
	}
	if _, err := s.Read(vell, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.GuardaEscriptura(vell); err != nil {
		t.Fatalf("llegit: %v", err)
	}
	// Algú el canvia després de la lectura.
	os.WriteFile(vell, []byte("package a\n\nvar X = 1\n"), 0o644)
	if err := s.GuardaEscriptura(vell); err == nil || !strings.Contains(err.Error(), "ha canviat") {
		t.Fatalf("canviat: %v", err)
	}
	// El que el model ha escrit ell mateix (Coneix) es pot tornar a escriure.
	os.WriteFile(nou, []byte("hola\n"), 0o644)
	s.Coneix(nou)
	if err := s.GuardaEscriptura(nou); err != nil {
		t.Fatalf("escrit pel model: %v", err)
	}
}

// Reescriu (el rename de lsp) no trepitja un fitxer que ha canviat
// mentrestant i, si escriu, el model el coneix al dia.
func TestReescriu(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.go")
	os.WriteFile(p, []byte("func Suma() {}\n"), 0o644)
	s := nouSeguiment()
	if _, err := s.Reescriu(p, "una altra cosa\n", "x"); err == nil {
		t.Fatal("ha canviat: no s'hi ha de tocar")
	}
	out, err := s.Reescriu(p, "func Suma() {}\n", "func Add() {}\n")
	if err != nil || !strings.Contains(out, "Add") {
		t.Fatalf("%q %v", out, err)
	}
	if err := s.GuardaEscriptura(p); err != nil {
		t.Fatalf("després de Reescriu el model el té al dia: %v", err)
	}
}

// Un fitxer que el mapa ha adjuntat sencer compta com a vist mentre no
// canviï (t3 del banc).
func TestGuardaEscripturaVistAlMapa(t *testing.T) {
	p := filepath.Join(t.TempDir(), "utils.py")
	os.WriteFile(p, []byte("def f():\n    pass\n"), 0o644)
	VistSencer(p, []byte("def f():\n    pass\n"))
	s := nouSeguiment()
	if err := s.GuardaEscriptura(p); err != nil {
		t.Fatalf("adjuntat pel mapa: %v", err)
	}
	os.WriteFile(p, []byte("def g():\n    pass\n"), 0o644)
	if err := s.GuardaEscriptura(p); err == nil {
		t.Fatal("ha canviat des que es va adjuntar")
	}
}
