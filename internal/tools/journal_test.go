package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalRewind(t *testing.T) {
	j := NewJournal()
	dir := t.TempDir()
	old := filepath.Join(dir, "vell.txt")
	mustWrite(t, old, "original\n")
	nou := filepath.Join(dir, "nou.txt")

	j.Snap(old)
	j.Snap(nou) // no existeix → nil
	if _, err := os.Stat(nou); err == nil {
		t.Fatal("nou no hauria d'existir encara")
	}
	mustWrite(t, old, "canviat\n")
	mustWrite(t, nou, "creat\n")
	j.Snap(old) // segon snap: conserva el primer

	if got := j.Files(); len(got) != 2 || got[0] != old || got[1] != nou {
		t.Fatalf("files=%v", got)
	}
	sum, err := j.Rewind()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sum, "restaurats") || !strings.Contains(sum, "esborrats") {
		t.Fatalf("resum=%q", sum)
	}
	if raw, _ := os.ReadFile(old); string(raw) != "original\n" {
		t.Fatalf("vell=%q", raw)
	}
	if _, err := os.Stat(nou); !os.IsNotExist(err) {
		t.Fatal("nou hauria d'haver desaparegut")
	}
	if len(j.Files()) != 0 {
		t.Fatal("el journal ha de quedar buit")
	}
}

// E2b: marques de conversa + neteja en rewind.
func TestConvoMarks(t *testing.T) {
	j := NewJournal()
	p := filepath.Join(t.TempDir(), "f.txt")
	mustWrite(t, p, "v0")
	j.MarkConvo(2) // seq 0, conversa amb 2 missatges
	j.Snap(p)
	mustWrite(t, p, "v1")
	j.MarkConvo(4) // seq 1, conversa amb 4
	j.Snap(p)
	mustWrite(t, p, "v2")
	if got := j.ConvoLenAt(1); got != 4 {
		t.Fatalf("ConvoLenAt(1)=%d, volia 4", got)
	}
	if got := j.ConvoLenAt(0); got != 0 {
		t.Fatalf("ConvoLenAt(0)=%d, volia 0", got)
	}
	if _, err := j.RewindTo(1); err != nil {
		t.Fatal(err)
	}
	if got := j.ConvoLenAt(1); got != 4 {
		t.Fatalf("després de rewind, ConvoLenAt(1)=%d", got)
	}
	if got := j.ConvoLenAt(99); got != 4 {
		t.Fatalf("el més proper per sota: %d", got)
	}
}
