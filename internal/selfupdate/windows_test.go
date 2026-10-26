package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVersioDelRepoIDesfasament: el binari es compara amb la versió del
// codi, no només el repositori amb el remot.
func TestVersioDelRepoIDesfasament(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\n// versio\nvar version = \"v1.17.1\"\n"), 0o600)
	if got := versioDelRepo(dir); got != "v1.17.1" {
		t.Fatalf("versió del codi: %q", got)
	}
	if versioCurta("gregal v1.11.0") != "v1.11.0" || versioCurta("") != "" {
		t.Fatal("versió del binari mal llegida")
	}
	if versioDelRepo(t.TempDir()) != "" {
		t.Fatal("sense main.go, cap versió")
	}
}
