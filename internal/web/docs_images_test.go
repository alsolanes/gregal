package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/gdocs"
	"gregal/internal/ooxml"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// docImageReq puja body a /api/docs/{id}/images o, amb name, en demana una.
func docImageReq(s *Server, method, target, id, name string, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.SetPathValue("id", id)
	if name != "" {
		req.SetPathValue("name", name)
		s.handleDocImage(rec, req)
	} else {
		s.handleDocImages(rec, req)
	}
	return rec
}

func uploadImage(t *testing.T, s *Server, id, scope string) string {
	t.Helper()
	rec := docImageReq(s, http.MethodPost, "/api/docs/"+id+"/images"+scope, id, "", pngBytes(t))
	var out struct{ Path string }
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil || !strings.HasPrefix(out.Path, "images/") {
		t.Fatalf("pujar: %d %s", rec.Code, rec.Body.String())
	}
	return out.Path
}

func TestDocImageUploadServeAndRefuse(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	d, err := st.Create("Pla", "Text", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	p := uploadImage(t, s, d.ID, "")
	if !strings.HasSuffix(p, ".png") {
		t.Fatalf("ruta: %s", p)
	}
	name := strings.TrimPrefix(p, "images/")
	rec := docImageReq(s, http.MethodGet, "/api/docs/"+d.ID+"/images/"+name, d.ID, name, nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || !bytes.Equal(rec.Body.Bytes(), pngBytes(t)) {
		t.Fatalf("servir: %d %v", rec.Code, rec.Header())
	}

	// Només imatges, i no gaire grosses.
	if rec := docImageReq(s, http.MethodPost, "/api/docs/"+d.ID+"/images", d.ID, "", []byte("<svg onload=alert(1)>")); rec.Code != http.StatusBadRequest {
		t.Fatalf("no és una imatge: %d %s", rec.Code, rec.Body.String())
	}
	big := append(pngBytes(t), make([]byte, maxDocImage)...)
	if rec := docImageReq(s, http.MethodPost, "/api/docs/"+d.ID+"/images", d.ID, "", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("massa gran: %d %s", rec.Code, rec.Body.String())
	}
	for _, bad := range []string{"..", "../" + d.ID + ".json", `..\x.png`, "nohies.png"} {
		if rec := docImageReq(s, http.MethodGet, "/api/docs/"+d.ID+"/images/x", d.ID, bad, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%q: %d", bad, rec.Code)
		}
	}

	// Un document d'un altre projecte no es troba des d'aquest, ni per pujar ni per llegir.
	s.cwd = t.TempDir()
	if rec := docImageReq(s, http.MethodPost, "/api/docs/"+d.ID+"/images", d.ID, "", pngBytes(t)); rec.Code != http.StatusNotFound {
		t.Fatalf("pujar a un altre projecte: %d", rec.Code)
	}
	if rec := docImageReq(s, http.MethodGet, "/api/docs/"+d.ID+"/images/"+name, d.ID, name, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("llegir d'un altre projecte: %d", rec.Code)
	}

	// Amb ?scope=mine, la carpeta de la persona.
	mine, _ := gdocs.OpenAt(s.windDocsDir())
	m, err := mine.Create("Viatge", "", gdocs.AuthorUser)
	if err != nil {
		t.Fatal(err)
	}
	uploadImage(t, s, m.ID, "?scope=mine")
	if rec := docImageReq(s, http.MethodPost, "/api/docs/"+m.ID+"/images", m.ID, "", pngBytes(t)); rec.Code != http.StatusNotFound {
		t.Fatalf("sense scope: %d", rec.Code)
	}
}

func mediaParts(t *testing.T, rec *httptest.ResponseRecorder, prefix string) int {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	pk, err := ooxml.OpenBytes(rec.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range pk.Names() {
		if strings.HasPrefix(p, prefix) {
			n++
		}
	}
	return n
}

// Les imatges d'un document només surten de la seva carpeta images: ni
// rutes absolutes, ni amb .., ni fitxers del projecte (el seguiment de 072).
func TestDocExportTakesImagesOnlyFromTheDocsFolder(t *testing.T) {
	s := backlogServer(t)
	st, _ := gdocs.Open(s.cwd)
	outside := filepath.Join(s.cwd, "secret.png")
	if err := os.WriteFile(outside, pngBytes(t), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := st.Create("Pla", "Text", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	p := uploadImage(t, s, d.ID, "")
	rel, err := filepath.Rel(filepath.Join(st.Dir, d.ID, "images"), outside)
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{outside, filepath.ToSlash(rel), "images/" + filepath.ToSlash(rel), "secret.png"} {
		if _, err := st.Insert(d.ID, "", "![fora]("+src+")", gdocs.AuthorAgent); err != nil {
			t.Fatal(err)
		}
	}
	if n := mediaParts(t, docExport(s, d.ID), "word/media/image"); n != 0 {
		t.Fatalf("%d imatges de fora de la carpeta del document al Word", n)
	}
	if _, err := st.Insert(d.ID, "", "![La foto]("+p+")", gdocs.AuthorUser); err != nil {
		t.Fatal(err)
	}
	if n := mediaParts(t, docExport(s, d.ID), "word/media/image"); n != 1 {
		t.Fatalf("la imatge del document: %d al Word", n)
	}

	deck, err := st.CreateDeck("Llançament")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{outside, "secret.png", filepath.ToSlash(rel)} {
		if _, err := st.InsertSlide(deck.ID, "", ooxml.SlideSpec{Layout: "image", Title: "Fora", Image: src}, gdocs.AuthorAgent); err != nil {
			t.Fatal(err)
		}
	}
	if n := mediaParts(t, docExportAs(s, deck.ID, "pptx"), "ppt/media/image"); n != 0 {
		t.Fatalf("%d imatges de fora de la carpeta de la presentació al PowerPoint", n)
	}
	q := uploadImage(t, s, deck.ID, "")
	if _, err := st.InsertSlide(deck.ID, "", ooxml.SlideSpec{Layout: "image", Title: "Dins", Image: q}, gdocs.AuthorAgent); err != nil {
		t.Fatal(err)
	}
	if n := mediaParts(t, docExportAs(s, deck.ID, "pptx"), "ppt/media/image"); n != 1 {
		t.Fatalf("la imatge de la presentació: %d al PowerPoint", n)
	}
}
