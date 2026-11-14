package web

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
)

var fakeJPEG = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("jpeg-body")...)
var fakePNG = append([]byte("\x89PNG\r\n\x1a\n"), []byte("png-body")...)

var imageLine = regexp.MustCompile(`!\[([^\]]*)\]\((/api/winds/image\?id=[a-f0-9]{24}\.(jpg|png|webp))\)`)

// Sense cap model d'imatges configurat, el vent fa servir el servei gratuït;
// la imatge queda desada al servidor i es baixa per la ruta amb clau.
func TestWindImageFreeServiceSavesAndServes(t *testing.T) {
	s, _ := companionTestServer(t)
	var asked string
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.String()
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(fakeJPEG)
	}))
	defer free.Close()
	useFreeService(t, free.URL)

	out, err := s.imageDo(context.Background(), imageArgs{Wind: "llevant", Prompt: "a bowl of fideuà, top view", Title: "La fideuà [nova]", Shape: "landscape"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asked, "fideu") || !strings.Contains(asked, "width=1152") {
		t.Fatalf("free service request = %q", asked)
	}
	m := imageLine.FindStringSubmatch(out)
	if m == nil || m[1] != "La fideuà (nova)" || m[3] != "jpg" {
		t.Fatalf("tool result = %q", out)
	}

	rec := httptest.NewRecorder()
	s.handleWindImage(rec, httptest.NewRequest(http.MethodGet, m[2], nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Body.String() != string(fakeJPEG) {
		t.Fatalf("serve: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, bad := range []string{"../config.yaml", "abc.jpg", strings.Repeat("a", 24) + ".svg"} {
		rec := httptest.NewRecorder()
		s.handleWindImage(rec, httptest.NewRequest(http.MethodGet, "/api/winds/image?id="+bad, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: %d", bad, rec.Code)
		}
	}
}

// Amb roles.image: primer /images/generations (OpenAI i compatibles); si el
// proveïdor no el té, el xat amb modalities (OpenRouter, Gemini).
func TestWindImageUsesTheConfiguredModel(t *testing.T) {
	s, _ := companionTestServer(t)
	var paths []string
	chatOnly := false
	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("no key on %s", r.URL.Path)
		}
		b64 := base64.StdEncoding.EncodeToString(fakePNG)
		switch {
		case strings.HasSuffix(r.URL.Path, "/images/generations") && !chatOnly:
			fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, b64)
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			fmt.Fprintf(w, `{"choices":[{"message":{"content":"","images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,%s"}}]}}]}`, b64)
		default:
			http.NotFound(w, r)
		}
	}))
	defer prov.Close()
	s.cfg.Providers["img"] = config.Provider{BaseURL: prov.URL + "/v1", APIKey: "k"}
	s.cfg.Roles["image"] = config.Role{Provider: "img", Model: "gpt-image-1"}

	out, err := s.imageDo(context.Background(), imageArgs{Prompt: "a festa major poster"})
	if err != nil || !strings.Contains(out, "gpt-image-1") || imageLine.FindStringSubmatch(out)[3] != "png" {
		t.Fatalf("images endpoint: %q %v", out, err)
	}
	chatOnly = true
	paths = nil
	out, err = s.imageDo(context.Background(), imageArgs{Prompt: "a festa major poster"})
	if err != nil || imageLine.FindString(out) == "" || len(paths) != 2 || !strings.HasSuffix(paths[1], "/chat/completions") {
		t.Fatalf("chat fallback: %q %v %v", out, err, paths)
	}

	s.cfg.Roles["image"] = config.Role{Model: "off"}
	if _, err := s.imageDo(context.Background(), imageArgs{Prompt: "x"}); err == nil || !strings.Contains(err.Error(), "switched off") {
		t.Fatalf("off: %v", err)
	}
}

// Un servei que torna una pàgina d'error en lloc d'una imatge no deixa res desat.
func TestWindImageRejectsWhatIsNotAPicture(t *testing.T) {
	s, _ := companionTestServer(t)
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html>rate limited</html>")
	}))
	defer free.Close()
	useFreeService(t, free.URL)
	if _, err := s.imageDo(context.Background(), imageArgs{Prompt: "x"}); err == nil {
		t.Fatal("an HTML page was accepted as a picture")
	}
	if links := s.windImageLinks(time.Now().Add(-time.Minute), ""); links != "" {
		t.Fatalf("something was saved: %q", links)
	}
}

// Si el vent ha fet la imatge però no l'ha posada a la resposta, l'app
// l'afegeix al final; si ja hi és, no la repeteix.
func TestWindImageForgottenIsAppended(t *testing.T) {
	s, _ := companionTestServer(t)
	start := time.Now()
	id, err := s.saveWindImage(fakeJPEG, "jpg")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.windDocLinks(start, "Aquí tens el cartell."); !strings.Contains(got, windImageLink(id)) {
		t.Fatalf("forgotten picture not appended: %q", got)
	}
	if got := s.windDocLinks(start, "![cartell]("+windImageLink(id)+")"); strings.Contains(got, id) {
		t.Fatalf("picture repeated: %q", got)
	}
}

// useFreeService apunta el servei gratuït a un servidor de prova i no espera
// de debò els 45 s entre imatges.
func useFreeService(t *testing.T, base string) {
	oldBase, oldWait := pollinationsBase, pollinationsWait
	pollinationsBase = base + "/prompt/"
	pollinationsWait = func(time.Duration) time.Duration { return 0 }
	pollinationsMu.Lock()
	pollinationsLast = time.Time{}
	pollinationsMu.Unlock()
	t.Cleanup(func() { pollinationsBase, pollinationsWait = oldBase, oldWait })
}

// El servei gratuït fa una imatge cada 45 s: si diu que encara no toca, o
// falla de passada, Gregal espera i ho torna a provar en lloc de fallar.
func TestWindImageFreeServiceWaitsItsTurn(t *testing.T) {
	s, _ := companionTestServer(t)
	calls := 0
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 2:
			w.WriteHeader(http.StatusPaymentRequired)
			fmt.Fprint(w, `{"error":{"code":"FREE_LIMIT","message":"Free limit reached: 1 image every 45 seconds per IP. Retry after 24s"}}`)
		default:
			_, _ = w.Write(fakeJPEG)
		}
	}))
	defer free.Close()
	useFreeService(t, free.URL)
	var waits []time.Duration
	pollinationsWait = func(d time.Duration) time.Duration { waits = append(waits, d); return 0 }
	out, err := s.imageDo(context.Background(), imageArgs{Prompt: "x"})
	if err != nil || imageLine.FindString(out) == "" || calls != 3 {
		t.Fatalf("out=%q err=%v calls=%d", out, err, calls)
	}
	if len(waits) != 2 || waits[1] < 20*time.Second || waits[1] > 26*time.Second {
		t.Fatalf("waits = %v (the second should follow «Retry after 24s»)", waits)
	}
}
