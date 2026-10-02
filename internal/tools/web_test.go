package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchTextNet(t *testing.T) {
	allowLocalFetch = true
	defer func() { allowLocalFetch = false }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Hola</title></head><body><script>var x=1;</script><h1>Títol</h1><p>Text amb <b>negreta</b>.</p></body></html>`))
	}))
	defer srv.Close()
	out, err := WebFetch(srv.URL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Títol") || !strings.Contains(out, "negreta") {
		t.Fatalf("text net incomplet: %q", out)
	}
	if strings.Contains(out, "<") || strings.Contains(out, "var x=1") {
		t.Fatalf("HTML no netejat: %q", out)
	}
}

func TestWebFetchBlocaLocalhost(t *testing.T) {
	allowLocalFetch = false
	if _, err := WebFetch("http://127.0.0.1:4000/search?q=x", 0); err == nil {
		t.Fatal("cal bloquejar 127.0.0.1")
	}
	if _, err := WebFetch("http://localhost:8097/api/state", 0); err == nil {
		t.Fatal("cal bloquejar localhost")
	}
	if _, err := WebFetch("http://169.254.169.254/latest/meta-data/", 0); err == nil {
		t.Fatal("cal bloquejar metadades cloud")
	}
	if _, err := WebFetch("ftp://example.com/x", 0); err == nil {
		t.Fatal("cal rebutjar esquema ftp")
	}
}

func TestWebFetchRebutjaNoTextual(t *testing.T) {
	allowLocalFetch = true
	defer func() { allowLocalFetch = false }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{1, 2, 3})
	}))
	defer srv.Close()
	if _, err := WebFetch(srv.URL, 0); err == nil {
		t.Fatal("cal rebutjar imatges")
	}
}

func TestWebSearchMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("format=%q", r.URL.Query().Get("format"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"title":"T1","url":"https://example.com/1","content":"resum u"},{"title":"","url":"https://example.com/2","content":""}]}`))
	}))
	defer srv.Close()
	t.Setenv("SEARXNG_URL", srv.URL)
	nomesSearXNG = true
	defer func() { nomesSearXNG = false }()
	WebCacheClear()
	out, err := WebSearch("prova", 8)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "T1") || !strings.Contains(out, "https://example.com/1") ||
		!strings.Contains(out, "resum u") || !strings.Contains(out, "https://example.com/2") {
		t.Fatalf("cerca incompleta: %q", out)
	}
	if _, err := WebSearch("  ", 8); err == nil {
		t.Fatal("query buida ha de fallar")
	}
}

func TestHtmlToText(t *testing.T) {
	out := htmlToText(`<style>a{}</style><!-- c --><p>Hola <b>món</b></p><br>fi &amp; pau`)
	if strings.Contains(out, "a{}") || strings.Contains(out, "<") {
		t.Fatalf("restes HTML: %q", out)
	}
	if !strings.Contains(out, "Hola món") || !strings.Contains(out, "fi & pau") {
		t.Fatalf("text perdut: %q", out)
	}
}

func TestCadenaFirecrawlQuanDirecteFalla(t *testing.T) {
	allowLocalFetch = true
	defer func() { allowLocalFetch = false }()
	fc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"markdown":"# Article real\n\nContingut llegible.","title":"Article"}}`))
	}))
	defer fc.Close()
	t.Setenv("FIRECRAWL_URL", fc.URL)
	// Port tancat: el directe falla i ha de guanyar el render.
	out, err := WebFetch("http://127.0.0.1:1/no-hi-es", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "navegador local") || !strings.Contains(out, "Article real") {
		t.Fatalf("cadena inesperada: %.150s", out)
	}
}

func TestCadenaWaybackQuanRenderFalla(t *testing.T) {
	allowLocalFetch = true
	defer func() { allowLocalFetch = false }()
	var snap *httptest.Server
	snap = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/wayback/") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"archived_snapshots":{"closest":{"url":"` + snap.URL + `/web/20260102/https://example.com/a","timestamp":"20260102120000"}}}`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Vell</title></head><body><p>Text arxivat antic.</p></body></html>`))
	}))
	defer snap.Close()
	oldBase := waybackAPIBase
	waybackAPIBase = snap.URL
	defer func() { waybackAPIBase = oldBase }()
	t.Setenv("FIRECRAWL_URL", "http://127.0.0.1:1/")
	out, when, err := webFetchWayback("https://example.com/a")
	if err != nil {
		t.Fatal(err)
	}
	if when != "2026-01-02" || !strings.Contains(out, "Text arxivat") {
		t.Fatalf("wayback inesperat (%s): %.120s", when, out)
	}
}

func TestIntersticialsIDeteccio(t *testing.T) {
	if !isInterstitialTitle("Just a moment...") || !isInterstitialTitle("406 WAF Forbidden") {
		t.Fatal("hauria de detectar Cloudflare/WAF")
	}
	if isInterstitialTitle("Diari ARA | Notícies") {
		t.Fatal("fals positiu amb titular normal")
	}
	if htmlTitle("<html><head><title>  Hola &amp; adéu </title></head></html>") != "Hola & adéu" {
		t.Fatal("títol mal extret")
	}
	// El 429 d'archive.today amb cos gros: èxit fals, cal rebutjar-lo.
	fake := "<html><head><title>archive.ph</title></head><body>" + strings.Repeat("x", 60000) + "Too Many Requests, retry later</body></html>"
	if archiveBodyOK(fake) {
		t.Fatal("el 429 amb cos gros hauria de ser rebutjat")
	}
	good := "<html><head><title>Article bo</title></head><body><p>" + strings.Repeat("text llegible ", 400) + "</p></body></html>"
	if !archiveBodyOK(good) {
		t.Fatal("una còpia bona hauria de passar")
	}
}
