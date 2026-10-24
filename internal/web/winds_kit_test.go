package web

import (
	"testing"

	"gregal/internal/companion"
)

// Els vents propis no tenen specialty de colla: el seu ofici viu al rol. La
// cara els posa l'objecte del seu ofici endevinat d'aquell text.
func TestInferKitDelRol(t *testing.T) {
	casos := []struct{ role, kit string }{
		{"Música i descobertes", "music"},
		{"Esports", "sports"},
		{"Futbol i Barça", "sports"},
		{"Tecnologia i novetats digitals", "tech"},
		{"Recerca i experiments de laboratori", "research"},
		{"Llibres i tardes de lectura", "books"},
		{"Compres i comparacions de preus", "shopping"},
		{"Machine learning i models", "ml"},
		{"Seguretat informàtica", "security"},
		{"Sèries temporals", ""},           // cap keyword → cara neta
		{"calma i benestar", ""},           // idem
		{"Music and songs", "music"},       // anglès
		{"Football news", "sports"},        // anglès
		{"", ""},                           // rol buit
	}
	for _, c := range casos {
		if got := inferKit(c.role); got != c.kit {
			t.Errorf("inferKit(%q) = %q, volia %q", c.role, got, c.kit)
		}
	}
}

// kitOf manen la specialty de colla; el rol només s'usa quan no n'hi ha.
func TestKitOfManaSpecialty(t *testing.T) {
	w := companion.Wind{Name: "Garbí", Role: "Esports", Specialty: "sports"}
	if got := kitOf(w); got != "sports" {
		t.Errorf("kitOf amb specialty = %q, volia sports", got)
	}
	w2 := companion.Wind{Name: "Tramuntana", Role: "Tecnologia i novetats digitals"}
	if got := kitOf(w2); got != "tech" {
		t.Errorf("kitOf sense specialty = %q, volia tech", got)
	}
}
