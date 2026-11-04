package web

import (
	"strings"
	"testing"
)

// Després d'una volta rebutjada, el fil ha de dir què queda obert, no només
// «La volta de revisió ha acabat»: l'usuari ha de veure si la colla hi està
// d'acord i què falta.
func TestCrewRoundEndPublishesOpenIssues(t *testing.T) {
	say := func(c, e string) string { return c }
	verdict := say("La volta de revisió ha acabat.", "The review round has ended.")
	answer := "builder wrote main.py\n\nOutstanding review feedback:\nLa funció demanada no valida l'entrada negativa."
	msg := crewRoundVerdict(say, verdict, answer, true)
	if !strings.Contains(msg, "Queda obert") && !strings.Contains(msg, "Still open") {
		t.Fatalf("el missatge de fi de volta no diu què queda obert: %q", msg)
	}
	if !strings.Contains(msg, "valida l'entrada negativa") {
		t.Fatalf("el missatge no porta els problemes oberts de la revisió: %q", msg)
	}
	// Aprovada: el missatge ha de dir que estan d'acord, no només «ha acabat».
	answerOK := "builder wrote main.py"
	msg = crewRoundVerdict(say, verdict, answerOK, false)
	if !strings.Contains(msg, "d'acord") && !strings.Contains(msg, "agree") {
		t.Fatalf("volta aprovada: no diu que la colla hi està d'acord: %q", msg)
	}
}
