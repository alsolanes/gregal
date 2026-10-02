package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Un error de tipus (no de sintaxi) en un mòdul Go surt al resultat de
// l'edició com a [diagnòstic] amb la línia del go vet.
func TestDiagnosticaGoVetDiuElsErrorsDeTipus(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sense go al PATH")
	}
	SetDiagMode("full")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module prova\n\ngo 1.22\n"), 0o600)
	p := filepath.Join(dir, "calc.go")
	os.WriteFile(p, []byte("package calc\n\nfunc Suma(a, b int) int { return a + c }\n"), 0o600)
	d := Diagnostica(p)
	if !strings.Contains(d, "undefined: c") || (!strings.HasPrefix(d, "[diagnòstic] ") && !strings.HasPrefix(d, "[sintaxi] ")) {
		t.Fatalf("diagnòstic: %q", d)
	}
	// Arreglat: silenci.
	os.WriteFile(p, []byte("package calc\n\nfunc Suma(a, b int) int { return a + b }\n"), 0o600)
	if d := Diagnostica(p); d != "" {
		t.Fatalf("amb el fitxer bo no ha de dir res: %q", d)
	}
	// Amb hooks.diag: syntax no es passa el vet.
	SetDiagMode("syntax")
	t.Cleanup(func() { SetDiagMode("") })
	os.WriteFile(p, []byte("package calc\n\nfunc Suma(a, b int) int { return a + c }\n"), 0o600)
	if d := Diagnostica(p); d != "" {
		t.Fatalf("en mode syntax no hi ha vet: %q", d)
	}
}

// Un .go fora de cap mòdul no es veta (l'error seria del go, no del fitxer).
func TestDiagnosticaGoForaDeModulCalla(t *testing.T) {
	SetDiagMode("full")
	t.Cleanup(func() { SetDiagMode("") })
	p := filepath.Join(t.TempDir(), "solt.go")
	os.WriteFile(p, []byte("package solt\n\nfunc F() int { return x }\n"), 0o600)
	if d := Diagnostica(p); d != "" {
		t.Fatalf("fora de mòdul: %q", d)
	}
}

func TestDiagnosticaPython(t *testing.T) {
	if _, err := exec.LookPath("python"); err != nil {
		if _, err := exec.LookPath("python3"); err != nil {
			t.Skip("sense python")
		}
	}
	SetDiagMode("full")
	t.Cleanup(func() { SetDiagMode("") })
	p := filepath.Join(t.TempDir(), "mal.py")
	os.WriteFile(p, []byte("def f(:\n    pass\n"), 0o600)
	d := Diagnostica(p)
	if !strings.Contains(d, "line 1") || (!strings.Contains(d, "[diagnòstic]") && !strings.Contains(d, "[sintaxi]")) {
		t.Fatalf("python: %q", d)
	}
}
