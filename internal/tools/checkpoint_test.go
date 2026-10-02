package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTmp(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTmp(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// Tres escriptures, tornar al checkpoint 1: cal l'estat exacte d'aleshores.
func TestRewindToParcial(t *testing.T) {
	j := NewJournal()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeTmp(t, a, "v0")
	j.SnapOp(a, "write")
	writeTmp(t, a, "v1") // ev 1
	j.SnapOp(a, "write")
	writeTmp(t, a, "v2") // ev 2
	j.SnapOp(b, "write")
	writeTmp(t, b, "nou") // ev 3 (fitxer nou)

	if got := j.Seq(); got != 3 {
		t.Fatalf("seq=%d", got)
	}
	if _, err := j.RewindTo(1); err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, a); got != "v1" {
		t.Fatalf("a=%q, volia v1", got)
	}
	if _, err := os.Stat(b); err == nil {
		t.Fatal("b hauria d'haver desaparegut (era nou a ev 3)")
	}
	if got := j.Seq(); got != 1 {
		t.Fatalf("seq=%d després, volia 1", got)
	}
	// El journal queda consistent: rewind total restaura v0.
	if _, err := j.Rewind(); err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, a); got != "v0" {
		t.Fatalf("a=%q, volia v0", got)
	}
}

// Edit + write barrejats sobre el mateix fitxer.
func TestRewindToEdit(t *testing.T) {
	j := NewJournal()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeTmp(t, a, "hola món")
	j.SnapOp(a, "edit") // ev 1 (abans: "hola món")
	writeTmp(t, a, "adeu món")
	j.SnapOp(a, "write") // ev 2 (abans: "adeu món")
	writeTmp(t, a, "tercera")

	if _, err := j.RewindTo(1); err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, a); got != "adeu món" {
		t.Fatalf("a=%q, volia 'adeu món'", got)
	}
}

// Fora de rang → error, sense tocar res.
func TestRewindToRang(t *testing.T) {
	j := NewJournal()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeTmp(t, a, "x")
	j.SnapOp(a, "write")
	writeTmp(t, a, "y")
	for _, bad := range []int{-1, 2, 99} {
		if _, err := j.RewindTo(bad); err == nil {
			t.Fatalf("seq %d hauria de fallar", bad)
		}
	}
	if got := readTmp(t, a); got != "y" {
		t.Fatalf("a=%q, no s'havia de tocar", got)
	}
}

// RewindTo al seq actual = no-op ("res a desfer").
func TestRewindToNoop(t *testing.T) {
	j := NewJournal()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeTmp(t, a, "x")
	j.SnapOp(a, "write")
	writeTmp(t, a, "y")
	out, err := j.RewindTo(1)
	if err != nil {
		t.Fatal(err)
	}
	if out != "res a desfer" {
		t.Fatalf("out=%q", out)
	}
}

// Checkpoints llista en ordre amb op i seq.
func TestCheckpointsOrdre(t *testing.T) {
	j := NewJournal()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeTmp(t, a, "x")
	j.SnapOp(a, "write")
	j.SnapOp(a, "edit")
	cps := j.Checkpoints()
	if len(cps) != 2 || cps[0].Seq != 1 || cps[1].Seq != 2 {
		t.Fatalf("cps=%+v", cps)
	}
	if cps[0].Op != "write" || cps[1].Op != "edit" {
		t.Fatalf("ops=%q,%q", cps[0].Op, cps[1].Op)
	}
}
