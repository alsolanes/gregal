package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebImages(t *testing.T) {
	allowLocalFetch = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("categories") != "images" {
			t.Errorf("categories = %q, volia images", r.URL.Query().Get("categories"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"title":"Foto cistella","url":"https://pagina.com","img_src":"https://img.com/a.jpg"},
			{"title":"Sense imatge","url":"https://x.com","img_src":""}
		]}`))
	}))
	defer srv.Close()
	t.Setenv("SEARXNG_URL", srv.URL)
	out, err := WebImages(context.Background(), "cistella basquet", 6)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://img.com/a.jpg") || !strings.Contains(out, "Foto cistella") {
		t.Errorf("resposta sense la imatge:\n%s", out)
	}
	if strings.Contains(out, "Sense imatge") {
		t.Errorf("resultats sense img_src no hi han de sortir:\n%s", out)
	}
}

// TestWebImagesCommonsFallback: sense SearXNG, les imatges surten de Wikimedia
// Commons, en ordre de rellevància, sense SVG i amb la miniatura lleugera.
func TestWebImagesCommonsFallback(t *testing.T) {
	allowLocalFetch = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("gsrnamespace") != "6" || !strings.Contains(r.URL.Query().Get("gsrsearch"), "Camp Nou") {
			t.Errorf("consulta inesperada: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":{"pages":{
			"9":{"title":"File:Camp_Nou_obres_2025.jpg","index":2,"imageinfo":[{"url":"https://upload.wikimedia.org/a.jpg","thumburl":"https://upload.wikimedia.org/thumb/a.jpg/960px-a.jpg","descriptionurl":"https://commons.wikimedia.org/wiki/File:Camp_Nou_obres_2025.jpg","mime":"image/jpeg"}]},
			"7":{"title":"File:Camp Nou logo.svg","index":1,"imageinfo":[{"url":"https://upload.wikimedia.org/logo.svg","descriptionurl":"https://commons.wikimedia.org/wiki/File:Camp_Nou_logo.svg","mime":"image/svg+xml"}]},
			"5":{"title":"File:Camp Nou des de l'aire.png","index":3,"imageinfo":[{"url":"https://upload.wikimedia.org/b.png","descriptionurl":"https://commons.wikimedia.org/wiki/File:b.png","mime":"image/png"}]}
		}}}`))
	}))
	defer srv.Close()
	t.Setenv("SEARXNG_URL", "")
	saved, savedLead := commonsAPI, wikiLeadAPI
	commonsAPI, wikiLeadAPI = srv.URL, ""
	t.Cleanup(func() { commonsAPI, wikiLeadAPI = saved, savedLead })
	out, err := WebImages(context.Background(), "Camp Nou", 6)
	if err != nil {
		t.Fatal(err)
	}
	first, second := strings.Index(out, "960px-a.jpg"), strings.Index(out, "b.png")
	if first < 0 || second < 0 || first > second || strings.Contains(out, "logo.svg") {
		t.Fatalf("ordre, miniatura o filtre SVG malament:\n%s", out)
	}
	if !strings.Contains(out, "1. Camp Nou obres 2025") || !strings.Contains(out, "page: https://commons.wikimedia.org/wiki/File:Camp_Nou_obres_2025.jpg") {
		t.Fatalf("títol o pàgina malament:\n%s", out)
	}
}

func TestWebImagesPutsTheArticlePhotoFirst(t *testing.T) {
	allowLocalFetch = true
	commons := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"query":{"pages":{"1":{"title":"File:Home dormint.jpg","index":1,"imageinfo":[{"thumburl":"https://upload.wikimedia.org/dorm.jpg","descriptionurl":"https://commons.wikimedia.org/wiki/File:d.jpg","mime":"image/jpeg"}]}}}}`))
	}))
	defer commons.Close()
	wiki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("prop") != "pageimages" || r.URL.Query().Get("gsrsearch") != "La plaça del Diamant" {
			t.Errorf("consulta inesperada: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"query":{"pages":{"42":{"title":"The Time of the Doves","thumbnail":{"source":"https://upload.wikimedia.org/coberta.jpg"}}}}}`))
	}))
	defer wiki.Close()
	t.Setenv("SEARXNG_URL", "")
	saved, savedLead := commonsAPI, wikiLeadAPI
	commonsAPI, wikiLeadAPI = commons.URL, wiki.URL+"/w/api.php"
	t.Cleanup(func() { commonsAPI, wikiLeadAPI = saved, savedLead })
	out, err := WebImages(context.Background(), "La plaça del Diamant", 6)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "1. The Time of the Doves\n   image: https://upload.wikimedia.org/coberta.jpg") || !strings.Contains(out, "2. Home dormint") {
		t.Fatalf("la foto de l'article ha d'anar primer:\n%s", out)
	}
}
