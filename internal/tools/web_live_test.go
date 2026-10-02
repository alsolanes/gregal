package tools

// Prova viva de la cerca i la lectura web contra internet. Salta sense
// GREGAL_LIVE_WEB=1 (CI ràpida i sense xarxa). Amb la variable, comprova
// que la cerca respon en pocs segons amb resultats fusionats i que el
// lector treu un article net (títol + markdown compacte).
//
// Ús: GREGAL_LIVE_WEB=1 go test ./internal/tools/ -run TestLiveWeb -v

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveWebSearchIFetch(t *testing.T) {
	if os.Getenv("GREGAL_LIVE_WEB") == "" {
		t.Skip("sense GREGAL_LIVE_WEB=1")
	}
	WebCacheClear()
	t0 := time.Now()
	out, err := WebSearch("charmbracelet bubbletea go tui framework", 8)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cerca en %v:\n%s", time.Since(t0), out)
	if !strings.Contains(out, "github.com/charmbracelet/bubbletea") {
		t.Fatalf("cerca sense el resultat esperat")
	}
	if d := time.Since(t0); d > 10*time.Second {
		t.Fatalf("cerca massa lenta: %v", d)
	}
	// Repetida: cache, immediata.
	t1 := time.Now()
	if _, err := WebSearch("charmbracelet bubbletea go tui framework", 8); err != nil || time.Since(t1) > 50*time.Millisecond {
		t.Fatalf("cache de cerca: err=%v temps=%v", err, time.Since(t1))
	}

	for _, u := range []string{
		"https://github.com/charmbracelet/bubbletea",
		"https://go.dev/blog/loopvar-preview",
		"https://en.wikipedia.org/wiki/Text-based_user_interface",
	} {
		t2 := time.Now()
		page, err := WebFetchOpts_(u, WebFetchOpts{MaxChars: 6000})
		if err != nil {
			t.Errorf("%s: %v", u, err)
			continue
		}
		t.Logf("%s en %v: %d caràcters\n%.600s\n…", u, time.Since(t2), len(page), page)
		if !strings.HasPrefix(page, "# ") {
			t.Errorf("%s: sense títol al capdamunt", u)
		}
		if strings.Contains(page, "<div") || strings.Contains(page, "<script") {
			t.Errorf("%s: HTML sense netejar", u)
		}
	}
	// find: només paràgrafs amb el terme.
	page, err := WebFetchOpts_("https://en.wikipedia.org/wiki/Text-based_user_interface", WebFetchOpts{Find: "ncurses"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, "[¶") || !strings.Contains(strings.ToLower(page), "ncurses") {
		t.Fatalf("find: %.400s", page)
	}
	t.Logf("find ncurses:\n%.800s", page)
}
