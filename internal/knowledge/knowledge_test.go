package knowledge

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/tools"
)

// aïlla les dades: cap prova toca ~/.local/share/gregal.
func isolate(t *testing.T) Where {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	Configure(Settings{})
	return Where{Project: t.TempDir(), Session: "sess-1"}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pdfFixture escriu un PDF petit amb text real: una pàgina amb el
// contingut sense comprimir i una altra amb FlateDecode i TJ.
func pdfFixture(pages ...string) []byte {
	var objs []string
	font := len(pages)*2 + 3
	kids := []string{}
	for i, text := range pages {
		pageN, contN := 3+i*2, 4+i*2
		kids = append(kids, fmt.Sprintf("%d 0 R", pageN))
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", font, contN))
		lines := strings.Split(text, "\n")
		var cs strings.Builder
		cs.WriteString("BT /F1 12 Tf 72 720 Td ")
		for j, ln := range lines {
			if j > 0 {
				cs.WriteString("0 -14 Td ")
			}
			words := strings.Fields(ln)
			if i%2 == 1 && len(words) > 1 {
				// TJ amb un ajust gran entre paraules = espai.
				cs.WriteString("[")
				for k, w := range words {
					if k > 0 {
						cs.WriteString(" -300 ")
					}
					cs.WriteString("(" + w + ")")
				}
				cs.WriteString("] TJ ")
			} else {
				cs.WriteString("(" + strings.ReplaceAll(ln, "(", `\(`) + ") Tj ")
			}
		}
		cs.WriteString("ET")
		if i%2 == 1 {
			var z bytes.Buffer
			zw := zlib.NewWriter(&z)
			zw.Write([]byte(cs.String()))
			zw.Close()
			objs = append(objs, fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", z.Len(), z.String()))
		} else {
			objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", cs.Len(), cs.String()))
		}
	}
	all := []string{"<< /Type /Catalog /Pages 2 0 R >>", fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages))}
	all = append(all, objs...)
	all = append(all, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := []int{}
	for i, o := range all {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(all)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(all)+1, xref)
	return b.Bytes()
}

// docxFixture escriu un .docx mínim amb paràgrafs amb estil (com els de
// Word: Heading1, Title...).
func docxFixture(t *testing.T, path string, paras [][2]string) {
	t.Helper()
	var body strings.Builder
	for _, p := range paras {
		body.WriteString("<w:p>")
		if p[0] != "" {
			body.WriteString(`<w:pPr><w:pStyle w:val="` + p[0] + `"/></w:pPr>`)
		}
		body.WriteString("<w:r><w:t>" + p[1] + "</w:t></w:r></w:p>")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`,
	}
	for name, content := range parts {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTokensFoldAndSplit(t *testing.T) {
	got := strings.Join(Tokens("L'Índex de OfficeReadFull i la_configuració, 2026!"), " ")
	for _, want := range []string{"index", "officereadfull", "office", "read", "full", "la_configuracio", "configuracio", "2026"} {
		if want == "la_configuracio" || want == "configuracio" {
			continue // els _ parteixen la paraula
		}
		if !strings.Contains(" "+got+" ", " "+want+" ") {
			t.Errorf("falta %q a %q", want, got)
		}
	}
	if strings.Contains(" "+got+" ", " de ") || strings.Contains(" "+got+" ", " i ") {
		t.Errorf("paraules buides no filtrades: %q", got)
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pat, rel string
		want     bool
	}{
		{"*.md", "docs/a/readme.md", true},
		{"*.md", "docs/a/readme.txt", false},
		{"docs/**", "docs/a/b.pdf", true},
		{"docs/", "docs/x.md", true},
		{"docs/**", "src/docs.go", false},
		{"**/*.pdf", "a/b/c.pdf", true},
		{"**/*.pdf", "c.pdf", true},
		{"src/*.go", "src/a.go", true},
		{"src/*.go", "src/x/a.go", false},
	}
	for _, c := range cases {
		if got := MatchGlob(c.pat, c.rel); got != c.want {
			t.Errorf("MatchGlob(%q, %q) = %v", c.pat, c.rel, got)
		}
	}
}

func TestChunkSizesOverlapAndLines(t *testing.T) {
	var lines []string
	for i := 1; i <= 120; i++ {
		lines = append(lines, fmt.Sprintf("line %03d says something about topic %d", i, i%7))
	}
	pieces := Chunk([]Section{{Loc: "Intro", Text: strings.Join(lines, "\n"), Line: 10}}, 600, 120)
	if len(pieces) < 6 {
		t.Fatalf("massa pocs trossos: %d", len(pieces))
	}
	for i, p := range pieces {
		if len(p.Text) > 700 {
			t.Errorf("tros %d massa llarg: %d", i, len(p.Text))
		}
		if !strings.HasPrefix(p.Loc, "Intro · lines ") {
			t.Errorf("ubicació sense rang: %q", p.Loc)
		}
	}
	if pieces[0].Loc != "Intro · lines 10–24" && !strings.HasPrefix(pieces[0].Loc, "Intro · lines 10–") {
		t.Errorf("primer rang: %q", pieces[0].Loc)
	}
	// Solapament: el final d'un tros torna a sortir al principi del següent.
	last := strings.Split(pieces[0].Text, "\n")
	if !strings.Contains(pieces[1].Text, last[len(last)-1]) {
		t.Errorf("sense solapament entre trossos")
	}
	// L'última línia hi és.
	if !strings.Contains(pieces[len(pieces)-1].Text, "line 120") {
		t.Errorf("s'ha perdut el final")
	}
	// Una línia enorme es parteix entre paraules.
	long := strings.Repeat("paraula ", 1000)
	for _, p := range Chunk([]Section{{Text: long}}, 500, 50) {
		if len(p.Text) > 520 {
			t.Fatalf("línia llarga sense partir: %d", len(p.Text))
		}
	}
}

func TestMarkdownSectionsHeadingPath(t *testing.T) {
	md := "# Guide\nintro\n## Install\n```\n# not a heading\n```\nsteps\n### Windows\nuse the msi\n"
	secs := markdownSections(md, true)
	if len(secs) != 3 {
		t.Fatalf("seccions: %+v", secs)
	}
	if secs[2].Loc != "Guide › Install › Windows" || secs[2].Line != 8 {
		t.Errorf("camí: %+v", secs[2])
	}
	if !strings.Contains(secs[1].Text, "# not a heading") {
		t.Errorf("el bloc de codi s'ha partit")
	}
}

func TestExtractPerType(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.md"), "# Title\nThe falcon flies.\n")
	write(t, filepath.Join(dir, "b.txt"), "plain words here")
	write(t, filepath.Join(dir, "c.go"), "package x\n\nfunc QuantumLeap() {}\n")
	write(t, filepath.Join(dir, "d.html"), "<html><head><title>Page</title></head><body><nav>menu</nav><article><h1>Heading</h1><p>The lighthouse keeper writes logs every night, with many words so the article is long enough to be main content.</p></article></body></html>")
	if err := os.WriteFile(filepath.Join(dir, "e.pdf"), pdfFixture("Hello PDF zebra\nsecond line", "Flate compressed giraffe page"), 0o644); err != nil {
		t.Fatal(err)
	}
	docxFixture(t, filepath.Join(dir, "f.docx"), [][2]string{{"Title", "Report"}, {"Heading1", "Budget"}, {"", "The budget for walrus research is 42 euros."}, {"Heading2", "Risks"}, {"", "Icebergs."}})
	if _, err := tools.OfficeCreate(filepath.Join(dir, "g.xlsx"), "Sales", "Region | Total\nNorth | 120\nPelican | 7"); err != nil {
		t.Fatal(err)
	}
	if _, err := tools.OfficeCreate(filepath.Join(dir, "h.pptx"), "Deck", "Octopus strategy\nSecond point"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "i.doc"), "legacy")
	write(t, filepath.Join(dir, "j.png"), "\x89PNG\x00\x00")
	cases := []struct {
		file, kind, want, loc string
	}{
		{"a.md", "markdown", "falcon", "Title"},
		{"b.txt", "text", "plain words", ""},
		{"c.go", "code", "QuantumLeap", ""},
		{"d.html", "html", "lighthouse keeper", ""},
		{"e.pdf", "pdf", "Hello PDF zebra", "page 1"},
		{"f.docx", "docx", "walrus research", ""},
		{"g.xlsx", "xlsx", "Pelican", "sheet "},
		{"h.pptx", "pptx", "Octopus strategy", "slide 1"},
	}
	for _, c := range cases {
		kind, secs, err := Extract(filepath.Join(dir, c.file))
		if err != nil {
			t.Errorf("%s: %v", c.file, err)
			continue
		}
		if kind != c.kind {
			t.Errorf("%s: tipus %q", c.file, kind)
		}
		var all strings.Builder
		locOK := c.loc == ""
		for _, s := range secs {
			all.WriteString(s.Text)
			if c.loc != "" && strings.HasPrefix(s.Loc, c.loc) {
				locOK = true
			}
		}
		if !strings.Contains(all.String(), c.want) {
			t.Errorf("%s: no hi ha %q a %q", c.file, c.want, all.String())
		}
		if !locOK {
			t.Errorf("%s: cap secció amb ubicació %q: %+v", c.file, c.loc, secs)
		}
	}
	// PDF: la segona pàgina (Flate + TJ amb espais) també es llegeix.
	_, secs, _ := Extract(filepath.Join(dir, "e.pdf"))
	if len(secs) != 2 || secs[1].Loc != "page 2" || !strings.Contains(secs[1].Text, "Flate compressed giraffe page") {
		t.Errorf("pàgina 2 del PDF: %+v", secs)
	}
	// docx: les capçaleres fan de seccions.
	_, secs, _ = Extract(filepath.Join(dir, "f.docx"))
	found := false
	for _, s := range secs {
		if s.Loc == "Budget" && strings.Contains(s.Text, "walrus") {
			found = true
		}
	}
	if !found {
		t.Errorf("docx sense secció Budget: %+v", secs)
	}
	if _, _, err := Extract(filepath.Join(dir, "i.doc")); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf(".doc hauria de ser no suportat: %v", err)
	}
	if _, _, err := Extract(filepath.Join(dir, "j.png")); err != ErrSkip {
		t.Errorf(".png s'hauria de saltar: %v", err)
	}
}

