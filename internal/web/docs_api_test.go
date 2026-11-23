package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"gregal/internal/gdocs"
	"gregal/internal/ooxml"
)

func docExport(s *Server, id string) *httptest.ResponseRecorder {
	return docExportAs(s, id, "docx")
}

func docExportAs(s *Server, id, format string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/"+id+"/export?format="+format, nil)
	req.SetPathValue("id", id)
	s.handleDocExport(rec, req)
	return rec
}

func TestDocExportPptx(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.CreateDeck("Llançament")
	if err != nil {
		t.Fatal(err)
	}
	after := ""
	for _, sl := range []ooxml.SlideSpec{
		{Layout: "title", Title: "Llançament 2027"},
		{Layout: "content", Title: "Fases", Bullets: []string{"Beta", "Obert"}},
		{Layout: "two_columns", Title: "Riscos", Left: "Temps", Right: "Cost"},
	} {
		if d, err = st.InsertSlide(d.ID, after, sl, gdocs.AuthorAgent); err != nil {
			t.Fatal(err)
		}
		after = d.Blocks[len(d.Blocks)-1].ID
	}
	rec := docExportAs(s, d.ID, "pptx")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "Llançament.pptx") {
		t.Fatalf("%d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	pk, err := ooxml.OpenBytes(rec.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if iss := ooxml.NewIssues(pk, nil); len(iss) > 0 {
		t.Fatalf("pptx invàlid: %v", iss)
	}
	text, err := ooxml.ReadPptx(pk)
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := strings.Index(text, "# Llançament 2027"), strings.Index(text, "# Fases"), strings.Index(text, "# Riscos")
	if a < 0 || b < a || c < b || !strings.Contains(text, "Beta") || !strings.Contains(text, "Cost") {
		t.Fatalf("diapositives:\n%s", text)
	}
}

func TestDocExportNomesEnElFormatDelTipus(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, _ := st.CreateDeck("Llançament")
	if rec := docExportAs(s, d.ID, "docx"); rec.Code != http.StatusBadRequest {
		t.Fatalf("una presentació en docx: %d", rec.Code)
	}
	text, _ := st.Create("Plan", "a", gdocs.AuthorAgent)
	if rec := docExportAs(s, text.ID, "pptx"); rec.Code != http.StatusBadRequest {
		t.Fatalf("un document de text en pptx: %d", rec.Code)
	}
	if rec := docExportAs(s, text.ID, "xlsx"); rec.Code != http.StatusBadRequest {
		t.Fatalf("un document de text en xlsx: %d", rec.Code)
	}
}

