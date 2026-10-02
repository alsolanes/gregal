package tools

// Prova viva del navegador amb el Chrome/Edge de la màquina (headless).
// Salta sense GREGAL_LIVE_BROWSER=1. Comprova el circuit real: engegar el
// navegador amb el perfil del gregal, obrir una pàgina, llegir-la en
// markdown amb elements numerats, fer captura i tancar.
//
// Ús: GREGAL_LIVE_BROWSER=1 go test ./internal/tools/ -run TestLiveBrowser -v

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveBrowser(t *testing.T) {
	if os.Getenv("GREGAL_LIVE_BROWSER") == "" {
		t.Skip("sense GREGAL_LIVE_BROWSER=1")
	}
	if BrowserExecutable() == "" {
		t.Skip("cap Chrome/Edge a la màquina")
	}
	t.Setenv("GREGAL_BROWSER_HEADLESS", "1")
	t.Setenv("GREGAL_BROWSER_PROFILE", t.TempDir())
	defer CloseBrowser()
	t0 := time.Now()
	out, _, err := BrowserAction(nil, "open", map[string]any{"url": "https://example.com/"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("open en %v:\n%s", time.Since(t0), out)
	if !strings.Contains(out, "Example Domain") || !strings.Contains(out, "[1] a") {
		t.Fatalf("lectura inesperada")
	}
	t1 := time.Now()
	res, _, err := BrowserAction(nil, "click", map[string]any{"target": "1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("click en %v: %s", time.Since(t1), res)
	if !strings.Contains(res, "iana.org") {
		t.Fatalf("el clic havia de portar a iana.org: %s", res)
	}
	txt, imgs, err := BrowserAction(nil, "screenshot", nil)
	if err != nil || len(imgs) != 1 || len(imgs[0]) < 1000 {
		t.Fatalf("captura: %v %d", err, len(imgs))
	}
	t.Logf("%s (%d bytes de data URL)", txt, len(imgs[0]))
	val, _, err := BrowserAction(nil, "eval", map[string]any{"js": "document.querySelectorAll('a').length + ' enllaços'"})
	if err != nil || !strings.Contains(val, "enllaços") {
		t.Fatalf("eval: %v %q", err, val)
	}
}
