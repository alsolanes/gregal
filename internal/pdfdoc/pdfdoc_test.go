package pdfdoc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var bg = context.Background()

// copia posa una còpia d'un fixture en un directori temporal.
func copia(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func textOf(t *testing.T, path string) []string {
	t.Helper()
	pages, err := ReadPages(bg, path)
	if err != nil {
		t.Fatalf("ReadPages %s: %v", path, err)
	}
	var out []string
	for _, p := range pages {
		out = append(out, p.Text)
	}
	return out
}

func TestReadText(t *testing.T) {
	got := textOf(t, filepath.Join("testdata", "text3.pdf"))
	if len(got) != 3 {
		t.Fatalf("pàgines: %d %q", len(got), got)
	}
	for i, w := range []string{"giraffe", "walrus", "pelican"} {
		if !strings.Contains(got[i], w) {
			t.Errorf("pàgina %d: %q no conté %q", i+1, got[i], w)
		}
	}
	if !strings.Contains(got[0], "Pàgina") {
		t.Errorf("accents: %q", got[0])
	}
}

func TestReadScanHasNoText(t *testing.T) {
	got := textOf(t, filepath.Join("testdata", "scan.pdf"))
	if len(got) != 1 || strings.TrimSpace(got[0]) != "" {
		t.Fatalf("una pàgina escanejada no té text: %q", got)
	}
}

func TestInfoAndFields(t *testing.T) {
	inf, err := ReadInfo(bg, filepath.Join("testdata", "form.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if inf.Pages != 1 || !inf.Form || len(inf.Sizes) != 1 || !strings.Contains(inf.Sizes[0], "A4") {
		t.Fatalf("info: %+v", inf)
	}
	names := map[string]string{}
	for _, f := range inf.Fields {
		names[f.Name] = f.Type
	}
	for _, n := range []string{"nom", "accepto", "ciutat"} {
		if names[n] == "" {
			t.Errorf("falta el camp %s: %+v", n, inf.Fields)
		}
	}
}

func TestMergeSplitSelectDelete(t *testing.T) {
	a := copia(t, "text3.pdf")
	dir := filepath.Dir(a)
	merged := filepath.Join(dir, "merged.pdf")
	if err := Merge(bg, []string{a, filepath.Join("testdata", "scan.pdf"), a}, merged); err != nil {
		t.Fatal(err)
	}
	if n, _ := PageCount(bg, merged); n != 7 {
		t.Fatalf("merge: %d pàgines, volia 7", n)
	}

	// Extreure i reordenar: 3,1 → pelican, giraffe.
	sel := filepath.Join(dir, "sel.pdf")
	if err := Select(bg, a, sel, "3,1"); err != nil {
		t.Fatal(err)
	}
	got := textOf(t, sel)
	if len(got) != 2 || !strings.Contains(got[0], "pelican") || !strings.Contains(got[1], "giraffe") {
		t.Fatalf("select 3,1: %q", got)
	}
	if err := Select(bg, a, sel, "2-last"); err != nil {
		t.Fatal(err)
	}
	if got := textOf(t, sel); len(got) != 2 || !strings.Contains(got[0], "walrus") {
		t.Fatalf("select 2-last: %q", got)
	}

	// Split per talls: [1] [2-3].
	outs, err := Split(bg, a, filepath.Join(dir, "parts"), 0, "2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 2 || !strings.HasSuffix(outs[0], "text3_1.pdf") || !strings.HasSuffix(outs[1], "text3_2-3.pdf") {
		t.Fatalf("split: %v", outs)
	}
	if n, _ := PageCount(bg, outs[1]); n != 2 {
		t.Fatalf("split segon tros: %d", n)
	}
	// Split per mida: una pàgina per fitxer.
	if outs, err = Split(bg, a, filepath.Join(dir, "one"), 1, "", nil); err != nil || len(outs) != 3 {
		t.Fatalf("split span 1: %v %v", outs, err)
	}

	// Esborrar in situ.
	if err := Delete(bg, a, a, "2"); err != nil {
		t.Fatal(err)
	}
	if got := textOf(t, a); len(got) != 2 || strings.Contains(strings.Join(got, " "), "walrus") {
		t.Fatalf("delete 2: %q", got)
	}
	if err := Delete(bg, a, a, ""); err == nil {
		t.Fatal("delete sense pàgines hauria de fallar")
	}
}

func TestRotate(t *testing.T) {
	a := copia(t, "text3.pdf")
	if err := Rotate(bg, a, a, 90, "1"); err != nil {
		t.Fatal(err)
	}
	inf, err := ReadInfo(bg, a)
	if err != nil {
		t.Fatal(err)
	}
	// La pàgina 1 girada es veu apaïsada; les altres no.
	if len(inf.Dims) != 3 || inf.Dims[0][0] <= inf.Dims[0][1] || inf.Dims[1][0] >= inf.Dims[1][1] {
		t.Fatalf("rotate: %v", inf.Dims)
	}
	if err := Rotate(bg, a, a, 45, ""); err == nil {
		t.Fatal("45 graus hauria de fallar")
	}
}

func TestAddTextAndStamp(t *testing.T) {
	a := copia(t, "text3.pdf")
	if err := AddText(bg, a, a, StampOpts{Text: "CONFIDENCIAL", Pages: "1", Size: 40, Color: "#CC0000", Opacity: 0.5, Rotation: 45}); err != nil {
		t.Fatal(err)
	}
	if err := AddText(bg, a, a, StampOpts{Text: "Signat: Aleix", Pages: "2-3", Position: "bl", X: 72, Y: 40}); err != nil {
		t.Fatal(err)
	}
	got := textOf(t, a)
	if !strings.Contains(got[0], "CONFIDENCIAL") || !strings.Contains(got[0], "giraffe") {
		t.Fatalf("segell p1: %q", got[0])
	}
	if !strings.Contains(got[2], "Signat: Aleix") || strings.Contains(got[0], "Signat") {
		t.Fatalf("text p3: %q / p1: %q", got[2], got[0])
	}
	if err := AddText(bg, a, a, StampOpts{Text: "x", Position: "middle"}); err == nil {
		t.Fatal("position invàlida hauria de fallar")
	}
}

func TestFillForm(t *testing.T) {
	a := copia(t, "form.pdf")
	filled, missing, err := FillForm(bg, a, a, map[string]string{"nom": "Núria Puig", "accepto": "sí", "ciutat": "Lleida", "inexistent": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filled) != 3 || len(missing) != 1 || missing[0] != "inexistent" {
		t.Fatalf("filled=%v missing=%v", filled, missing)
	}
	fs, err := Fields(bg, a)
	if err != nil {
		t.Fatal(err)
	}
	vals := map[string]string{}
	for _, f := range fs {
		vals[f.Name] = f.Value
	}
	if vals["nom"] != "Núria Puig" || vals["accepto"] != "true" || vals["ciutat"] != "Lleida" {
		t.Fatalf("valors: %v", vals)
	}
	if _, _, err := FillForm(bg, a, a, map[string]string{"ciutat": "Vic"}); err == nil {
		t.Fatal("una opció que no hi és hauria de fallar")
	}
	if _, _, err := FillForm(bg, copia(t, "text3.pdf"), filepath.Join(t.TempDir(), "o.pdf"), map[string]string{"x": "y"}); err == nil {
		t.Fatal("un PDF sense formulari hauria de fallar")
	}
}

func TestAnnotate(t *testing.T) {
	a := copia(t, "text3.pdf")
	if err := Annotate(bg, a, a, AnnotOpts{Kind: "note", Pages: "2", X: 300, Y: 700, Text: "Revisar aquesta xifra", Author: "Gregal"}); err != nil {
		t.Fatal(err)
	}
	if err := Annotate(bg, a, a, AnnotOpts{Kind: "highlight", Pages: "1", X: 70, Y: 755, W: 200, H: 20}); err != nil {
		t.Fatal(err)
	}
	if err := Annotate(bg, a, a, AnnotOpts{Kind: "link", Pages: "3", X: 70, Y: 700, W: 100, H: 20, URL: "https://example.org"}); err != nil {
		t.Fatal(err)
	}
	inf, err := ReadInfo(bg, a)
	if err != nil {
		t.Fatal(err)
	}
	var note bool
	types := map[string]bool{}
	for _, an := range inf.Annots {
		types[an.Type] = true
		if an.Page == 2 && strings.Contains(an.Content, "Revisar") {
			note = true
		}
	}
	if !note || len(inf.Annots) < 3 {
		t.Fatalf("anotacions: %+v", inf.Annots)
	}
	if err := Annotate(bg, a, a, AnnotOpts{Kind: "note"}); err == nil {
		t.Fatal("una nota sense text hauria de fallar")
	}
}

func TestSetInfo(t *testing.T) {
	a := copia(t, "text3.pdf")
	if err := SetInfo(bg, a, a, map[string]string{"title": "Informe d'àrea", "author": "Gregal"}); err != nil {
		t.Fatal(err)
	}
	inf, err := ReadInfo(bg, a)
	if err != nil {
		t.Fatal(err)
	}
	if inf.Title != "Informe d'àrea" || inf.Author != "Gregal" {
		t.Fatalf("info: %+v", inf)
	}
}

func TestBrokenOutputNeverReplacesTheOriginal(t *testing.T) {
	a := copia(t, "text3.pdf")
	before, _ := os.ReadFile(a)
	if err := Select(bg, a, a, "9"); err == nil {
		t.Fatal("una pàgina inexistent hauria de fallar")
	}
	after, _ := os.ReadFile(a)
	if string(before) != string(after) {
		t.Fatal("un error ha modificat l'original")
	}
	bad := filepath.Join(t.TempDir(), "bad.pdf")
	os.WriteFile(bad, []byte("%PDF-1.4\nbrossa"), 0o644)
	if _, err := ReadInfo(bg, bad); err == nil {
		t.Fatal("un PDF trencat hauria de fallar")
	}
}

func TestSelection(t *testing.T) {
	for in, want := range map[string]int{"": 0, "all": 0, "1-3,5": 2, "2-last": 1, "odd": 1} {
		sel, err := Selection(in)
		if err != nil || len(sel) != want {
			t.Errorf("%q → %v %v", in, sel, err)
		}
	}
	if _, err := Selection("a-b-c"); err == nil {
		t.Error("selecció invàlida hauria de fallar")
	}
}
