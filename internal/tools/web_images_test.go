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
