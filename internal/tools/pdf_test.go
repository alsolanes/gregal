package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pdfFixture copia un PDF petit de internal/pdfdoc/testdata a un directori
// temporal (els tests escriuen al fitxer).
func pdfFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "pdfdoc", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPdfReadText(t *testing.T) {
	p := pdfFixture(t, "text3.pdf")
	out, imgs, err := PdfRead(context.Background(), PdfReadOpts{Path: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 0 || !strings.Contains(out, "PDF de 3 pàgines") || !strings.Contains(out, "— Pàgina 2 —") || !strings.Contains(out, "walrus") {
		t.Fatalf("pdf_read: %q", out)
	}
	out, _, _ = PdfRead(context.Background(), PdfReadOpts{Path: p, Pages: "3"})
	if strings.Contains(out, "giraffe") || !strings.Contains(out, "pelican") {
		t.Fatalf("pages=3: %q", out)
	}
	// office_read també llegeix PDF (els models el fan servir per a tot).
	if txt, err := OfficeRead(p); err != nil || !strings.Contains(txt, "giraffe") {
		t.Fatalf("office_read d'un PDF: %q %v", txt, err)
	}
}

func TestPdfReadScannedPageSaysSo(t *testing.T) {
	p := pdfFixture(t, "scan.pdf")
	out, imgs, err := PdfRead(context.Background(), PdfReadOpts{Path: p})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sense capa de text") || !strings.Contains(out, "Pàgines sense text: 1") {
		t.Fatalf("pàgina escanejada: %q", out)
	}
	// Amb rasteritzador (pdftoppm o Windows), la pàgina ve com a imatge.
	if PdfRenderAvailable() != "" && len(imgs) != 1 {
		t.Fatalf("amb %s no ha tornat la imatge: %q", PdfRenderAvailable(), out)
	}
	if PdfRenderAvailable() == "" && len(imgs) != 0 {
		t.Fatal("sense rasteritzador no hi pot haver imatges")
	}
}

func TestPdfReadInfo(t *testing.T) {
	p := pdfFixture(t, "form.pdf")
	out, _, err := PdfRead(context.Background(), PdfReadOpts{Path: p, Detail: "info"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"Pàgines: 1", "A4", "Formulari (3 camps", "nom [text]", "ciutat [combo]", "Girona | Lleida | Tarragona"} {
		if !strings.Contains(out, w) {
			t.Errorf("info sense %q:\n%s", w, out)
		}
	}
}

func TestPdfEditJournalAndRewind(t *testing.T) {
	ctx := context.Background()
	p := pdfFixture(t, "text3.pdf")
	before, _ := os.ReadFile(p)
	Active = NewJournal()
	out, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "stamp", Text: "APROVAT", Pages: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "segell") || !strings.Contains(out, "3 pàgines") {
		t.Fatalf("resum: %q", out)
	}
	if txt, _ := PdfText(p); !strings.Contains(txt, "APROVAT") {
		t.Fatalf("el segell no hi és: %q", txt)
	}
	if _, err := Active.Rewind(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if string(after) != string(before) {
		t.Fatal("/rewind no ha restaurat el PDF")
	}
}

func TestPdfEditOps(t *testing.T) {
	ctx := context.Background()
	Active = NewJournal()
	p := pdfFixture(t, "text3.pdf")
	dir := filepath.Dir(p)

	// extract sense out: <nom>_p<pàgines>.pdf al costat.
	out, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "extract", Pages: "2-3"})
	if err != nil {
		t.Fatal(err)
	}
	ext := filepath.Join(dir, "text3_p2a3.pdf")
	if !strings.Contains(out, ext) {
		t.Fatalf("extract: %q", out)
	}
	// Un out que ja existeix (i no és el mateix fitxer) demana overwrite.
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "extract", Pages: "1", Out: ext}); err == nil || !strings.Contains(err.Error(), "overwrite") {
		t.Fatalf("hauria de demanar overwrite: %v", err)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "extract", Pages: "1", Out: ext, Overwrite: true}); err != nil {
		t.Fatal(err)
	}

	merged := filepath.Join(dir, "tot.pdf")
	if _, err := PdfEdit(ctx, PdfEditOpts{Op: "merge", Paths: []string{p, ext}, Out: merged}); err != nil {
		t.Fatal(err)
	}
	if txt, _ := PdfText(merged); !strings.Contains(txt, "PDF de 4 pàgines") {
		t.Fatalf("merge: %q", txt)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Op: "merge", Paths: []string{p}}); err == nil {
		t.Fatal("merge sense out hauria de fallar")
	}

	out, err = PdfEdit(ctx, PdfEditOpts{Path: merged, Op: "split", At: "3", Out: filepath.Join(dir, "parts")})
	if err != nil || !strings.Contains(out, "dividit en 2 fitxers") {
		t.Fatalf("split: %q %v", out, err)
	}

	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "reorder", Pages: "3,2,1"}); err != nil {
		t.Fatal(err)
	}
	if txt, _ := PdfText(p); strings.Index(txt, "pelican") > strings.Index(txt, "giraffe") {
		t.Fatalf("reorder: %q", txt)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "delete_pages", Pages: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "rotate", Degrees: 180}); err != nil {
		t.Fatal(err)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "watermark", Text: "ESBORRANY"}); err != nil {
		t.Fatal(err)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "add_text", Text: "Pàg. revisada", Position: "br", X: -40, Y: 30, Size: 9}); err != nil {
		t.Fatal(err)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "annotate", Kind: "note", Pages: "1", X: 100, Y: 100, Text: "Mira-ho"}); err != nil {
		t.Fatal(err)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "set_info", Fields: map[string]string{"title": "Prova"}}); err != nil {
		t.Fatal(err)
	}
	info, _ := PdfInfoText(ctx, p)
	if !strings.Contains(info, "Títol: Prova") || !strings.Contains(info, "Mira-ho") || !strings.Contains(info, "Pàgines: 2") {
		t.Fatalf("estat final: %s", info)
	}
	if _, err := PdfEdit(ctx, PdfEditOpts{Path: p, Op: "volar"}); err == nil {
		t.Fatal("op desconeguda hauria de fallar")
	}
}

