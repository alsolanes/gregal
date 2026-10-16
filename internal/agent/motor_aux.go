package agent

import (
	"fmt"
	"strings"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Peces que el motor necessita i que abans vivien repartides entre el
// TUI i el bucle del headless.

// GuiaErrorServidor és el que rep el model quan la crida ha petat amb un
// error de servidor i encara queden reintents. tascaDe reconeix el prefix
// (PrefixErrorServidor, i el català d'abans) per no prendre-la per la
// consigna de l'usuari.
func GuiaErrorServidor(err error) string {
	return fmt.Sprintf(PrefixErrorServidor+" with a server error (%v). Retry the last step; if the tool call was very long, split it into smaller pieces.", err)
}

// PrefixErrorServidor encapçala GuiaErrorServidor.
const PrefixErrorServidor = "The model call failed"

// PreguntaSenseUI és el resultat d'una crida question on no hi ha cap
// interfície per triar opcions (headless, Telegram, loop de proves). Abans
// deia «continua amb el teu millor criteri» i el model, davant d'una tasca
// impossible, feia canvis que ningú no havia demanat.
const PreguntaSenseUI = "QUESTION NOT DELIVERED: this is a non-interactive run and nobody can answer it. Do not make changes the user did not explicitly request. If you cannot continue without this decision, stop now and put the question in your final answer; otherwise continue only with the requested work."

// IndexPregunta retorna l'índex de la primera crida que és una pregunta
// a l'usuari (-1 si no n'hi ha cap). Els models li diuen de maneres
// diferents segons com s'hagin entrenat.
func IndexPregunta(calls []llm.ToolCall) int {
	for i, c := range calls {
		switch c.Function.Name {
		case "question", "ask_question", "ask", "AskUserQuestion":
			return i
		}
	}
	return -1
}

// ValidaPregunta comprova que els arguments d'una pregunta siguin
// utilitzables (query + 1-4 opcions).
func ValidaPregunta(argsJSON string) error {
	_, _, err := tools.ParseQuestion(argsJSON)
	return err
}

// todoEstat i todoLlista aïllen el motor del paquet d'eines. Amb el ganxo
// Todos (la web, on cada pestanya té la seva checklist) manen les del
// client; sense, la global del procés, que és la del TUI i el headless.
func (t *Torn) todoEstat() (fets, total int) {
	if t.o.Todos != nil {
		return tools.StatsTodos(t.o.Todos())
	}
	return tools.TodoStats()
}

func (t *Torn) todoLlista() []tools.TodoItem {
	if t.o.Todos != nil {
		return t.o.Todos()
	}
	return tools.TodoList()
}

// RecordatoriLectures va enganxat a una lectura quan el model n'ha fet
// dues de soles en passos seguits.
const RecordatoriLectures = "\n\n[reminder] If you need to read or search more things, request them all in a single step: they run in parallel, and every step is a full wait for the model."

// recordatoriLectures compta els passos seguits amb una sola lectura i,
// al segon, torna el recordatori (dos cops per torn com a molt). Al banc
// A/B contra opencode, amb el mateix model, Gregal llegia quatre fitxers en
// quatre passos i opencode en un: el prompt ja ho demanava, però un
// recordatori al resultat, com el del checklist, el model sí que el veu.
func (t *Torn) recordatoriLectures(res []Execucio) string {
	if len(res) != 1 || !ParallelSafe(res[0].Call.Function.Name) {
		t.lecturesSoles = 0
		return ""
	}
	t.lecturesSoles++
	if t.lecturesSoles < 2 || t.avisosLectures >= 2 {
		return ""
	}
	t.lecturesSoles = 0
	t.avisosLectures++
	return RecordatoriLectures
}

// lotMecanic diu si un lot d'eines és feina que ha anat bé, sense
// informació nova per pensar: edicions aplicades, la checklist, o
// comprovacions en verd. Una lectura, una cerca, una ordre qualsevol o
// qualsevol error el fan no mecànic: allà és on el model ha de raonar.
func lotMecanic(res []Execucio) bool {
	if len(res) == 0 {
		return false
	}
	for _, r := range res {
		if strings.HasPrefix(r.Sortida, "ERROR:") || SemblaFracas(r.Sortida) || strings.Contains(r.Sortida, "[diagnòstic]") || strings.Contains(r.Sortida, "[sintaxi]") {
			return false
		}
		switch r.Call.Function.Name {
		case "write", "edit", "patch", "todowrite":
		case "bash":
			if !esComprovacioCrida(r.Call) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// ThinkPerPas és el mode de raonament d'una crida al model. Amb think:
// auto, el model raona on aporta (planificar, decidir l'arreglo després de
// llegir, entendre un vermell) i no on el moviment és obvi: després d'una
// edició aplicada o d'una comprovació verda, la pregunta d'ampliació i la
// síntesi. Mesurat contra halogen: una pregunta curta costava 80-93 s i
// 3.100 tokens raonant, i 0,4-2 s sense, amb la mateixa resposta. Qualsevol
// altre valor del rol passa tal qual.
func ThinkPerPas(rolThink string, p Pas) string {
	if rolThink != "auto" {
		return rolThink
	}
	switch p.Ordre {
	case OrdreAmplia, OrdreSintesi:
		return "no"
	case OrdrePasModel:
		if p.Mecanic {
			return "no"
		}
	}
	return ""
}

// preguntaAmplia és la pregunta d'ampliació amb la checklist que toca.
func (t *Torn) preguntaAmplia() string {
	if t.o.Todos != nil {
		return PreguntaAmbLlista(t.o.Todos())
	}
	return PreguntaAmbEstat()
}

// RepSintesi tanca el torn amb la resposta final demanada sense eines.
// No és RepPas: allà una resposta sense eines encara pot reobrir el torn
// (checklist a mitges, reparació fallida), i després d'una síntesi això
// seria un bucle.
func (t *Torn) RepSintesi(content string, err error) []Event {
	if err != nil {
		// La síntesi rep tot el transcript i pot no cabre després d'una
		// feina llarga: un cop, es retalla i es torna a demanar (Seguent
		// torna OrdreSintesi, perquè sintesiDemanada continua viva).
		if ok, actual := ParseContextExceeded(err); ok && !t.sintesiReintent {
			t.sintesiReintent = true
			if evs, fet := t.retallaPerContext(actual); fet {
				t.sintesiDemanada = true
				return evs
			}
		}
		// Ni la síntesi no surt (proveïdor caigut, temps): el resum mecànic
		// de l'historial. La feina feta no es perd mai per un error final.
		t.acabat = true
		if t.execs == 0 {
			return []Event{errEv("agent: error: " + err.Error())}
		}
		if t.interromput == "" {
			t.interromput = err.Error()
		}
		return t.resumMecanic("agent: error: " + err.Error())
	}
	t.acabat = true
	net, hiEra := SenseEinaText(strings.TrimSpace(content))
	// La síntesi va sense tools: si el model hi escriu una crida en DSML,
	// això no és una resposta (abans arribava tal qual a l'usuari).
	if n, dsml := llm.SenseCridesText(net); dsml {
		net, hiEra = n, true
	}
	var evs []Event
	if hiEra {
		evs = append(evs, nota(AvisEinaText))
	}
	// Només una crida i res més: un altre intent dient-li que ara no hi ha
	// eines (finalPrompt hi afegeix sintesiSenseEines).
	if net == "" && hiEra && !t.sintesiEinaReintent {
		t.sintesiEinaReintent = true
		t.acabat = false
		t.sintesiDemanada = true
		return evs
	}
	if net == "" {
		if t.execs > 0 {
			return append(evs, t.resumMecanic("")...)
		}
		return append(evs, notaClau("app.tornBuit", t.passos, t.Limit()))
	}
	t.resposta = net
	t.Hist = append(t.Hist, llm.Message{Role: "assistant", Content: net})
	evs = append(evs, t.avisDebugRestant()...)
	return append(evs, Event{Tipus: EvResposta, Text: net})
}

// resumMecanic tanca el torn amb ResumDeFeina quan el model no ha pogut
// escriure la síntesi: què s'ha consultat i l'últim que havia dit.
func (t *Torn) resumMecanic(avisText string) []Event {
	motiu := t.interromput
	if motiu == "" && t.esgotat {
		motiu = MotiuPassos
	}
	t.resposta = ResumDeFeina(t.Hist, motiu, t.lang())
	t.Hist = append(t.Hist, llm.Message{Role: "assistant", Content: t.resposta})
	var evs []Event
	if avisText != "" {
		evs = append(evs, avis(avisText))
	}
	return append(evs, Event{Tipus: EvResposta, Text: t.resposta})
}
