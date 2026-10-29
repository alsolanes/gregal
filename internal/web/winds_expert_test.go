package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
)

// L'estudi desa un coneixement real al perfil del vent i reporta el progrés.
func TestExpertDesaConeixement(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	oldLoop := expertLoopActual()
	defer func() { setExpertLoop(oldLoop) }()
	setExpertLoop(func(s *Server, ctx context.Context, brief string, onPas func(int, string)) (exploracio, error) {
		return exploracio{text: "---\nnom: barca-basquet\ntitol: Barça de bàsquet\ndescripcio: El que he après\n---\nEl Barça de bàsquet juga a la Palau Blaugrana. Fonts: [ACB](https://acb.com).\n---END---"}, nil
	})
	s, _ := companionTestServer(t)
	s.winds = companion.NewWindStore(t.TempDir())
	if _, err := s.winds.Save(companion.Wind{Name: "Expertí", Role: "música"}); err != nil {
		t.Fatal(err)
	}

	// POST engega
	b := strings.NewReader(`{"wind":"Expertí","tema":"bàsquet del Barça"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/winds/expert", b)
	rec := httptest.NewRecorder()
	s.handleWindsExpert(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d", rec.Code)
	}
	for i := 0; i < 50; i++ {
		e := expertDe("experti")
		if e != nil && faseDe(e) == expertFaseFet {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	e := expertDe("experti")
	if e == nil || faseDe(e) != expertFaseFet {
		t.Fatalf("fase final = %v", e)
	}

	// el coneixement és al perfil del vent
	dir := s.windSkillsDir("experti")
	raw, err := os.ReadFile(filepath.Join(dir, "barca-basquet.md"))
	if err != nil {
		t.Fatalf("skill desat: %v", err)
	}
	if !strings.Contains(string(raw), "Palau Blaugrana") {
		t.Errorf("cos sense el contingut: %s", raw)
	}

	// GET reporta l'estat
	req2 := httptest.NewRequest(http.MethodGet, "/api/winds/expert?wind=experti", nil)
	rec2 := httptest.NewRecorder()
	s.handleWindsExpert(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET = %d", rec2.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got["fase"] != expertFaseFet {
		t.Errorf("fase via GET = %v", got["fase"])
	}

	// segon estudi mentre un és en marxa: rebutjat
	expertMu.Lock()
	expertActius["experti"] = &expertEstat{wind: "experti", tema: "x", fase: expertFaseCerca}
	expertMu.Unlock()
	req3 := httptest.NewRequest(http.MethodPost, "/api/winds/expert", strings.NewReader(`{"wind":"Expertí","tema":"rebuild"}`))
	rec3 := httptest.NewRecorder()
	s.handleWindsExpert(rec3, req3)
	if rec3.Code != http.StatusConflict {
		t.Errorf("segon estudi = %d, volia 409", rec3.Code)
	}
}

// extrauSkill: front matter, ---END--- i tanques de codi.
func TestExtrauSkill(t *testing.T) {
	nom, titol, desc, cos, ok := extrauSkill("blabla\n\x60\x60\x60markdown\n---\nnom: futbol\ntitol: Futbol\ndescripcio: Regles\n---\nCos del coneixement.\n---END---")
	if !ok || nom != "futbol" || titol != "Futbol" || desc != "Regles" || !strings.Contains(cos, "Cos del coneixement") {
		t.Errorf("extrauSkill = %q %q %q %q ok=%v", nom, titol, desc, cos, ok)
	}
}

func faseDe(e *expertEstat) string {
	fase, _ := e.estat()
	return fase
}
