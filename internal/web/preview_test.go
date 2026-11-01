package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// La previsualització és el que fa que «serveix-me la web» funcioni de
// veritat: el servidor és el Gregal, no un `python3 -m http.server` que mor.

func previewServer(t *testing.T) (*Server, string) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	// Cada test té la seva carpeta de projectes: si no, s'hi barrejarien els
	// projectes de debò de la casa.
	t.Setenv("GREGAL_PROJECTES", t.TempDir())
	s, _ := companionTestServer(t)
	dir := t.TempDir()
	s.mu.Lock()
	s.cwd = dir
	s.mu.Unlock()
	return s, dir
}

func TestPreviewServeixLaWebDelVent(t *testing.T) {
	s, dir := previewServer(t)
	html := "<!doctype html><title>PIXEL QUEST</title><canvas id=game></canvas>"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", s.previewBase()+"index.html", nil)
	rec := httptest.NewRecorder()
	s.handlePreview(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != html {
		t.Fatalf("no serveix el fitxer: %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type %q", ct)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("una web en proves no es pot cachear: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestPreviewCalLaClauDelEnllac(t *testing.T) {
	s, dir := previewServer(t)
	os.WriteFile(filepath.Join(dir, "secret.html"), []byte("no"), 0o644)
	for _, u := range []string{"/p/", "/p/1234567890abcdef/index.html", "/p/" + s.previewSig()} {
		req := httptest.NewRequest("GET", u, nil)
		rec := httptest.NewRecorder()
		s.handlePreview(rec, req)
		if u == "/p/"+s.previewSig() {
			// La clau sola és l'índex: ha de llistar, no servir fitxers.
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), "secret.html") {
				t.Fatalf("l'índex no llista: %d", rec.Code)
			}
			continue
		}
		if rec.Code != 404 {
			t.Fatalf("%s hauria de ser 404, és %d", u, rec.Code)
		}
	}
}

func TestPreviewNoSurtDeLaCarpeta(t *testing.T) {
	s, dir := previewServer(t)
	os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("ok"), 0o644)
	base := s.previewBase()
	for _, rel := range []string{
		"../etc/passwd", "..%2Fetc%2Fpasswd", "/etc/passwd", "a/../../b",
		".git/config", ".env", "node_modules/x/index.js", ".gregal/out/x",
	} {
		req := httptest.NewRequest("GET", base+rel, nil)
		rec := httptest.NewRecorder()
		s.handlePreview(rec, req)
		if rec.Code == 200 {
			t.Fatalf("%q hauria d'estar fora: %d", rel, rec.Code)
		}
	}
}

func TestPreviewIndexLlistaCarpetesIFitxers(t *testing.T) {
	s, dir := previewServer(t)
	os.MkdirAll(filepath.Join(dir, "joc"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.html"), []byte("<p>a"), 0o644)
	os.WriteFile(filepath.Join(dir, ".amagat"), []byte("x"), 0o644)
	rec := httptest.NewRecorder()
	s.handlePreview(rec, httptest.NewRequest("GET", s.previewBase(), nil))
	body := rec.Body.String()
	for _, vull := range []string{"a.html", "joc/"} {
		if !strings.Contains(body, vull) {
			t.Fatalf("l'índex no ensenya %q: %s", vull, body)
		}
	}
	if strings.Contains(body, ".amagat") {
		t.Fatal("l'índex no ha d'ensenyar fitxers amagats")
	}
}

func TestPreviewInfoDiuQuinesWebsHiHa(t *testing.T) {
	s, dir := previewServer(t)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<p>joc"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# res"), 0o644)
	os.MkdirAll(filepath.Join(dir, "web"), 0o755)
	os.WriteFile(filepath.Join(dir, "web", "altra.html"), []byte("<p>x"), 0o644)

	rec := httptest.NewRecorder()
	s.handlePreviewInfo(rec, httptest.NewRequest("GET", "/api/preview", nil))
	var out struct {
		Root  string       `json:"root"`
		Base  string       `json:"base"`
		Webs  []previewWeb `json:"webs"`
		Index string       `json:"index"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v — %s", err, rec.Body.String())
	}
	if out.Root != dir || out.Base != s.previewBase() || out.Index != s.previewBase() {
		t.Fatalf("resposta estranya: %+v", out)
	}
	if len(out.Webs) != 2 {
		t.Fatalf("esperava 2 webs, %d: %+v", len(out.Webs), out.Webs)
	}
	if !strings.HasSuffix(out.Webs[0].Path, "index.html") || strings.Contains(out.Webs[0].Path, "web/") {
		t.Fatalf("index.html ha d'anar primer: %+v", out.Webs)
	}
}

// La clau canvia amb el token: un enllaç antic deixa de funcionar quan es
// canvia el token del servidor (i el token no apareix mai a la URL).
func TestPreviewClauDepenDelToken(t *testing.T) {
	s, _ := previewServer(t)
	abans := s.previewBase()
	s.Token = "token-nou-different"
	despres := s.previewBase()
	if abans == despres {
		t.Fatal("canviar el token ha de canviar la clau de previsualització")
	}
	if strings.Contains(despres, s.Token) {
		t.Fatal("el token no pot viatjar a la URL")
	}
	// I l'enllaç vell ja no obre res.
	rec := httptest.NewRecorder()
	s.handlePreview(rec, httptest.NewRequest("GET", abans, nil))
	if rec.Code != 404 {
		t.Fatalf("l'enllaç antic hauria de donar 404, %d", rec.Code)
	}
}

func TestPreviewNomesLectura(t *testing.T) {
	s, dir := previewServer(t)
	req := httptest.NewRequest("POST", s.previewBase()+"index.html", strings.NewReader("x"))
	rec := httptest.NewRecorder()
	s.handlePreview(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("escriure-hi ha de ser 405, %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		t.Fatal("no s'hi pot escriure res")
	}
}

// A Android la carpeta de dades és /data/user/0 (enllaç a /data/data): si el
// costat resolt es compara amb l'escrit, tot dona 404 al mòbil i 200 al
// servidor. Aquí es reprodueix amb un enllaç de debò.
func TestPreviewResolEnllacosDeLaCarpeta(t *testing.T) {
	real := t.TempDir()
	if err := os.MkdirAll(filepath.Join(real, "joc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "joc", "index.html"), []byte("<p>x"), 0o644); err != nil {
		t.Fatal(err)
	}
	enllac := filepath.Join(t.TempDir(), "enllac")
	if err := os.Symlink(real, enllac); err != nil {
		t.Skipf("sense enllaços: %v", err)
	}
	full, ok := previewResolve(enllac, "joc/index.html")
	if !ok {
		t.Fatal("una carpeta enllaçada ha de poder-se publicar")
	}
	if raw, err := os.ReadFile(full); err != nil || string(raw) != "<p>x" {
		t.Fatalf("no llegeix el fitxer: %q %v", raw, err)
	}
	// I el que és fora segueix essent fora.
	if _, ok := previewResolve(enllac, "../fora.txt"); ok {
		t.Fatal("no es pot sortir de la carpeta")
	}
}
