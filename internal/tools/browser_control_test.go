package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// La comprovació de sessió no llegeix mai el contingut dels camps.
func TestLoginCheckNeverReadsFieldValues(t *testing.T) {
	for _, bad := range []string{".value", "innerText", "textContent", "outerHTML", "localStorage", "cookie"} {
		if strings.Contains(jsLoginCheck, bad) {
			t.Fatalf("jsLoginCheck llegeix %s", bad)
		}
	}
	if got := SafePageURL("https://exemple.cat/entra?token=abc#x"); got != "https://exemple.cat/entra" {
		t.Fatalf("SafePageURL: %q", got)
	}
	for _, raw := range []string{"about:blank", "", "javascript:alert(1)", "file:///etc/passwd"} {
		if got := SafePageURL(raw); got != "" {
			t.Fatalf("SafePageURL(%q) = %q", raw, got)
		}
	}
}

// Amb la persona al control, cap acció (ni captura ni tancar) no arriba al
// navegador.
func TestBrowserActionWaitsForUserControl(t *testing.T) {
	held := true
	SetBrowserControlGuard(func(string) bool { return held })
	t.Cleanup(func() { SetBrowserControlGuard(nil) })
	old := BrowserControlWait
	BrowserControlWait = 50 * time.Millisecond
	t.Cleanup(func() { BrowserControlWait = old })
	for _, act := range []string{"screenshot", "read", "close", "handoff"} {
		if _, _, err := BrowserActionFor(context.Background(), "scope", act, nil); !errors.Is(err, ErrBrowserUserControl) {
			t.Fatalf("%s amb la persona al control: %v", act, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	BrowserControlWait = time.Minute
	if _, _, err := BrowserActionFor(ctx, "scope", "read", nil); err == nil || errors.Is(err, ErrBrowserUserControl) {
		t.Fatalf("aturar el torn talla l'espera: %v", err)
	}
}
