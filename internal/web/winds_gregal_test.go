package web

import (
	"context"
	"strings"
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

// Gregal sap a cada torn qui té a l'equip i de què en sap (el «cap de gabinet»):
// sense la llista no consultava ningú i la interfície et feia anar a parlar amb
// l'especialista. Els altres vents no la reben.
func TestGregalTeamBriefListsTheSpecialists(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevantada","color":"#EFC69A","role":"Cuina i receptes"}`)
	g, ok := s.ensureGregalWind()
	if !ok {
		t.Fatal("sense Gregal")
	}
	brief := s.gregalTeamBrief(g)
	if !strings.Contains(brief, "YOUR TEAM") || !strings.Contains(brief, "Llevantada") || !strings.Contains(brief, "Cuina i receptes") {
		t.Fatalf("falta l'equip:\n%s", brief)
	}
	if strings.Contains(brief, "- Gregal") {
		t.Fatal("Gregal no s'ha de llistar a si mateix")
	}
	for _, w := range mustWinds(s) {
		if w.ID != gregalWindID && s.gregalTeamBrief(w) != "" {
			t.Fatalf("%s no és Gregal i ha rebut l'equip", w.Name)
		}
	}
}
