package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func namesGet(t *testing.T, s *Server, q string) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsNames(rec, httptest.NewRequest(http.MethodGet, "/api/winds/names"+q, nil))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Names []string `json:"names"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Names
}

func setCollective(t *testing.T, s *Server, sing, plur string) {
	t.Helper()
	rec := httptest.NewRecorder()
	body := `{"singular":"` + sing + `","plural":"` + plur + `"}`
	s.handleWindsCollective(rec, httptest.NewRequest(http.MethodPost, "/api/winds/collective", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

// Sense triar res, noms de vent però no els de la rosa dels vents de sempre;
// amb «fullets», noms de fullet; mai un que ja porta algun ajudant.
func TestNamesFollowTheCollective(t *testing.T) {
	s := rutinaTestServer(t)
	old := namesGenerator
	namesGenerator = func(*Server, context.Context, string) (string, error) { t.Fatal("presets must not call the model"); return "", nil }
	t.Cleanup(func() { waitBackground(); namesGenerator = old })

	for _, n := range namesGet(t, s, "?n=24&lang=ca") {
		if n == "Tramuntana" || n == "Xaloc" {
			t.Fatalf("massa previsible: %s", n)
		}
	}
	setCollective(t, s, "Fullet", "Fullets")
	got := namesGet(t, s, "?n=24&lang=ca")
	if len(got) < 10 || !containsName(namePresets["fullets"]["ca"], got[0]) {
		t.Fatalf("noms de fullet: %v", got)
	}
	if _, err := s.winds.Save(windNamed("", "Bolet")); err != nil {
		t.Fatal(err)
	}
	for _, n := range namesGet(t, s, "?n=24&lang=ca") {
		if n == "Bolet" {
			t.Fatal("un nom ja fet servir no es torna a proposar")
		}
	}
	if c := s.loadCollective(); c.Plural != "Fullets" || !c.Asked {
		t.Fatal("el col·lectiu es desa al servidor")
	}
}

// Un col·lectiu inventat demana els noms al model una sola vegada (memòria
// cau); si el model no respon, noms neutres, mai noms de vent.
func TestCustomCollectiveAsksTheModelOnce(t *testing.T) {
	s := rutinaTestServer(t)
	calls := 0
	old := namesGenerator
	namesGenerator = func(_ *Server, _ context.Context, prompt string) (string, error) {
		calls++
		if !strings.Contains(prompt, "\"pirates\"") || !strings.Contains(prompt, "existing character") {
			t.Errorf("prompt: %s", prompt)
		}
		return `{"names":["Barbaroja Petita","Calma","Àncora","Gavina","Escuma","Timó","Brúixola","Corall"]}`, nil
	}
	t.Cleanup(func() { waitBackground(); namesGenerator = old })
	setCollective(t, s, "pirata", "pirates")
	waitBackground()
	before := calls
	got := namesGet(t, s, "?n=8&lang=ca")
	if len(got) != 7 || !containsName(got, "Timó") || containsName(got, "Barbaroja Petita") {
		t.Fatalf("noms: %v", got)
	}
	if calls != before {
		t.Fatalf("la segona vegada ha de sortir de la memòria cau (%d → %d)", before, calls)
	}
	namesGenerator = func(*Server, context.Context, string) (string, error) { return "no json", nil }
	setCollective(t, s, "drac", "dracs")
	waitBackground()
	if got := namesGet(t, s, "?lang=en"); !containsName(neutralNames["en"], got[0]) {
		t.Fatalf("reserva neutra: %v", got)
	}
}

// Les colles i les colles fetes prenen els noms del col·lectiu.
func TestCrewAndSquadNamesUseTheCollective(t *testing.T) {
	s := rutinaTestServer(t)
	setCollective(t, s, "Minairó", "Minairons")
	got := crewSanitise(crewProposal{Create: []crewNew{{Role: "cuina"}}}, nil, "receptes", true, s.crewNamePool(context.Background(), true)...)
	if len(got.Create) == 0 || !containsName(namePresets["minairons"]["ca"], got.Create[0].Name) {
		t.Fatalf("colla: %+v", got.Create)
	}
	if rule := s.collectiveNameRule("Catalan"); !strings.Contains(rule, "Minairons") {
		t.Fatalf("regla: %s", rule)
	}
	rec := httptest.NewRecorder()
	s.handleWindSquads(rec, httptest.NewRequest(http.MethodPost, "/api/winds/squads", strings.NewReader(`{"id":"`+windSquadTemplates[0].ID+`","lang":"ca"}`)))
	var out struct {
		Added []struct{ Name string } `json:"added"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Added) == 0 {
		t.Fatalf("colla feta: %s", rec.Body.String())
	}
	for _, a := range out.Added {
		if !containsName(namePresets["minairons"]["ca"], a.Name) {
			t.Fatalf("nom de la plantilla en lloc d'un minairó: %s", a.Name)
		}
	}
	if sys := windSuggestSystem("Pessic", "cuina", "Minairons"); strings.Contains(sys, "compass") || !strings.Contains(sys, "Minairons") {
		t.Fatalf("persona: %s", sys)
	}
}

func containsName(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
