package web

import (
	"strings"
	"testing"

	"gregal/internal/companion"
)

// Un cuiner i un esportiu no poden respondre igual: cada ofici porta les seves
// fonts de dades, el seu format i les seves vigilàncies.
func TestTradeKitsMakeEachWindASpecialist(t *testing.T) {
	for role, want := range map[string]string{
		"Cuina de mercat":        "cooking",
		"Receptes i menús":       "cooking",
		"Esports":                "sports",
		"Salut i forma física":   "fitness",
		"El temps i la muntanya": "weather",
		"Cinema i televisió":     "movies",
		"Llibres i cultura":      "books",
		"Viatges":                "travel",
		"Finances de casa":       "finance",
		"Sèries temporals":       "",
	} {
		if got := inferKit(role); got != want {
			t.Errorf("%q: kit %q, want %q", role, got, want)
		}
	}
	cook := windTradeBrief(companion.Wind{Name: "Llevant", Role: "Cuina de mercat"})
	sport := windTradeBrief(companion.Wind{Name: "Garbí", Role: "Esports"})
	if !strings.Contains(cook, "themealdb.com") || !strings.Contains(cook, "recipe card") {
		t.Fatalf("the cook needs its sources and format: %s", cook)
	}
	if !strings.Contains(sport, "thesportsdb.com/api/v1/json/123") || !strings.Contains(sport, "Madrid time") || strings.Contains(sport, "themealdb") {
		t.Fatalf("the sports wind needs its own sources: %s", sport)
	}
	if windTradeBrief(companion.Wind{Name: "X", Role: "Res a veure"}) != "" {
		t.Fatal("a wind without a known trade gets no kit")
	}
	for key, k := range tradeKits {
		if k.Format == "" || k.CA == "" || k.EN == "" {
			t.Errorf("%s: incomplete kit", key)
		}
		for _, src := range k.Sources {
			if !strings.HasPrefix(src.URL, "https://") {
				t.Errorf("%s: %s must be https", key, src.Name)
			}
		}
		for _, w := range k.Watches {
			if minutesOf(w.Time) < 0 || w.PromptCA == "" || w.PromptEN == "" {
				t.Errorf("%s: watch %q incomplete", key, w.CA)
			}
		}
	}
}

func TestTradeProfileShowsLevelSourcesAndWatches(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	p := s.tradeProfile(companion.Wind{ID: "garbi", Name: "Garbí", Role: "Esports"})
	if p == nil || p["kit"] != "sports" || p["level"].(int) < 1 || len(p["sources"].([]string)) == 0 || len(p["watches"].([]map[string]any)) == 0 {
		t.Fatalf("profile: %+v", p)
	}
}
