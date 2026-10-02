package web

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func testOfficeDocxBytes() []byte {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, body := range map[string]string{
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>hola api</w:t></w:r></w:p></w:body></w:document>`,
	} {
		fw, _ := zw.Create(name)
		fw.Write([]byte(body))
	}
	zw.Close()
	return b.Bytes()
}

func TestOfficeRoundTrip(t *testing.T) {
	s := goalTestServer(t)

	// Upload.
	upBody, _ := json.Marshal(map[string]string{
		"name": "prova.docx",
		"data": base64.StdEncoding.EncodeToString(testOfficeDocxBytes()),
	})
	req := httptest.NewRequest("POST", "/api/office/upload", bytes.NewReader(upBody))
	w := httptest.NewRecorder()
	s.handleOfficeUpload(w, req)
	if w.Code != 200 {
		t.Fatalf("upload %d: %s", w.Code, w.Body.String())
	}
	var up map[string]any
	json.Unmarshal(w.Body.Bytes(), &up)
	id, _ := up["id"].(string)
	if id == "" || up["kind"] != "docx" {
		t.Fatalf("upload resposta: %v", up)
	}

	// Read.
	rdBody, _ := json.Marshal(map[string]string{"id": id})
	req = httptest.NewRequest("POST", "/api/office/read", bytes.NewReader(rdBody))
	w = httptest.NewRecorder()
	s.handleOfficeRead(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "hola api") {
		t.Fatalf("read %d: %s", w.Code, w.Body.String())
	}

	// Edit (replace).
	edBody, _ := json.Marshal(map[string]string{"id": id, "op": "replace", "find": "hola", "replace": "adeu"})
	req = httptest.NewRequest("POST", "/api/office/edit", bytes.NewReader(edBody))
	w = httptest.NewRecorder()
	s.handleOfficeEdit(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "adeu") {
		t.Fatalf("edit %d: %s", w.Code, w.Body.String())
	}

	// Download: ha de ser un zip que conté el text nou.
	req = httptest.NewRequest("GET", "/api/office/download?id="+id, nil)
	w = httptest.NewRecorder()
	s.handleOfficeDownload(w, req)
	if w.Code != 200 {
		t.Fatalf("download %d", w.Code)
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("download no és zip: %v", err)
	}
	found := false
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			raw, _ := io.ReadAll(rc)
			rc.Close()
			found = strings.Contains(string(raw), "adeu")
		}
	}
	if !found {
		t.Fatal("el download no porta el text editat")
	}

	// Rebutjos: extensió dolenta i id inexistent.
	bad, _ := json.Marshal(map[string]string{"name": "f.pdf", "data": "eA=="})
	req = httptest.NewRequest("POST", "/api/office/upload", bytes.NewReader(bad))
	w = httptest.NewRecorder()
	s.handleOfficeUpload(w, req)
	if w.Code != 400 {
		t.Fatalf("pdf hauria de ser 400, és %d", w.Code)
	}
	req = httptest.NewRequest("POST", "/api/office/read", bytes.NewReader([]byte(`{"id":"noexisteix"}`)))
	w = httptest.NewRecorder()
	s.handleOfficeRead(w, req)
	if w.Code != 404 {
		t.Fatalf("id fals hauria de ser 404, és %d", w.Code)
	}
}

func TestOfficeEditGuards(t *testing.T) {
	s := goalTestServer(t)
	upBody, _ := json.Marshal(map[string]string{
		"name": "prova.docx",
		"data": base64.StdEncoding.EncodeToString(testOfficeDocxBytes()),
	})
	req := httptest.NewRequest("POST", "/api/office/upload", bytes.NewReader(upBody))
	w := httptest.NewRecorder()
	s.handleOfficeUpload(w, req)
	var up map[string]any
	json.Unmarshal(w.Body.Bytes(), &up)
	id := up["id"].(string)

	// set_cell en docx → 400.
	ed, _ := json.Marshal(map[string]string{"id": id, "op": "set_cell", "sheet": "F", "cell": "A1", "value": "x"})
	req = httptest.NewRequest("POST", "/api/office/edit", bytes.NewReader(ed))
	w = httptest.NewRecorder()
	s.handleOfficeEdit(w, req)
	if w.Code != 400 {
		t.Fatalf("set_cell en docx hauria de ser 400, és %d", w.Code)
	}
}
