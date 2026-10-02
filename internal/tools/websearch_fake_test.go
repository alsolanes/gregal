package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newFakeSearch serveix un SearXNG (/searx/search) i un DuckDuckGo HTML
// (/ddg) falsos amb un resultat comú i un de propi cadascun.
func newFakeSearch(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/searx/"):
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[{"title":"Comú","url":"https://x.cat/comu","content":"c"},{"title":"Només searx","url":"https://x.cat/nomes-searx","content":"s"}]}`))
		case strings.HasPrefix(r.URL.Path, "/ddg"):
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><a class="result__a" href="https://x.cat/comu">Comú</a><div class="result__snippet">c</div><a class="result__a" href="https://x.cat/nomes-ddg">Només ddg</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
}
