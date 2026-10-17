package tools

import (
	"os"
	"strings"
	"testing"
)

// replace_all substitueix totes les aparicions, també en CRLF, i sense cap
// aparició exacta torna a la via d'un sol bloc.
func TestEditReplaceAll(t *testing.T) {
	p := fitxer(t, "x := comptador\r\ny := comptador + 1\r\n")
	s := nouSeguiment()
	if _, err := s.EditAll(p, "comptador", "total"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "x := total\r\ny := total + 1\r\n" {
		t.Fatalf("got %q", b)
	}
	if _, err := s.EditAll(p, "x := total\ny := total", "a := total\nb := total"); err != nil {
		t.Fatal(err)
	}
	if b, _ = os.ReadFile(p); string(b) != "a := total\r\nb := total + 1\r\n" {
		t.Fatalf("CRLF: got %q", b)
	}
	// Sense replace_all, dues aparicions continuen sent ambigües.
	if _, err := s.Edit(p, "total", "suma"); err == nil {
		t.Fatal("edit sense replace_all hauria de ser ambigu")
	}
}

// old buit és un argument perdut, no «old igual a new».
func TestEditOldBuitDiuQueFalta(t *testing.T) {
	p := fitxer(t, "a\n")
	_, err := nouSeguiment().Edit(p, "", "b")
	if err == nil || !strings.Contains(err.Error(), "old_string és buit") {
		t.Fatalf("err=%v", err)
	}
}
