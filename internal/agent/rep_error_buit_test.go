package agent

import (
	"errors"
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// El 400 buit de zen (cos sense error) amb històric gran s'ha de tractar
// com a context excedit: es retalla i el torn continua en lloc de morir.
func TestRepError400BuitRetallaIContinua(t *testing.T) {
	tools.TodoClear()
	project := t.TempDir()
	// Històric gran: molts missatges amb contingut per passar de 24k tokens.
	hist := []llm.Message{{Role: "user", Content: "feina"}}
	big := strings.Repeat("text del missatge ", 3000) // ~54KB ≈ 13k tokens
	for i := 0; i < 3; i++ {
		hist = append(hist, llm.Message{Role: "assistant", Content: big},
			llm.Message{Role: "user", Content: big})
	}
	tn := NouTorn(OpcionsTorn{
		Pol:  &Policy{ProjectDir: project},
		Mode: ModeAutonomous,
		Hist: hist,
	})
	if llm.EstimateTokens(tn.histAmbSys()) < 24_000 {
		t.Fatalf("l'històric de prova és massa petit: %d tokens", llm.EstimateTokens(tn.histAmbSys()))
	}
	abans := len(tn.Hist)
	evs := tn.RepPas("", nil, errors.New(`provider https://opencode.ai/zen/go/v1: HTTP 400: {"model":"deepseek-v4.1-flash"}`))
	// El torn NO mor: recupera retallant l'històric.
	mort := false
	for _, e := range evs {
		if e.Tipus == EvError {
			mort = true
		}
	}
	if mort {
		t.Fatalf("el torn ha mort amb el 400 buit: %+v", evs)
	}
	if len(tn.Hist) >= abans {
		t.Errorf("l'històric no s'ha retallat: %d → %d", abans, len(tn.Hist))
	}
	// Amb l'històric ja petit, el mateix 400 ja no es recupera: mor.
	if llm.EstimateTokens(tn.histAmbSys()) > 24_000 {
		t.Fatalf("encara gran després del retallat: %d", llm.EstimateTokens(tn.histAmbSys()))
	}
}

// El raonament del pas es penga al missatge d'assistant quan el pas porta
// crides d'eina: zen (deepseek en thinking) l'exigeix de tornada dins de la
// cadena i, sense ell, la petició següent mor amb 400.
func TestRepPasPenjaRaonament(t *testing.T) {
	calls := []llm.ToolCall{cridaProva("1", "read", `{"path":"x"}`)}
	tn := NouTorn(OpcionsTorn{
		Pol:  &Policy{ProjectDir: t.TempDir()},
		Mode: ModeAutonomous,
		Hist: []llm.Message{{Role: "user", Content: "a"}},
	})
	tn.RepPasRaonat("resposta", "penso-ho", calls, nil)
	if len(tn.Hist) != 2 {
		t.Fatalf("històric: %d", len(tn.Hist))
	}
	if tn.Hist[1].ReasoningContent != "penso-ho" {
		t.Errorf("el raonament no s'ha penjat: %q", tn.Hist[1].ReasoningContent)
	}
	// Una resposta final (sense crides) no el necessita: no s'hi posa.
	tn2 := NouTorn(OpcionsTorn{
		Pol:  &Policy{ProjectDir: t.TempDir()},
		Mode: ModeAutonomous,
		Hist: []llm.Message{{Role: "user", Content: "a"}},
	})
	tn2.RepPasRaonat("resposta", "penso-ho", nil, nil)
	if tn2.Hist[1].ReasoningContent != "" {
		t.Errorf("sense crides no hi ha d'haver raonament: %q", tn2.Hist[1].ReasoningContent)
	}
}
