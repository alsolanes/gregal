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
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/"+id+"/export?format=docx", nil)
	req.SetPathValue("id", id)
	s.handleDocExport(rec, req)
	return rec
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

func TestDocBlockWritesAsTheUser(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.Create("Pla", "# Objectius\n\nPrimer paràgraf\n\nSegon paràgraf", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	b1, b2 := d.Blocks[1].ID, d.Blocks[2].ID
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", d.ID)
		s.handleDocStream(w, r)
	}))
	defer srv.Close()
	var streams []*bufio.Scanner
	for range 2 {
		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()
		streams = append(streams, bufio.NewScanner(resp.Body))
	}

	code, out := docWrite(t, s, http.MethodPut, "/", `{"md":"Primer, a mà","base_version":1}`, d.ID, b1)
	blk, _ := out["block"].(map[string]any)
	if code != 200 || out["version"] != 2.0 || blk["md"] != "Primer, a mà" || blk["author"] != gdocs.AuthorUser || blk["version"] != 2.0 {
		t.Fatalf("put: %d %v", code, out)
	}
	for _, r := range streams {
		if c := nextBlockEvent(t, r); c.Op != "replace" || c.Block.ID != b1 || c.Author != gdocs.AuthorUser || c.Version != 2 {
			t.Fatalf("event put: %+v", c)
		}
	}

	// Algú altre l'ha canviat després de la versió 1: 409 amb el bloc d'ara.
	code, out = docWrite(t, s, http.MethodPut, "/", `{"md":"Una altra","base_version":1}`, d.ID, b1)
	blk, _ = out["block"].(map[string]any)
	if code != http.StatusConflict || blk["md"] != "Primer, a mà" || blk["version"] != 2.0 {
		t.Fatalf("conflicte: %d %v", code, out)
	}
	if code, out = docWrite(t, s, http.MethodDelete, "/?base_version=1", "", d.ID, b1); code != http.StatusConflict {
		t.Fatalf("conflicte delete: %d %v", code, out)
	}

	code, out = docWrite(t, s, http.MethodPost, "/", `{"after":"`+b1+`","md":"Nou u\n\nNou dos"}`, d.ID)
	nous, _ := out["blocks"].([]any)
	if code != 200 || out["version"] != 3.0 || len(nous) != 2 || nous[0].(map[string]any)["md"] != "Nou u" {
		t.Fatalf("post: %d %v", code, out)
	}
	for _, r := range streams {
		if c := nextBlockEvent(t, r); c.Op != "insert" || c.After != b1 || c.Block.MD != "Nou u" || c.Author != gdocs.AuthorUser {
			t.Fatalf("event post: %+v", c)
		}
		if c := nextBlockEvent(t, r); c.Op != "insert" || c.Block.MD != "Nou dos" {
			t.Fatalf("event post 2: %+v", c)
		}
	}

	if code, out = docWrite(t, s, http.MethodDelete, "/?base_version=1", "", d.ID, b2); code != 200 || out["version"] != 4.0 {
		t.Fatalf("delete: %d %v", code, out)
	}
	for _, r := range streams {
		if c := nextBlockEvent(t, r); c.Op != "delete" || c.Block.ID != b2 || c.Author != gdocs.AuthorUser {
			t.Fatalf("event delete: %+v", c)
		}
	}
	got, _ := st.Get(d.ID)
	if got.Markdown() != "# Objectius\n\nPrimer, a mà\n\nNou u\n\nNou dos" {
		t.Fatalf("document:\n%s", got.Markdown())
	}
	if code, _ := docWrite(t, s, http.MethodPut, "/", `{"md":"x","base_version":9}`, d.ID, "noexist"); code != http.StatusNotFound {
		t.Fatalf("bloc que no hi és: %d", code)
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
	s.emitDocChanged(emit, `{"action":"create","title":"Pla"}`, "Created document "+d.ID+" with 1 blocks.")
	s.emitDocChanged(emit, `{"action":"insert","id":"`+d.ID+`","md":"Dos"}`, "Document "+d.ID+" · Pla · v2\n")
	s.emitDocChanged(emit, `{"action":"read","id":"`+d.ID+`"}`, "Document "+d.ID+" · Pla · v2\n")
	s.emitDocChanged(emit, `{"action":"replace","id":"`+d.ID+`"}`, "ERROR: there is no block with that id")
	if len(got) != 2 || got[0]["id"] != d.ID || got[0]["title"] != "Pla" || got[1]["id"] != d.ID {
		t.Fatalf("%v", got)
	}
}