// pressupost fa un full de dos fulls amb una fórmula que creua d'un a l'altre.
func pressupost(t *testing.T, st *gdocs.Store) gdocs.Doc {
	t.Helper()
	d, err := st.CreateSheet("Pressupost", "Dades", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if d, err = st.SetCells(d.ID, d.Blocks[0].ID, map[string]string{"A1": "10", "A2": "20", "A3": "=SUM(A1:A2)"}, gdocs.AuthorAgent, 0); err != nil {
		t.Fatal(err)
	}
	if d, err = st.InsertSheet(d.ID, d.Blocks[0].ID, "Resum", gdocs.AuthorAgent); err != nil {
		t.Fatal(err)
	}
	if d, err = st.SetCells(d.ID, d.Blocks[1].ID, map[string]string{"B2": "=Dades!A3*2"}, gdocs.AuthorAgent, 0); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDocExportXlsx(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d := pressupost(t, st)
	rec := docExportAs(s, d.ID, "xlsx")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "Pressupost.xlsx") {
		t.Fatalf("%d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	f, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	if got := f.GetSheetList(); len(got) != 2 || got[0] != "Dades" || got[1] != "Resum" {
		t.Fatalf("fulls %v", got)
	}
	for _, c := range []struct{ sheet, cell, formula, value string }{
		{"Dades", "A3", "SUM(A1:A2)", "30"},
		{"Resum", "B2", "Dades!A3*2", "60"},
		{"Dades", "A1", "", "10"},
	} {
		fx, err1 := f.GetCellFormula(c.sheet, c.cell)
		v, err2 := f.CalcCellValue(c.sheet, c.cell)
		if err1 != nil || err2 != nil || fx != c.formula || v != c.value {
			t.Fatalf("%s!%s: %q = %q (%v %v)", c.sheet, c.cell, fx, v, err1, err2)
		}
	}
}

func TestDocCellsPutAndConflict(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.CreateSheet("Pressupost", "", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	full := d.Blocks[0].ID
	code, out := docWrite(t, s, http.MethodPut, "/", `{"cells":{"A1":"2","A2":"=A1*3"},"base_version":1}`, d.ID, full)
	blk, _ := out["block"].(map[string]any)
	vals, _ := blk["values"].(map[string]any)
	if code != 200 || out["version"] != 2.0 || blk["author"] != gdocs.AuthorUser || vals["A2"] != "6" {
		t.Fatalf("put: %d %v", code, out)
	}
	code, out = docWrite(t, s, http.MethodPut, "/", `{"cells":{"A1":"5"},"base_version":1}`, d.ID, full)
	blk, _ = out["block"].(map[string]any)
	if code != http.StatusConflict || blk["version"] != 2.0 {
		t.Fatalf("conflicte: %d %v", code, out)
	}
	if code, out = docWrite(t, s, http.MethodPut, "/", `{"cells":{},"base_version":2}`, d.ID, full); code != http.StatusBadRequest {
		t.Fatalf("sense cel·les: %d %v", code, out)
	}
	if code, out = docWrite(t, s, http.MethodPut, "/", `{"cells":{"A1":"5"},"base_version":2}`, "noexist", full); code != http.StatusNotFound {
		t.Fatalf("document que no hi és: %d %v", code, out)
	}
}

func TestDocExportDocx(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.Create("Pla de llançament", "# Objectius\n\n- Primer\n- Segon\n\n| Fase | Data |\n|---|---|\n| Beta | Març |", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	rec := docExport(s, d.ID)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "Pla_de_llançament.docx") {
		t.Fatalf("%d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	pk, err := ooxml.OpenBytes(rec.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if iss := ooxml.NewIssues(pk, nil); len(iss) > 0 {
		t.Fatalf("docx invàlid: %v", iss)
	}
	doc, err := ooxml.OpenDocx(pk)
	if err != nil {
		t.Fatal(err)
	}
	text := ooxml.ParaTextAll(doc.Body)
	for _, w := range []string{"Objectius", "Primer", "Segon", "Fase", "Data", "Beta", "Març"} {
		if !strings.Contains(text, w) {
			t.Fatalf("falta %q a:\n%s", w, text)
		}
	}

	// Un document d'un altre projecte no es troba des d'aquest.
	s.cwd = t.TempDir()
	if rec := docExport(s, d.ID); rec.Code != http.StatusNotFound {
		t.Fatalf("un altre projecte: %d", rec.Code)
	}
}

func docGet(s *Server, h func(*Server, http.ResponseWriter, *http.Request), id string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/"+id, nil)
	req.SetPathValue("id", id)
	h(s, rec, req)
	return rec
}

func TestDocsListAndGet(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.Create("Pla", "# Objectius\n\nPrimer paràgraf", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Items []map[string]any `json:"items"`
	}
	rec := docGet(s, (*Server).handleDocs, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Items) != 1 {
		t.Fatalf("llista: %d %s", rec.Code, rec.Body.String())
	}
	for _, k := range []string{"id", "title", "kind", "version", "updated"} {
		if _, ok := list.Items[0][k]; !ok {
			t.Fatalf("falta %q a %v", k, list.Items[0])
		}
	}
	var got gdocs.Doc
	rec = docGet(s, (*Server).handleDoc, d.ID)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got.Blocks) != 2 || got.Blocks[1].MD != "Primer paràgraf" {
		t.Fatalf("document: %d %s", rec.Code, rec.Body.String())
	}

	// Un id d'un altre projecte no es troba des d'aquest.
	s.cwd = t.TempDir()
	for _, h := range []func(*Server, http.ResponseWriter, *http.Request){(*Server).handleDoc, (*Server).handleDocStream} {
		if rec := docGet(s, h, d.ID); rec.Code != http.StatusNotFound {
			t.Fatalf("un altre projecte: %d", rec.Code)
		}
	}
}

func TestDocStream(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.Create("Pla", "# Objectius\n\nPrimer paràgraf", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", d.ID)
		s.handleDocStream(w, r)
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := st.Replace(d.ID, d.Blocks[1].ID, "Segon paràgraf", gdocs.AuthorUser, d.Version); err != nil {
		t.Fatal(err)
	}
	c := nextBlockEvent(t, bufio.NewScanner(resp.Body))
	if c.Op != "replace" || c.Version != 2 || c.Block.ID != d.Blocks[1].ID || c.Block.MD != "Segon paràgraf" || c.Author != gdocs.AuthorUser {
		t.Fatalf("event: %+v", c)
	}
}

// nextBlockEvent llegeix l'SSE fins al següent event block.
func nextBlockEvent(t *testing.T, sc *bufio.Scanner) gdocs.Change {
	t.Helper()
	event := ""
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		}
		if event == "block" && strings.HasPrefix(line, "data: ") {
			var c gdocs.Change
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &c); err != nil {
				t.Fatal(err)
			}
			return c
		}
	}
	t.Fatalf("cap event block: %v", sc.Err())
	return gdocs.Change{}
}

// docWrite crida una ruta d'escriptura de blocs i en torna el codi i el cos.
func docWrite(t *testing.T, s *Server, method, target, body string, path ...string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.SetPathValue("id", path[0])
	if len(path) > 1 {
		req.SetPathValue("block", path[1])
		s.handleDocBlock(rec, req)
	} else {
		s.handleDocBlocks(rec, req)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	return rec.Code, out
}

// docBlocksFixture crea un document de tres blocs i hi obre dos streams.
func docBlocksFixture(t *testing.T) (*Server, *gdocs.Store, gdocs.Doc, []*bufio.Scanner) {
	t.Helper()
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.Create("Pla", "# Objectius\n\nPrimer paràgraf\n\nSegon paràgraf", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", d.ID)
		s.handleDocStream(w, r)
	}))
	t.Cleanup(srv.Close)
	var streams []*bufio.Scanner
	for range 2 {
		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		})
		streams = append(streams, bufio.NewScanner(resp.Body))
	}
	return s, st, d, streams
}

