package ooxml

import (
	"os"
	"path/filepath"
	"reflect"
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

// Els blocs tal com els escriu l'editor del web (doc-view.test.cjs, «styling
// with the toolbar writes the blocks that the Word export test reads»).
func TestDocxExportaElsEstilsDeLEditor(t *testing.T) {
	d := NewDocx(DocxOptions{Theme: GetTheme("default")})
	d.AppendMarkdown("# [Títol]{font=\"Georgia\"}\n{align=center}\n\nText en [vermell]{color=#c0392b bg=#fff3a3} i [gran]{font=\"Times New Roman\" size=14}.\n{align=justify}")
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
	jc := map[string]string{}
	runs := map[string]string{}
	for _, p := range Paragraphs(doc) {
		if j := p.Path("w:pPr", "w:jc"); j != nil {
			jc[ParaText(p)] = j.Get("w:val")
		}
		for _, r := range p.Elems("w:r") {
			if tx, rpr := r.Child("w:t"), r.Child("w:rPr"); tx != nil && rpr != nil {
				runs[tx.InnerText()] = rpr.String()
			}
		}
	}
	if want := map[string]string{"Títol": "center", "Text en vermell i gran.": "both"}; !reflect.DeepEqual(jc, want) {
		t.Fatalf("alineacions: %v", jc)
	}
	for text, wants := range map[string][]string{
		"Títol":   {`w:ascii="Georgia"`},
		"vermell": {`<w:color w:val="C0392B"/>`, `w:fill="FFF3A3"`},
		"gran":    {`w:ascii="Times New Roman"`, `<w:sz w:val="28"/>`},
	} {
		for _, want := range wants {
			if !strings.Contains(runs[text], want) {
				t.Errorf("run %q sense %s: %s", text, want, runs[text])
			}
		}
	}
}

func TestDocxPortaElFormatAmpliat(t *testing.T) {
	d := NewDocx(DocxOptions{Theme: GetTheme("default")})
	d.AppendMarkdown("## Head\n{align=center}\n\nA [red]{color=#c0392b bg=#fff3a3 font=\"Georgia\" size=14} and ++under++.\n{align=justify}\n\nRight\n{align=right}")
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
	jc := map[string]string{}
	runs := map[string]string{}
	for _, p := range Paragraphs(doc) {
		if j := p.Path("w:pPr", "w:jc"); j != nil {
			jc[ParaText(p)] = j.Get("w:val")
		}
		for _, r := range p.Elems("w:r") {
			tx, rpr := r.Child("w:t"), r.Child("w:rPr")
			if tx != nil && rpr != nil {
				runs[tx.InnerText()] = rpr.String()
			}
		}
	}
	if want := map[string]string{"Head": "center", "A red and under.": "both", "Right": "right"}; !reflect.DeepEqual(jc, want) {
		t.Fatalf("alineacions: %v", jc)
	}
	for _, want := range []string{`<w:rFonts w:ascii="Georgia" w:hAnsi="Georgia" w:eastAsia="Georgia" w:cs="Georgia"/>`, `<w:color w:val="C0392B"/>`, `<w:sz w:val="28"/>`, `<w:szCs w:val="28"/>`, `<w:shd w:val="clear" w:color="auto" w:fill="FFF3A3"/>`} {
		if !strings.Contains(runs["red"], want) {
			t.Errorf("run red sense %s: %s", want, runs["red"])
		}
	}
	if !strings.Contains(runs["under"], `<w:u w:val="single"/>`) || strings.Contains(runs["A "], "w:u") {
		t.Errorf("subratllat: %q / %q", runs["under"], runs["A "])
	}
}
