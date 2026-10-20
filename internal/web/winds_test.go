package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/goal"
)

// windGoalFile desa un objectiu de prova al goalDir del servidor.
func windGoalFile(t *testing.T, s *Server, id, title, status string) {
	t.Helper()
	if err := goal.Save(s.goalDir, goal.Goal{ID: id, Title: title, Status: status, Project: s.projectName()}); err != nil {
		t.Fatal(err)
	}
}

// Vents de Gregal: catàleg desat en disc, xat amb context de projecte,
// i històric durador. La feina proactiva és un job del sistema, no del vent.

// TestWindCatalogSaveListDelete comprova el cicle de vida d'un vent al disc:
// crear amb nom/color, llistar, i esborrar. El color invàlid cau al defecte.
func TestWindCatalogSaveListDelete(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)

	rec := postJSON(t, s.handleWinds, `{"name":"Tramuntana","color":"#29ADFF","role":"vigilant del sistema"}`)
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	// Color sense #: es normalitza al taronja per defecte.
	rec = postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"blau","role":"música"}`)
	if rec.Code != 200 {
		t.Fatalf("save2: %d", rec.Code)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/winds", nil)
	body := httptest.NewRecorder()
	s.handleWinds(body, get)
	if body.Code != 200 {
		t.Fatalf("list: %d", body.Code)
	}
	var out struct {
		Winds []map[string]any `json:"winds"`
	}
	if err := json.Unmarshal(body.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Winds) != 2 {
		t.Fatalf("vull 2 vents, tinc %d", len(out.Winds))
	}
	if out.Winds[0]["id"] != "tramuntana" || out.Winds[0]["color"] != "#29ADFF" {
		t.Errorf("tramuntana mal desada: %+v", out.Winds[0])
	}
	if out.Winds[1]["color"] != "#FF9F00" {
		t.Errorf("el color invàlid hauria de caure al defecte: %+v", out.Winds[1])
	}
	// La persona NO viatja al client.
	if _, has := out.Winds[0]["persona"]; has {
		t.Error("la persona del vent no ha de viatjar al client")
	}

	// Esborra el segon (DELETE real).
	req := httptest.NewRequest(http.MethodDelete, "/api/winds", strings.NewReader(`{"id":"xaloc"}`))
	body2 := httptest.NewRecorder()
	s.handleWinds(body2, req)
	if body2.Code != 200 {
		t.Fatalf("delete: %d", body2.Code)
	}
	if _, ok := s.winds.ByID("xaloc"); ok {
		t.Error("xaloc hauria de ser esborrat")
	}
}

// TestWindChatGroundedInProjectContext comprova que el system del xat porta
// la persona del vent + el context real del projecte (objectius pendents).
func TestWindChatGroundedInProjectContext(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	if rec := postJSON(t, s.handleWinds, `{"name":"Migjorn","color":"#A3E24E","role":"notícies"}`); rec.Code != 200 {
		t.Fatalf("save: %d", rec.Code)
	}
	windGoalFile(t, s, "g1", "Acabar la versió vents", "pendent")

	var gotBrief string
	saved := windLoop
	windLoop = func(sv *Server, ctx context.Context, brief string) (string, error) {
		gotBrief = brief
		return "buf, quin dia!", nil
	}
	t.Cleanup(func() { windLoop = saved })

	rec := postJSON(t, s.handleWindChat, `{"wind":"migjorn","message":"què fem avui?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	// El brief porta la persona, el context de projecte i l'ordre de cercar.
	for _, want := range []string{"Migjorn", "Acabar la versió vents", s.projectName(), "web_search"} {
		if !strings.Contains(gotBrief, want) {
			t.Errorf("brief sense %q:\n%s", want, gotBrief)
		}
	}

	// GET torna l'historial desat.
	req := httptest.NewRequest(http.MethodGet, "/api/winds/chat?wind=migjorn", nil)
	body := httptest.NewRecorder()
	s.handleWindChat(body, req)
	if body.Code != 200 {
		t.Fatalf("get: %d", body.Code)
	}
	if !strings.Contains(body.Body.String(), "què fem avui?") {
		t.Errorf("historial no desat: %s", body.Body.String())
	}
}

// TestWindChatValidation: vent desconegut → 400; missatge buit → 400.
func TestWindChatValidation(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	if rec := postJSON(t, s.handleWindChat, `{"wind":"fantasma","message":"hola"}`); rec.Code != 400 {
		t.Errorf("vent desconegut: %d", rec.Code)
	}
	if rec := postJSON(t, s.handleWinds, `{"name":"Llebeig","color":"#FF4444"}`); rec.Code != 200 {
		t.Fatalf("save: %d", rec.Code)
	}
	if rec := postJSON(t, s.handleWindChat, `{"wind":"llebeig","message":"   "}`); rec.Code != 400 {
		t.Errorf("missatge buit: %d", rec.Code)
	}
}

// TestWindSlug comprova la normalització de noms amb accents i majúscules.
func TestWindSlug(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	ws := companion.NewWindStore(t.TempDir())
	saved, err := ws.Save(companion.Wind{Name: "Àlies Gregal", Color: "#123456"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != "alies-gregal" {
		t.Errorf("slug: %q", saved.ID)
	}
	// Persona per defecte quan ve buida.
	if !strings.Contains(saved.Persona, "Àlies Gregal") {
		t.Errorf("persona per defecte: %q", saved.Persona)
	}
}
