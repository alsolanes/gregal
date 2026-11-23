package web

import (
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