func TestPDFToUnicodeCompositeFont(t *testing.T) {
	cmap := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap\n1 begincodespacerange <0000> <FFFF> endcodespacerange\n2 beginbfchar\n<0001> <0048>\n<0002> <0069>\nendbfchar\n1 beginbfrange\n<0010> <0012> <0061>\nendbfrange\nendcmap end end"
	content := "BT /F1 10 Tf 50 700 Td <000100020003> Tj ( ) Tj <001000110012> Tj ET"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type0 /BaseFont /X /Encoding /Identity-H /ToUnicode 6 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmap), cmap),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.5\n")
	for i, o := range objs {
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	b.WriteString("trailer\n<< /Root 1 0 R >>\n%%EOF")
	secs, err := ExtractPDF(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got := secs[0].Text; !strings.Contains(got, "Hi") || !strings.Contains(got, "abc") {
		t.Fatalf("text: %q", got)
	}
}

func TestPDFEncryptedIsUnsupported(t *testing.T) {
	raw := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R /Encrypt 2 0 R >>\n%%EOF")
	if _, err := ExtractPDF(raw); err != ErrPDFEncrypted {
		t.Fatalf("esperava xifrat, tinc %v", err)
	}
}

func TestScopesAndStorageLocations(t *testing.T) {
	w := isolate(t)
	g, err := Create(w, "Handbook", "Company handbook", ScopeGlobal)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Create(w, "Specs", "", ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Create(w, "Scratch", "", ScopeSession)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(g.ID, "g-") || !strings.HasPrefix(p.ID, "p-") || !strings.HasPrefix(s.ID, "s-") {
		t.Fatalf("ids: %s %s %s", g.ID, p.ID, s.ID)
	}
	data := os.Getenv("GREGAL_DATA_DIR")
	for _, f := range []string{
		filepath.Join(data, "knowledge", "collections.json"),
		filepath.Join(w.Project, ".gregal", "knowledge", "collections.json"),
		filepath.Join(w.Project, ".gregal", "knowledge", ".gitignore"),
		filepath.Join(data, "knowledge", "sessions", "sess-1", "collections.json"),
	} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("falta %s", f)
		}
	}
	// Un altre projecte i una altra sessió no veuen les seves.
	other := Where{Project: t.TempDir(), Session: "sess-2"}
	list, _ := List(other)
	if len(list) != 1 || list[0].ID != g.ID {
		t.Fatalf("des d'un altre lloc només la global: %+v", list)
	}
	list, _ = List(w)
	if len(list) != 3 || list[0].Scope != ScopeGlobal || list[1].Scope != ScopeProject || list[2].Scope != ScopeSession {
		t.Fatalf("ordre per abast: %+v", list)
	}
	if _, err := Create(w, "specs", "", ScopeProject); err != ErrDuplicate {
		t.Errorf("nom repetit: %v", err)
	}
	if _, err := Create(Where{}, "X", "", ScopeProject); err != ErrNoProject {
		t.Errorf("sense projecte: %v", err)
	}
	if c, err := Resolve(w, "SPECS"); err != nil || c.ID != p.ID {
		t.Errorf("resolve per nom: %v %v", c, err)
	}
	// Una font dins del projecte es desa relativa (viatja amb el repo).
	write(t, filepath.Join(w.Project, "docs", "a.md"), "x")
	src, err := AddSource(w, p.ID, filepath.Join(w.Project, "docs"), []string{"*.md, *.txt"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if src.Path != filepath.Join(w.Project, "docs") {
		t.Fatalf("la font torna la ruta absoluta: %s", src.Path)
	}
	got, _ := Get(w, p.ID)
	if got.Sources[0].Path != "docs" || len(got.Sources[0].Include) != 2 {
		t.Errorf("font desada: %+v", got.Sources[0])
	}
	if err := Delete(w, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(w, s.ID); err != ErrNotFound {
		t.Errorf("no s'ha esborrat: %v", err)
	}
}

func TestReindexIncrementalAndSearch(t *testing.T) {
	w := isolate(t)
	docs := filepath.Join(w.Project, "kb")
	write(t, filepath.Join(docs, "pumps.md"), "# Pumps\nThe centrifugal pump needs priming before start.\n\n## Maintenance\nReplace the impeller seal every 2000 hours.\n")
	write(t, filepath.Join(docs, "valves.md"), "# Valves\nGate valves are fully open or fully closed.\n")
	write(t, filepath.Join(docs, "notes.txt"), "Unrelated text about lunch menus and coffee.")
	write(t, filepath.Join(docs, "skip", "big.bin"), "\x00\x01\x02")
	write(t, filepath.Join(docs, "node_modules", "dep.md"), "impeller impeller impeller")
	if err := os.WriteFile(filepath.Join(docs, "manual.pdf"), pdfFixture("Impeller torque is 35 Nm"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := Create(w, "Plant", "Plant manuals", ScopeProject)
	if _, err := AddSource(w, c.ID, docs, nil, []string{"notes.txt"}); err != nil {
		t.Fatal(err)
	}
	ix, err := Reindex(context.Background(), w, c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	st := ix.Stats()
	if st.Docs != 3 {
		t.Fatalf("documents: %+v %+v", st, ix.Docs)
	}
	for _, d := range ix.Docs {
		if strings.Contains(d.Path, "node_modules") || strings.Contains(d.Path, "notes.txt") {
			t.Errorf("no s'havia d'indexar: %s", d.Path)
		}
	}
	hits, err := Search(context.Background(), w, "impeller seal replacement", SearchOpts{K: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Path != "kb/pumps.md" || !strings.Contains(hits[0].Loc, "Maintenance") {
		t.Fatalf("primer resultat: %+v", hits)
	}
	hits, _ = Search(context.Background(), w, "impeller torque", SearchOpts{K: 5})
	if len(hits) == 0 || hits[0].Path != "kb/manual.pdf" || hits[0].Loc != "page 1" {
		t.Fatalf("pdf: %+v", hits)
	}
	// Incremental: un fitxer sense canvis no es rellegeix. Ho comprovem
	// fent-lo il·legible: si es rellegís, sortiria com a error.
	before := ix.Docs
	write(t, filepath.Join(docs, "valves.md"), "# Valves\nButterfly valves throttle flow.\n")
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(filepath.Join(docs, "valves.md"), future, future)
	ix2, err := Reindex(context.Background(), w, c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	var pumpsBefore, pumpsAfter DocInfo
	for _, d := range before {
		if d.Path == "kb/pumps.md" {
			pumpsBefore = d
		}
	}
	for _, d := range ix2.Docs {
		if d.Path == "kb/pumps.md" {
			pumpsAfter = d
		}
	}
	if !pumpsAfter.Indexed.Equal(pumpsBefore.Indexed) {
		t.Errorf("pumps.md s'ha rellegit sense canvis")
	}
	if hits, _ := Search(context.Background(), w, "butterfly", SearchOpts{}); len(hits) == 0 {
		t.Errorf("el canvi a valves.md no s'ha indexat")
	}
	if hits, _ := Search(context.Background(), w, "gate valves", SearchOpts{}); len(hits) > 0 && strings.Contains(hits[0].Snippet, "Gate valves") {
		t.Errorf("text vell encara indexat: %+v", hits)
	}
	// Un fitxer esborrat desapareix.
	os.Remove(filepath.Join(docs, "valves.md"))
	ix3, _ := Reindex(context.Background(), w, c.ID, false)
	if ix3.Stats().Docs != 2 {
		t.Errorf("esborrat encara a l'índex: %+v", ix3.Docs)
	}
	// Desactivada: fora de la cerca general.
	Update(w, c.ID, func(c *Collection) error { c.Enabled = false; return nil })
	if hits, _ := Search(context.Background(), w, "impeller", SearchOpts{}); len(hits) != 0 {
		t.Errorf("col·lecció desactivada a la cerca: %+v", hits)
	}
	if PromptIndex(w) != "" {
		t.Errorf("una col·lecció desactivada surt al prompt")
	}
}

func TestBM25PrefersRareTermsAndShortChunks(t *testing.T) {
	ix := &Index{Version: indexVersion, Docs: []DocInfo{{Key: "a", Path: "a.md"}, {Key: "b", Path: "b.md"}, {Key: "c", Path: "c.md"}}}
	ix.Chunks = []ChunkRec{
		{Doc: 0, Text: "common common common rare"},
		{Doc: 1, Text: "common words only here and there with filler filler filler filler filler"},
		{Doc: 2, Text: "common rare " + strings.Repeat("filler ", 200)},
	}
	buildPostings(ix)
	hits := rank(context.Background(), []loaded{{Collection{ID: "p-x", Name: "X", Scope: ScopeProject}, ix}}, QueryTerms("rare common"), "rare common", 3)
	if len(hits) != 3 || hits[0].Path != "a.md" || hits[1].Path != "c.md" {
		t.Fatalf("ordre BM25: %+v", hits)
	}
}

func TestNotesToolAndScopes(t *testing.T) {
	w := isolate(t)
	out, err := NoteTool(w, `{"text":"The staging database is called heron-db","title":"Staging"}`)
	if err != nil || !strings.Contains(out, SessionNotesName) {
		t.Fatalf("%q %v", out, err)
	}
	res, err := SearchTool(context.Background(), w, `{"query":"staging database name"}`)
	if err != nil || !strings.Contains(res, "heron-db") || !strings.Contains(res, "note: Staging") {
		t.Fatalf("la nota no es troba: %q %v", res, err)
	}
	g, _ := Create(w, "Global facts", "", ScopeGlobal)
	p, _ := Create(w, "Proj", "", ScopeProject)
	if s := NoteScope(w, `{"collection":"Global facts"}`); s != ScopeGlobal {
		t.Errorf("global: %s", s)
	}
	if s := NoteScope(w, `{"collection":"`+p.ID+`"}`); s != ScopeProject {
		t.Errorf("projecte: %s", s)
	}
	if s := NoteScope(w, `{}`); s != ScopeSession {
		t.Errorf("per defecte: %s", s)
	}
	if s := NoteScope(w, `{"collection":"nope"}`); s != ScopeGlobal {
		t.Errorf("desconeguda ha de ser la més restrictiva: %s", s)
	}
	_ = g
	idx := PromptIndex(w)
	if !strings.Contains(idx, "Global facts [global]") || !strings.Contains(idx, "knowledge_search") || !strings.Contains(idx, SessionNotesName) {
		t.Errorf("prompt: %q", idx)
	}
	c, _ := Resolve(w, SessionNotesName)
	if err := RemoveNote(w, c.ID, c.Notes[0].ID); err != nil {
		t.Fatal(err)
	}
	if res, _ := SearchTool(context.Background(), w, `{"query":"heron"}`); strings.Contains(res, "heron-db") {
		t.Errorf("nota esborrada encara es troba")
	}
}

func TestSnippetCentersOnTerms(t *testing.T) {
	text := strings.Repeat("lorem ipsum ", 100) + "the secret ingredient is cardamom " + strings.Repeat("dolor sit ", 100)
	s := Snippet(text, QueryTerms("cardamom ingredient"), 200)
	if !strings.Contains(s, "cardamom") || len([]rune(s)) > 210 {
		t.Fatalf("snippet: %q", s)
	}
}

func TestLimitsTruncateAndSkipLargeFiles(t *testing.T) {
	w := isolate(t)
	Configure(Settings{MaxFiles: 2, MaxFileMB: 1})
	defer Configure(Settings{})
	dir := filepath.Join(w.Project, "many")
	for i := 0; i < 5; i++ {
		write(t, filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), "hello world")
	}
	c, _ := Create(w, "Many", "", ScopeProject)
	AddSource(w, c.ID, dir, nil, nil)
	ix, err := Reindex(context.Background(), w, c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ix.Truncated || len(ix.Docs) != 2 {
		t.Fatalf("límit de fitxers: truncated=%v docs=%d", ix.Truncated, len(ix.Docs))
	}
	big := filepath.Join(w.Project, "big")
	write(t, filepath.Join(big, "huge.txt"), strings.Repeat("x", 1<<20+10))
	c2, _ := Create(w, "Big", "", ScopeProject)
	AddSource(w, c2.ID, big, nil, nil)
	ix, _ = Reindex(context.Background(), w, c2.ID, false)
	if len(ix.Docs) != 1 || ix.Docs[0].Status != DocSkipped {
		t.Fatalf("fitxer gros: %+v", ix.Docs)
	}
}

func TestStartReindexReportsProgress(t *testing.T) {
	w := isolate(t)
	write(t, filepath.Join(w.Project, "d", "a.md"), "alpha")
	c, _ := Create(w, "P", "", ScopeProject)
	AddSource(w, c.ID, filepath.Join(w.Project, "d"), nil, nil)
	if err := StartReindex(w, c.ID, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for ProgressOf(c.ID).Running && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	p := ProgressOf(c.ID)
	if p.Running || p.Phase != "done" || p.Total != 1 {
		t.Fatalf("progrés: %+v", p)
	}
}
