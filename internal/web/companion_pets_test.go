package web

import (
	"context"
	"encoding/json"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// GET /api/companion/pets torna el catàleg de 4 mascotes, sense persones.
func TestCompanionPetsEndpoint(t *testing.T) {
	s, _ := companionTestServer(t)

	var got struct {
		Pets []map[string]any `json:"pets"`
	}
	code := getJSON(t, s.handleCompanionPets, "/api/companion/pets", &got)
	if code != 200 {
		t.Fatalf("pets: %d", code)
	}
	if len(got.Pets) != 4 {
		t.Fatalf("esperava 4 mascotes, tinc %d", len(got.Pets))
	}
	for _, p := range got.Pets {
		if _, ok := p["persona"]; ok {
			t.Error("la persona no ha de viatjar al client")
		}
		if p["id"] == "" || p["name"] == "" || p["role"] == "" {
			t.Errorf("mascota incompleta: %v", p)
		}
	}
}

// POST /api/companion/checkin amb pet=mel: el checkiner rep la persona de
// la Mel; la resposta marca quina mascota ha parlat. Sense pet: base.
func TestCheckinRoutesPetPersona(t *testing.T) {
	s, _ := companionTestServer(t)

	var gotSystem, gotUser string
	saved := petCheckiner
	petCheckiner = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, system, user string) (string, error) {
		gotSystem, gotUser = system, user
		return "Check-in de prova de la Mel", nil
	}
	t.Cleanup(func() { waitBackground(); petCheckiner = saved })

	rec := postJSON(t, s.handleCompanionCheckinPet, `{"pet":"mel"}`)
	if rec.Code != 200 {
		t.Fatalf("checkin mel: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Checkin string `json:"checkin"`
		Pet     string `json:"pet"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Pet != "mel" {
		t.Errorf("la resposta ha de dir la mascota, tinc %q", out.Pet)
	}
	// La persona de la Mel ha d'arribar al checkiner via la via del system:
	// capturem què s'ha passat com a «memory» quan hi ha pet.
	if gotSystem == "" || gotUser == "" {
		t.Error("el checkiner no ha rebut memòria/digest")
	}
}

// POST amb pet desconegut → 400, cap checkiner disparat.
func TestCheckinUnknownPet400(t *testing.T) {
	s, _ := companionTestServer(t)
	called := false
	saved := petCheckiner
	petCheckiner = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, system, user string) (string, error) {
		called = true
		return "", nil
	}
	t.Cleanup(func() { waitBackground(); petCheckiner = saved })

	rec := postJSON(t, s.handleCompanionCheckinPet, `{"pet":"fantasma"}`)
	if rec.Code != 400 {
		t.Fatalf("pet desconegut: %d (esperava 400)", rec.Code)
	}
	if called {
		t.Error("cap crida al model amb pet desconegut")
	}
}
