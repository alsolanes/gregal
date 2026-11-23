package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	if len(got) != 2 || got[0]["id"] != d.ID || got[0]["title"] != "Pla" || got[1]["id"] != d.ID {
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
