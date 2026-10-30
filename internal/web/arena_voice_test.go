package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestArenaVoiceHTTPEngineCachesAndServes(t *testing.T) {
	s := cornerTestServer(t)
	var calls int32
	var got map[string]any
	tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Path != "/v1/audio/speech" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "bad", 400)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &got)
		w.Write([]byte("ID3fake-mp3"))
	}))
	defer tts.Close()
	s.saveVoiceCfg(arenaVoiceCfg{Engine: "http", URL: tts.URL, Key: "k", Model: "chatterbox", Voices: map[string]string{"caster": "Bob"},
		Extra: json.RawMessage(`{"caster":{"exaggeration":0.9}}`)})
	if !s.voiceReady("ca") {
		t.Fatal("el motor http configurat ha d'estar a punt")
	}
	key := s.voiceLater("**Quina jugada** 🏈!", "caster", "ca")
	if key == "" {
		t.Fatal("clau buida")
	}
	for i := 0; i < 100; i++ {
		if p, _ := s.voicePath(key); p != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if p, ext := s.voicePath(key); p == "" || ext != ".mp3" {
		t.Fatalf("l'àudio no s'ha generat: %q %q", p, ext)
	}
	if got["input"] != "Quina jugada !" && got["input"] != "Quina jugada!" {
		t.Errorf("el text ha d'anar net de markdown i emojis: %q", got["input"])
	}
	if got["voice"] != "Bob" || got["exaggeration"] != 0.9 || !strings.Contains(got["instructions"].(string), "sports") {
		t.Errorf("petició: %+v", got)
	}
	// La mateixa frase no es torna a generar.
	if k2 := s.voiceLater("**Quina jugada** 🏈!", "caster", "ca"); k2 != key || atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("la frase havia de sortir de la memòria cau: %d crides", calls)
	}
	// Es serveix, també a un espectador amb l'enllaç.
	s.Token = "secret"
	spect := s.arenaSpectatorKey(false)
	req := httptest.NewRequest("GET", "/api/arena/voice?k="+key, nil)
	req.Header.Set("X-Arena-Spectator", spect)
	rec := httptest.NewRecorder()
	s.Hub().Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/mpeg" || !bytes.HasPrefix(rec.Body.Bytes(), []byte("ID3")) {
		t.Fatalf("servir l'àudio: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	// L'estat no ensenya mai la clau del servidor de veu.
	st := httptest.NewRecorder()
	s.handleArenaVoice(st, httptest.NewRequest("GET", "/api/arena/voice", nil))
	if strings.Contains(st.Body.String(), `"key":"k"`) {
		t.Fatalf("la clau no pot sortir: %s", st.Body.String())
	}
	// Una clau de fitxer estranya no surt del directori.
	bad := httptest.NewRecorder()
	s.handleArenaVoice(bad, httptest.NewRequest("GET", "/api/arena/voice?k=../../settings", nil))
	if bad.Code != 404 {
		t.Fatalf("clau dolenta: %d", bad.Code)
	}
}

func TestExtractArchiveStaysInside(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"piper/piper.exe", "../escape.txt", "piper/espeak-ng-data/x"} {
		w, _ := zw.Create(name)
		w.Write([]byte("x"))
	}
	zw.Close()
	dir := t.TempDir()
	if err := extractArchive(buf.Bytes(), "piper_windows_amd64.zip", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "piper", "piper.exe")); err != nil {
		t.Fatal("no s'ha extret")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.txt")); err == nil {
		t.Fatal("un fitxer ha sortit del directori")
	}
	_ = context.Background
}
