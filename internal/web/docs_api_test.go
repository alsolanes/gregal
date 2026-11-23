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

func TestDocsListGetAndStream(t *testing.T) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", d.ID)
		s.handleDocStream(w, r)
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if _, err := st.Replace(d.ID, d.Blocks[1].ID, "Segon paràgraf", gdocs.AuthorUser); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(resp.Body)
	event, seen := "", false
	for !seen && sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		}
		if event == "block" && strings.HasPrefix(line, "data: ") {
			var c gdocs.Change
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &c); err != nil {
				t.Fatal(err)
			}
			if c.Op != "replace" || c.Version != 2 || c.Block.ID != d.Blocks[1].ID || c.Block.MD != "Segon paràgraf" || c.Author != gdocs.AuthorUser {
				t.Fatalf("event: %+v", c)
			}
			seen = true
		}
	}
	if !seen {
		t.Fatalf("cap event block: %v", sc.Err())
	}
	resp.Body.Close()
	srv.Close()

	// Un id d'un altre projecte no es troba des d'aquest.
	s.cwd = t.TempDir()
	for _, h := range []func(*Server, http.ResponseWriter, *http.Request){(*Server).handleDoc, (*Server).handleDocStream} {
		if rec := docGet(s, h, d.ID); rec.Code != http.StatusNotFound {
			t.Fatalf("un altre projecte: %d", rec.Code)
		}
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
