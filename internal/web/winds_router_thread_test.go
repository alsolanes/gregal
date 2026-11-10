package web

import (
	"context"
	"testing"

	"gregal/internal/llm"
)

// Una resposta curta a un vent («sí, compara'ls», una proposta seva) la
// continua qui portava el fil, no el membre que menys ha parlat: abans
// responia un altre vent per dir que allò no era seu.
func TestRotllanaFollowUpStaysWithTheLastSpeaker(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	rot, ok := s.rotllanaByID(id)
	if !ok {
		t.Fatal("no rotllana")
	}
	tail := []llm.Message{
		{Role: "user", Content: "quins auriculars em compro?"},
		{Role: "assistant", SpeakerID: "xaloc", Content: "Mira aquests tres models."},
	}
	got := s.routeRotllanaSpeakers(context.Background(), rot, "", msgFollowUp, tail)
	if len(got) != 1 || got[0].ID != "xaloc" {
		t.Fatalf("la proposta l'havia de continuar el Xaloc, no %v", got)
	}
	// Si un altre membre hi encaixa clarament (el seu ofici), el fil canvia de veu.
	got = s.routeRotllanaSpeakers(context.Background(), rot, "", "i per sopar, una recepta de cuina?", tail)
	if len(got) != 1 || got[0].ID != "llevant" {
		t.Fatalf("una pregunta de cuina l'havia de respondre el Llevant, no %v", got)
	}
}

const msgFollowUp = "Sí, compara'ls"
