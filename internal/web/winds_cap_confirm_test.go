package web

import (
	"context"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// «Endavant» a una proposta de la mascota la fa, amb la petició original;
// abans anava al model com un missatge nou i no passava res.
func TestCapEndavantFaLaProposta(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	mar, _ := s.winds.Save(companion.Wind{Name: "Marinada", Role: "compres i receptes"})
	calls := 0
	reply := ""
	old := capDecider
	t.Cleanup(func() { capDecider = old })
	capDecider = func(_ *Server, _ context.Context, _ []llm.Message) (string, error) {
		calls++
		return reply, nil
	}

	reply = `{"action":"crew","task":"Fer una web de receptes amb buscador","reply":"Això és feina per a una colla 🛠️"}`
	out := capPost(t, s, `{"message":"fes-me una web de receptes","lang":"ca"}`)
	if out["action"] != "crew" || calls != 1 {
		t.Fatalf("proposta: %v", out)
	}
	out = capPost(t, s, `{"message":"Endavant, crea-la!","lang":"ca"}`)
	if out["action"] != "crew" || out["confirm"] != true || out["task"] != "Fer una web de receptes amb buscador" ||
		out["message"] != "fes-me una web de receptes" || calls != 1 {
		t.Fatalf("endavant colla: %v (crides %d)", out, calls)
	}

	reply = `{"action":"wind","winds":["` + mar.ID + `"],"reply":"Això és cosa de la Marinada"}`
	capPost(t, s, `{"message":"què faig per sopar?","lang":"ca"}`)
	out = capPost(t, s, `{"message":"sí","lang":"ca"}`)
	if out["action"] != "wind" || out["confirm"] != true || out["message"] != "què faig per sopar?" || out["winds"].([]any)[0] != mar.ID {
		t.Fatalf("sí al vent: %v", out)
	}

	// Un «sí, però…» porta informació nova: va al model.
	reply = `{"action":"self","reply":"D'acord"}`
	before := calls
	out = capPost(t, s, `{"message":"sí, però que sigui vegana","lang":"ca"}`)
	if out["confirm"] == true || calls != before+1 {
		t.Fatalf("sí amb condicions no és un endavant: %v", out)
	}
}

func TestCapIsConfirm(t *testing.T) {
	for _, m := range []string{"endavant", "Sí!", "som-hi", "d'acord, fes-ho", "ok crea la colla", "go ahead", "yes please", "Endavant, crea-la"} {
		if !capIsConfirm(m) {
			t.Errorf("%q hauria de ser un endavant", m)
		}
	}
	for _, m := range []string{"", "no", "fes-me una web", "sí, però en anglès", "què és una colla?", "endavant amb la part de pagaments i després ja veurem"} {
		if capIsConfirm(m) {
			t.Errorf("%q no és un endavant", m)
		}
	}
}
