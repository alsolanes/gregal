package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Mode local (sense VM ni noVNC): /api/ordinador/live dona una captura del
// navegador de Gregal en aquest servidor, o live:false si no n'hi ha cap.
func TestOrdinadorLiveLocalMode(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	old := liveShot
	t.Cleanup(func() { liveShot = old })

	liveShot = func() ([]byte, string, string, time.Time, bool) { return nil, "", "", time.Time{}, false }
	w := httptest.NewRecorder()
	s.handleOrdinadorLive(w, httptest.NewRequest("GET", "/api/ordinador/live", nil))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 200 || out["live"] != false || out["mode"] != "local" {
		t.Fatalf("sense navegador: %d %v", w.Code, out)
	}

	liveShot = func() ([]byte, string, string, time.Time, bool) {
		return []byte{0xff, 0xd8, 0xff}, "https://renfe.example/bitllets", "Bitllets", time.Now(), true
	}
	w = httptest.NewRecorder()
	s.handleOrdinadorLive(w, httptest.NewRequest("GET", "/api/ordinador/live", nil))
	out = nil
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["live"] != true || out["url"] != "https://renfe.example/bitllets" || !strings.HasPrefix(out["image"].(string), "data:image/jpeg;base64,") {
		t.Fatalf("amb navegador: %v", out)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("la captura no s'ha de desar a cap memòria cau")
	}
	w = httptest.NewRecorder()
	s.handleOrdinadorLive(w, httptest.NewRequest("POST", "/api/ordinador/live", nil))
	if w.Code != 405 {
		t.Fatalf("només GET: %d", w.Code)
	}
}