// expectEvent comprova que cada stream rep un event que compleix ok.
func expectEvent(t *testing.T, streams []*bufio.Scanner, what string, ok func(gdocs.Change) bool) {
	t.Helper()
	for _, r := range streams {
		if c := nextBlockEvent(t, r); !ok(c) {
			t.Fatalf("event %s: %+v", what, c)
		}
	}
}

func TestDocBlockPutAndConflict(t *testing.T) {
	s, _, d, streams := docBlocksFixture(t)
	b1 := d.Blocks[1].ID
	code, out := docWrite(t, s, http.MethodPut, "/", `{"md":"Primer, a mà","base_version":1}`, d.ID, b1)
	blk, _ := out["block"].(map[string]any)
	if code != 200 || out["version"] != 2.0 || blk["md"] != "Primer, a mà" || blk["author"] != gdocs.AuthorUser || blk["version"] != 2.0 {
		t.Fatalf("put: %d %v", code, out)
	}
	expectEvent(t, streams, "put", func(c gdocs.Change) bool {
		return c.Op == "replace" && c.Block.ID == b1 && c.Author == gdocs.AuthorUser && c.Version == 2
	})

	// Algú altre l'ha canviat després de la versió 1: 409 amb el bloc d'ara.
	code, out = docWrite(t, s, http.MethodPut, "/", `{"md":"Una altra","base_version":1}`, d.ID, b1)
	blk, _ = out["block"].(map[string]any)
	if code != http.StatusConflict || blk["md"] != "Primer, a mà" || blk["version"] != 2.0 {
		t.Fatalf("conflicte: %d %v", code, out)
	}
	if code, out = docWrite(t, s, http.MethodDelete, "/?base_version=1", "", d.ID, b1); code != http.StatusConflict {
		t.Fatalf("conflicte delete: %d %v", code, out)
	}
	if code, _ := docWrite(t, s, http.MethodPut, "/", `{"md":"x","base_version":9}`, d.ID, "noexist"); code != http.StatusNotFound {
		t.Fatalf("bloc que no hi és: %d", code)
	}
}

