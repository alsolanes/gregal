package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
)

// Un servidor que arrenca amb una colla a mig fer (estat disc/working però cap
// procés viu) ha de dir-ho al fil: sense aquest missatge, l'usuari només veu
// el debat plegat i cap conclusió.
func TestCrewStartupMarksInterruptedRuns(t *testing.T) {
	s, home := companionTestServer(t)
	a, _ := s.winds.Save(companion.Wind{Name: "Pixel", Role: "Constructor"})
	b, _ := s.winds.Save(companion.Wind{Name: "Tramuntana", Role: "Revisora"})
	rot, err := s.saveRotllana(Rotllana{Name: "colla tallada", Members: []string{a.ID, b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	st := &crewState{Task: "fes una web", Ca: true, Round: 2, Started: time.Now()}
	st.Waiting = false
	saveCrewState(s, rot.ID, st)
	// El fitxer de job diu "working": el procés va morir sense netejar.
	_ = os.MkdirAll(filepath.Dir(crewJobPath(s, rot.ID)), 0o700)
	_ = os.WriteFile(crewJobPath(s, rot.ID), []byte(`{"step":"working"}`), 0o600)

	s.crewRecoverInterrupted()

	hist := s.crewHistory(rot)
	var last string
	for _, m := range hist {
		if m.Role == "assistant" && m.Kind == "result" {
			last = m.Content
		}
	}
	if last == "" {
		t.Fatalf("cap missatge d'interrupció al fil: %d missatges", len(hist))
	}
	if !strings.Contains(last, "aturada") && !strings.Contains(last, "reinici") {
		t.Fatalf("el missatge no explica la interrupció: %q", last)
	}
	// I l'estat queda net: la rotllana no queda enLimbo.
	if _, ok := loadCrewState(s, rot.ID); ok {
		t.Fatalf("l'estat de la colla hauria de quedar esborrat")
	}
	if crewChatBusy(rot.ID) {
		t.Fatalf("la colla no hauria de quedar ocupada")
	}
	_ = home
}
