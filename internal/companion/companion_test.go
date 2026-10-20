package companion

import (
	"strings"
	"testing"

	"gregal/internal/llm"
)

func TestExtractPersonalFiltersAndCaps(t *testing.T) {
	// El model proposa 3 fets: un bo, un secret (prohibit), un duplicat.
	raw := `{"facts": [
		"Fa anys que treballa amb models locals al Strix Halo",
		"La seva clau API és sk-abc123",
		"Fa anys que treballa amb models locals al Strix Halo",
		"Prefereix respostes curtes i directes"
	]}`
	facts := ParseExtract(raw, nil)
	if len(facts) != 2 {
		t.Fatalf("esperava 2 fets vàlids, got %d: %v", len(facts), facts)
	}
	for _, f := range facts {
		if strings.Contains(strings.ToLower(f), "sk-") {
			t.Errorf("un secret ha passat el filtre: %s", f)
		}
	}
}

func TestExtractPersonalDedupesAgainstExisting(t *testing.T) {
	raw := `{"facts": ["Prefereix respostes curtes i directes", "Té dos fills petits"]}`
	existing := []string{"- 2026-10-19: prefereix respostes curtes i directes"}
	facts := ParseExtract(raw, existing)
	if len(facts) != 1 {
		t.Fatalf("esperava 1 fet nou (dedupe), got %d: %v", len(facts), facts)
	}
	if !strings.Contains(facts[0], "fills") {
		t.Errorf("el fet retingut hauria de ser el nou, got %s", facts[0])
	}
}

func TestExtractPersonalRejectsEmptyAndLong(t *testing.T) {
	long := strings.Repeat("a", 300)
	raw := `{"facts": ["", "` + long + `", "Això sí"]}`
	facts := ParseExtract(raw, nil)
	if len(facts) != 1 || !strings.Contains(facts[0], "Això sí") {
		t.Fatalf("esperava només el fet curt, got %v", facts)
	}
}

func TestCheckinPromptContainsMemoryAndSession(t *testing.T) {
	mem := "Em dic Aleix. Projecte Marea."
	digest := "Avui hem parlat del selector de models."
	p := CheckinPrompt(mem, digest)
	if !strings.Contains(p, "Aleix") || !strings.Contains(p, "selector") {
		t.Errorf("el prompt de check-in ha de dur la memòria i la sessió")
	}
	if !strings.Contains(p, "check-in") {
		t.Errorf("el prompt ha de dir què és (check-in)")
	}
}

func TestCheckinPromptEmptyMemoryStillWorks(t *testing.T) {
	p := CheckinPrompt("", "hem parlat de x")
	if p == "" {
		t.Fatal("prompt buit amb memòria buida")
	}
}

func TestSuggestFromChatTurnSkipsShortTurns(t *testing.T) {
	// Un torn de 2 missatges curts no mereix extracció: cost per res.
	hist := []llm.Message{
		{Role: "user", Content: "hola"},
		{Role: "assistant", Content: "ep!"},
	}
	if ShouldExtract(hist) {
		t.Error("un torn curt no hauria de proposar fets")
	}
	// Un torn real amb contingut personal, sí.
	hist = append(hist,
		llm.Message{Role: "user", Content: "Resulta que m'he comprat una bicicleta elèctrica i la faig servir cada dia per anar a la feó, que és a 10 km"},
		llm.Message{Role: "assistant", Content: "Bona compra! Una e-bike per 10 km diaris és ideal."},
	)
	if !ShouldExtract(hist) {
		t.Error("un torn amb contingut personal hauria de proposar fets")
	}
}

func TestCheckinKeepsShort(t *testing.T) {
	// El check-in és una notificació: ha de ser curt per defecte.
	if CheckinMaxChars > 600 {
		t.Errorf("un check-in de %d caràcters és massa llarg per una notificació", CheckinMaxChars)
	}
}
