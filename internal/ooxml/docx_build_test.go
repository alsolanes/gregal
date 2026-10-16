package ooxml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleMD = `## Resum executiu

Aquest informe analitza **tres opcions** per migrar la plataforma, amb *riscos* i ` + "`costos`" + `. Més detalls a [la documentació](https://example.com).

- Primer punt important
- Segon punt amb **negreta**
  - Subpunt
- Tercer punt

1. Pas u
2. Pas dos
3. Pas tres

| Opció | Cost (€) | Termini |
|---|---|---|
| A | 12.000 | 3 mesos |
| B | 8.500 | 5 mesos |
| C | 20.000 | 2 mesos |

> Una cita destacada per remarcar la conclusió.

` + "```go\nfunc main() {\n\tfmt.Println(\"hola\")\n}\n```" + `

---

### Conclusió

Recomanem l'opció **B**.
`

func TestNewDocxRichIsValid(t *testing.T) {
	d := NewDocx(DocxOptions{Title: "Informe de migració", Subtitle: "Octubre 2026", PageNumbers: true, Theme: GetTheme("default")})
	d.AppendMarkdown("[TOC]\n\n" + sampleMD)
	data, err := d.Pkg.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pk, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if iss := Validate(pk); len(iss) > 0 {
		t.Fatalf("problemes: %v", iss)
	}
	doc, _ := pk.XML("word/document.xml")
	s := doc.String()
	for _, want := range []string{`w:pStyle w:val="Heading2"`, `w:numId`, `<w:tbl>`, `w:pStyle w:val="Quote"`, `w:pStyle w:val="Code"`, `w:hyperlink`, `TOC \o`, `w:pStyle w:val="Title"`} {
		if !strings.Contains(s, want) {
			t.Errorf("falta %s", want)
		}
	}
	if strings.Contains(s, "**") || strings.Contains(s, "• ") {
		t.Error("markdown sense convertir")
	}
	if out := os.Getenv("GREGAL_OOXML_OUT"); out != "" {
		os.WriteFile(filepath.Join(out, "rich.docx"), data, 0o644)
	}
}

func TestDocxFromTemplateMapsStyles(t *testing.T) {
	// La plantilla de LibreOffice té estils amb ids propis: el
	// constructor ha de trobar-los pel nom canònic o afegir-ne.
	d, err := DocxFromTemplate("testdata/lo.docx")
	if err != nil {
		t.Fatal(err)
	}
	d.ReplaceBody("# Títol\n\nText\n\n- a\n- b\n")
	data, _ := d.Pkg.Bytes()
	pk, _ := OpenBytes(data)
	if iss := Validate(pk); len(iss) > 0 {
		t.Fatalf("problemes: %v", iss)
	}
	doc, _ := pk.XML("word/document.xml")
	if !strings.Contains(doc.String(), "w:numId") {
		t.Error("llista sense numeració")
	}
	if strings.Contains(doc.InnerText(), "CLAU") {
		t.Error("el cos de la plantilla no s'ha substituït")
	}
}
