package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/agent"
)

// TestUserSkillsCrud comprova el cicle complet de les skills d'usuari:
// desar (amb normalització del nom), llistar (només l'índex), llegir el cos,
// i esborrar. Les skills globals no s'hi barregen.
func TestUserSkillsCrud(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t)
	anna := hub.SessionFor(&User{Name: "anna"}, "s1")

	save := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/skills", strings.NewReader(body))
		rec := httptest.NewRecorder()
		anna.handleUserSkills(rec, req)
		return rec
	}

	// Desa una skill (nom amb majúscules i espais → normalitzat).
	rec := save(`{"nom":"Preferències de la Carla","titol":"Com parla la Carla","descripcio":"Tons i paraules que fa servir","cos":"A la Carla li agrada que li parlis amb comparacions del mar."}`)
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}

	// Llista: només índex, sense cos.
	get := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	body := httptest.NewRecorder()
	anna.handleUserSkills(body, get)
	if body.Code != 200 {
		t.Fatalf("list: %d", body.Code)
	}
	var list struct {
		Skills []map[string]string `json:"skills"`
	}
	if err := json.Unmarshal(body.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Skills) != 1 || !strings.Contains(list.Skills[0]["nom"], "preferencies") {
		t.Fatalf("llistat erroni: %+v", list.Skills)
	}
	if list.Skills[0]["cos"] != "" {
		// el cos no viatja a la llista — és el contracte
		t.Errorf("la llista no ha de portar el cos: %+v", list.Skills[0])
	}

	// Llegeix el cos.
	get2 := httptest.NewRequest(http.MethodGet, "/api/skills?nom=preferencies-de-la-carla", nil)
	body2 := httptest.NewRecorder()
	anna.handleUserSkills(body2, get2)
	if body2.Code != 200 {
		t.Fatalf("get: %d %s", body2.Code, body2.Body.String())
	}
	if !strings.Contains(body2.Body.String(), "comparacions del mar") {
		t.Errorf("cos perdut: %s", body2.Body.String())
	}

	// Esborra.
	req := httptest.NewRequest(http.MethodDelete, "/api/skills", strings.NewReader(`{"nom":"preferencies-de-la-carla"}`))
	body3 := httptest.NewRecorder()
	anna.handleUserSkills(body3, req)
	if body3.Code != 200 {
		t.Fatalf("delete: %d", body3.Code)
	}

	// Un altre usuari no la veu (ja era esborrada; desem una altra i ho comprovem).
	save(`{"nom":"secret-anna","titol":"s","cos":"notaaa"}`)
	bert := hub.SessionFor(&User{Name: "bert"}, "s1")
	gb := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	bb := httptest.NewRecorder()
	bert.handleUserSkills(bb, gb)
	if strings.Contains(bb.Body.String(), "secret-anna") {
		t.Error("bert veu les skills de l'anna: no són per usuari")
	}

	// Sense usuari: 400 (no és un servidor multiusuari).
	anon := hub.SessionFor(nil, "default")
	ga := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	ba := httptest.NewRecorder()
	anon.handleUserSkills(ba, ga)
	if ba.Code != http.StatusBadRequest {
		t.Errorf("sense usuari hauria de ser 400: %d", ba.Code)
	}
}

// TestUserSkillIndexInPrompt: les skills de l'usuari entren a l'índex del
// prompt de la SEVA sessió, no a la dels altres.
func TestUserSkillIndexInPrompt(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t)
	anna := hub.SessionFor(&User{Name: "anna"}, "s1")

	req := httptest.NewRequest(http.MethodPost, "/api/skills", strings.NewReader(`{"nom":"veles-de-la-llista","titol":"Quines veles","cos":"La Llista té floc i aparell de càtum."}`))
	rec := httptest.NewRecorder()
	anna.handleUserSkills(rec, req)
	if rec.Code != 200 {
		t.Fatalf("save: %d", rec.Code)
	}

	idx := anna.userSkillsDirs()
	if len(idx) == 0 || !strings.Contains(idx[0], filepath.Join("users", ".identity-v1")) {
		t.Fatalf("dirs d'usuari buits: %v", idx)
	}
	// ContextProjecteExtra amb els dirs de l'anna inclou la skill.
	out := contextProjecteExtraForTest(anna, idx)
	if !strings.Contains(out, "veles-de-la-llista") {
		t.Errorf("la skill de l'anna no surt a l'índex del prompt: %s", out)
	}
	// Sense els dirs (una altra persona), no hi surt.
	out2 := contextProjecteExtraForTest(anna, nil)
	if strings.Contains(out2, "veles-de-la-llista") {
		t.Error("la skill de l'anna surt a l'índex d'un altre: filtrat trencat")
	}
}

// contextProjecteExtraForTest aïlla ContextProjecteExtra per al test.
func contextProjecteExtraForTest(s *Server, dirs []string) string {
	return agent.ContextProjecteExtra("/tmp", "test", dirs...)
}
