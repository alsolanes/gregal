package web

import (
	"context"
	"strings"
	"testing"
	"time"
)

func stubHeartbeat(t *testing.T, replies ...string) *[]string {
	t.Helper()
	var briefs []string
	old := windHeartbeatLoop
	i := 0
	windHeartbeatLoop = func(_ *Server, _ context.Context, brief string) (exploracio, error) {
		briefs = append(briefs, brief)
		r := replies[min(i, len(replies)-1)]
		i++
		return exploracio{text: r}, nil
	}
	t.Cleanup(func() { waitBackground(); windHeartbeatLoop = old })
	return &briefs
}

// «Quan acabi el partit avisa'm del resultat»: l'avís porta una feina i a
// l'hora el vent fa un torn de debò i diu el resultat.
func TestActionReminderRunsARealTurn(t *testing.T) {
	s := remindTestServer(t)
	briefs := stubHeartbeat(t, "El Barça ha guanyat 2-1.")
	j, err := saveWindReminderTask(s, "xaloc", "+30m", "Resultat del partit", "Busca el resultat final del Barça-Madrid", 0)
	if err != nil {
		t.Fatal(err)
	}
	if j.Task == "" {
		t.Fatal("la feina no s'ha desat")
	}
	run, err := s.executeWindRemind(context.Background(), j)
	if err != nil || run.Status != "ok" {
		t.Fatalf("run = %+v err = %v", run, err)
	}
	if !strings.Contains((*briefs)[0], "Busca el resultat final") {
		t.Fatal("la consigna no porta la feina")
	}
	if run.Output != "Xaloc: El Barça ha guanyat 2-1." {
		t.Fatalf("output = %q", run.Output)
	}
	if list, _ := s.jobs.List(); list[0].Enabled {
		t.Fatal("hauria d'apagar-se un cop fet")
	}
}

// El partit encara no ha acabat: NOT_YET reprograma sense parlar; al final diu el resultat.
func TestActionReminderRetriesOnNotYet(t *testing.T) {
	s := remindTestServer(t)
	stubHeartbeat(t, "NOT_YET", "Final: 2-1.")
	j, err := saveWindReminderTask(s, "xaloc", "+30m", "", "Resultat del partit", 15)
	if err != nil {
		t.Fatal(err)
	}
	if j.MaxTries != remindMaxTries || j.RetryMin != 15 {
		t.Fatalf("reintents = %d/%d", j.RetryMin, j.MaxTries)
	}
	run, _ := s.executeWindRemind(context.Background(), j)
	if run.Output != "" {
		t.Fatalf("no hauria de parlar: %q", run.Output)
	}
	list, _ := s.jobs.List()
	if !list[0].Enabled || list[0].Tries != 1 || list[0].NextAt == nil || !list[0].NextAt.After(time.Now().Add(10*time.Minute)) {
		t.Fatalf("no s'ha reprogramat: %+v", list[0])
	}
	run, _ = s.executeWindRemind(context.Background(), list[0])
	if run.Output != "Xaloc: Final: 2-1." {
		t.Fatalf("output = %q", run.Output)
	}
	if list, _ := s.jobs.List(); list[0].Enabled {
		t.Fatal("hauria d'apagar-se")
	}
}

// Sense reintents, un NOT_YET no es perd: l'última passada sempre contesta.
func TestActionReminderLastTryAlwaysAnswers(t *testing.T) {
	s := remindTestServer(t)
	briefs := stubHeartbeat(t, "Encara no hi ha resultat publicat.")
	j, _ := saveWindReminderTask(s, "xaloc", "+30m", "x", "Mira-ho", 0)
	run, _ := s.executeWindRemind(context.Background(), j)
	if run.Output == "" || strings.Contains((*briefs)[0], "reply with exactly NOT_YET") {
		t.Fatal("l'última passada no pot tornar a ajornar")
	}
}
