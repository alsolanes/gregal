package ooxml

import (
	"bytes"
	"strings"
	"testing"
)

var fixtures = []string{"testdata/lo.docx", "testdata/lo.pptx", "testdata/lo.xlsx"}

// Llegir i tornar a escriure tots els parts XML no ha de canviar res
// que importi: mateixos prefixos, mateix text, paquet vàlid.
func TestRoundTripPreservesPrefixes(t *testing.T) {
	for _, f := range fixtures {
		pk, err := Open(f)
		if err != nil {
			t.Fatal(f, err)
		}
		for _, n := range pk.Names() {
			if !isXMLPart(n) {
				continue
			}
			doc, err := pk.XML(n)
			if err != nil {
				t.Fatalf("%s %s: %v", f, n, err)
			}
			out := doc.Bytes()
			if bytes.Contains(out, []byte("_xmlns")) || bytes.Contains(out, []byte(`xmlns="w"`)) {
				t.Fatalf("%s %s: prefixos trencats", f, n)
			}
			again, err := Parse(out)
			if err != nil {
				t.Fatalf("%s %s: no es torna a llegir: %v", f, n, err)
			}
			if again.InnerText() != doc.InnerText() {
				t.Fatalf("%s %s: el text ha canviat", f, n)
			}
		}
		data, err := pk.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		pk2, err := OpenBytes(data)
		if err != nil {
			t.Fatal(err)
		}
		if iss := Validate(pk2); len(iss) > 0 {
			t.Fatalf("%s: problemes després del round-trip: %v", f, iss)
		}
	}
}

func TestValidateCatchesBrokenRel(t *testing.T) {
	pk, err := Open("testdata/lo.docx")
	if err != nil {
		t.Fatal(err)
	}
	pk.AddRel("word/document.xml", RelImage, "word/media/no-hi-es.png")
	iss := Validate(pk)
	if len(iss) == 0 || !strings.Contains(strings.Join(iss, "\n"), "no-hi-es.png") {
		t.Fatalf("s'esperava un problema de relació: %v", iss)
	}
	orig, _ := Open("testdata/lo.docx")
	if n := NewIssues(pk, orig); len(n) != 1 {
		t.Fatalf("NewIssues: %v", n)
	}
}

func TestRelPath(t *testing.T) {
	cases := [][3]string{
		{"word/document.xml", "word/media/image1.png", "media/image1.png"},
		{"ppt/slides/slide1.xml", "ppt/slideLayouts/slideLayout2.xml", "../slideLayouts/slideLayout2.xml"},
		{"x", "word/document.xml", "word/document.xml"},
	}
	for _, c := range cases {
		if got := RelPath(c[0], c[1]); got != c[2] {
			t.Errorf("RelPath(%s,%s)=%s, volia %s", c[0], c[1], got, c[2])
		}
		if back := RelTarget(c[0], c[2]); c[0] != "x" && back != c[1] {
			t.Errorf("RelTarget invers: %s", back)
		}
	}
}

func TestTemplateToDocument(t *testing.T) {
	pk, _ := Open("testdata/lo.docx")
	pk.SetOverride("word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml")
	if !pk.IsTemplate() {
		t.Fatal("hauria de ser plantilla")
	}
	pk.ToDocument()
	if pk.IsTemplate() || !strings.Contains(pk.ContentType("word/document.xml"), "document.main") {
		t.Fatal("no s'ha convertit")
	}
}