func TestPdfFillForm(t *testing.T) {
	Active = NewJournal()
	p := pdfFixture(t, "form.pdf")
	out, err := PdfEdit(context.Background(), PdfEditOpts{Path: p, Op: "fill_form", Fields: map[string]string{"nom": "Jordi", "accepto": "true", "no_hi_es": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "omplerts 2 camps") || !strings.Contains(out, "no existeixen: no_hi_es") {
		t.Fatalf("fill_form: %q", out)
	}
	// Sense op però amb fields: és fill_form (no hi ha cap altra op amb fields).
	if out, err := PdfEdit(context.Background(), PdfEditOpts{Path: p, Fields: map[string]string{"ciutat": "Girona"}}); err != nil || !strings.Contains(out, "omplerts 1 camps") {
		t.Fatalf("fill_form implícit: %q %v", out, err)
	}
	info, _ := PdfInfoText(context.Background(), p)
	if !strings.Contains(info, `nom [text] pàg. 1 = "Jordi"`) {
		t.Fatalf("valors: %s", info)
	}
}

func TestPdfEditTargets(t *testing.T) {
	if got := PdfEditTargets(PdfEditOpts{Path: "a.pdf", Op: "rotate"}); got[0] != "a.pdf" {
		t.Fatal(got)
	}
	if got := PdfEditTargets(PdfEditOpts{Op: "merge", Out: "o.pdf"}); got[0] != "o.pdf" {
		t.Fatal(got)
	}
	if got := PdfEditTargets(PdfEditOpts{Path: filepath.Join("d", "a.pdf"), Op: "split"}); got[0] != "d" {
		t.Fatal(got)
	}
}
