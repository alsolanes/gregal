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
func TestCrewWriteListPersistsPerSuccessfulWrite(t *testing.T) {
	s, id := rotllanaChatter(t, "chat")
	if err := saveCrewState(s, id, &crewState{Task: "task", Started: time.Now()}); err != nil {
		t.Fatal(err)
	}
	// noteCrewWriteDurable és el callback d'una escriptura real: el fitxer ha
	// d'existir al workspace, com en una volta de colla de veritat.
	written := filepath.Join(s.sessDir(), "project", "index.html")
	if err := os.MkdirAll(filepath.Dir(written), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(written, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := noteCrewWriteDurable(s, id, s.sessDir(), written); err != nil {
		t.Fatal(err)
	}
	st, ok := loadCrewState(s, id)
	if !ok || len(st.Files) != 1 || st.Files[0] != "project/index.html" {
		t.Fatalf("durable files = %+v, ok=%v", st, ok)
	}
}

func TestCrewStateWriteIsAtomicAndReportsIOErrors(t *testing.T) {
	s, id := rotllanaChatter(t, "chat")
	if err := saveCrewState(s, id, &crewState{Task: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := saveCrewState(s, id, &crewState{Task: "second"}); err != nil {
		t.Fatal(err)
	}
	st, ok := loadCrewState(s, id)
	if !ok || st.Task != "second" {
		t.Fatalf("state after atomic replace = %+v, ok=%v", st, ok)
	}
	if err := os.Remove(crewStatePath(s, id)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(crewStatePath(s, id), 0o700); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(crewStatePath(s, id))
	if err := saveCrewState(s, id, &crewState{Task: "cannot replace directory"}); err == nil {
		t.Fatal("state persistence failure was not returned")
	}
}

func TestCrewJobStepPersistsAtomicallyAndReportsFailure(t *testing.T) {
	s, id := rotllanaChatter(t, "chat")
	job := &crewJob{path: crewJobPath(s, id)}
	if err := job.set("working"); err != nil {
		t.Fatal(err)
	}
	if step := crewStepDesat(s, id); step != "interrupted" {
		t.Fatalf("working job on disk should recover as interrupted, got %q", step)
	}
	if err := os.Remove(job.path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(job.path, 0o700); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(job.path)
	if err := job.set("done"); err == nil {
		t.Fatal("job-state write failure was not reported")
	}
}

func TestCrewStartupMarksInterruptedRuns(t *testing.T) {
	s, home := companionTestServer(t)
	a, _ := s.winds.Save(companion.Wind{Name: "Pixel", Role: "Constructor"})
	b, _ := s.winds.Save(companion.Wind{Name: "Tramuntana", Role: "Revisora"})
	rot, err := s.saveRotllana(Rotllana{Name: "colla tallada", Members: []string{a.ID, b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	st := &crewState{Task: "fes una web", Ca: true, Round: 2, Started: time.Now(), Files: []string{"project/index.html"}}
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
	if !strings.Contains(last, "project/index.html") {
		t.Fatalf("recuperació ha perdut la llista de fitxers desada: %q", last)
	}
	// I l'estat queda net: la rotllana no queda enLimbo.
	if _, ok := loadCrewState(s, rot.ID); ok {
		t.Fatalf("l'estat de la colla hauria de quedar esborrat")
	}
	if crewChatBusyFor(s, rot.ID) {
		t.Fatalf("la colla no hauria de quedar ocupada")
	}
	_ = home
}