func TestDocBlockPostAndDelete(t *testing.T) {
	s, st, d, streams := docBlocksFixture(t)
	b1, b2 := d.Blocks[1].ID, d.Blocks[2].ID
	code, out := docWrite(t, s, http.MethodPost, "/", `{"after":"`+b1+`","md":"Nou u\n\nNou dos"}`, d.ID)
	nous, _ := out["blocks"].([]any)
	if code != 200 || out["version"] != 2.0 || len(nous) != 2 || nous[0].(map[string]any)["md"] != "Nou u" {
		t.Fatalf("post: %d %v", code, out)
	}
	expectEvent(t, streams, "post", func(c gdocs.Change) bool {
		return c.Op == "insert" && c.After == b1 && c.Block.MD == "Nou u" && c.Author == gdocs.AuthorUser
	})
	expectEvent(t, streams, "post 2", func(c gdocs.Change) bool { return c.Op == "insert" && c.Block.MD == "Nou dos" })

	if code, out = docWrite(t, s, http.MethodDelete, "/?base_version=1", "", d.ID, b2); code != 200 || out["version"] != 3.0 {
		t.Fatalf("delete: %d %v", code, out)
	}
	expectEvent(t, streams, "delete", func(c gdocs.Change) bool {
		return c.Op == "delete" && c.Block.ID == b2 && c.Author == gdocs.AuthorUser
	})
	got, _ := st.Get(d.ID)
	if got.Markdown() != "# Objectius\n\nPrimer paràgraf\n\nNou u\n\nNou dos" {
		t.Fatalf("document:\n%s", got.Markdown())
	}
}

func TestDocChangedWhenTheAgentWrites(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, _ := st.Create("Pla", "Un", gdocs.AuthorAgent)
	var got []map[string]string
	emit := func(ev string, v any) {
		if ev == "doc_changed" {
			got = append(got, v.(map[string]string))
		}
	}
	s.emitDocChanged(emit, s.cwd, "", `{"action":"create","title":"Pla"}`, "Created document "+d.ID+" with 1 blocks.")
	s.emitDocChanged(emit, s.cwd, "", `{"action":"insert","id":"`+d.ID+`","md":"Dos"}`, "Document "+d.ID+" · Pla · v2\n")
	s.emitDocChanged(emit, s.cwd, "", `{"action":"read","id":"`+d.ID+`"}`, "Document "+d.ID+" · Pla · v2\n")
	s.emitDocChanged(emit, s.cwd, "", `{"action":"replace","id":"`+d.ID+`"}`, "ERROR: there is no block with that id")
	s.emitDocChanged(emit, s.cwd, "", `{"action":"set_cells","id":"`+d.ID+`"}`, "Document "+d.ID+" · Pla · v3\n")
	if len(got) != 3 || got[0]["id"] != d.ID || got[0]["title"] != "Pla" || got[1]["id"] != d.ID || got[2]["id"] != d.ID {
		t.Fatalf("%v", got)
	}
}

