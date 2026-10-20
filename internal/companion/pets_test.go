package companion

import (
	"strings"
	"testing"
)

// Catàleg de mascotes: 4, ids estables, persona no buida, nom curt de la UI.
func TestPetsCatalog(t *testing.T) {
	pets := Pets()
	if len(pets) != 4 {
		t.Fatalf("esperava 4 mascotes, tinc %d", len(pets))
	}
	seen := map[string]bool{}
	for _, p := range pets {
		if seen[p.ID] {
			t.Errorf("id repetit: %s", p.ID)
		}
		seen[p.ID] = true
		if p.Name == "" || p.Role == "" {
			t.Errorf("mascota %s sense nom o rol de UI", p.ID)
		}
		if len([]rune(p.Persona)) < 80 {
			t.Errorf("mascota %s amb persona massa curta (%d): una veu es nota", p.ID, len([]rune(p.Persona)))
		}
		if strings.Contains(p.Persona, "sk-") {
			t.Errorf("mascota %s amb secret a la persona", p.ID)
		}
	}
	for _, id := range []string{"mel", "llumen", "pixel", "mica"} {
		if _, ok := PetByID(id); !ok {
			t.Errorf("falta la mascota %s", id)
		}
	}
	if _, ok := PetByID("noexist"); ok {
		t.Error("PetByID ha de tornar false per desconeguts")
	}
}

// Les veus són diferents entre elles: sense personalitats distintes no hi ha fun.
func TestPetVoicesDiffer(t *testing.T) {
	pets := Pets()
	for i := 0; i < len(pets); i++ {
		for j := i + 1; j < len(pets); j++ {
			if pets[i].Persona == pets[j].Persona {
				t.Errorf("%s i %s tenen la mateixa persona", pets[i].ID, pets[j].ID)
			}
			if PetCheckinSystem(pets[i]) == PetCheckinSystem(pets[j]) {
				t.Errorf("%s i %s tenen el mateix system de check-in", pets[i].ID, pets[j].ID)
			}
		}
	}
}

// El check-in d'una mascota arrossega la seva persona i la base del company.
func TestPetCheckinSystemKeepsGrounding(t *testing.T) {
	pet, _ := PetByID("mel")
	s := PetCheckinSystem(pet)
	if !strings.Contains(s, "Mel") {
		t.Error("el system del check-in ha de dir el nom de la mascota")
	}
	for _, must := range []string{"memory", "500 characters", "Never invent"} {
		if !strings.Contains(s, must) {
			t.Errorf("el system perd l'ancoratge del company: falta %q", must)
		}
	}
}

// El prompt d'usuari és el mateix de sempre: memòria + digest.
func TestPetCheckinPromptReusesBase(t *testing.T) {
	pet, _ := PetByID("pixel")
	got := PetCheckinPrompt(pet, "MEM", "DIG")
	if !strings.Contains(got, "MEM") || !strings.Contains(got, "DIG") || !strings.Contains(got, "500") {
		t.Errorf("PetCheckinPrompt ha de ser el CheckinPrompt de sempre, tinc %q", got)
	}
}
