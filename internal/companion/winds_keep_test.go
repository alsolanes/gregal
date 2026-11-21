package companion

import (
	"strings"
	"testing"
)

// La guia de targetes (vents, Gregal i grups) diu com proposar desar una
// resposta que la persona voldrà tornar a veure, amb quins tipus, i que els
// desats es busquen amb l'eina notes.
func TestCardsGuideProposesKeeping(t *testing.T) {
	for _, want := range []string{"```gregal-keep {\"title\",\"kind\"}", "never for chat, news or one-off answers", "At most one per reply", "notes", "search"} {
		if !strings.Contains(CardsGuide, want) {
			t.Errorf("CardsGuide: falta %q", want)
		}
	}
	// Els tipus, en l'ordre de POST /api/notes: el servidor rebutja qualsevol altre.
	if !strings.Contains(CardsGuide, "recipe|tip|plan|route|compare|facts|result|chart|web|text") {
		t.Error("CardsGuide: falten els tipus de desat")
	}
}
