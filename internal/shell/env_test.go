package shell

import (
	"strings"
	"testing"
)

func TestAmbUTF8AfegeixSenseTrepitjar(t *testing.T) {
	if j := strings.Join(ambUTF8([]string{"PATH=x"}), "\n"); !strings.Contains(j, "PYTHONIOENCODING=utf-8") {
		t.Fatalf("falta PYTHONIOENCODING: %s", j)
	}
	if j := strings.Join(ambUTF8([]string{"PATH=x", "pythonioencoding=latin-1"}), "\n"); strings.Contains(j, "PYTHONIOENCODING=utf-8") {
		t.Fatalf("ha trepitjat la variable de l'usuari: %s", j)
	}
	// open() ha de seguir amb la codificació de sempre.
	if j := strings.Join(ambUTF8(nil), "\n"); strings.Contains(j, "PYTHONUTF8") {
		t.Fatalf("PYTHONUTF8 canviaria open(): %s", j)
	}
}

func TestDecodeCP1252(t *testing.T) {
	if got := decodeCP1252([]byte("TOT B\xc9")); got != "TOT BÉ" {
		t.Fatalf("got %q", got)
	}
}
