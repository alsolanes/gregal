package tools

import (
	"errors"
	"testing"
)

// Sense cap navegador obert, el visor no pot fer res i ho diu.
func TestLiveActWithoutBrowser(t *testing.T) {
	liveMu.Lock()
	oldScope := liveScope
	liveScope = "sessio-que-no-existeix"
	liveMu.Unlock()
	t.Cleanup(func() { liveMu.Lock(); liveScope = oldScope; liveMu.Unlock() })
	browserMu.Lock()
	oldInst := browserInst
	browserInst = nil
	browserMu.Unlock()
	t.Cleanup(func() { browserMu.Lock(); browserInst = oldInst; browserMu.Unlock() })
	for _, k := range []string{"tap", "type", "key", "scroll", "back"} {
		if err := LiveAct(LiveInput{Kind: k, Text: "x", Key: "Enter"}); !errors.Is(err, ErrNoLiveBrowser) {
			t.Fatalf("%s: %v", k, err)
		}
	}
}
