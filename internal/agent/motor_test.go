package agent

import (
	"errors"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

func cridaProva(id, name, args string) llm.ToolCall {
	var c llm.ToolCall
	c.ID = id
	c.Function.Name = name
	c.Function.Arguments = args
	return c
}

func tornProva(mode string, maxSteps int) *Torn {
	cfg := &config.Config{Mode: mode, Agent: config.AgentCfg{MaxSteps: maxSteps}}
	return NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: DefaultPolicy(), Mode: mode, MaxSteps: maxSteps,
		Tasca: "fes una cosa",
		Hist:  []llm.Message{{Role: "user", Content: "fes una cosa"}},
		Rol:   func() (string, string) { return "p", "m" },
	})
}

// claus retorna les claus i18n dels events (per comprovar què s'ha dit).
func claus(evs []Event) string {
	var out []string
	for _, e := range evs {
		if e.Clau != "" {
			out = append(out, e.Clau)
		} else if e.Eina != "" {
			out = append(out, string(rune('0'+int(e.Tipus)))+":"+e.Eina)
		} else {
			out = append(out, e.Text)
		}
	}
	return strings.Join(out, "|")
}

func TestMotorCamiNormal(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)

	p := tn.Seguent()
	if p.Ordre != OrdrePasModel {
		t.Fatalf("primer ha de demanar un pas: %v", p.Ordre)
	}
	if p.Passos != 1 {
		t.Fatalf("hauria de ser el pas 1: %d", p.Passos)
	}

	// El model demana una lectura: read és allow, va directe a executar.
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p = tn.Seguent()
	if p.Ordre != OrdreExecuta || len(p.Calls) != 1 {
		t.Fatalf("hauria d'executar la lectura: %v", p.Ordre)
	}
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "llegit x.go"}})
	if tn.Execucions() != 1 {
		t.Fatalf("una execució comptada: %d", tn.Execucions())
	}

	p = tn.Seguent()
	if p.Ordre != OrdrePasModel {
		t.Fatalf("després d'executar torna al model: %v", p.Ordre)
	}
	tn.RepPas("ja està fet", nil, nil)
	if !tn.Acabat() || tn.Resposta() != "ja està fet" {
		t.Fatalf("el torn hauria d'acabar amb la resposta: %q", tn.Resposta())
	}
	if p := tn.Seguent(); p.Ordre != OrdreAcaba {
		t.Fatalf("acabat: %v", p.Ordre)
	}
}

func TestMotorDemanaPermisIExecuta(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	// write fora de projecte: la política per defecte demana permís.
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "write", `{"path":"/fora/x.go","content":"hola"}`)}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdreAprova {
		t.Fatalf("hauria de demanar permís: %v", p.Ordre)
	}
	tn.RepAprovacio(true)
	p = tn.Seguent()
	if p.Ordre != OrdreExecuta || len(p.Calls) != 1 {
		t.Fatalf("aprovada, s'ha d'executar: %v", p.Ordre)
	}
}

func TestMotorDenegarRespónAlModel(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "write", `{"path":"/fora/x.go","content":"hola"}`)}, nil)
	tn.Seguent()
	tn.RepAprovacio(false)
	darrer := tn.Hist[len(tn.Hist)-1]
	if darrer.Role != "tool" || !strings.Contains(darrer.Content, "REBUTJADA") {
		t.Fatalf("cap crida pot quedar sense tool_result: %+v", darrer)
	}
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("després de denegar, continua: %v", p.Ordre)
	}
}

func TestMotorAutoAprovaSaltaElPermis(t *testing.T) {
	tools.TodoClear()
	cfg := &config.Config{Mode: ModeCode}
	tn := NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeCode, MaxSteps: 10,
		Hist:       []llm.Message{{Role: "user", Content: "x"}},
		AutoAprova: func() bool { return true },
	})
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "write", `{"path":"/fora/x.go","content":"hola"}`)}, nil)
	if p := tn.Seguent(); p.Ordre != OrdreExecuta {
		t.Fatalf("amb auto-aprovació no s'ha de demanar res: %v", p.Ordre)
	}
}

func TestMotorEinaBloquejadaEnModeXat(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeChat, 10)
	tn.Seguent()
	evs := tn.RepPas("", []llm.ToolCall{cridaProva("1", "write", `{"path":"x.go","content":"hola"}`)}, nil)
	if !strings.Contains(claus(evs), "write") {
		t.Fatalf("hauria de pintar la crida i el resultat: %s", claus(evs))
	}
	darrer := tn.Hist[len(tn.Hist)-1]
	if !strings.Contains(darrer.Content, "BLOQUEJADA") {
		t.Fatalf("el model ha de saber que s'ha bloquejat: %q", darrer.Content)
	}
}

