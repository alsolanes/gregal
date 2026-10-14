package agent

import (
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Un missatge escrit mentre corre una eina entra DESPRÉS del seu tool
// result (mai entremig, que els proveïdors estrictes rebutgen) i el pas
// següent del model ja el porta.
func TestInterjeccioEntraDespresDelLot(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdreExecuta {
		t.Fatalf("expected the read to run: %v", p.Ordre)
	}
	if !tn.Interjeccio("fes-ho en Python, no en Go") {
		t.Fatal("the message must be accepted while working")
	}
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "llegit"}})
	p = tn.Seguent()
	if p.Ordre != OrdrePasModel {
		t.Fatalf("back to the model: %v", p.Ordre)
	}
	h := tn.Hist
	last, prev := h[len(h)-1], h[len(h)-2]
	if last.Role != "user" || !strings.Contains(last.Content, "fes-ho en Python") || prev.Role != "tool" {
		t.Fatalf("the message must follow the tool result: prev=%s last=%s %q", prev.Role, last.Role, last.Content)
	}
	if tn.guiesPendents() != 0 {
		t.Fatal("the message must be delivered once")
	}
}

// Si arriba quan el model ja havia donat la resposta final, el torn es
// reobre i continua amb el missatge.
func TestInterjeccioReobreUnTornAcabat(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("ja està fet", nil, nil)
	if !tn.Acabat() {
		t.Fatal("the turn should be finished")
	}
	if !tn.Interjeccio("afegeix també un test") {
		t.Fatal("a finished turn must reopen for a late message")
	}
	if tn.Acabat() || tn.Resposta() != "" {
		t.Fatal("the turn must be open again without a final answer")
	}
	p := tn.Seguent()
	if p.Ordre != OrdrePasModel || !strings.Contains(p.Hist[len(p.Hist)-1].Content, "afegeix també un test") {
		t.Fatalf("the next model step must carry the message: %v", p.Ordre)
	}
	// La resposta anterior queda a l'historial com a pas de treball.
	found := false
	for _, m := range p.Hist {
		if m.Role == "assistant" && m.Content == "ja està fet" {
			found = true
		}
	}
	if !found {
		t.Fatal("the earlier answer stays in the history")
	}
}

func TestInterjeccioBuidaOFallitNoEntra(t *testing.T) {
	tn := tornProva(ModeCode, 10)
	if tn.Interjeccio("   ") {
		t.Fatal("empty messages are ignored")
	}
	tn.mor("error")
	if tn.Interjeccio("hola") {
		t.Fatal("a failed turn does not reopen")
	}
}
