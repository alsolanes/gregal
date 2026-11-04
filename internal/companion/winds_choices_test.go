package companion

import (
	"strings"
	"testing"
)

// Els vents saben fer preguntes amb opcions i oferir respostes ràpides: les
// dues consignes (xat i grup) ho han de dir.
func TestWindPromptsExplainQuestionsAndChoices(t *testing.T) {
	w := Wind{Name: "Garbí", Role: "Esports"}
	for name, sys := range map[string]string{
		"chat":  WindChatSystem(w, "", "", nil, "", ""),
		"group": RotllanaSystem(w, "- Garbí (Esports)", "", ""),
	} {
		for _, want := range []string{"question tool", "```choices", "2-4 short options"} {
			if !strings.Contains(sys, want) {
				t.Errorf("%s: falta %q", name, want)
			}
		}
	}
}
