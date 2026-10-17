package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projecte(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\n// Suma no és suma_total.\nfunc Suma(x, y int) int { return x + y }\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package a\n\nfunc Usa() int {\n\treturn Suma(1, 2) + Suma(3, 4)\n}\n"), 0o644)
	return dir
}

// El servidor és aquest mateix binari de test (fake_test.go).
func ambServidorFals(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("GREGAL_FAKE_LSP", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	vell := lookPath
	lookPath = func(string) (string, error) { return exe, nil }
	Configure([]Server{{Lang: "go", LanguageID: "go", Commands: [][]string{{"fake-gopls"}}, Extensions: []string{".go"}}}, false)
	m := &Manager{}
	t.Cleanup(func() {
		m.Close()
		lookPath = vell
		Configure(nil, false)
	})
	return m
}

func TestDefinicioReferenciesHover(t *testing.T) {
	dir := projecte(t) // abans que el servidor: les neteges van a l'inrevés
	m := ambServidorFals(t)
	ctx := context.Background()
	out, _, err := m.Do(ctx, dir, Request{Op: "definition", Path: "b.go", Line: 4, Symbol: "Suma"})
	if err != nil || out != "a.go:4:6: func Suma(x, y int) int { return x + y }" {
		t.Fatalf("definition: %q %v", out, err)
	}
	// Sense línia: la primera on surt el símbol com a paraula.
	out, _, err = m.Do(ctx, dir, Request{Op: "references", Path: "b.go", Symbol: "Suma"})
	if err != nil || !strings.HasPrefix(out, "4 results\n") || !strings.Contains(out, "b.go:4:9: return Suma(1, 2) + Suma(3, 4)") {
		t.Fatalf("references: %q %v", out, err)
	}
	out, _, err = m.Do(ctx, dir, Request{Op: "hover", Path: "a.go", Line: 4, Symbol: "Suma"})
	if err != nil || out != "func Suma()" {
		t.Fatalf("hover: %q %v", out, err)
	}
	if _, _, err = m.Do(ctx, dir, Request{Op: "hover", Path: "a.go", Line: 4, Symbol: "NoHiEs"}); err == nil {
		t.Fatal("un símbol que no és a la línia ha de fallar")
	}
}

// El rename torna els canvis de tots els fitxers (no els escriu) i no toca
// el que no és la paraula sencera (suma_total al comentari).
func TestRename(t *testing.T) {
	dir := projecte(t) // abans que el servidor: les neteges van a l'inrevés
	m := ambServidorFals(t)
	_, edits, err := m.Do(context.Background(), dir, Request{Op: "rename", Path: "a.go", Line: 4, Symbol: "Suma", NewName: "Add"})
	if err != nil || len(edits) != 2 {
		t.Fatalf("rename: %v %+v", err, edits)
	}
	if !strings.Contains(edits[0].New, "func Add(x, y int)") || !strings.Contains(edits[0].New, "// Add no és suma_total.") {
		t.Fatalf("a.go: %q", edits[0].New)
	}
	if !strings.Contains(edits[1].New, "return Add(1, 2) + Add(3, 4)") || edits[1].Edits != 2 {
		t.Fatalf("b.go: %q", edits[1].New)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "a.go")); strings.Contains(string(raw), "Add") {
		t.Fatal("Do no ha d'escriure res")
	}
}

// Diagnòstics: s'espera la publicació després d'obrir o canviar el fitxer.
func TestDiagnosticsDespresDeCanviar(t *testing.T) {
	dir := projecte(t) // abans que el servidor: les neteges van a l'inrevés
	m := ambServidorFals(t)
	out, _, err := m.Do(context.Background(), dir, Request{Op: "diagnostics", Path: "a.go"})
	if err != nil || out != "a.go:1:1: warning: 4 lines (fake)" {
		t.Fatalf("diagnostics: %q %v", out, err)
	}
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc Suma() {}\n\n\n"), 0o644)
	out, _, _ = m.Do(context.Background(), dir, Request{Op: "diagnostics", Path: "a.go"})
	if out != "a.go:1:1: warning: 5 lines (fake)" {
		t.Fatalf("després del canvi: %q", out)
	}
}

func TestSenseServidor(t *testing.T) {
	vell := lookPath
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	Configure(nil, false)
	t.Cleanup(func() { lookPath = vell; Configure(nil, false) })
	if len(Disponibles()) != 0 {
		t.Fatal("sense binaris no hi ha servidors")
	}
	if _, _, err := (&Manager{}).Do(context.Background(), t.TempDir(), Request{Op: "hover", Path: "x.go", Line: 1}); err == nil {
		t.Fatal("ha de dir que no hi ha servidor")
	}
}

func TestUTF16(t *testing.T) {
	l := "x := \"😀\" + nom"
	if c := ColUTF16(l, 11); c != 12 {
		t.Fatalf("ColUTF16=%d", c)
	}
	if r := ColRunes(l, 12); r != 11 {
		t.Fatalf("ColRunes=%d", r)
	}
	if p := Path(URI(filepath.Join(t.TempDir(), "a b.go"))); !strings.HasSuffix(p, "a b.go") {
		t.Fatalf("URI/Path: %q", p)
	}
}
