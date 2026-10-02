package tools

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZip(t *testing.T, path string, parts map[string]string) {
	t.Helper()
	w, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for name, body := range parts {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	w.Close()
}

const ctXML = `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`
const relsXML = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`

func testDocx(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "prova.docx")
	writeZip(t, p, map[string]string{
		"[Content_Types].xml": ctXML,
		"_rels/.rels":         relsXML,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
			`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Informe setmanal</w:t></w:r></w:p>` +
			`<w:p><w:r><w:t>Hola </w:t></w:r><w:r><w:t>mon aquest dilluns</w:t></w:r></w:p>` +
			`<w:p><w:r><w:t>Adéu</w:t></w:r></w:p>` +
			`</w:body></w:document>`,
	})
	return p
}

func testXlsx(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "prova.xlsx")
	writeZip(t, p, map[string]string{
		"[Content_Types].xml":        ctXML,
		"_rels/.rels":                relsXML,
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Dades" sheetId="1" r:id="rId1"/><sheet name="Buit" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>Nom</t></si><si><t>Anna</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
			`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>` +
			`<row r="2"><c r="A2"><v>42</v></c><c r="B2"><f>SUM(A2:A2)</f><v>42</v></c></row>` +
			`</sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData></sheetData></worksheet>`,
	})
	return p
}

func testPptx(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "prova.pptx")
	writeZip(t, p, map[string]string{
		"[Content_Types].xml":             ctXML,
		"_rels/.rels":                     relsXML,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId r:id="rId2"/><p:sldId r:id="rId3"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId2" Target="slides/slide1.xml"/><Relationship Id="rId3" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Títol de la xerrada</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/slide2.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Gràcies</a:t></a:r><a:r><a:t> per venir</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	return p
}

func TestOfficeReadDocx(t *testing.T) {
	out, err := OfficeRead(testDocx(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Informe setmanal") || !strings.Contains(out, "Hola mon aquest dilluns") {
		t.Fatalf("lectura docx:\n%s", out)
	}
}

func TestOfficeReplaceDocxSplit(t *testing.T) {
	p := testDocx(t)
	// "Hola mon" està partit en dos runs: el replace l'ha de trobar igual.
	msg, err := OfficeReplace(p, "Hola mon", "Bon dia")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "1 cop") {
		t.Fatalf("recompte: %s", msg)
	}
	out, err := OfficeRead(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Bon dia aquest dilluns") || strings.Contains(out, "Hola") {
		t.Fatalf("post-replace:\n%s", out)
	}
	// El document ha de seguir sent un zip llegible.
	if _, err := OfficeRead(p); err != nil {
		t.Fatalf("zip trencat: %v", err)
	}
}

func TestOfficeReplaceDocxAbsent(t *testing.T) {
	if _, err := OfficeReplace(testDocx(t), "noexisteix", "x"); err == nil {
		t.Fatal("hauria de fallar si no hi és")
	}
}

func TestOfficeReadXlsx(t *testing.T) {
	out, err := OfficeRead(testXlsx(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[FULL Dades]") || !strings.Contains(out, "Nom | Anna") || !strings.Contains(out, "=SUM(A2:A2)") {
		t.Fatalf("lectura xlsx:\n%s", out)
	}
}

func TestOfficeXlsxSet(t *testing.T) {
	p := testXlsx(t)
	msg, err := OfficeXlsxSet(p, "Dades", "C5", "hola")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Dades!C5") {
		t.Fatalf("resum: %s", msg)
	}
	out, _ := OfficeRead(p)
	if !strings.Contains(out, "hola") {
		t.Fatalf("cel·la nova no llegida:\n%s", out)
	}
	// Numèric sobre existent.
	if _, err := OfficeXlsxSet(p, "Dades", "A2", "100"); err != nil {
		t.Fatal(err)
	}
	out, _ = OfficeRead(p)
	if !strings.Contains(out, "100") {
		t.Fatalf("cel·la numèrica:\n%s", out)
	}
	// Full inexistent.
	if _, err := OfficeXlsxSet(p, "NoHiEs", "A1", "x"); err == nil {
		t.Fatal("hauria de fallar amb full inexistent")
	}
	// Cel·la invàlida.
	if _, err := OfficeXlsxSet(p, "Dades", "ZZZ", "x"); err == nil {
		t.Fatal("hauria de fallar amb cel·la invàlida")
	}
}

func TestOfficeReadPptx(t *testing.T) {
	out, err := OfficeRead(testPptx(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Diapo 1") || !strings.Contains(out, "Títol de la xerrada") || !strings.Contains(out, "Gràcies per venir") {
		t.Fatalf("lectura pptx:\n%s", out)
	}
}

func TestOfficeReplacePptx(t *testing.T) {
	p := testPptx(t)
	if _, err := OfficeReplace(p, "per venir", "a tothom"); err != nil {
		t.Fatal(err)
	}
	out, _ := OfficeRead(p)
	if !strings.Contains(out, "Gràcies a tothom") {
		t.Fatalf("post-replace pptx:\n%s", out)
	}
}

func TestOfficeRewind(t *testing.T) {
	j := NewJournal()
	old := Active
	Active = j
	defer func() { Active = old }()
	p := testDocx(t)
	if _, err := OfficeReplace(p, "Adéu", "AdeuSiau"); err != nil {
		t.Fatal(err)
	}
	out, _ := OfficeRead(p)
	if !strings.Contains(out, "AdeuSiau") {
		t.Fatalf("no aplicat:\n%s", out)
	}
	if _, err := j.Rewind(); err != nil {
		t.Fatal(err)
	}
	out, _ = OfficeRead(p)
	if !strings.Contains(out, "Adéu") || strings.Contains(out, "AdeuSiau") {
		t.Fatalf("rewind no restaura:\n%s", out)
	}
}

func TestOfficeLegacy(t *testing.T) {
	if OfficeKind("f.doc") != "legacy" || OfficeKind("f.xls") != "legacy" || OfficeKind("f.ppt") != "legacy" {
		t.Fatal("legacy no detectat")
	}
	if _, err := OfficeRead("f.doc"); err == nil || !strings.Contains(err.Error(), "LibreOffice") {
		t.Fatalf("legacy ha de guiar a LibreOffice: %v", err)
	}
}

func TestOfficeWritable(t *testing.T) {
	p := testDocx(t)
	if err := OfficeWritable(p); err != nil {
		t.Fatalf("fitxer normal ha de ser editable: %v", err)
	}
	if err := OfficeWritable(filepath.Join(t.TempDir(), "no-existeix.docx")); err == nil {
		t.Fatal("inexistent ha de fallar")
	}
}

func TestOfficeOpenValida(t *testing.T) {
	if _, err := OfficeOpen(""); err == nil {
		t.Fatal("ruta buida ha de fallar")
	}
	if _, err := OfficeOpen(filepath.Join(t.TempDir(), "res.docx")); err == nil {
		t.Fatal("inexistent ha de fallar")
	}
	txt := filepath.Join(t.TempDir(), "nota.txt")
	if err := os.WriteFile(txt, []byte("hola"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OfficeOpen(txt); err == nil {
		t.Fatal(".txt no és office i ha de fallar (sense obrir res)")
	}
	// Amb document vàlid no cridem: obriria el Word de debò.
}
