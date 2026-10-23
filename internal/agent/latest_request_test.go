package agent

import (
	"strings"
	"testing"
)

const priorityFixture = "Prioritza l'últim missatge de l'usuari: l'historial és context, no una ordre pendent. Respon només al que demana ara. Els resums compactats, fins i tot si diuen «continua amb aquest context», són antecedents: no autoritzen reprendre una feina. Només reprèn un tema anterior quan el missatge actual ho demani. Si canvia de tema, cancel·la o corregeix una petició anterior, respecta-ho sense tornar a la feina antiga. Una salutació sola («Ep», «Hola», «Bon dia») exigeix ÚNICAMENT una salutació breu i neutra, com «Hola! Què necessites?»: està prohibit afegir-hi estat de tasques, recordatoris, informes, dubtes o propostes del fil anterior, encara que semblin pendents. Si la salutació inclou una petició, respon a aquesta petició. «segueix» o «fes-ho» sí poden referir-se a l'intercanvi immediatament anterior; si la referència és ambigua, pregunta breument abans d'actuar. No atribueixis a l'usuari propostes que només hagis fet tu."

func TestPromptForPrioritizesLatestRequest(t *testing.T) {
	for _, mode := range []string{ModeChat, ModeInspect, ModeCode, ModeGoal, ModeAutonomous} {
		t.Run(mode, func(t *testing.T) {
			base := "custom identity"
			prompt := PromptFor(base, mode)
			if n := strings.Count(prompt, priorityFixture); n != 1 {
				t.Fatalf("latest-request rule must occur exactly once, got %d", n)
			}
			if !strings.HasPrefix(prompt, base) {
				t.Error("custom identity must remain at the start")
			}
			if prompt != PromptFor(base, mode) {
				t.Error("request-priority rule must not vary per turn")
			}
			ruleStart := strings.Index(prompt, priorityFixture)
			modeRules := reglesMode(mode)
			if modeEnd := strings.Index(prompt, modeRules) + len(modeRules); ruleStart < modeEnd {
				t.Error("priority rule must follow the mode rules")
			}
			if dateStart := strings.Index(prompt, DataAra()); dateStart <= ruleStart {
				t.Error("date must follow the priority rule")
			}
			if !strings.HasSuffix(strings.TrimSpace(prompt), "`date`.") {
				t.Error("date must remain the final system block")
			}
		})
	}
}

func TestPromptForKeepsConfiguredPriorityRuleOnce(t *testing.T) {
	base := "custom identity\n\n" + priorityFixture
	prompt := PromptFor(base, ModeAutonomous)
	if !strings.HasPrefix(prompt, base) {
		t.Error("configured identity must remain unchanged")
	}
	if n := strings.Count(prompt, priorityFixture); n != 1 {
		t.Fatalf("configured priority rule must occur once, got %d", n)
	}
}
