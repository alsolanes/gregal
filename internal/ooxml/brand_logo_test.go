package ooxml

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func logoPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func zipParts(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func TestBrandLogoInDocxHeaderAndPptxMaster(t *testing.T) {
	th := GetTheme("default")
	th.Name = "brand-acme"
	th.Logo, th.LogoExt = logoPNG(t, 200, 50), "png"
	dir := t.TempDir()

	d := NewDocx(DocxOptions{Theme: th, Title: "Informe"})
	d.AppendMarkdown("# Hola\n\nText.")
	dp := filepath.Join(dir, "a.docx")
	if err := d.Save(dp); err != nil {
		t.Fatal(err)
	}
	parts := zipParts(t, dp)
	if _, ok := parts["word/media/brand-logo.png"]; !ok {
		t.Fatal("docx lacks the logo image")
	}
	hdr := parts["word/header1.xml"]
	if !strings.Contains(hdr, `<a:blip r:embed="`) || !strings.Contains(hdr, "Informe") {
		t.Fatalf("header has no logo or title:\n%s", hdr)
	}
	// 200x50 a 0,9 cm d'alçada: 4 vegades més ample que alt.
	if !strings.Contains(hdr, `cx="1296000" cy="324000"`) {
		t.Fatalf("logo not scaled to its aspect ratio:\n%s", hdr)
	}
	if !strings.Contains(parts["word/_rels/header1.xml.rels"], "media/brand-logo.png") {
		t.Fatal("header rels do not point to the logo")
	}

	p := NewPptx(th, "Deck")
	pp := filepath.Join(dir, "a.pptx")
	if err := p.Pkg.Save(pp); err != nil {
		t.Fatal(err)
	}
	parts = zipParts(t, pp)
	if _, ok := parts["ppt/media/brand-logo.png"]; !ok {
		t.Fatal("pptx lacks the logo image")
	}
	if !strings.Contains(parts["ppt/slideMasters/slideMaster1.xml"], `name="Logo"`) {
		t.Fatal("slide master has no logo picture")
	}

	// Sense logo, res de tot això.
	plain := NewDocx(DocxOptions{Theme: GetTheme("default"), Title: "X"})
	pp2 := filepath.Join(dir, "b.docx")
	if err := plain.Save(pp2); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(zipParts(t, pp2)["word/header1.xml"], "blip") {
		t.Fatal("a theme without logo got one")
	}
}
