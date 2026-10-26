package companion

import (
	"strings"
	"testing"
)

// TestWindPromptsStartWithOwnName: cada vent sap com es diu des de la primera
// línia, al xat individual i en grup, i que l'humà és algú altre.
func TestWindPromptsStartWithOwnName(t *testing.T) {
	w := Wind{Name: "Garbí", Role: "Esports", Persona: "Ets apassionat del futbol."}
	for name, prompt := range map[string]string{
		"chat":  WindChatSystem(w, "", "", nil, "", ""),
		"group": RotllanaSystem(w, "- Garbí (Esports)\n- Curie (Dades)", "", ""),
	} {
		if !strings.HasPrefix(prompt, "YOUR NAME IS Garbí.") {
			t.Errorf("%s: el prompt no comença pel nom propi: %q", name, prompt[:60])
		}
		if !strings.Contains(prompt, "The human you talk to is NOT Garbí") {
			t.Errorf("%s: no separa el vent de l'humà", name)
		}
	}
	if WindIdentity(Wind{}) != "" {
		t.Error("un vent sense nom no ha d'inventar-ne cap")
	}
}
