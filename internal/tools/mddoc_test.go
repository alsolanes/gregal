package tools

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRichLine(t *testing.T) {
	for _, c := range []struct {
		in     string
		text   string
		size   string
		bullet bool
		skip   bool
		bold   bool
		italic bool
	}{
		{"# Títol", "Títol", "30", false, false, false, false},
		{"## Sub", "Sub", "28", false, false, false, false},
		{"### Petit", "Petit", "26", false, false, false, false},
		{"- element", "element", "", true, false, false, false},
		{"* element", "element", "", true, false, false, false},
		{"Hola **món**!", "Hola món!", "", false, false, true, false},
		{"| a | b |", "a · b", "", false, false, false, false},
		{"|---|---|", "", "", false, true, false, false},
		{"---", "", "", false, false, false, false},
		{"> cita", "cita", "", false, false, false, false},
		{"1. primer", "primer", "", false, false, false, false},
		{"text [enllaç](http://x)", "text enllaç", "", false, false, false, false},
		{"a ** b", "a ** b", "", false, false, false, false},
		{"sense res", "sense res", "", false, false, false, false},
	} {
		rl := parseRichLine(c.in)
		if rl.skip != c.skip || rl.size != c.size || rl.bullet != c.bullet {
			t.Errorf("%q → size=%q bullet=%v skip=%v", c.in, rl.size, rl.bullet, rl.skip)
		}
		var b strings.Builder
		nb, ni := 0, 0
		for _, r := range rl.runs {
			b.WriteString(r.text)
			if r.bold {
				nb++
			}
			if r.italic {
				ni++
			}
		}
		if b.String() != c.text {
			t.Errorf("%q → text=%q, volia %q", c.in, b.String(), c.text)
		}
		if (nb > 0) != c.bold || (ni > 0) != c.italic {
			t.Errorf("%q → bold=%d italic=%d", c.in, nb, ni)
		}
	}
	// Cursiva sola i codi: el codi no interpreta el ** de dins.
	rl := parseRichLine("fes *això* i `a**b`")
	var b strings.Builder
	for _, r := range rl.runs {
		b.WriteString(r.text)
	}
	if b.String() != "fes això i a**b" {
		t.Fatalf("runs=%q", b.String())
	}
}

// Crear amb markdown i rellegir: cap marca ha de sobreviure al document.
func TestCrearDocxNetejaMarkdown(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.docx")
	md := "# Informe\n\nText amb **negreta** i *cursiva*.\n\n- primer\n- segon\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```\n# no és títol\n```\n\nFinal `codi` i [enllaç](http://x)."
	if _, err := OfficeCreate(p, "Títol **fort**", md); err != nil {
		t.Fatal(err)
	}
	raw, err := OfficeRead(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, brut := range []string{"**", "*", "```", "|", "](", "---", "##"} {
		if strings.Contains(raw, brut) {
			t.Errorf("marca %q al document llegit:\n%s", brut, raw)
		}
	}
	for _, net := range []string{"Informe", "negreta", "primer", "•", "a · b", "Final", "codi", "enllaç", "Títol fort"} {
		if !strings.Contains(raw, net) {
			t.Errorf("falta %q al document:\n%s", net, raw)
		}
	}
	// El codi entre tanques queda literal.
	if !strings.Contains(raw, "# no és títol") {
		t.Errorf("el codi ha de quedar literal:\n%s", raw)
	}
	// I la negreta és format de debò (<w:b/>), no asteriscs.
	z, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	trobat := false
	for _, f := range z.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf strings.Builder
		_, _ = io.Copy(&buf, rc)
		rc.Close()
		if strings.Contains(buf.String(), "<w:b/>") && strings.Contains(buf.String(), ">negreta<") {
			trobat = true
		}
	}
	if !trobat {
		t.Error("cal un run amb <w:b/> per a la negreta")
	}
}

// El replace amb markdown troba text pla i desa text pla.
func TestReplaceNetajaMarkdown(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.docx")
	if _, err := OfficeCreate(p, "T", "Hola món."); err != nil {
		t.Fatal(err)
	}
	if _, err := OfficeReplace(p, "món", "món **gran**"); err != nil {
		t.Fatal(err)
	}
	raw, err := OfficeRead(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "**") || !strings.Contains(raw, "món gran") {
		t.Errorf("replace mal netejat: %q", raw)
	}
	// El find amb marques també troba (el doc té text pla).
	if _, err := OfficeReplace(p, "**món** gran", "món"); err != nil {
		t.Fatalf("find amb markdown hauria de trobar: %v", err)
	}
}

// set_cell amb markdown desa text pla.
func TestSetCellNetejaMarkdown(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "full.xlsx")
	if _, err := OfficeCreate(p, "T", "a | b"); err != nil {
		t.Fatal(err)
	}
	if _, err := OfficeXlsxSet(p, "Full1", "A2", "**fet**"); err != nil {
		t.Fatal(err)
	}
	raw, err := OfficeRead(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "**") || !strings.Contains(raw, "fet") {
		t.Errorf("cel·la mal netejada: %q", raw)
	}
}
