package tools

import (
	"strings"
	"testing"
	"time"
)

func TestBrowserHandoffAsksTheAppPaneAndRereadsThePage(t *testing.T) {
	p := newPontFals(t, "tok")
	ambModeNet(t, "app", p.srv.URL+"/?token=tok")
	if _, _, err := BrowserAction(nil, "open", map[string]any{"url": "http://exemple.cat/login"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := BrowserAction(nil, "handoff", map[string]any{"text": "  Inicia sessió\n i prem Fet ", "seconds": 120.0})
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	req := p.handoff
	p.mu.Unlock()
	if req["message"] != "Inicia sessió i prem Fet" || req["seconds"] != 120.0 {
		t.Fatalf("handoff request: %#v", req)
	}
	if !strings.Contains(out, "The user says it is done") || !strings.Contains(out, "Benvingut") {
		t.Fatalf("handoff output must confirm and re-read the page: %s", out)
	}

	p.mu.Lock()
	p.handoffReply = `{"ok":true,"done":false,"reason":"cancelled"}`
	p.mu.Unlock()
	if out, _, err := BrowserAction(nil, "handoff", map[string]any{"text": "Resol el CAPTCHA"}); err != nil || !strings.Contains(out, "cancelled") {
		t.Fatalf("cancelled handoff: %v %s", err, out)
	}
	p.mu.Lock()
	p.handoffReply = `{"ok":true,"done":false,"reason":"timeout"}`
	p.mu.Unlock()
	if out, _, err := BrowserAction(nil, "handoff", map[string]any{"text": "Tria el compte"}); err != nil || !strings.Contains(out, "did not answer in time") {
		t.Fatalf("timed-out handoff: %v %s", err, out)
	}
	if _, _, err := BrowserAction(nil, "handoff", map[string]any{}); err == nil || !strings.Contains(err.Error(), "needs text") {
		t.Fatalf("handoff without text: %v", err)
	}
}

func TestBrowserHandoffIsAReadOnlyCallAndNotABatchStep(t *testing.T) {
	if !BrowserCallSegura(map[string]any{"action": "handoff", "text": "x"}) {
		t.Fatal("handoff does not act on the page: it must not ask for permission")
	}
	b := &Browser{refs: map[int]string{}}
	out, _, err := b.batch(browserArgs{"steps": []any{map[string]any{"action": "handoff", "text": "x"}}})
	if err == nil || !strings.Contains(err.Error(), "not allowed inside a batch") {
		t.Fatalf("handoff inside batch: %v %s", err, out)
	}
	if !strings.Contains(BrowserToolDescription, "handoff") || !strings.Contains(ErrCredentialField.Error(), "handoff") {
		t.Fatal("the model must learn to hand credentials over with handoff")
	}
}

func TestHandoffWaitStaysWithinBounds(t *testing.T) {
	cases := map[float64]time.Duration{0: HandoffDefaultWait, -5: HandoffDefaultWait, 3: 15 * time.Second, 90: 90 * time.Second, 99999: HandoffMaxWait}
	for in, want := range cases {
		if got := handoffWait(browserArgs{"seconds": in}); got != want {
			t.Errorf("handoffWait(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestHandoffNeedsAVisibleBrowser(t *testing.T) {
	b := &Browser{refs: map[int]string{}, mode: BrowserModeHeadless}
	if _, _, err := b.Handoff("Inicia sessió", time.Minute); err == nil || !strings.Contains(err.Error(), "headless") {
		t.Fatalf("headless handoff: %v", err)
	}
}
