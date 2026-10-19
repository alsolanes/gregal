package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/skills"
)

func TestSkillsAPIConsultaIGuardaPreferencies(t *testing.T) {
	original := skills.Habilitades()
	t.Cleanup(func() { _ = skills.DefineixHabilitades(original) })
	s := goalTestServer(t)
	if err := skills.DefineixHabilitades(skills.PerDefecte()); err != nil {
		t.Fatal(err)
	}

	get := httptest.NewRecorder()
	s.handleSkills(get, httptest.NewRequest(http.MethodGet, "/api/skills", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET skills: %d %s", get.Code, get.Body.String())
	}
	var response struct {
		Skills []skillPreference `json:"skills"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Skills) != len(skills.NomsCurades()) {
		t.Fatalf("catàleg inesperat: %+v", response.Skills)
	}

	put := httptest.NewRecorder()
	s.handleSkills(put, httptest.NewRequest(http.MethodPut, "/api/skills", strings.NewReader(`{"enabled":["statistics"]}`)))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT skills: %d %s", put.Code, put.Body.String())
	}
	if got := skills.Habilitades(); len(got) != 1 || got[0] != "statistics" {
		t.Fatalf("skills actives: %v", got)
	}
	if s.cfg.Skills.Enabled == nil || len(s.cfg.Skills.Enabled) != 1 || s.cfg.Skills.Enabled[0] != "statistics" {
		t.Fatalf("no s'ha desat la preferència a config: %+v", s.cfg.Skills)
	}
	loaded, _, err := config.Load(s.cfgPath)
	if err != nil {
		t.Fatalf("no es pot tornar a carregar la preferència desada: %v", err)
	}
	if len(loaded.Skills.Enabled) != 1 || loaded.Skills.Enabled[0] != "statistics" {
		t.Fatalf("la preferència no ha sobreviscut a la recàrrega: %+v", loaded.Skills)
	}
	if err := skills.ValidaHabilitades([]string{"unknown"}); err == nil {
		t.Fatal("s'hauria de rebutjar un nom desconegut")
	}
}
