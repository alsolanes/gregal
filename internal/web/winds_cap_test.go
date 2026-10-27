package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

func capPost(t *testing.T, s *Server, body string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleCap(rec, httptest.NewRequest(http.MethodPost, "/api/winds/cap", bytes.NewBufferString(body)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCapDelega: la mascota passa la feina a qui toca, i l'usuari ho pot canviar.
func TestCapDelega(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	mar, _ := s.winds.Save(companion.Wind{Name: "Marinada", Role: "compres i receptes"})
	gar, _ := s.winds.Save(companion.Wind{Name: "Garbí", Role: "esports"})
	calls := 0
	reply := ""
	old := capDecider
	t.Cleanup(func() { capDecider = old })
	capDecider = func(_ *Server, _ context.Context, msgs []llm.Message) (string, error) {
		calls++
		return reply, nil
	}

	// Un vent nomenat va directe, sense model.
	out := capPost(t, s, `{"message":"Garbí, com ha quedat el Barça?","lang":"ca"}`)
	if out["action"] != "wind" || out["winds"].([]any)[0] != gar.ID || calls != 0 {
		t.Fatalf("nomenat: %v (crides %d)", out, calls)
	}
	if !strings.Contains(out["reply"].(string), "Garbí") {
		t.Fatalf("frase d'entrega sense nom: %v", out["reply"])
	}

	// El model tria un vent; la mascota ho diu.
	reply = `{"action":"wind","winds":["` + mar.ID + `"],"reply":"Això és cosa de la Marinada 🛒"}`
	out = capPost(t, s, `{"message":"què faig per sopar?","lang":"ca"}`)
	if out["action"] != "wind" || out["winds"].([]any)[0] != mar.ID {
		t.Fatalf("delega: %v", out)
	}

	// L'usuari prefereix el Garbí: la decisió nova substitueix l'anterior al fil.
	out = capPost(t, s, `{"message":"què faig per sopar?","lang":"ca","force":"wind:`+gar.ID+`"}`)
	if out["action"] != "wind" || out["winds"].([]any)[0] != gar.ID {
		t.Fatalf("canvi: %v", out)
	}
	hist := out["history"].([]any)
	if n := len(hist); n != 4 || hist[n-2].(map[string]any)["text"] != "què faig per sopar?" {
		t.Fatalf("el fil repeteix la pregunta: %v", hist)
	}

	// Un id inventat no passa: ho respon ella.
	reply = `{"action":"wind","winds":["xyz"],"reply":""}`
	out = capPost(t, s, `{"message":"hola","lang":"ca"}`)
	if out["action"] != "self" || out["reply"] == "" {
		t.Fatalf("id inventat: %v", out)
	}

	// Feina llarga: colla, amb l'encàrrec.
	reply = `{"action":"crew","task":"Una web de receptes de tardor","reply":"Munto una colla 🛠️"}`
	out = capPost(t, s, `{"message":"fes-me una web de receptes de tardor","lang":"ca"}`)
	if out["action"] != "crew" || out["task"] != "Una web de receptes de tardor" {
		t.Fatalf("colla: %v", out)
	}

	// Prosa sense JSON = resposta seva.
	reply = "Bon dia! Avui fa sol."
	out = capPost(t, s, `{"message":"bon dia","lang":"ca"}`)
	if out["action"] != "self" || out["reply"] != "Bon dia! Avui fa sol." {
		t.Fatalf("prosa: %v", out)
	}

	// Rotllana amb els mateixos vents: reaprofita la que ja hi ha.
	rot, err := s.saveRotllana(Rotllana{Name: "Sopar", Members: []string{gar.ID, mar.ID}})
	if err != nil {
		t.Fatal(err)
	}
	reply = `{"action":"rotllana","winds":["` + mar.ID + `","` + gar.ID + `"],"reply":""}`
	out = capPost(t, s, `{"message":"sopar i partit?","lang":"ca"}`)
	if out["action"] != "rotllana" || out["rotllana"] != rot.ID || out["reply"] == "" {
		t.Fatalf("rotllana: %v", out)
	}
}

// TestCapResum: en tornar, la mascota diu què ha passat des de l'última visita.
func TestCapResum(t *testing.T) {
	if got := capBriefText(capBrief{Done: 1, Titles: []string{"La web"}, Running: 2}, true); got != "Ja està llesta «La web», 2 feines en marxa." {
		t.Fatalf("ca: %q", got)
	}
	if got := capBriefText(capBrief{}, false); got != "All quiet. What do you need?" {
		t.Fatalf("en: %q", got)
	}
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	rec := httptest.NewRecorder()
	s.handleCap(rec, httptest.NewRequest(http.MethodGet, "/api/winds/cap?lang=ca", nil))
	var out struct {
		Name  string   `json:"name"`
		Brief capBrief `json:"brief"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Name != capName || out.Brief.Text == "" {
		t.Fatalf("%v %s", err, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleCapProfile(rec, httptest.NewRequest(http.MethodPost, "/api/winds/cap/profile", bytes.NewBufferString(`{"name":"Bufi"}`)))
	if !strings.Contains(rec.Body.String(), `"Bufi"`) {
		t.Fatalf("nom: %s", rec.Body.String())
	}
}
