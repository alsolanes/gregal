package agent

import (
	"strings"
	"testing"

	"gregal/internal/config"
)

// La instrumentació només serveix si el motiu és honest: el que NO ha de
// passar és que «no calia» i «s'ha saltat» surtin iguals al registre.
func TestScoutReasonDiuPerQue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.Scout = "auto"

	if got := ScoutReason(cfg, ModeChat, "hola", ""); !strings.Contains(got, "mode chat") {
		t.Fatalf("en mode xat el motiu ha de dir que és xat, i diu %q", got)
	}

	// Sense workspace no es pot saber la mida: s'ha de dir, no inventar-se-la.
	if got := ScoutReason(cfg, ModeCode, "investiga l'arrel d'aquest error", ""); !strings.Contains(got, "sense workspace") {
		t.Fatalf("sense workspace el motiu ha de dir-ho, i diu %q", got)
	}

	// always mana sobre la mida del projecte.
	cfg.Agent.Scout = "always"
	if got := ScoutReason(cfg, ModeCode, "investiga l'arrel d'aquest error", t.TempDir()); !strings.Contains(got, "always") {
		t.Fatalf("amb scout always el motiu ha de ser always, i diu %q", got)
	}

	// Un projecte petit s'ha de veure a la xifra, que és el que es vol mesurar.
	cfg2 := &config.Config{}
	cfg2.Agent.Scout = "auto"
	petit := t.TempDir()
	if got := ScoutReason(cfg2, ModeCode, "investiga l'arrel d'aquest error", petit); got == "" {
		t.Fatal("el motiu no pot ser buit")
	}
}
