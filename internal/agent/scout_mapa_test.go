package agent

import (
	"context"
	"strings"
	"testing"
)

// El scout de codi ha de rebre el mapa del projecte (llistat de fitxers,
// estat de git i pistes de símbols) calculat SENSE model. Sense això el
// primer pas se li anava a demanar el mapa i, amb el model local, cada pas
// són ~10-12 s abans que el coordinador arrenqui (mesurat: 45-55 s per torn).
func TestScoutDeCodiRepElMapaDelProjecte(t *testing.T) {
	dir := projecteDe(t, scoutMinFitxers+5)
	var gotPrompt string
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		gotPrompt = prompt
		def := SubagentFromContext(ctx)
		if def == nil || def.Name != ScoutAgentName {
			t.Fatalf("fill esperat scout, no %+v", def)
		}
		if def.MaxSteps != scoutMaxSteps {
			t.Fatalf("el scout ha de tenir %d passos, no %d", scoutMaxSteps, def.MaxSteps)
		}
		return "brief amb camins", nil
	})
	res := RunScout(context.Background(), ScoutOptions{
		Task:      "why does the login fail on the server?",
		Workspace: dir,
		Mode:      ModeCode,
	})
	if res.Err != nil || res.Brief != "brief amb camins" {
		t.Fatalf("resultat: %+v", res)
	}
	for _, want := range []string{"f0.go", "do not spend a step asking for them", "why does the login fail on the server?"} {
		if !strings.Contains(gotPrompt, want) {
			t.Fatalf("al prompt del scout hi falta %q:\n%s", want, gotPrompt)
		}
	}
}

// El pressupost ha de quedar curt: el torn no pot esperar més de mig minut
// per un resum que és opcional.
func TestScoutPressupostCurt(t *testing.T) {
	if scoutMaxSteps > 2 {
		t.Fatalf("scoutMaxSteps = %d; el torn no pot esperar tants passos de model amb un sol fil", scoutMaxSteps)
	}
	if scoutTimeout.Seconds() > 25 {
		t.Fatalf("scoutTimeout = %s; un scout encallat no ha de menjar-se mig minut del torn", scoutTimeout)
	}
}
