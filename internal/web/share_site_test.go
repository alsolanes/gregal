package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type siteResp struct {
	ID, Path, URL, Title string
	Public               bool
	Files, Bytes         int
	Expires              *time.Time
}

func publishSite(t *testing.T, h http.Handler, body string, want int) siteResp {
	t.Helper()
	rec := exportDo(h, "POST", "/api/shares/site", body)
	if rec.Code != want {
		t.Fatalf("publish %s: %d %s", body, rec.Code, rec.Body)
	}
	var r siteResp
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	return r
}

// Publicar, servir sense token i aïllat, republicar al mateix enllaç i
// despublicar.
func TestSitePublicarRepublicarDespublicar(t *testing.T) {
	_, h := exportTestServer(t)
	t.Setenv("GREGAL_PUBLIC_URL", "")
	r := publishSite(t, h, `{"kind":"html","content":"<h1>Forn v1</h1>","title":"Forn"}`, 200)
	if len(r.ID) != 40 || r.Path != "/share/"+r.ID+"/" || r.URL != "" || r.Public || r.Files != 1 {
		t.Fatalf("resposta: %+v", r)
	}
	page := exportDo(h, "GET", r.Path, "")
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Forn v1") {
		t.Fatalf("pàgina: %d %s", page.Code, page.Body)
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.HasPrefix(csp, "sandbox allow-scripts") || strings.Contains(csp, "allow-same-origin") {
		t.Fatalf("la web ha de córrer en un origen opac: %q", csp)
	}
	if ct := page.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type: %q", ct)
	}
	if rec := exportDo(h, "GET", "/share/"+r.ID, ""); rec.Code != http.StatusFound || rec.Header().Get("Location") != r.Path {
		t.Fatalf("sense barra: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// Republicar: mateix enllaç, contingut nou.
	r2 := publishSite(t, h, `{"id":"`+r.ID+`","kind":"html","content":"<h1>Forn v2</h1>","title":"Forn"}`, 200)
	if r2.ID != r.ID {
		t.Fatalf("republicar ha de mantenir l'id: %s", r2.ID)
	}
	if page := exportDo(h, "GET", r.Path, ""); !strings.Contains(page.Body.String(), "Forn v2") {
		t.Fatalf("republicat: %s", page.Body)
	}
	if entries, _ := filepath.Glob(filepath.Join(sharesDir(), r.ID+".*-*")); len(entries) != 0 {
		t.Fatalf("restes de la republicació: %v", entries)
	}
	// A la llista d'enllaços, com les converses.
	list := exportDo(h, "GET", "/api/shares", "")
	if !strings.Contains(list.Body.String(), `"kind":"site"`) || !strings.Contains(list.Body.String(), `"url":"`+r.Path+`"`) {
		t.Fatalf("llista: %s", list.Body)
	}
	if rec := exportDo(h, "DELETE", "/api/shares/"+r.ID, ""); rec.Code != 200 {
		t.Fatalf("despublicar: %d", rec.Code)
	}
	if rec := exportDo(h, "GET", r.Path, ""); rec.Code != 404 {
		t.Fatalf("despublicada: %d", rec.Code)
	}
	if _, err := os.Stat(siteDir(r.ID)); !os.IsNotExist(err) {
		t.Fatal("els fitxers s'han d'esborrar")
	}
}

// Només el propietari pot republicar o despublicar.
func TestSiteNomesElPropietari(t *testing.T) {
	_, h := exportTestServer(t)
	r := publishSite(t, h, `{"kind":"html","content":"<p>x</p>"}`, 200)
	m, _ := readShareMeta(r.ID)
	m.Owner = "una-altra"
	raw, _ := json.Marshal(m)
	_ = os.WriteFile(filepath.Join(sharesDir(), r.ID+".json"), raw, 0o600)
	publishSite(t, h, `{"id":"`+r.ID+`","kind":"html","content":"<p>meu</p>"}`, 404)
	if rec := exportDo(h, "DELETE", "/api/shares/"+r.ID, ""); rec.Code != 404 {
		t.Fatalf("despublicar d'una altra: %d", rec.Code)
	}
	if page := exportDo(h, "GET", r.Path, ""); !strings.Contains(page.Body.String(), "<p>x</p>") {
		t.Fatal("no s'ha de tocar")
	}
}

// Una carpeta del workspace: còpia, sense fitxers ocults, i els canvis
// posteriors no arriben a l'enllaç.
func TestSiteCarpetaEsUnaInstantania(t *testing.T) {
	s, h := exportTestServer(t)
	ws := t.TempDir()
	s.mu.Lock()
	s.cwd = ws
	s.mu.Unlock()
	site := filepath.Join(ws, "web")
	_ = os.MkdirAll(filepath.Join(site, "css"), 0o755)
	_ = os.MkdirAll(filepath.Join(site, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(site, "index.html"), []byte(`<link rel="stylesheet" href="css/a.css"><h1>v1</h1>`), 0o644)
	_ = os.WriteFile(filepath.Join(site, "css", "a.css"), []byte("h1{color:red}"), 0o644)
	_ = os.WriteFile(filepath.Join(site, ".env"), []byte("SECRET=1"), 0o644)
	_ = os.WriteFile(filepath.Join(site, ".git", "config"), []byte("x"), 0o644)
	r := publishSite(t, h, `{"dir":"web","title":"Web"}`, 200)
	if r.Files != 2 {
		t.Fatalf("fitxers: %d", r.Files)
	}
	if css := exportDo(h, "GET", r.Path+"css/a.css", ""); css.Code != 200 || !strings.HasPrefix(css.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("css: %d %q", css.Code, css.Header().Get("Content-Type"))
	}
	for _, p := range []string{".env", ".git/config", "../" + r.ID + ".json", "%2e%2e/" + r.ID + ".json", "css"} {
		if rec := exportDo(h, "GET", r.Path+p, ""); rec.Code == 200 {
			t.Fatalf("%s no s'ha de servir", p)
		}
	}
	_ = os.WriteFile(filepath.Join(site, "index.html"), []byte("<h1>v2</h1>"), 0o644)
	if page := exportDo(h, "GET", r.Path, ""); !strings.Contains(page.Body.String(), "v1") {
		t.Fatal("editar el projecte no ha de canviar la publicació")
	}
	publishSite(t, h, `{"dir":"web/css"}`, 400) // sense index.html
	publishSite(t, h, `{"dir":"../"}`, 400)
}

func TestSiteMarkdownCaducitatILimits(t *testing.T) {
	_, h := exportTestServer(t)
	t.Setenv("GREGAL_PUBLIC_URL", "https://vents.example.com/")
	r := publishSite(t, h, `{"kind":"markdown","content":"# Pla\n\n<script>x()</script>","title":"Pla","expires_days":7}`, 200)
	if !r.Public || r.URL != "https://vents.example.com"+r.Path || r.Expires == nil {
		t.Fatalf("resposta: %+v", r)
	}
	page := exportDo(h, "GET", r.Path, "")
	if !strings.Contains(page.Body.String(), "<h1>Pla</h1>") || strings.Contains(page.Body.String(), "<script>") {
		t.Fatalf("markdown: %s", page.Body)
	}
	// Caducada: 404 i fora del disc.
	m, _ := readShareMeta(r.ID)
	past := time.Now().Add(-time.Hour)
	m.Expires = &past
	raw, _ := json.Marshal(m)
	_ = os.WriteFile(filepath.Join(sharesDir(), r.ID+".json"), raw, 0o600)
	if rec := exportDo(h, "GET", r.Path, ""); rec.Code != 404 {
		t.Fatalf("caducada: %d", rec.Code)
	}
	if _, err := readShareMeta(r.ID); err == nil {
		t.Fatal("caducada s'ha d'esborrar")
	}
	publishSite(t, h, `{"kind":"html","content":"x","expires_days":400}`, 400)
	publishSite(t, h, `{"kind":"exe","content":"x"}`, 400)
	publishSite(t, h, `{"kind":"html","content":"  "}`, 400)
	publishSite(t, h, `{"id":"`+strings.Repeat("a", 40)+`","kind":"html","content":"x"}`, 404)
}
