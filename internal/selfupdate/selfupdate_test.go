package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDedueixRepo: l'ordre s'ha de poder cridar des de dins del projecte
// sense dir-li on és, i no ha de confondre cap altre go.mod amb el de Gregal.
func TestDedueixRepo(t *testing.T) {
	arrel := t.TempDir()
	sub := filepath.Join(arrel, "internal", "web")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(arrel, "go.mod"), []byte("module gregal\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DedueixRepo(sub); got != arrel {
		t.Fatalf("DedueixRepo(%s) = %q, esperava %q", sub, got, arrel)
	}
	if got := DedueixRepo(arrel); got != arrel {
		t.Fatalf("des de l'arrel = %q", got)
	}
}

// TestDedueixRepoNoEsConfon: un altre projecte Go no és Gregal.
func TestDedueixRepoNoEsConfon(t *testing.T) {
	arrel := t.TempDir()
	if err := os.WriteFile(filepath.Join(arrel, "go.mod"), []byte("module unaaltracosa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DedueixRepo(arrel); got != "" {
		t.Fatalf("s'ha confós: %q", got)
	}
}

// TestGoBinari: sempre hi ha d'haver una cadena d'eines utilitzable; si no,
// val més el nom sol (i que l'error vingui de l'exec) que una ruta inventada.
func TestGoBinari(t *testing.T) {
	if got := goBinari(); got == "" {
		t.Fatal("goBinari() buit")
	}
}

// TestShort: les capçaleres de commit han de quedar curtes.
func TestShort(t *testing.T) {
	if got := short("b066ff9aabbccdd"); got != "b066ff9" {
		t.Fatalf("short = %q", got)
	}
	if got := short("b066ff9"); got != "b066ff9" {
		t.Fatalf("short = %q", got)
	}
}

// TestTruncateLines: un error de git pot ser enorme; el missatge no ho ha de
// ser (i ha de dir que n'ha tallat).
func TestTruncateLines(t *testing.T) {
	got := truncateLines("a\nb\nc\nd\n", 2)
	if got != "a\nb\n… (2 línies més)" {
		t.Fatalf("truncateLines = %q", got)
	}
}
