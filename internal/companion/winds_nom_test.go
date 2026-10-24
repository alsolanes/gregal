package companion

import (
	"strings"
	"testing"
)

// TestWindChatSystemDiuQuiEs: el prompt sempre diu amb qui es parla; si el
// nom no es coneix, mana preguntar-lo, mai inventar-lo ni agafar el d'un
// altre vent.
func TestWindChatSystemDiuQuiEs(t *testing.T) {
	w := Wind{ID: "xaloc", Name: "Xaloc", Color: "#20BE92", Persona: "sóc Xaloc", Role: "vigilant"}

	// Nom conegut: surt al prompt, un sol cop.
	amb := WindChatSystem(w, "", "Gregal", nil, "", "Aleix")
	if !strings.Contains(amb, "You chat with one person: Aleix") {
		t.Errorf("falta el nom conegut: %s", amb)
	}

	// Nom desconegut: mana preguntar i prohibeix manllevar.
	sense := WindChatSystem(w, "", "Gregal", nil, "", "")
	if !strings.Contains(sense, "do NOT know") || !strings.Contains(sense, "NEVER invent") {
		t.Errorf("falta la consigna de preguntar: %s", sense)
	}
	if strings.Contains(sense, "Xaloc") && strings.Count(sense, "Xaloc") > 1 {
		// El nom del vent hi és (persona); el de l'usuari no hi és.
		t.Logf("persona amb el nom del vent: correcte mentre no hi hagi cap altre nom d'usuari")
	}
	// La VEU CARD no ha de dir "from memory" (el nom no hi era mai):
	if strings.Contains(sense, "you know their name from memory") {
		t.Errorf("la voice card encara promet el nom des de la memòria")
	}
}

// TestRotllanaSystemDiuQuiEs: a la rotllana, l'humà té nom i queda separat
// dels vents.
func TestRotllanaSystemDiuQuiEs(t *testing.T) {
	w := Wind{ID: "xaloc", Name: "Xaloc", Color: "#20BE92", Persona: "sóc Xaloc", Role: "vigilant"}
	amb := RotllanaSystem(w, "Llevant (llibres)", "", "Aleix")
	if !strings.Contains(amb, "one human in this group: Aleix") {
		t.Errorf("falta l'humà amb nom: %s", amb)
	}
	sense := RotllanaSystem(w, "Llevant (llibres)", "", "")
	if !strings.Contains(sense, "NEVER borrow the name of another wind") {
		t.Errorf("falta la prohibició de manllevar noms: %s", sense)
	}
}
