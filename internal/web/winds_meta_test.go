package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
)

// Les preferències d'organització (favorits i seccions) es desen al
// winds.json: mateixa llista al mòbil i a la web. Aquí provem l'API:
// llegir buit, desar, validar claus i no perdre els vents en desar.

func metaTestServer(t *testing.T) *Server {
	t.Helper()
	s, _ := companionTestServer(t)
	s.winds = companion.NewWindStore(t.TempDir())
	if _, err := s.winds.Save(companion.Wind{Name: "Xaloc", Color: "#EFA68F", Role: "Música i descobertes"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func metaCall(t *testing.T, s *Server, method string, body map[string]any, v any) int {
	t.Helper()
	var rec *httptest.ResponseRecorder
	if method == http.MethodGet {
		rec = httptest.NewRecorder()
		s.handleWindsMeta(rec, httptest.NewRequest(http.MethodGet, "/api/winds/meta", nil))
	} else {
		raw, _ := json.Marshal(body)
		rec = httptest.NewRecorder()
		s.handleWindsMeta(rec, httptest.NewRequest(http.MethodPost, "/api/winds/meta", strings.NewReader(string(raw))))
	}
	if v != nil && rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("resposta no vàlida: %v", err)
		}
	}
	return rec.Code
}

func TestMetaEmptyByDefault(t *testing.T) {
	s := metaTestServer(t)
	var out struct {
		Meta companion.WindMeta `json:"meta"`
	}
	if code := metaCall(t, s, http.MethodGet, nil, &out); code != 200 {
		t.Fatalf("GET = %d, esperava 200", code)
	}
	if len(out.Meta.Favorites) != 0 || len(out.Meta.Sections) != 0 {
		t.Fatalf("esperava meta buida, té %d favorits i %d seccions", len(out.Meta.Favorites), len(out.Meta.Sections))
	}
}

func TestMetaSaveAndRoundTrip(t *testing.T) {
	s := metaTestServer(t)
	in := map[string]any{
		"favorites": []string{"wind:xaloc", "rot:r1", "wind:xaloc", "bogus:", ""},
		"sections": []map[string]any{{
			"id":    "feina",
			"name":  "Feina",
			"items": []string{"wind:xaloc", "rot:r1", "wind:xaloc"},
		}},
	}
	var saved struct {
		Meta companion.WindMeta `json:"meta"`
	}
	if code := metaCall(t, s, http.MethodPost, in, &saved); code != 200 {
		t.Fatalf("POST = %d, esperava 200", code)
	}
	if len(saved.Meta.Favorites) != 2 {
		t.Fatalf("favorits: esperava 2 (dedup + claus vàlides), té %d: %v", len(saved.Meta.Favorites), saved.Meta.Favorites)
	}
	if len(saved.Meta.Sections) != 1 || len(saved.Meta.Sections[0].Items) != 2 {
		t.Fatalf("secció: esperava 1 amb 2 ítems, té %+v", saved.Meta.Sections)
	}
	// Round-trip: el que s'ha desat és el que es llegeix.
	var out struct {
		Meta companion.WindMeta `json:"meta"`
	}
	if code := metaCall(t, s, http.MethodGet, nil, &out); code != 200 || len(out.Meta.Favorites) != 2 || len(out.Meta.Sections) != 1 {
		t.Fatalf("round-trip: GET = %d, meta = %+v", code, out.Meta)
	}
	// Els vents segueixen intactes després de desar meta.
	winds, err := s.winds.List()
	if err != nil || len(winds) != 1 || winds[0].Name != "Xaloc" {
		t.Fatalf("vents canviats després de desar meta: %v (%v)", winds, err)
	}
}

func TestMetaRejectsInvalid(t *testing.T) {
	s := metaTestServer(t)
	// Secció sense nom → 400.
	if code := metaCall(t, s, http.MethodPost, map[string]any{"sections": []map[string]any{{"id": "x", "items": nil}}}, nil); code != 400 {
		t.Fatalf("secció sense nom = %d, esperava 400", code)
	}
	// Ids de secció repetits → 400.
	if code := metaCall(t, s, http.MethodPost, map[string]any{"sections": []map[string]any{
		{"id": "a", "name": "A", "items": nil}, {"id": "a", "name": "B", "items": nil},
	}}, nil); code != 400 {
		t.Fatalf("ids repetits = %d, esperava 400", code)
	}
	// Només POST/GET.
	rec := httptest.NewRecorder()
	s.handleWindsMeta(rec, httptest.NewRequest(http.MethodPut, "/api/winds/meta", nil))
	if rec.Code != 405 {
		t.Fatalf("PUT = %d, esperava 405", rec.Code)
	}
}
