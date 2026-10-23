package agent

import (
	"strings"
	"testing"
)

// Al mòbil les ordres de shell estan denegades: la salutació no les promet.
func TestSmallTalkGreetingOnMobile(t *testing.T) {
	t.Setenv("GREGAL_PLATFORM", "android")
	r, ok := SmallTalkFor("hola", "ca", false, false)
	if !ok || strings.Contains(r, "ordres") || !strings.Contains(r, "documents") {
		t.Fatalf("mobile greeting: %q", r)
	}
	t.Setenv("GREGAL_PLATFORM", "")
	if r, _ := SmallTalkFor("hello", "en", false, false); !strings.Contains(r, "run commands") {
		t.Fatalf("desktop greeting changed: %q", r)
	}
}
