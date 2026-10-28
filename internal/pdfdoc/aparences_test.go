package pdfdoc

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// tjDe torna el text de les cadenes «(…) Tj» de les aparences (bytes
// WinAnsi passats a UTF-8) i les fonts que fan servir.
func tjDe(t *testing.T, path string) (string, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := api.ReadContext(bg, bytes.NewReader(data), conf())
	if err != nil {
		t.Fatal(err)
	}
	var text, fonts strings.Builder
	for nr, e := range ctx.XRefTable.Table {
		if e == nil || e.Free {
			continue
		}
		o, err := ctx.XRefTable.Dereference(*types.NewIndirectRef(nr, 0))
		if err != nil {
			continue
		}
		sd, ok := o.(types.StreamDict)
		if !ok || sd.Decode() != nil || !bytes.Contains(sd.Content, []byte(" Tj")) {
			continue
		}
		for k := 0; k < len(sd.Content); k++ {
			if sd.Content[k] != '(' {
				continue
			}
			raw, fi, ok := literal(sd.Content, k)
			if !ok {
				t.Fatalf("cadena trencada: %q", sd.Content)
			}
			b, _ := types.Unescape(string(raw))
			for _, c := range b {
				text.WriteRune(rune(c))
			}
			text.WriteByte('|')
			k = fi - 1
		}
		if m := reTf.FindSubmatch(sd.Content); m != nil {
			fonts.Write(m[1])
			fonts.WriteByte(' ')
		}
	}
	return text.String(), fonts.String()
}

// pdfcpu v0.16.3 draws accented letters of multiline fields with a
// standard font as spaces ("Energ tica"); FillForm repairs the appearance.
func TestFillFormAppearanceKeepsAccents(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	a := copia(t, "form-helv.pdf")
	v := "Cooperativa Energètica del Garraf; instal·lació a escoles públiques"
	if _, _, err := FillForm(bg, a, a, map[string]string{"entitat": v, "calendari": "Inici: febrer 2027"}); err != nil {
		t.Fatal(err)
	}
	text, fonts := tjDe(t, a)
	if !strings.Contains(text, v) || strings.Contains(text, "Energ tica") {
		t.Fatalf("aparença: %q", text)
	}
	if !strings.Contains(fonts, fontAparenca) || !strings.Contains(text, "Inici: febrer 2027") {
		t.Fatalf("fonts %q, text %q", fonts, text)
	}
}

// A narrow field: pdfcpu wraps the value over several lines, and an accent
// can fall at the start of a line.
func TestFillFormAppearanceWrappedAccents(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	a := copia(t, "form-narrow.pdf")
	v := "La comunitat energètica és àmplia i útil: repartirà l'excedent entre cent vuitanta llars veïnes"
	if _, _, err := FillForm(bg, a, a, map[string]string{"nota": v}); err != nil {
		t.Fatal(err)
	}
	text, _ := tjDe(t, a)
	if strings.Count(text, "|") < 3 {
		t.Fatalf("no s'ha partit en línies: %q", text)
	}
	joined := strings.ReplaceAll(text, "|", " ")
	for _, w := range []string{"energètica", "és", "àmplia", "útil:", "repartirà", "veïnes"} {
		if !strings.Contains(joined, w) {
			t.Fatalf("falta %q a l'aparença: %q", w, text)
		}
	}
}
