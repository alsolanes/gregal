package web

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// docxMinim construeix un .docx real (zip amb [Content_Types], _rels i
// word/document.xml) amb el text donat, sense cap fixture al repo.
func docxMinim(t *testing.T, dir, text string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	add("word/document.xml", `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>`+text+`</w:t></w:r></w:p></w:body></w:document>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "informe.docx")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// Un @document.docx adjunt al xat ha de ser el seu text, no els bytes del
// zip (el TUI ja ho feia; la web l'inlinava en cru). I una ruta absoluta de
// Windows (C:\…) és absoluta: abans s'enganxava darrere del workspace.
func TestExpandMentionsOfficeIRutaAbsoluta(t *testing.T) {
	s := hubTestServer(t)
	dir := t.TempDir()
	docx := docxMinim(t, dir, "Pressupost del pilar tres")
	txt := filepath.Join(dir, "notes.txt")
	os.WriteFile(txt, []byte("línia de notes\n"), 0o600)

	out := s.expandMentions("revisa @"+docx+" i @"+txt+".", s.cwd)
	if !strings.Contains(out, "Pressupost del pilar tres") {
		t.Fatalf("el docx s'ha d'adjuntar com a text:\n%s", out)
	}
	if strings.Contains(out, "PK\x03\x04") || strings.Contains(out, "word/document.xml") {
		t.Fatalf("no s'han d'inlinar els bytes del zip:\n%s", out)
	}
	if !strings.Contains(out, "línia de notes") {
		t.Fatalf("la ruta absoluta (%s) no s'ha llegit: %s", txt, out)
	}
	if runtime.GOOS == "windows" && !strings.Contains(docx, `\`) {
		t.Fatalf("la prova hauria d'haver usat una ruta amb barres invertides: %s", docx)
	}
}

// El punt final d'una frase no forma part de la ruta.
func TestExpandMentionsPuntuacioFinal(t *testing.T) {
	s := hubTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("hola\n"), 0o600)
	if out := s.expandMentions("mira @"+p+".", s.cwd); !strings.Contains(out, "hola") || strings.Contains(out, "a.txt.:") {
		t.Fatalf("%s", out)
	}
}
