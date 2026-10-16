package ooxml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paraXML construeix un document mínim amb un paràgraf de runs donades.
func docWithRuns(t *testing.T, runs string) *Docx {
	t.Helper()
	d := NewDocx(DocxOptions{NoHeader: true})
	frag, err := ParseFragment(`<w:p xmlns:w="` + nsW + `">` + runs + `</w:p>`)
	if err != nil {
		t.Fatal(err)
	}
	frag[0].Del("xmlns:w")
	d.AppendBlocksRaw(frag[0])
	return d
}

func reopen(t *testing.T, d *Docx) *Docx {
	t.Helper()
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
	d2, err := OpenDocx(pk)
	if err != nil {
		t.Fatal(err)
	}
	return d2
}

func TestReplaceKeepsOtherRunsFormatting(t *testing.T) {
	d := docWithRuns(t, `<w:r><w:t xml:space="preserve">Hola </w:t></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>món</w:t></w:r><w:r><w:t xml:space="preserve"> i adeu CLAU.</w:t></w:r>`)
	n, err := d.Replace("CLAU", "NOU", ReplaceOpts{})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	d = reopen(t, d)
	s := d.Doc.String()
	if !strings.Contains(s, `<w:b/></w:rPr><w:t>món</w:t>`) {
		t.Fatalf("s'ha perdut la negreta d'una altra run: %s", s)
	}
	if !strings.Contains(d.Doc.InnerText(), "i adeu NOU.") {
		t.Fatal("no s'ha substituït")
	}
}

func TestReplaceAcrossSplitRuns(t *testing.T) {
	// El Word sovint parteix «{{client}}» en tres runs iguals.
	d := docWithRuns(t, `<w:r><w:t>Benvolgut {{cli</w:t></w:r><w:r><w:rPr><w:lang w:val="ca-ES"/></w:rPr><w:t>ent</w:t></w:r><w:r><w:t>}},</w:t></w:r>`)
	n, left := d.FillVars(map[string]string{"client": "Sr. Puig"})
	if n != 1 || len(left) != 0 {
		t.Fatalf("n=%d left=%v", n, left)
	}
	if got := d.Doc.InnerText(); !strings.Contains(got, "Benvolgut Sr. Puig,") {
		t.Fatalf("text: %q", got)
	}
}

func TestTrackedReplaceAndDelete(t *testing.T) {
	d := docWithRuns(t, `<w:r><w:t xml:space="preserve">El termini és de tres mesos.</w:t></w:r>`)
	d.AppendMarkdown("Paràgraf sobrer que cal treure.")
	if _, err := d.Replace("tres", "quatre", ReplaceOpts{Track: true, Author: "Revisor"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.DeleteParagraphs("sobrer", false, true, "Revisor"); err != nil {
		t.Fatal(err)
	}
	d = reopen(t, d)
	s := d.Doc.String()
	for _, want := range []string{`<w:del w:id=`, `<w:delText xml:space="preserve">tres</w:delText>`, `<w:ins w:id=`, `w:author="Revisor"`} {
		if !strings.Contains(s, want) {
			t.Errorf("falta %s", want)
		}
	}
	txt, _ := ReadDocx(d.Pkg, 0)
	if !strings.Contains(txt, "esborrat: «tres»") {
		t.Errorf("la lectura no mostra l'esborrat: %s", txt)
	}
	// Acceptar-ho tot deixa el text final net.
	d.AcceptAll()
	d = reopen(t, d)
	if got := d.Doc.InnerText(); !strings.Contains(got, "quatre mesos") || strings.Contains(got, "tres") || strings.Contains(got, "sobrer") {
		t.Fatalf("després d'acceptar: %q", got)
	}
}

func TestTrackedReplaceMarksOnlyTheChange(t *testing.T) {
	d := docWithRuns(t, `<w:r><w:t xml:space="preserve">Es pagaran en un termini de 30 dies des de la recepció.</w:t></w:r>`)
	n, err := d.Replace("Es pagaran en un termini de 30 dies des de la recepció.", "Es pagaran en un termini de 45 dies naturals des de la recepció.", ReplaceOpts{Track: true})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	d = reopen(t, d)
	var del, ins []string
	for _, x := range d.Doc.Find("w:delText") {
		del = append(del, x.InnerText())
	}
	for _, x := range d.Doc.Find("w:ins") {
		ins = append(ins, x.InnerText())
	}
	// Tall per paraules: el canvi mínim és «30 dies» → «45 dies naturals».
	if strings.Join(del, "|") != "30 dies" || strings.Join(ins, "|") != "45 dies naturals" {
		t.Fatalf("canvi no mínim: del=%q ins=%q", del, ins)
	}
	d.AcceptAll()
	if got := d.Doc.InnerText(); !strings.Contains(got, "termini de 45 dies naturals des de la recepció.") {
		t.Fatalf("després d'acceptar: %q", got)
	}
}

func TestInsertAndComment(t *testing.T) {
	d := NewDocx(DocxOptions{NoHeader: true})
	d.AppendMarkdown("## Introducció\n\nPrimer paràgraf.\n\n## Final\n\nÚltim.")
	if _, _, err := d.Insert("Primer paràgraf", "after", "- nou punt\n- un altre", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AddComment("Últim", "Cal ampliar aquesta part.", "Aleix"); err != nil {
		t.Fatal(err)
	}
	d = reopen(t, d)
	txt, _ := ReadDocx(d.Pkg, 0)
	if !strings.Contains(txt, "- nou punt") || !strings.Contains(txt, "[comentari #") || !strings.Contains(txt, "Cal ampliar") {
		t.Fatalf("lectura:\n%s", txt)
	}
	i1, i2 := strings.Index(txt, "Primer paràgraf"), strings.Index(txt, "nou punt")
	if i1 < 0 || i2 < i1 {
		t.Fatal("ordre incorrecte")
	}
	if out := os.Getenv("GREGAL_OOXML_OUT"); out != "" {
		data, _ := d.Pkg.Bytes()
		os.WriteFile(filepath.Join(out, "edited.docx"), data, 0o644)
	}
}

func TestEditRealLibreOfficeDocx(t *testing.T) {
	pk, err := Open("testdata/lo.docx")
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenDocx(pk)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := d.Replace("CLAU", "NOU", ReplaceOpts{}); n != 1 {
		t.Fatalf("n=%d", n)
	}
	if _, _, err := d.Insert("Paràgraf final", "before", "Text **afegit**.", false, ""); err != nil {
		t.Fatal(err)
	}
	orig, _ := Open("testdata/lo.docx")
	data, _ := d.Pkg.Bytes()
	pk2, _ := OpenBytes(data)
	if iss := NewIssues(pk2, orig); len(iss) > 0 {
		t.Fatalf("problemes nous: %v", iss)
	}
	s, _ := pk2.Raw("word/document.xml")
	if strings.Contains(string(s), "_xmlns") {
		t.Fatal("prefixos trencats")
	}
	txt, _ := ReadDocx(pk2, 0)
	if !strings.Contains(txt, "NOU") || !strings.Contains(txt, "| Nom | Valor |") {
		t.Fatalf("lectura:\n%s", txt)
	}
}
