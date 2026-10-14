package agent

// Peces del motor per al mode debug, les apostes de confiança i el crític
// del contracte autònom. Viuen aquí perquè motor.go només hi tingui els
// punts d'entrada (una línia cadascun).

import (
	"context"
	"fmt"
	"strings"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// capsTorn és l'estat del torn per a aquestes peces.
type capsTorn struct {
	debugBase    map[string]int
	debugRetries int
	// apostes: id de la crida → predicció (i d'on ha sortit).
	apostes map[string]apostaPendent
	// criticaFeta: el contracte autònom ja ha passat pel crític.
	criticaFeta bool
}

type apostaPendent struct {
	p      float64
	source string
	cmd    string
}

// dirTorn és el workspace del torn.
func (t *Torn) dirTorn() string {
	if t.o.Workspace != "" {
		return t.o.Workspace
	}
	if t.o.Pol != nil {
		return t.o.Pol.ProjectDir
	}
	return ""
}

func (t *Torn) ctxTorn() context.Context {
	if t.o.Ctx != nil {
		return t.o.Ctx
	}
	return context.Background()
}

// capsInici es crida una vegada en començar el torn.
func (t *Torn) capsInici() {
	if t.o.Mode == ModeDebug {
		if dir := t.dirTorn(); dir != "" {
			t.caps.debugBase = DebugTagCounts(dir)
		}
	}
}

// debugRestants són els fitxers amb instrumentació nova d'aquest torn.
func (t *Torn) debugRestants() []string {
	if t.o.Mode != ModeDebug || t.caps.debugBase == nil {
		return nil
	}
	return DebugTagsNous(t.caps.debugBase, DebugTagCounts(t.dirTorn()))
}

// debugGate impedeix tancar un torn de debug amb instrumentació nova: fins
// a dues vegades torna el model a la feina (segueix=true). Després deixa
// tancar, amb un avís perquè la persona ho sàpiga.
func (t *Torn) debugGate() ([]Event, bool) {
	llocs := t.debugRestants()
	if len(llocs) == 0 {
		return nil, false
	}
	if t.caps.debugRetries < 2 {
		t.caps.debugRetries++
		t.Hist = append(t.Hist, llm.Message{Role: "user", Content: GuiaDebugNeteja(llocs)})
		return []Event{avis("debug: " + DebugTag + " instrumentation still present in " + strings.Join(llocs, ", ") + ": asking the agent to clean up")}, true
	}
	return []Event{avis("debug: " + DebugTag + " instrumentation left in " + strings.Join(llocs, ", "))}, false
}

// avisDebugRestant avisa a la síntesi forçada (sense eines no es pot
// netejar) si queda instrumentació.
func (t *Torn) avisDebugRestant() []Event {
	if llocs := t.debugRestants(); len(llocs) > 0 {
		return []Event{avis("debug: " + DebugTag + " instrumentation left in " + strings.Join(llocs, ", "))}
	}
	return nil
}

// preparaAposta desa la predicció d'una comprovació que s'executarà: la del
// paràmetre bet o, amb agent.bets: side, la de la crida lateral.
func (t *Torn) preparaAposta(c llm.ToolCall) {
	if c.Function.Name != "bash" || BetsMode(t.o.Cfg) == "off" || !esComprovacioCrida(c) {
		return
	}
	cmd, p, ok := apostaDeArgs(c.Function.Arguments)
	source := "model"
	if !ok && t.o.Aposta != nil {
		p, ok = t.o.Aposta(t.ctxTorn(), cmd)
		source = "side"
	}
	if !ok {
		return
	}
	if t.caps.apostes == nil {
		t.caps.apostes = map[string]apostaPendent{}
	}
	t.caps.apostes[c.ID] = apostaPendent{p: p, source: source, cmd: cmd}
}

// resolAposta casa el resultat d'una comprovació amb la seva predicció, la
// registra i torna l'event per a la fila de l'eina.
func (t *Torn) resolAposta(r Execucio) (Event, bool) {
	ap, ok := t.caps.apostes[r.Call.ID]
	if !ok {
		return Event{}, false
	}
	delete(t.caps.apostes, r.Call.ID)
	passed := !(strings.HasPrefix(r.Sortida, "ERROR:") || SemblaFracas(r.Sortida))
	a := Aposta{Session: t.o.Sessio, Project: t.dirTorn(), Command: retallaText(ap.cmd, 300), P: ap.p, Passed: passed, Source: ap.source}
	_ = RegistraAposta(a)
	return Event{Tipus: EvNota, Text: BetChip(a) + " · " + retallaText(ap.cmd, 80), Eina: "bash", ID: r.Call.ID, Aposta: &a}, true
}

// criticaContracte passa pel crític el primer contracte de fites del mode
// autònom (el primer todowrite amb dues fites o més) i, si hi ha
// problemes, demana al model que revisi el pla una vegada.
func (t *Torn) criticaContracte(res []Execucio) []Event {
	if !t.autonom() || t.caps.criticaFeta || t.o.CriticaPla == nil {
		return nil
	}
	hiHa := false
	for _, r := range res {
		if r.Call.Function.Name == "todowrite" && !strings.HasPrefix(r.Sortida, "ERROR:") {
			hiHa = true
		}
	}
	if !hiHa {
		return nil
	}
	llista := t.todoLlista()
	if len(llista) < 2 {
		return nil
	}
	t.caps.criticaFeta = true
	c, err := t.o.CriticaPla(t.ctxTorn(), t.o.Tasca, renderTodosPla(llista))
	if err != nil {
		return []Event{nota("plan critic: " + err.Error())}
	}
	if len(c.Issues) == 0 {
		return []Event{nota("plan critic: no concerns")}
	}
	t.guies = append(t.guies, GuiaCriticaContracte(c))
	return []Event{avis(fmt.Sprintf("plan critic: %d concerns\n%s", len(c.Issues), RenderCritique(c)))}
}

func renderTodosPla(items []tools.TodoItem) string {
	var b strings.Builder
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s\n", i+1, it.Title)
	}
	return b.String()
}
