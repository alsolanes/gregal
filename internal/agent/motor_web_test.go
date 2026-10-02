package agent

import (
	"errors"
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Peces que el motor ha guanyat perquè la web el pugui conduir en comptes
// de portar un bucle propi: la checklist del client, l'id de la crida als
// events, saber si la síntesi l'ha forçada el motor, la síntesi davant
// d'una resposta buida, el reintent de la síntesi per context i el topall
// de recuperacions.

// Amb el ganxo Todos, el motor mira la checklist del client, no la global:
// a la web cada pestanya té la seva.
func TestMotorTodosDelClient(t *testing.T) {
	tools.TodoClear()
	propia := []tools.TodoItem{{Title: "a", Status: "done"}, {Title: "b", Status: "pending"}}
	tn := tornProva(ModeCode, 10)
	tn.o.Todos = func() []tools.TodoItem { return propia }
	tn.Seguent()
	evs := tn.RepPas("ja està", nil, nil)
	if !strings.Contains(claus(evs), "app.todosPendents") {
		t.Fatalf("amb la checklist del client a mitges, continua: %s", claus(evs))
	}
	if tn.Acabat() {
		t.Fatal("no pot tancar amb la checklist del client a mitges")
	}
	// I la pregunta d'ampliació porta la checklist del client.
	tn2 := tornProva(ModeCode, 1)
	tn2.o.Todos = func() []tools.TodoItem { return propia }
	tn2.Seguent()
	tn2.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p := tn2.Seguent()
	tn2.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	if p := tn2.Seguent(); p.Ordre != OrdreAmplia || !strings.Contains(histUltim(p), "b") {
		t.Fatalf("la pregunta d'ampliació ha de dur la checklist del client: %v\n%s", p.Ordre, histUltim(p))
	}
}

// Els events d'una crida porten el seu id: el client aparella cada targeta
// amb el seu resultat.
func TestMotorEventsAmbID(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	c1 := cridaProva("id-1", "read", `{"path":"x.go"}`)
	evs := tn.RepPas("", []llm.ToolCall{c1}, nil)
	trobat := false
	for _, e := range evs {
		if e.Tipus == EvCrida && e.ID == "id-1" {
			trobat = true
		}
	}
	if !trobat {
		t.Fatalf("la crida ha de dur el seu id: %+v", evs)
	}
}

// Dues lectures soles en passos seguits: recordatori al resultat (no com a
// missatge d'usuari), i com a molt dos cops per torn.
func TestMotorRecordatoriLecturesJuntes(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 40)
	tn.Seguent()
	llegeix := func(nom string) string {
		c := cridaProva("r-"+nom, "read", `{"path":"`+nom+`.go"}`)
		tn.RepPas("", []llm.ToolCall{c}, nil)
		p := tn.Seguent()
		tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "contingut"}})
		tn.Seguent()
		return tn.Hist[len(tn.Hist)-1].Content
	}
	if strings.Contains(llegeix("alpha"), "[recordatori]") {
		t.Fatal("una lectura sola encara no és un patró")
	}
	if !strings.Contains(llegeix("beta"), "[recordatori]") {
		t.Fatal("a la segona lectura sola seguida, recordatori")
	}
	for _, n := range []string{"gamma", "delta", "epsilon", "zeta"} {
		llegeix(n)
	}
	n := 0
	for _, m := range tn.Hist {
		if m.Role == "user" && strings.Contains(m.Content, "[recordatori]") {
			t.Fatal("el recordatori va al resultat de l'eina, mai com a missatge d'usuari")
		}
		if m.Role == "tool" && strings.Contains(m.Content, "demana-les totes en un sol pas") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("com a molt dos recordatoris per torn, n'hi ha %d", n)
	}
}

// Una resposta buida no tanca el torn en blanc: primer, síntesi.
func TestMotorRespostaBuidaDemanaSintesi(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("", nil, nil)
	if tn.Acabat() {
		t.Fatal("una resposta buida no pot tancar el torn en blanc")
	}
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("toca síntesi: %v", p.Ordre)
	}
	tn.RepSintesi("resum", nil)
	if tn.Resposta() != "resum" {
		t.Fatalf("resposta: %q", tn.Resposta())
	}
}

// El pressupost exhaurit marca la síntesi com a forçada.
func TestMotorEsgotatPelPressupost(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p := tn.Seguent()
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	if tn.Esgotat() {
		t.Fatal("encara no")
	}
	if p := tn.Seguent(); p.Ordre != OrdreAmplia {
		t.Fatalf("primer, pregunta d'ampliació: %v", p.Ordre)
	}
	tn.RepAmpliacio("FINAL", nil)
	if p := tn.Seguent(); p.Ordre != OrdreSintesi || !tn.Esgotat() {
		t.Fatalf("FINAL a l'ampliació és una síntesi forçada: %v %v", p.Ordre, tn.Esgotat())
	}
}

// Una síntesi que no hi cap es retalla i es torna a demanar un cop.
func TestMotorSintesiReintentaPerContext(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	for i := 0; i < 12; i++ {
		tn.Hist = append(tn.Hist,
			llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{cridaProva("c"+string(rune('a'+i)), "read", `{}`)}},
			llm.Message{Role: "tool", ToolCallID: "c" + string(rune('a'+i)), Content: strings.Repeat("sortida llarga ", 400)})
	}
	tn.sintesiDemanada = true
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("síntesi: %v", p.Ordre)
	}
	abans := len(tn.Hist)
	tn.RepSintesi("", errors.New("HTTP 400: context_length_exceeded (window 4096)"))
	if tn.Acabat() {
		t.Fatal("el primer cop, retalla i torna a provar")
	}
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("torna a demanar la síntesi: %v (hist %d → %d)", p.Ordre, abans, len(tn.Hist))
	}
	tn.RepSintesi("", errors.New("HTTP 400: context_length_exceeded (window 4096)"))
	if !tn.Acabat() {
		t.Fatal("el segon cop ja no hi ha reintent")
	}
}

// Les recuperacions de context seguides tenen topall: no hi ha bucle encara
// que el proveïdor digui sempre que no hi cap.
func TestMotorTopallRecuperacionsContext(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 50)
	for i := 0; i < 40; i++ {
		tn.Hist = append(tn.Hist,
			llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{cridaProva("k"+string(rune('A'+i)), "read", `{}`)}},
			llm.Message{Role: "tool", ToolCallID: "k" + string(rune('A'+i)), Content: strings.Repeat("x ", 300)})
	}
	errCtx := errors.New("HTTP 400: context_length_exceeded (window 512)")
	for i := 0; i < 20 && !tn.Acabat(); i++ {
		tn.Seguent()
		tn.RepPas("", nil, errCtx)
	}
	if !tn.Acabat() || !tn.Fallit() {
		t.Fatal("amb el proveïdor dient sempre que no hi cap, el torn acaba mort, no fa voltes")
	}
}
