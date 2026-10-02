package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func writePatchTmp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// Tres blocs d'un cop, inclosa inserció per línia.
func TestPatchMulti(t *testing.T) {
	p := writePatchTmp(t, "a1\nb1\nc1\nd1\n")
	err := Patch(p, []PatchOp{
		{Old: "a1", New: "a2"},
		{Old: "c1", New: "c2"},
		{AfterLine: 2, New: "X"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(mustReadPatch(t, p))
	if got != "a2\nb1\nX\nc2\nd1\n" {
		t.Fatalf("resultat: %q", got)
	}
}

// Bloc ambigu: error i fitxer intacte (atomicitat).
func TestPatchAtomic(t *testing.T) {
	orig := "x\ny\nx\n"
	p := writePatchTmp(t, orig)
	if err := Patch(p, []PatchOp{{Old: "a-ok", New: "b"}, {Old: "x", New: "z"}}); err == nil {
		t.Fatal("hauria de fallar (ambigu)")
	}
	if got := string(mustReadPatch(t, p)); got != orig {
		t.Fatalf("fitxer tocat: %q", got)
	}
	if err := Patch(p, []PatchOp{{Old: "no-hi-és", New: "b"}}); err == nil {
		t.Fatal("hauria de fallar (absent)")
	}
	if got := string(mustReadPatch(t, p)); got != orig {
		t.Fatalf("fitxer tocat: %q", got)
	}
	if err := Patch(p, []PatchOp{{AfterLine: 99, New: "b"}}); err == nil {
		t.Fatal("hauria de fallar (fora de rang)")
	}
}

func mustReadPatch(t *testing.T, p string) []byte {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
