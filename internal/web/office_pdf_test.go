package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Un PDF puja pel mateix camí que els documents d'Office: la resposta en
// dona la ruta (perquè la UI l'adjunti com a @ruta), es llegeix com a text
// i accepta les edicions de pdf_edit que no creen fitxers nous.
func TestOfficeAPIPDF(t *testing.T) {
	s := goalTestServer(t)
	raw, err := os.ReadFile(filepath.Join("..", "pdfdoc", "testdata", "text3.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	call := func(h func(*Server, *httptest.ResponseRecorder, *bytes.Reader), body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		h(s, w, bytes.NewReader(b))
		return w
	}
	upload := func(s *Server, w *httptest.ResponseRecorder, r *bytes.Reader) {
		s.handleOfficeUpload(w, httptest.NewRequest("POST", "/api/office/upload", r))
	}
	read := func(s *Server, w *httptest.ResponseRecorder, r *bytes.Reader) {
		s.handleOfficeRead(w, httptest.NewRequest("POST", "/api/office/read", r))
	}
	edit := func(s *Server, w *httptest.ResponseRecorder, r *bytes.Reader) {
		s.handleOfficeEdit(w, httptest.NewRequest("POST", "/api/office/edit", r))
	}

	w := call(upload, map[string]string{"name": "Informe anual.pdf", "data": base64.StdEncoding.EncodeToString(raw)})
	if w.Code != 200 {
		t.Fatalf("upload %d: %s", w.Code, w.Body.String())
	}
	var up struct{ ID, Kind, Path, Name string }
	json.Unmarshal(w.Body.Bytes(), &up)
	if up.Kind != "pdf" || up.ID == "" || !strings.HasSuffix(up.Path, ".pdf") || strings.Contains(up.Path, " ") {
		t.Fatalf("upload resposta: %+v", up)
	}

	w = call(read, map[string]string{"id": up.ID})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "giraffe") || !strings.Contains(w.Body.String(), `"kind":"pdf"`) {
		t.Fatalf("read %d: %s", w.Code, w.Body.String())
	}

	w = call(edit, map[string]any{"id": up.ID, "op": "stamp", "text": "REBUT", "pages": "1"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "REBUT") {
		t.Fatalf("edit stamp %d: %s", w.Code, w.Body.String())
	}
	w = call(edit, map[string]any{"id": up.ID, "op": "merge"})
	if w.Code != 400 {
		t.Fatalf("merge per API hauria de ser 400: %d %s", w.Code, w.Body.String())
	}

	// La menció @ruta del xat adjunta el text del PDF, no els bytes.
	got := s.expandMentions("resumeix @"+up.Path, "")
	if !strings.Contains(got, "PDF de 3 pàgines") || !strings.Contains(got, "pelican") || strings.Contains(got, "%PDF") {
		t.Fatalf("menció: %q", got)
	}

	// Un fitxer que diu ser PDF i no ho és no es desa.
	w = call(upload, map[string]string{"name": "fals.pdf", "data": base64.StdEncoding.EncodeToString([]byte("hola"))})
	if w.Code != 400 {
		t.Fatalf("PDF fals: %d %s", w.Code, w.Body.String())
	}
}