func TestMotorRepeticioAcabaEnSintesi(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 50)
	c := cridaProva("1", "read", `{"path":"x.go"}`)
	// El doom salta a la tercera crida idèntica; a partir d'aquí els
	// passos no aporten res i dos seguits així tanquen el torn.
	for i := 0; i < 14; i++ {
		p := tn.Seguent()
		switch p.Ordre {
		case OrdreSintesi:
			return
		case OrdreExecuta:
			tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "igual"}})
		case OrdrePasModel:
			tn.RepPas("", []llm.ToolCall{c}, nil)
		default:
			t.Fatalf("ordre inesperada: %v", p.Ordre)
		}
	}
	t.Fatal("repetir sense avançar hauria d'acabar en síntesi")
}

func TestMotorPressupostDemanaAmpliacio(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.Seguent() // pas 1
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p := tn.Seguent()
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	p = tn.Seguent()
	if p.Ordre != OrdreAmplia {
		t.Fatalf("esgotat el pressupost, ha de preguntar si continuar: %v", p.Ordre)
	}
	tn.RepAmpliacio("CONTINUA 5", nil)
	if tn.Limit() != 6 {
		t.Fatalf("el límit hauria de créixer: %d", tn.Limit())
	}
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("amb pressupost nou, continua: %v", p.Ordre)
	}
}

func TestMotorPressupostFinalFaSintesi(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p := tn.Seguent()
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	tn.Seguent() // OrdreAmplia
	tn.RepAmpliacio("FINAL", nil)
	p = tn.Seguent()
	if p.Ordre != OrdreSintesi {
		t.Fatalf("FINAL ha de portar a la síntesi: %v", p.Ordre)
	}
	tn.RepSintesi("he fet això i falta allò", nil)
	if !tn.Acabat() || tn.Resposta() == "" {
		t.Fatal("la síntesi ha de tancar el torn amb text")
	}
}

func TestMotorFaLlocAbansDeLaSintesiFinal(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.o.Finestra = func() (int, int) { return 2048, 100 }
	tn.Hist = []llm.Message{{Role: "user", Content: "tasca inicial"}}
	for i := 0; i < 10; i++ {
		tn.Hist = append(tn.Hist, llm.Message{Role: "assistant", Content: strings.Repeat("detall ", 100)})
	}
	tn.sintesiDemanada = true

	if p := tn.Seguent(); p.Ordre != OrdreCompacta {
		t.Fatalf("ha de compactar abans de sintetitzar: %v", p.Ordre)
	}
	tn.RepCompactacio(RoomResult{Hist: []llm.Message{{Role: "user", Content: "resum de la tasca"}}})
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("després de compactar ha de sintetitzar: %v", p.Ordre)
	}
}

func TestMotorSintesiNoReobreElTorn(t *testing.T) {
	// Amb el checklist a mitges, una resposta normal reobre el torn; una
	// síntesi no, o seria un bucle.
	tools.TodoSet([]tools.TodoItem{{Title: "a", Status: "pending"}})
	defer tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.Seguent()
	tn.RepSintesi("resum final", nil)
	if !tn.Acabat() {
		t.Fatal("després d'una síntesi el torn s'acaba sempre")
	}
}

func TestMotorReintentaErrorDeServidor(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	evs := tn.RepPas("", nil, errors.New("provider x: HTTP 502"))
	if tn.Acabat() {
		t.Fatal("un 502 no pot matar el torn al primer intent")
	}
	if !strings.Contains(claus(evs), "app.serverReintentCurt") {
		t.Fatalf("hauria de dir que reintenta: %s", claus(evs))
	}
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("ha de tornar a provar: %v", p.Ordre)
	}
	tn.RepPas("", nil, errors.New("provider x: HTTP 502"))
	tn.Seguent()
	tn.RepPas("", nil, errors.New("provider x: HTTP 502"))
	if !tn.Acabat() {
		t.Fatal("al tercer error el torn es dona per perdut")
	}
}

func TestMotorCridaEscritaEnTextEsReintenta(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("<tool_call>{\"name\":\"read\"}</tool_call>", nil, nil)
	if tn.Acabat() {
		t.Fatal("no s'ha de donar per bona una crida escrita al text")
	}
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("ha de tornar a demanar el pas: %v", p.Ordre)
	}
}

