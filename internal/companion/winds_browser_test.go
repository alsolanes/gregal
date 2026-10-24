package companion

import (
	"strings"
	"testing"
)

// Els vents han de saber que poden obrir pàgines amb el navegador:
// sense la menció al prompt, el model no descobreix l'eina tot sol.
func TestRotllanaSystemMencionaBrowser(t *testing.T) {
	w := Wind{ID: "xaloc", Name: "Xaloc", Color: "#20BE92", Persona: "amics i música", Role: "música i descobertes"}
	s := RotllanaSystem(w, "Llevant (llibres)", "")
	if !strings.Contains(strings.ToLower(s), "browser") {
		t.Fatal("el prompt de la rotllana no menciona l'eina browser")
	}
	if !strings.Contains(s, "never click or type") {
		t.Fatal("el prompt no deixa clar que el navegador només mira")
	}
}

func TestWindChatSystemMencionaBrowser(t *testing.T) {
	w := Wind{ID: "llevant", Name: "Llevant", Color: "#1A9BEE", Persona: "lector", Role: "llibres i cultura"}
	s := WindChatSystem(w, "", "Gregal", nil, "")
	if !strings.Contains(strings.ToLower(s), "browser") {
		t.Fatal("el prompt del xat no menciona l'eina browser")
	}
}
