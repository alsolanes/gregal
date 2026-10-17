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
