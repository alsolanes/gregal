package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/tools"
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
	s.handleOrdinadorLive(w, httptest.NewRequest("PUT", "/api/ordinador/live", nil))
	if w.Code != 405 {
		t.Fatalf("només GET i POST: %d", w.Code)
	}
}

// Des del mòbil o la web es fa servir: un toc pren el control (Gregal
// s'espera) i torna la captura de després. El navegador d'un altre
// propietari ni es veu ni es toca.
func TestOrdinadorLiveInputTakesControl(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	oldShot, oldAct, oldOwner, oldSettle := liveShot, liveAct, liveOwner, liveSettle
	t.Cleanup(func() { liveShot, liveAct, liveOwner, liveSettle = oldShot, oldAct, oldOwner, oldSettle })
	liveSettle = 0
	liveShot = func() ([]byte, string, string, time.Time, bool) {
		return []byte{0xff, 0xd8, 0xff}, "https://example.org/", "Example", time.Now(), true
	}
	var got []tools.LiveInput
	liveAct = func(in tools.LiveInput) error { got = append(got, in); return nil }
	liveOwner = func() string { return tools.OrdinadorLocal }
	key := tools.OwnerKeyFor("")
	t.Cleanup(func() { s.ordinadorLeases().Release(key) })

	w := httptest.NewRecorder()
	s.handleOrdinadorLive(w, httptest.NewRequest("POST", "/api/ordinador/live", strings.NewReader(`{"kind":"tap","x":0.5,"y":0.25}`)))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 200 || out["live"] != true || out["controlOwner"] != "user" {
		t.Fatalf("toc: %d %v", w.Code, out)
	}
	if len(got) != 1 || got[0].Kind != "tap" || got[0].X != 0.5 || got[0].Y != 0.25 {
		t.Fatalf("acció: %+v", got)
	}
	if !tools.BrowserControlHeld("") {
		t.Fatal("amb el control de la persona, l'agent s'ha d'esperar")
	}

	// Un altre propietari: ni captura ni acció.
	liveOwner = func() string { return "un-altre" }
	w = httptest.NewRecorder()
	s.handleOrdinadorLive(w, httptest.NewRequest("POST", "/api/ordinador/live", strings.NewReader(`{"kind":"type","text":"hola"}`)))
	out = nil
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["live"] != false || out["image"] != nil || len(got) != 1 {
		t.Fatalf("navegador d'un altre: %v %+v", out, got)
	}

	// Sense navegador obert: 409 i el missatge.
	liveOwner = func() string { return tools.OrdinadorLocal }
	liveAct = func(tools.LiveInput) error { return tools.ErrNoLiveBrowser }
	w = httptest.NewRecorder()
	s.handleOrdinadorLive(w, httptest.NewRequest("POST", "/api/ordinador/live", strings.NewReader(`{"kind":"back"}`)))
	if w.Code != 409 {
		t.Fatalf("sense navegador: %d", w.Code)
	}
}