func TestMotorPreguntaBloquejaIAjornaLaResta(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	q := cridaProva("1", "question", `{"query":"quin?","options":[{"label":"a"},{"label":"b"}]}`)
	altra := cridaProva("2", "read", `{"path":"x.go"}`)
	tn.RepPas("", []llm.ToolCall{q, altra}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdrePregunta {
		t.Fatalf("ha de preguntar: %v", p.Ordre)
	}
	// La lectura del mateix pas queda ajornada, amb el seu tool_result.
	trobat := false
	for _, m := range tn.Hist {
		if m.Role == "tool" && m.ToolCallID == "2" && strings.Contains(m.Content, "AJORNADA") {
			trobat = true
		}
	}
	if !trobat {
		t.Fatal("cap crida pot quedar sense tool_result")
	}
	tn.RepPregunta("L'usuari ha triat: a")
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("un cop responem, continua: %v", p.Ordre)
	}
}

func TestMotorTopallAutonomAturaElTorn(t *testing.T) {
	tools.TodoClear()
	cfg := &config.Config{Mode: ModeAutonomous, Agent: config.AgentCfg{
		Autonomous: config.AutonomousCfg{MaxToolSteps: 1, CheckpointEvery: 100},
	}}
	tn := NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeAutonomous, MaxSteps: 10,
		Hist: []llm.Message{{Role: "user", Content: "x"}},
		Rol:  func() (string, string) { return "p", "m" },
	})
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p := tn.Seguent()
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	// S'atura, però amb resum: abans s'aturava en sec i el client es
	// quedava sense cap resposta.
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("passat max_tool_steps, el torn autònom va a la síntesi: %v", p.Ordre)
	}
	if !tn.Esgotat() {
		t.Fatal("i és una síntesi forçada pel motor")
	}
	tn.RepSintesi("resum", nil)
	if !tn.Acabat() || tn.Resposta() != "resum" {
		t.Fatalf("després del resum, acabat: %v %q", tn.Acabat(), tn.Resposta())
	}
}

func TestMotorCancellar(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "write", `{"path":"/fora/x","content":"a"}`)}, nil)
	tn.Seguent() // aprovació pendent
	tn.Cancella()
	if !tn.Acabat() {
		t.Fatal("cancel·lar tanca el torn")
	}
	if p := tn.Seguent(); p.Ordre != OrdreAcaba {
		t.Fatalf("i no demana res més: %v", p.Ordre)
	}
}

func TestMotorEsperaUsuari(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	if tn.EsperaUsuari() {
		t.Fatal("encara no espera ningú")
	}
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "write", `{"path":"/fora/x","content":"a"}`)}, nil)
	tn.Seguent()
	if !tn.EsperaUsuari() {
		t.Fatal("amb una aprovació oberta, espera l'usuari")
	}
}

// Si el model no diu ni CONTINUA ni FINAL (o la crida peta) però la
// checklist és a mitges, el torn continua: tancar a mig fer perquè falta
// una paraula exacta és el que passava amb els models pensadors.
func TestAmpliacioContinuaSiLaChecklistEsAMitges(t *testing.T) {
	tools.TodoSet([]tools.TodoItem{{Title: "a", Status: "done"}, {Title: "b", Status: "pending"}})
	t.Cleanup(tools.TodoClear)
	torn := tornProva(ModeCode, 2)
	limit := torn.Limit()
	evs := torn.RepAmpliacio("Hmm, crec que encara queda feina per fer.", nil)
	if torn.Limit() != limit+ChunkAmpliacio || torn.sintesiDemanada {
		t.Fatalf("amb checklist a mitges s'ha de continuar: límit %d→%d, síntesi=%v", limit, torn.Limit(), torn.sintesiDemanada)
	}
	if len(evs) != 1 || evs[0].Clau != "app.ampliaPerChecklist" {
		t.Fatalf("event inesperat: %+v", evs)
	}
	// Amb error de la crida, igual.
	torn.execs++ // hi ha hagut feina nova des de l'última ampliació
	limit = torn.Limit()
	torn.RepAmpliacio("", errors.New("context deadline exceeded"))
	if torn.Limit() != limit+ChunkAmpliacio {
		t.Fatal("amb error i checklist a mitges també s'ha de continuar")
	}
	// Sense checklist pendent, el de sempre: síntesi.
	tools.TodoClear()
	torn.RepAmpliacio("potser", nil)
	if !torn.sintesiDemanada {
		t.Fatal("sense checklist pendent i sense CONTINUA, síntesi")
	}
}