func TestDocPageServesTheEditorOfAnExistingDoc(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.mu.Lock()
	s.cwd = t.TempDir()
	s.mu.Unlock()
	st, _ := gdocs.Open(s.cwd)
	d, err := st.Create("Pla", "# Objectius", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	mux := s.Hub().mux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/doc/"+d.ID, nil))
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") || !strings.Contains(rec.Body.String(), "/app/doc-page.js") {
		t.Fatalf("pàgina: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/doc/zzzzzzzzzzzz", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("id desconegut: %d %s", rec.Code, rec.Body.String())
	}

	// Amb token, la pàgina surt sense mirar res i no crea cap sessió.
	s.Token = "secret"
	h := s.Hub()
	h.mu.RLock()
	before := len(h.sess)
	h.mu.RUnlock()
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/doc/zzzzzzzzzzzz?session=anonima", nil))
	h.mu.RLock()
	after := len(h.sess)
	h.mu.RUnlock()
	if rec.Code != 200 || after != before {
		t.Fatalf("amb token: %d, sessions %d → %d", rec.Code, before, after)
	}
}

// «Document nou» de Gregal Docs: POST /api/docs?scope=mine fa un document
// de text buit a la carpeta de la persona, no al projecte. Buit, no té cap
// bloc amb autor: el primer que hi escriu la persona és seu.
func TestNewDocGoesToMyDocuments(t *testing.T) {
	s := backlogServer(t)
	rec := docReq(s, (*Server).handleDocs, "POST", "/api/docs?scope=mine", `{"title":"Notes del viatge","kind":"doc"}`)
	if rec.Code != 200 {
		t.Fatalf("crear: %d %s", rec.Code, rec.Body.String())
	}
	var d gdocs.Doc
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.ID == "" || d.Title != "Notes del viatge" || d.Kind != gdocs.KindDoc || len(d.Blocks) != 0 {
		t.Fatalf("document nou: %+v", d)
	}
	if rec := docReq(s, (*Server).handleDocs, "GET", "/api/docs?scope=mine", ""); !strings.Contains(rec.Body.String(), d.ID) {
		t.Fatalf("no és a «Els meus documents»: %s", rec.Body.String())
	}
	if rec := docReq(s, (*Server).handleDocs, "GET", "/api/docs", ""); strings.Contains(rec.Body.String(), d.ID) {
		t.Fatalf("ha anat al projecte: %s", rec.Body.String())
	}
	if rec := docReq(s, (*Server).handleDocBlocks, "POST", "/api/docs/x/blocks?scope=mine", `{"after":"","md":"Primer dia"}`, d.ID); rec.Code != 200 {
		t.Fatalf("primer bloc: %d %s", rec.Code, rec.Body.String())
	}
	mine, _ := gdocs.Open(s.windDocsDir())
	if got, _ := mine.Get(d.ID); len(got.Blocks) != 1 || got.Blocks[0].Author != gdocs.AuthorUser {
		t.Fatalf("blocs: %+v", got.Blocks)
	}
}

func TestNewDocRefusesUnknownKindAndNoTitle(t *testing.T) {
	s := backlogServer(t)
	for _, body := range []string{`{"title":"Pla","kind":"poema"}`, `{"title":"Pla"}`, `{"title":"  ","kind":"doc"}`, `nope`} {
		if rec := docReq(s, (*Server).handleDocs, "POST", "/api/docs?scope=mine", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := docReq(s, (*Server).handleDocs, "GET", "/api/docs?scope=mine", ""); strings.Contains(rec.Body.String(), `"id"`) {
		t.Fatalf("s'ha creat algun document: %s", rec.Body.String())
	}
}

type docAnswer struct {
	Version int
	Block   gdocs.Block
	Blocks  []gdocs.Block
}

func answer(t *testing.T, rec *httptest.ResponseRecorder) (out docAnswer) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	return out
}

// newDeckWithSlide crea una presentació a «Els meus documents» i hi posa
// una diapositiva, tot per les rutes del panell. Torna el document i la
// diapositiva.
func newDeckWithSlide(t *testing.T, s *Server) (string, string) {
	t.Helper()
	rec := docReq(s, (*Server).handleDocs, "POST", "/api/docs?scope=mine", `{"title":"Llançament","kind":"deck"}`)
	var d gdocs.Doc
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &d) != nil || d.Kind != gdocs.KindDeck || len(d.Blocks) != 0 {
		t.Fatalf("presentació nova: %d %s", rec.Code, rec.Body.String())
	}
	rec = docReq(s, (*Server).handleDocBlocks, "POST", "/api/docs/x/blocks?scope=mine", `{"after":"","slide":{"layout":"content","title":"Fases"}}`, d.ID)
	nou := answer(t, rec)
	if rec.Code != 200 || len(nou.Blocks) != 1 || nou.Blocks[0].Slide == nil || nou.Blocks[0].Slide.Title != "Fases" || nou.Blocks[0].Author != gdocs.AuthorUser {
		t.Fatalf("diapositiva nova: %d %s", rec.Code, rec.Body.String())
	}
	return d.ID, nou.Blocks[0].ID
}

func TestNewDeckAndItsSlidesFromThePanel(t *testing.T) {
	s := backlogServer(t)
	id, sl := newDeckWithSlide(t, s)
	rec := docReq(s, (*Server).handleDocBlock, "PUT", "/api/docs/x/blocks/y?scope=mine", `{"slide":{"layout":"content","title":"Fases","bullets":["Beta","Obert"]},"base_version":2}`, id, sl)
	if out := answer(t, rec); rec.Code != 200 || out.Block.Slide == nil || len(out.Block.Slide.Bullets) != 2 || out.Block.Version != 3 {
		t.Fatalf("desa: %d %s", rec.Code, rec.Body.String())
	}
	rec = docReq(s, (*Server).handleDocBlock, "PUT", "/api/docs/x/blocks/y?scope=mine", `{"slide":{"layout":"content","title":"Una altra"},"base_version":2}`, id, sl)
	if out := answer(t, rec); rec.Code != http.StatusConflict || out.Block.Slide == nil || out.Block.Slide.Title != "Fases" {
		t.Fatalf("conflicte: %d %s", rec.Code, rec.Body.String())
	}
}

// Una edició que no encaixa amb el document és culpa de qui l'envia: 400.
func TestDeckRefusesEditsOfAnotherKind(t *testing.T) {
	s := backlogServer(t)
	id, sl := newDeckWithSlide(t, s)
	for _, body := range []string{`{"md":"Text","base_version":2}`, `{"slide":{"layout":"poema","title":"Fases"},"base_version":2}`} {
		if rec := docReq(s, (*Server).handleDocBlock, "PUT", "/api/docs/x/blocks/y?scope=mine", body, id, sl); rec.Code != http.StatusBadRequest {
			t.Fatalf("put %s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := docReq(s, (*Server).handleDocBlocks, "POST", "/api/docs/x/blocks?scope=mine", `{"after":"","md":"Text"}`, id); rec.Code != http.StatusBadRequest {
		t.Fatalf("md a una presentació: %d %s", rec.Code, rec.Body.String())
	}
}

// newSheet crea un full de càlcul a «Els meus documents» per la ruta del
// panell, com fan l'app d'Android i la web. Neix amb un full buit.
func newSheet(t *testing.T, s *Server) gdocs.Doc {
	t.Helper()
	rec := docReq(s, (*Server).handleDocs, "POST", "/api/docs?scope=mine", `{"title":"Pressupost","kind":"sheet"}`)
	var d gdocs.Doc
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &d) != nil || d.Kind != gdocs.KindSheet || len(d.Blocks) != 1 {
		t.Fatalf("full de càlcul nou: %d %s", rec.Code, rec.Body.String())
	}
	return d
}

// «Full nou» hi afegeix un full, i se n'editen les cel·les.
func TestNewSheetAndItsSheetsFromThePanel(t *testing.T) {
	s := backlogServer(t)
	d := newSheet(t, s)
	rec := docReq(s, (*Server).handleDocBlocks, "POST", "/api/docs/x/blocks?scope=mine", `{"after":"`+d.Blocks[0].ID+`","name":"Vendes"}`, d.ID)
	nou := answer(t, rec)
	if rec.Code != 200 || len(nou.Blocks) != 1 || nou.Blocks[0].Name != "Vendes" || nou.Blocks[0].Author != gdocs.AuthorUser {
		t.Fatalf("full nou: %d %s", rec.Code, rec.Body.String())
	}
	rec = docReq(s, (*Server).handleDocBlock, "PUT", "/api/docs/x/blocks/y?scope=mine", `{"cells":{"A1":"2","A2":"=A1*3"},"base_version":2}`, d.ID, nou.Blocks[0].ID)
	if out := answer(t, rec); rec.Code != 200 || out.Block.Values["A2"] != "6" {
		t.Fatalf("cel·les: %d %s", rec.Code, rec.Body.String())
	}
}

// Una cel·la mal dita, un rang massa gran o un full repetit són culpa de
// qui els envia: 400, no 500.
func TestSheetRefusesBadInputWith400(t *testing.T) {
	s := backlogServer(t)
	d := newSheet(t, s)
	sh := d.Blocks[0]
	for _, body := range []string{`{"cells":{"1A":"3"},"base_version":1}`, `{"cells":{"A1":"=SUM(A1:Z100000)"},"base_version":1}`} {
		if rec := docReq(s, (*Server).handleDocBlock, "PUT", "/api/docs/x/blocks/y?scope=mine", body, d.ID, sh.ID); rec.Code != http.StatusBadRequest {
			t.Fatalf("put %s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := docReq(s, (*Server).handleDocBlocks, "POST", "/api/docs/x/blocks?scope=mine", `{"after":"","name":"`+sh.Name+`"}`, d.ID); rec.Code != http.StatusBadRequest {
		t.Fatalf("full repetit: %d %s", rec.Code, rec.Body.String())
	}
}
