package companion

import (
	"strings"
	"testing"
)

// En català «esports» és esport en general; en anglès (la llengua dels
// prompts) és videojocs competitius. El Garbí cercava e-sports.
func TestEsportsRoleIsNotESports(t *testing.T) {
	for _, role := range []string{"Esports", "esports", "Esport i salut", "Música i esports"} {
		if got := RolePrompt(role); !strings.Contains(got, "NOT e-sports") {
			t.Errorf("%q → %q: falta l'aclariment", role, got)
		}
	}
	for _, role := range []string{"Sports", "E-sports", "Shopping and comparisons", "Llibres i cultura", ""} {
		if got := RolePrompt(role); got != strings.TrimSpace(role) {
			t.Errorf("%q no s'ha de tocar: %q", role, got)
		}
	}
	w := Wind{Name: "Garbí", Role: "Esports"}
	if id := WindIdentity(w); !strings.Contains(id, "NOT e-sports") {
		t.Fatalf("la identitat ho ha d'aclarir: %s", id)
	}
	for name, sys := range map[string]string{
		"chat":  WindChatSystem(w, "", "", nil, "", ""),
		"group": RotllanaSystem(w, "- Garbí (Esports)", "", ""),
	} {
		if !strings.Contains(sys, "never the bare word «esports»") {
			t.Errorf("%s: falta la regla de cerca en català", name)
		}
	}
}
