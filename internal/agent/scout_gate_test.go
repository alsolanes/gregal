package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gregal/internal/config"
)

func TestScoutWantedInSaltaElsProjectesPetits(t *testing.T) {
	task := "Implement the missing endpoint described in TASK.md and make the tests in server_test.go pass"
	cfg := &config.Config{}
	if !ScoutWanted(cfg, ModeCode, task) {
		t.Fatal("la tasca hauria de demanar investigar")
	}
	petit := t.TempDir()
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(petit, fmt.Sprintf("f%d.go", i)), []byte("package x\n"), 0o644)
	}
	if ScoutWantedIn(cfg, ModeCode, task, petit) {
		t.Fatal("projecte de 5 fitxers: no hi hauria d'haver scout")
	}
	gran := t.TempDir()
	for i := 0; i < scoutMinFitxers+5; i++ {
		os.WriteFile(filepath.Join(gran, fmt.Sprintf("f%d.go", i)), []byte("package x\n"), 0o644)
	}
	if !ScoutWantedIn(cfg, ModeCode, task, gran) {
		t.Fatal("projecte gran: el scout hi ha de ser")
	}
	cfg.Agent.Scout = "always"
	if !ScoutWantedIn(cfg, ModeCode, task, petit) {
		t.Fatal("scout: always l'ha d'engegar també en un projecte petit")
	}
	cfg.Agent.Scout = "off"
	if ScoutWantedIn(cfg, ModeCode, task, gran) {
		t.Fatal("scout: off no l'ha d'engegar mai")
	}
}
