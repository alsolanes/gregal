package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShellAssetsHaveBrowserUsableContentTypes(t *testing.T) {
	s := goalTestServer(t)
	mux := s.Hub().mux()
	for _, asset := range []struct{ path, contentType string }{
		{"/app/polish.css", "text/css"},
		{"/app/workshop.css", "text/css"},
		{"/app/workshop.js", "text/javascript"},
		{"/app/shell-icons.js", "text/javascript"},
		{"/app/desktop-prefs.js", "text/javascript"},
		{"/app/desktop-updates.js", "text/javascript"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, asset.path, nil))
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), asset.contentType) || rec.Body.Len() == 0 {
			t.Fatalf("%s: status=%d type=%q bytes=%d", asset.path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
	}
}
