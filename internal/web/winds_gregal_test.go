package web

import (
	"context"
	"testing"
)

// B14: Gregal (l'assistent principal) existeix sempre, hereta el nom i el fil
// del Ventet, no surt com a especialista i el seu torn té l'eina wind.
func TestGregalMainWind(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Marinada","color":"#EFC69A","role":"compres"}`)
	capMu.Lock()
	st := s.loadCap()
	st.History = append(st.History, capMsg{From: "user", Text: "hola", Ts: 1}, capMsg{From: "cap", Text: "Hola!", Ts: 2})
	s.saveCap(st)
	capMu.Unlock()

	w, ok := s.ensureGregalWind()
	if !ok || w.ID != gregalWindID || w.Name != "Gregal" {
		t.Fatalf("Gregal: %+v %v", w, ok)
	}
	if again, _ := s.ensureGregalWind(); again.ID != w.ID {
		t.Fatal("no s'ha de crear dues vegades")
	}
	hist := windHistory(s, gregalWindID)
	hist.mu.Lock()
	n := len(hist.messages)
	hist.mu.Unlock()
	if n != 2 {
		t.Fatalf("l'historial del Ventet ha de passar al fil de Gregal: %d missatges", n)
	}
	for _, x := range mustWinds(s) {
		if x.ID == gregalWindID {
			t.Fatal("Gregal no es consulta a si mateix")
		}
	}
	for _, c := range s.windSpecialists("compres") {
		if c.ID == gregalWindID {
			t.Fatal("Gregal no surt com a especialista")
		}
	}
	main := s.withWindSteerKey(context.Background(), "wind", gregalWindID)
	other := s.withWindSteerKey(context.Background(), "wind", "marinada")
	if !windIsMainCtx(main) || windIsMainCtx(other) {
		t.Fatal("només el torn de Gregal és el principal")
	}
}
