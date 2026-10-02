package agent

import (
	"encoding/json"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// El motor del torn d'agent: una sola màquina d'estats per a tots els
// clients.
//
// Fins ara n'hi havia dues. La del headless vivia a run.go com un bucle
// `for`; la del TUI estava escampada per `Update` entre agentStepMsg,
// agentExecMsg, agentAskAdvanceMsg, agentExtensioMsg, agentCompactMsg i
// startAskFlow, amb tretze comptadors al Model que startAgent havia de
// recordar de posar a zero. Feien la mateixa feina i anaven divergint:
// els reintents d'error de servidor eren dos en una i un a l'altra, els
// topalls del mode autònom només hi eren en una, i la decisió de permisos
// estava escrita dues vegades. Cada arranjament s'havia de fer dos cops i
// se n'oblidava un.
//
// Aquí la màquina no fa E/S: rep resultats i diu què s'ha de fer després.
// Això la fa provable sense xarxa ni disc, i deixa que cada client posi
// la seva manera de cridar el model, executar eines i demanar permisos.
// El TUI hi encaixa perquè bubbletea ja funciona així (missatge → estat
// nou → ordre), i el headless només ha de fer un bucle al voltant de
// Seguent().

// Ordre és el que el motor demana al conductor.
type Ordre int

const (
	// OrdreRes: res a fer ara (s'espera una resposta de l'usuari).
	OrdreRes Ordre = iota
	// OrdrePasModel: demana un pas al model amb eines (Hist).
	OrdrePasModel
	// OrdreCompacta: fes lloc a l'historial fins a Budget tokens.
	OrdreCompacta
	// OrdreExecuta: executa Calls (lectures en paral·lel).
	OrdreExecuta
	// OrdreAprova: demana permís a l'usuari per a Call.
	OrdreAprova
	// OrdrePregunta: pregunta seleccionable del model (Call).
	OrdrePregunta
	// OrdreAmplia: pregunta al model si val la pena continuar (Hist).
	OrdreAmplia
	// OrdreSintesi: resposta final sense eines (Hist).
	OrdreSintesi
	// OrdreCheckpoint: comprovacions del mode autònom.
	OrdreCheckpoint
	// OrdreAcaba: torn acabat; Text és la resposta final.
	OrdreAcaba
	// OrdreError: torn mort; Text és el motiu.
	OrdreError
)

// Pas és una ordre amb el que cal per complir-la.
type Pas struct {
	Ordre  Ordre
	Hist   []llm.Message
	Calls  []llm.ToolCall
	Call   llm.ToolCall
	Text   string
	Budget int
	// Passos i Limit són per a la línia d'estat («agent 7/40»).
	Passos int
	Limit  int
	// Mecanic: aquest pas de model ve després d'una feina que ha anat bé
	// (una edició aplicada, una comprovació verda) i el següent moviment
	// sol ser obvi (córrer el test, respondre). Amb think: auto, el client
	// hi apaga el raonament (ThinkPerPas).
	Mecanic bool
}

// TipusEvent diu com s'ha de pintar un event.
type TipusEvent int

const (
	// EvNota: línia de sistema, apagada.
	EvNota TipusEvent = iota
	// EvAvis: alguna cosa que cal mirar (ambre).
	EvAvis
	// EvError: el torn ha anat malament (vermell).
	EvError
	// EvCrida: l'agent crida una eina (Eina, EinaArgs).
	EvCrida
	// EvResultat: el que ha tornat (Eina, Text, Fallada).
	EvResultat
	// EvResposta: la resposta final, per pintar com a markdown.
	EvResposta
	// EvDiff: el canvi d'una escriptura, ja pintat pel client (Text).
	// Va a part d'EvNota perquè el client el posi dins del rail de la
	// feina, sota la fila de l'eina, i no com una nota solta.
	EvDiff
)

// Event és una cosa que ha passat i que el client hauria de pintar.
// Clau és una clau de traducció (i Args el seu Sprintf); amb Clau buida
// mana Text. Així el motor no depèn de la i18n de cap client.
type Event struct {
	Tipus    TipusEvent
	Clau     string
	Args     []any
	Text     string
	Eina     string
	EinaArgs string
	Fallada  bool
	// ID és l'id de la crida (EvCrida, i EvResultat quan el resultat surt
	// del motor sense execució: bloquejada, repetida, ajornada). La web
	// aparella cada targeta d'eina amb el seu resultat per l'ordre
	// d'arribada, i amb l'id pot emetre la crida just abans del resultat.
	ID string
}

func nota(text string) Event  { return Event{Tipus: EvNota, Text: text} }
func avis(text string) Event  { return Event{Tipus: EvAvis, Text: text} }
func errEv(text string) Event { return Event{Tipus: EvError, Text: text} }
func notaClau(clau string, args ...any) Event {
	return Event{Tipus: EvNota, Clau: clau, Args: args}
}
func avisClau(clau string, args ...any) Event {
	return Event{Tipus: EvAvis, Clau: clau, Args: args}
}

// Execucio és el resultat d'haver executat una crida.
type Execucio struct {
	Call    llm.ToolCall
	Sortida string
	Imatges []string
	// Diff és la vista del canvi (write/edit/patch), ja pintada pel
	// client. El motor no la mira: només la torna a passar en un event.
	Diff string
}

// OpcionsTorn és el que el motor necessita saber del seu entorn.
type OpcionsTorn struct {
	Cfg      *config.Config
	Pol      *Policy
	Mode     string
	Tasca    string
	MaxSteps int
	// Hist és l'historial inicial (conversa + tasca), sense el system.
	Hist []llm.Message
	// SysPrompt i Finestra els aporta el client perquè depenen del rol
	// actiu, que pot canviar a mitja sessió.
	SysPrompt func() string
	Finestra  func() (win, budget int)
	Rol       func() (provider, model string)
	// Recordat diu si l'usuari ja ha aprovat aquesta crida per la sessió.
	Recordat func(name, argsJSON string) bool
	// AutoAprova salta el permís (mode permissiu del TUI, --auto-approve
	// del headless). Un deny continua bloquejant.
	AutoAprova func() bool
	// Todos és la checklist del client quan no és la global del procés:
	// a la web cada pestanya té la seva, i amb la global dues sessions es
	// trepitjaven. Nil = la global.
	Todos func() []tools.TodoItem
	// AprenFinestra rep la finestra real quan el proveïdor la diu en un
	// error de context, perquè el client l'apliqui al seu rol (la web la
	// desa al config de la sessió). Opcional.
	AprenFinestra func(finestra int)
	// Usage és el comptador de tokens reals que el client posa al context
	// de les crides (llm.WithUsage): el topall de cost autònom el fa servir
	// en comptes de l'estimació. Opcional.
	Usage *llm.UsageMeter
}

// Torn és l'estat d'un torn d'agent.
type Torn struct {
	o OpcionsTorn

	// Hist és l'historial viu del torn (sense el system prompt).
	Hist []llm.Message

	passos int
	extres int

	ampliacions int
	execs       int
	execsSnap   int

	// ratxa compta comprovacions que han fallat SEGUIDES des de l'última
	// que ha passat: és el detector del bucle «corro el test, el retoco,
	// corro el test». Cada execució és nova (l'avenç de potAmpliar cedia)
	// però cap verifica res. Una comprovació verda la posa a zero; una
	// edició que triomfa no la toca, que és exactament el cas que s'ha de
	// caçar.
	ratxa RatxaVermella

	doom       DoomTracker
	doomStalls int

	serverRetries  int
	textRetries    int
	todosRetries   int
	verifRetries   int
	compactatAlPas int
	execsSenseTodo int
	ultSortida     string
	// lecturesSoles: passos seguits amb una sola lectura (recordatori de
	// demanar-les juntes); avisosLectures: quants cops s'ha recordat.
	lecturesSoles  int
	avisosLectures int
	// mecanic: l'últim lot d'eines ha estat feina que ha anat bé i no
	// informació nova per pensar (Pas.Mecanic). Qualsevol altra cosa —
	// lectures, un vermell, una negativa, una pregunta— el torna a fals.
	mecanic bool

	perExecutar []llm.ToolCall
	pendents    []llm.ToolCall
	actual      llm.ToolCall
	teActual    bool
	pregunta    *llm.ToolCall

	autoInici          time.Time
	autoCheckpoint     int
	autoNextCheckpoint int
	checkpointPendent  bool
	longRunChecked     bool

	sintesiDemanada bool
	// esgotat: la síntesi l'ha forçada el motor (pressupost, model encallat
	// repetint, topall autònom, FINAL a l'ampliació), no una resposta del
	// model. El client ho diu abans del resum perquè una feina tallada no
	// sembli una feina acabada.
	esgotat bool
	// sintesiReintent: la síntesi ja s'ha reintentat un cop per context.
	sintesiReintent bool
	// recuperacions de context seguides: el topall evita un bucle si el
	// proveïdor continua dient que no hi cap per molt que es retalli.
	recuperacions int
	acabat        bool
	// fallit distingeix «el torn s'ha acabat» de «el torn s'ha mort».
	// Sense això, qui condueix no pot saber si ha de donar error.
	fallit   bool
	motiu    string
	resposta string
}

// NouTorn arrenca un torn.
func NouTorn(o OpcionsTorn) *Torn {
	if o.MaxSteps <= 0 {
		o.MaxSteps = 40
	}
	t := &Torn{o: o, Hist: append([]llm.Message{}, o.Hist...)}
	t.autoInici = time.Now()
	if o.Cfg != nil {
		t.autoNextCheckpoint = o.Cfg.AutonomousConfig().CheckpointEvery
	}
	return t
}

// Passos i Limit són per a la línia d'estat.
func (t *Torn) Passos() int { return t.passos }
func (t *Torn) Limit() int  { return t.o.MaxSteps + t.extres }

// Execucions són les eines executades de debò en aquest torn.
func (t *Torn) Execucions() int { return t.execs }

// Acabat diu si el torn ja no demanarà res més.
func (t *Torn) Acabat() bool { return t.acabat }

// Fallit diu si el torn s'ha mort (error) en comptes d'acabar.
func (t *Torn) Fallit() bool { return t.fallit }

// Esgotat diu si la síntesi final l'ha forçada el motor en comptes de
// ser la resposta del model (pressupost exhaurit, encallat, topall).
func (t *Torn) Esgotat() bool { return t.esgotat }

// Resposta és el text final (buit si encara no n'hi ha).
func (t *Torn) Resposta() string { return t.resposta }

// EsperaUsuari diu si el motor està aturat esperant una persona (permís
// o pregunta). Serveix al client per no encuar-hi feina a sobre.
func (t *Torn) EsperaUsuari() bool { return t.pregunta != nil || t.teActual }

// autonom diu si el torn corre en mode autònom.
func (t *Torn) autonom() bool { return t.o.Mode == ModeAutonomous }

// histAmbSys posa el system prompt al davant.
func (t *Torn) histAmbSys() []llm.Message {
	sys := ""
	if t.o.SysPrompt != nil {
		sys = t.o.SysPrompt()
	}
	if sys == "" {
		return append([]llm.Message{}, t.Hist...)
	}
	return append([]llm.Message{{Role: "system", Content: sys}}, t.Hist...)
}

// Seguent diu què cal fer ara. No té efectes secundaris més enllà de
// consumir la feina que entrega (les crides a executar, l'aprovació en
// curs): cridar-lo dues vegades seguides sense alimentar-lo no torna la
// mateixa ordre per sempre.
func (t *Torn) Seguent() Pas {
	if t.autonom() && !t.longRunChecked {
		t.longRunChecked = true
		if t.o.Cfg != nil && t.o.Rol != nil {
			provider, model := t.o.Rol()
			if !t.o.Cfg.LongRunsAllowed(provider, model) {
				t.acabat, t.fallit = true, true
				t.motiu = "el model " + provider + "/" + model + " no està habilitat per a tirades llargues"
				return Pas{Ordre: OrdreError, Text: t.motiu, Passos: t.passos, Limit: t.Limit()}
			}
		}
	}
	if t.acabat {
		if t.fallit {
			return Pas{Ordre: OrdreError, Text: t.motiu, Passos: t.passos, Limit: t.Limit()}
		}
		return Pas{Ordre: OrdreAcaba, Text: t.resposta, Passos: t.passos, Limit: t.Limit()}
	}
	if t.pregunta != nil {
		return t.pas(Pas{Ordre: OrdrePregunta, Call: *t.pregunta})
	}
	if len(t.perExecutar) > 0 {
		calls := t.perExecutar
		t.perExecutar = nil
		return t.pas(Pas{Ordre: OrdreExecuta, Calls: calls})
	}
	if t.checkpointPendent {
		t.checkpointPendent = false
		return t.pas(Pas{Ordre: OrdreCheckpoint})
	}
	if len(t.pendents) > 0 {
		t.actual, t.pendents = t.pendents[0], t.pendents[1:]
		t.teActual = true
		if t.o.AutoAprova != nil && t.o.AutoAprova() {
			// Aprovació automàtica: no es molesta ningú, però la crida
			// passa pel mateix camí (i pel mateix comptador).
			t.teActual = false
			return t.pas(Pas{Ordre: OrdreExecuta, Calls: []llm.ToolCall{t.actual}})
		}
		return t.pas(Pas{Ordre: OrdreAprova, Call: t.actual})
	}
	// Fer lloc també abans d'ampliar o sintetitzar: una tasca llarga pot
	// haver cabut a cada pas i, tot i així, desbordar el context al resum.
	if t.o.Finestra != nil && len(t.Hist) > KeepRecentOnCompact+1 && t.compactatAlPas != t.passos+1 {
		_, budget := t.o.Finestra()
		if budget > 0 && NeedsCompact(llm.EstimateTokens(t.histAmbSys()), budget) {
			t.compactatAlPas = t.passos + 1
			return t.pas(Pas{Ordre: OrdreCompacta, Budget: budget, Hist: t.histAmbSys()})
		}
	}
	// Síntesi ja decidida (el model ha dit FINAL, o s'ha encallat
	// repetint): es tanca amb resum encara que quedin passos. Va abans
	// del pressupost perquè la decisió no depèn del comptador.
	if t.sintesiDemanada {
		return t.pas(Pas{Ordre: OrdreSintesi, Hist: append(t.histAmbSys(), llm.Message{Role: "user", Content: t.finalPrompt()})})
	}
	// Pressupost exhaurit: primer es pregunta al model si val la pena
	// continuar; si no, síntesi final. Mai un torn buit. Quan el torn
	// s'ha fet llarg (ampliacions gastades o ratxa de vermelles), la
	// síntesi demana l'estat real del verd i del vermell: és el
	// checkpoint amb l'usuari que abans ningú feia mai.
	if t.passos >= t.Limit() {
		if t.potAmpliar() {
			return t.pas(Pas{Ordre: OrdreAmplia, Hist: append(t.histAmbSys(), llm.Message{Role: "user", Content: t.preguntaAmplia()})})
		}
		t.sintesiDemanada = true
		t.esgotat = true
		return t.pas(Pas{Ordre: OrdreSintesi, Hist: append(t.histAmbSys(), llm.Message{Role: "user", Content: t.finalPrompt()})})
	}
	t.passos++
	return t.pas(Pas{Ordre: OrdrePasModel, Hist: t.histAmbSys(), Mecanic: t.mecanic})
}

// pas omple els camps comuns.
func (t *Torn) pas(p Pas) Pas {
	p.Passos, p.Limit = t.passos, t.Limit()
	return p
}

// finalPrompt tria la síntesi (FinalPromptPer).
func (t *Torn) finalPrompt() string {
	return FinalPromptPer(t.passos, t.ampliacions, t.ratxa)
}

// potAmpliar diu si val la pena demanar més pressupost: queden
// ampliacions, no s'està encallat repetint, i (si ja n'ha demanat) ha
// executat eines de noves des de l'última. La ratxa de comprovacions
// vermelles també ho bloqueja: l'incident de 2026-09-28 (146 passos per
// una tasca trivial) va viure d'ampliacions concedides a un model que
// només «avançava» tornant a correr la mateixa suite vermella.
func (t *Torn) potAmpliar() bool {
	max := MaxAmpliacions
	if t.autonom() && t.o.Cfg != nil {
		a := t.o.Cfg.AutonomousConfig()
		max = (a.MaxToolSteps + ChunkAmpliacio - 1) / ChunkAmpliacio
	} else if !t.autonom() {
		// Interactiu: hi ha una persona mirant la pantalla, i el
		// contracte és parlar-hi cada tres trossos, no dotze.
		max = MaxAmpliacionsInteractiu
	}
	if t.ampliacions >= max || t.doomStalls >= 2 {
		return false
	}
	if t.ratxa.BloquejaAmpliacio(t.autonom()) {
		return false
	}
	if t.ampliacions > 0 && t.execs <= t.execsSnap {
		return false
	}
	return true
}

// Cancella tanca el torn per voluntat de l'usuari.
func (t *Torn) Cancella() []Event {
	t.acabat = true
	t.pregunta = nil
	t.perExecutar, t.pendents, t.teActual = nil, nil, false
	return []Event{notaClau("est.agentCancel")}
}

// mor tanca el torn amb un error.
func (t *Torn) mor(text string) []Event {
	t.acabat, t.fallit, t.motiu = true, true, text
	t.resposta = ""
	return []Event{errEv(text)}
}

// RepPas alimenta el motor amb la resposta del model a OrdrePasModel (o
// a OrdreSintesi: allà no hi ha eines i el text és la resposta final).
func (t *Torn) RepPas(content string, calls []llm.ToolCall, err error) []Event {
	if err != nil {
		return t.repError(err)
	}
	t.serverRetries = 0
	t.recuperacions = 0
	t.mecanic = false // fins que RepExecucions digui el contrari
	t.Hist = append(t.Hist, llm.Message{Role: "assistant", Content: content, ToolCalls: calls})
	if len(calls) == 0 {
		return t.repFinal(content)
	}
	return t.repCrides(calls)
}

// repError decideix si es reintenta el pas o si el torn es dona per mort.
func (t *Torn) repError(err error) []Event {
	// Topall de recuperacions seguides: si el proveïdor continua dient que
	// no hi cap per molt que es retalli, és millor morir amb l'error que
	// fer voltes (la web el tenia a 5; el motor no en tenia cap).
	if ok, actual := ParseContextExceeded(err); ok && t.recuperacions < 5 {
		if evs, fet := t.retallaPerContext(actual); fet {
			t.recuperacions++
			// El pas no s'ha arribat a fer: no compta.
			if t.passos > 0 {
				t.passos--
			}
			return evs
		}
	}
	// Error de servidor a mig torn: es reintenta amb guia (sovint la
	// crida era degenerada) abans de donar la feina per perduda.
	if llm.IsRetryable(err) && t.serverRetries < 2 {
		t.serverRetries++
		t.Hist = append(t.Hist, llm.Message{Role: "user", Content: GuiaErrorServidor(err)})
		if t.passos > 0 {
			t.passos--
		}
		return []Event{notaClau("app.serverReintentCurt")}
	}
	return t.mor("agent: error: " + err.Error())
}

// retallaPerContext fa lloc a l'historial quan el proveïdor diu que el
// prompt no hi cap (actual és la finestra que ha dit, 0 si no l'ha dit).
// Torna si ha pogut treure res.
func (t *Torn) retallaPerContext(actual int) ([]Event, bool) {
	if actual > 0 {
		if t.o.Rol != nil && t.o.Cfg != nil {
			prov, model := t.o.Rol()
			if p, hi := t.o.Cfg.Providers[prov]; hi {
				LearnWindow(p.BaseURL, model, actual)
			}
		}
		if t.o.AprenFinestra != nil {
			t.o.AprenFinestra(actual)
		}
	}
	if len(t.Hist) == 0 {
		return nil, false
	}
	abans := len(t.Hist)
	finestra := actual
	if finestra <= 0 && t.o.Finestra != nil {
		finestra, _ = t.o.Finestra()
	}
	if finestra <= 0 {
		finestra = DefaultWindow
	}
	fits := func(hist []llm.Message) bool {
		sys := ""
		if t.o.SysPrompt != nil {
			sys = t.o.SysPrompt()
		}
		if sys != "" {
			hist = append([]llm.Message{{Role: "system", Content: sys}}, hist...)
		}
		return !NeedsCompact(llm.EstimateTokens(hist), finestra)
	}
	podat, retallades := PruneToolOutputsUntil(t.Hist, fits)
	// La resposta exacta del proveïdor és autoritària: si retallar les
	// sortides no basta, reduïm la cua fins que el prompt torni a cabre.
	// TrimKeepSafe conserva la tasca en curs encara que haguem de treure
	// les crides d'eina i els resultats més recents.
	for keep := KeepRecentOnCompact; !fits(podat) && keep >= 0; keep-- {
		podat = TrimKeepSafe(podat, keep)
	}
	if len(podat) < len(t.Hist) || retallades > 0 {
		t.Hist = podat
		return []Event{notaClau("compact.fet", abans, len(t.Hist), 0)}, true
	}
	return nil, false
}

// repFinal tracta un torn que acaba sense cridar cap eina.
func (t *Torn) repFinal(content string) []Event {
	var evs []Event
	final := strings.TrimSpace(content)
	// Alguns models escriuen la crida d'eina al cos del missatge quan
	// l'estructurada no els surt. El text mai s'executa: es guia el model
	// perquè la refaci, amb topall.
	net, hiEra := SenseEinaText(final)
	if hiEra && t.textRetries < 2 {
		t.textRetries++
		t.Hist = append(t.Hist, llm.Message{Role: "user", Content: GuiaEinaText})
		return append(evs, nota(AvisEinaText))
	}
	if hiEra {
		final = net
		evs = append(evs, nota(AvisEinaText))
	}
	if final == "" {
		// Resposta buida (el proveïdor tanca sense contingut, o el model
		// només ha pensat): un cop, síntesi sense eines, com feia la web.
		// Si la síntesi també surt buida, RepSintesi avisa amb tornBuit.
		if !t.sintesiDemanada {
			t.sintesiDemanada = true
			return evs
		}
		t.acabat = true
		return append(evs, notaClau("app.tornBuit", t.passos, t.Limit()))
	}
	// Torn tancat amb passos pendents al checklist: típic del model que
	// anuncia què farà i s'atura. Es guia i continua, amb topall.
	if fets, total := t.todoEstat(); total > 0 && fets < total && t.todosRetries < 2 {
		t.todosRetries++
		t.Hist = append(t.Hist, llm.Message{Role: "user", Content: GuiaTodosPendents})
		return append(evs, notaClau("app.todosPendents", fets, total))
	}
	// Tasca de reparació amb l'última eina en vermell: sense això el
	// model afirma èxit igualment (mesurat en local).
	if TascaDeReparacio(t.o.Tasca) && SemblaFracas(t.ultSortida) && t.verifRetries < 2 {
		t.verifRetries++
		t.Hist = append(t.Hist, llm.Message{Role: "user", Content: GuiaVerificacioFallida})
		return append(evs, notaClau("app.verifReintent"))
	}
	t.acabat = true
	t.resposta = final
	return append(evs, Event{Tipus: EvResposta, Text: final})
}

// repCrides aplica permisos i repeticions a les crides d'un pas.
func (t *Torn) repCrides(calls []llm.ToolCall) []Event {
	var evs []Event
	// Pregunta seleccionable: bloqueja el pas. La resta de crides del
	// mateix pas queden ajornades amb el seu tool_result, perquè cap
	// crida no es quedi sense resposta.
	if qi := IndexPregunta(calls); qi >= 0 {
		return t.repPregunta(calls, qi)
	}
	plans := PlanCalls(t.o.Pol, t.o.Mode, &t.doom, calls)
	var executables []llm.ToolCall
	for i, c := range calls {
		evs = append(evs, Event{Tipus: EvCrida, Eina: c.Function.Name, EinaArgs: c.Function.Arguments, ID: c.ID})
		pl := plans[i]
		if pl.Fixed != "" {
			vista := "bloquejada: " + pl.Reason
			if pl.Reason == "repetida" {
				vista = ""
			}
			ev := Event{Tipus: EvResultat, Eina: c.Function.Name, Text: vista, Fallada: true, ID: c.ID}
			if vista == "" {
				ev.Clau = "app.repetida"
			}
			evs = append(evs, ev)
			t.Hist = append(t.Hist, ToolMsg(c, pl.Fixed))
			continue
		}
		if pl.Ask && !(t.o.Recordat != nil && t.o.Recordat(c.Function.Name, c.Function.Arguments)) {
			t.pendents = append(t.pendents, c)
			continue
		}
		executables = append(executables, c)
	}
	if len(executables) > 0 || len(t.pendents) > 0 {
		t.doomStalls = 0
		t.perExecutar = executables
		return evs
	}
	// Totes repetides o bloquejades: dues passes així i es tanca.
	t.doomStalls++
	if t.doomStalls >= 2 {
		t.sintesiDemanada = true
		t.esgotat = true
		evs = append(evs, avisClau("app.senseAccionsNoves"))
	}
	return evs
}

// repPregunta desa la pregunta i ajorna la resta de crides del pas.
func (t *Torn) repPregunta(calls []llm.ToolCall, qi int) []Event {
	var evs []Event
	qc := calls[qi]
	evs = append(evs, Event{Tipus: EvCrida, Eina: qc.Function.Name, EinaArgs: qc.Function.Arguments, ID: qc.ID})
	if err := ValidaPregunta(qc.Function.Arguments); err != nil {
		evs = append(evs, Event{Tipus: EvResultat, Eina: qc.Function.Name, Text: "pregunta malformada: " + err.Error(), Fallada: true, ID: qc.ID})
		t.Hist = append(t.Hist, ToolMsg(qc, "PREGUNTA MALFORMADA: "+err.Error()+". Reformula-la amb query + 1-4 opcions."))
	} else {
		q := qc
		t.pregunta = &q
	}
	for _, c := range calls {
		if c.ID == qc.ID {
			continue
		}
		evs = append(evs, Event{Tipus: EvCrida, Eina: c.Function.Name, EinaArgs: c.Function.Arguments, ID: c.ID})
		evs = append(evs, Event{Tipus: EvResultat, Eina: c.Function.Name, Text: "ajornada: primer la pregunta", Fallada: true, ID: c.ID})
		t.Hist = append(t.Hist, ToolMsg(c, "AJORNADA: l'usuari està responent una pregunta. No s'ha executat; torna a demanar-la després si cal."))
	}
	return evs
}

// RepPregunta alimenta la resposta de l'usuari a una pregunta del model.
func (t *Torn) RepPregunta(resposta string) []Event {
	q := t.pregunta
	t.pregunta = nil
	if q == nil {
		return nil
	}
	t.Hist = append(t.Hist, ToolMsg(*q, resposta))
	t.mecanic = false // la resposta d'una persona és per pensar-hi
	return nil
}

// RepAprovacio alimenta la decisió sobre l'aprovació en curs. Amb permes,
// la crida passa a executar-se; sense, es respon al model que s'ha
// rebutjat i s'avança a la següent. motiu canvia el text que veu el
// model (el headless hi diu que cal --auto-approve).
func (t *Torn) RepAprovacio(permes bool, motiu ...string) []Event {
	if !t.teActual {
		return nil
	}
	t.teActual = false
	c := t.actual
	if permes {
		t.perExecutar = []llm.ToolCall{c}
		return nil
	}
	text := "EINA REBUTJADA per l'usuari"
	if len(motiu) > 0 && strings.TrimSpace(motiu[0]) != "" {
		text = motiu[0]
	}
	t.Hist = append(t.Hist, ToolMsg(c, text))
	t.mecanic = false // una negativa obliga a replantejar
	return []Event{notaClau("est.cancellat")}
}

// RepExecucions alimenta els resultats de les eines executades.
func (t *Torn) RepExecucions(res []Execucio) []Event {
	var evs []Event
	hiHaTodo := false
	for _, r := range res {
		if r.Call.Function.Name == "todowrite" {
			hiHaTodo = true
		}
	}
	if hiHaTodo {
		t.execsSenseTodo = 0
	}
	t.mecanic = lotMecanic(res)
	guiaRatxa := false
	for i, r := range res {
		sortida := r.Sortida
		t.execs++
		if !hiHaTodo {
			t.execsSenseTodo++
		}
		// Pla viu: el recordatori del checklist s'enganxa a l'últim
		// resultat del pas, mai com a missatge d'usuari.
		if i == len(res)-1 && !hiHaTodo {
			if n := RecordatoriTodos(t.todoLlista(), t.execsSenseTodo); n != "" {
				sortida += n
				t.execsSenseTodo = 0
			}
		}
		if i == len(res)-1 {
			sortida += t.recordatoriLectures(res)
		}
		t.ultSortida = r.Sortida
		t.Hist = append(t.Hist, ToolMsg(r.Call, sortida, r.Imatges...))
		evs = append(evs, Event{
			Tipus: EvResultat, Eina: r.Call.Function.Name, Text: r.Sortida,
			Fallada: strings.HasPrefix(r.Sortida, "ERROR:"),
		})
		if r.Diff != "" {
			evs = append(evs, Event{Tipus: EvDiff, Text: r.Diff})
		}
		// La mateixa sortida tres cops seguits no és progrés encara que
		// l'eina corri.
		if t.doom.NoteOut(r.Sortida) {
			t.Hist = append(t.Hist, llm.Message{Role: "user", Content: DoomGuide})
			evs = append(evs, Event{Tipus: EvResultat, Eina: r.Call.Function.Name, Text: DoomGuide, Fallada: true})
		}
		// Comprovacions (bash) que fallen seguides: el bucle del
		// «corre el test, retoqueja, corre el test» executa eines noves
		// a cada pas —l'antic criteri d'avenç sempre el concedia— però
		// no verifica res. Cada tres fracassos seguits, guia; una bash
		// verda, comptador a zero. Les edicions no el toquen: que un
		// write triomfi no vol dir que la comprovació passi.
		// Es mira la sortida crua (sense el recordatori del checklist) i
		// només les ordres que són comprovacions: un grep buit no és un
		// test vermell, ni un sed verd un test verd.
		// La guia va després de tots els resultats del pas: un missatge
		// d'usuari entre dos resultats d'eina trenca l'historial per als
		// proveïdors estrictes.
		if t.ratxa.Nota(r.Call, r.Sortida) {
			guiaRatxa = true
		}
	}
	if guiaRatxa {
		// Tres guies ignorades (nou vermelles seguides) en interactiu: prou.
		// Bloquejar l'ampliació no basta si el vermell no és de l'agent (un
		// entorn trencat al banc A/B: «could not import fmt» a cada go
		// test) i el tros base encara té passos: es cremaven fins al final.
		// Síntesi honesta, i que decideixi la persona. En autònom, que té
		// checkpoints propis, la ratxa continua sent només guia.
		if !t.autonom() && t.ratxa.N >= 9 {
			t.sintesiDemanada = true
			t.esgotat = true
			t.perExecutar, t.pendents, t.teActual = nil, nil, false
			evs = append(evs, avisClau("app.ratxaVermella", t.ratxa.N))
		} else {
			t.Hist = append(t.Hist, llm.Message{Role: "user", Content: GuiaFracasRepetit})
			// A la pantalla, un avís curt: la guia és per al model.
			evs = append(evs, avisClau("app.ratxaVermella", t.ratxa.N))
		}
	}
	// Topall autònom (temps, eines, cost): prou feina, però amb resum. Abans
	// el torn s'aturava en sec i el client es quedava sense resposta; la
	// web en feia síntesi pel seu compte i els altres no.
	if motiu := t.topall(); motiu != "" {
		t.sintesiDemanada = true
		t.esgotat = true
		t.perExecutar, t.pendents, t.teActual = nil, nil, false
		return append(evs, avis("autònom: "+motiu+": torn aturat, faig el resum"))
	}
	if t.autonom() && t.execs >= t.autoNextCheckpoint {
		t.checkpointPendent = true
	}
	return evs
}

// RatxaVermella compta les comprovacions (bash de test, build, vet, lint)
// que han fallat SEGUIDES des de l'última que ha passat. La comparteixen el
// motor i el bucle propi de la web perquè el topall contra el «corre el
// test, retoqueja, corre el test» sigui el mateix a totes les pantalles.
type RatxaVermella struct{ N int }

// Nota registra el resultat d'una eina i diu si toca guiar el model
// (cada tres vermelles seguides). Les eines que no són comprovacions no hi
// fan res: ni una edició que triomfa ni un grep buit.
func (r *RatxaVermella) Nota(c llm.ToolCall, out string) bool {
	if c.Function.Name != "bash" || strings.TrimSpace(out) == "" || !esComprovacioCrida(c) {
		return false
	}
	if strings.HasPrefix(out, "ERROR:") || SemblaFracas(out) {
		r.N++
		return r.N%3 == 0
	}
	r.N = 0
	return false
}

// BloquejaAmpliacio diu si la ratxa ja no deixa demanar més pressupost.
// En autònom el TDD passa per vermells de debò i hi ha checkpoints
// propis: hi talla al doble (la guia ja s'ha ignorat dos cops).
func (r RatxaVermella) BloquejaAmpliacio(autonom bool) bool {
	if autonom {
		return r.N >= 6
	}
	return r.N >= 3
}

// FinalPromptPer tria la síntesi: la curta quan el torn s'ha tancat per la
// via ràpida, la llarga i honesta quan s'ha fet gran (ampliacions gastades
// o ratxa de vermelles) i l'usuari ha de decidir si continuar.
func FinalPromptPer(passos, ampliacions int, r RatxaVermella) string {
	if ampliacions >= 1 || r.N >= 3 {
		return FinalPromptLlarg(passos, r.N)
	}
	return FinalPrompt
}

// esComprovacioCrida llegeix l'ordre d'una crida bash i diu si és una
// comprovació (EsComprovacio).
func esComprovacioCrida(c llm.ToolCall) bool {
	var a struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(c.Function.Arguments), &a) != nil {
		return false
	}
	return EsComprovacio(a.Command)
}

// topall retorna el motiu d'aturada del mode autònom ("" per continuar).
func (t *Torn) topall() string {
	if !t.autonom() {
		return ""
	}
	prov, model := "", ""
	if t.o.Rol != nil {
		prov, model = t.o.Rol()
	}
	return TopallAutonom(t.o.Cfg, EstatAutonom{
		Execs: t.execs, Inici: t.autoInici, Hist: t.Hist, Provider: prov, Model: model,
		Usage: t.o.Usage,
	})
}

// RepCheckpoint alimenta el resultat d'un checkpoint autònom.
func (t *Torn) RepCheckpoint(resum, veredicte string) []Event {
	t.autoCheckpoint++
	if t.o.Cfg != nil {
		t.autoNextCheckpoint += t.o.Cfg.AutonomousConfig().CheckpointEvery
	}
	t.Hist = append(t.Hist, llm.Message{Role: "user", Content: resum})
	evs := []Event{notaClau("app.checkpoint", t.autoCheckpoint)}
	if veredicte != "" {
		if strings.HasPrefix(veredicte, "APROVAT") {
			evs = append(evs, nota("✓ "+veredicte))
		} else {
			evs = append(evs, avis("⚠ "+veredicte))
		}
	}
	return evs
}

// CheckpointNum és el número del checkpoint autònom en curs.
func (t *Torn) CheckpointNum() int { return t.autoCheckpoint }

// RepCompactacio alimenta l'historial ja compactat.
func (t *Torn) RepCompactacio(room RoomResult) []Event {
	var evs []Event
	if room.Hist != nil {
		t.Hist = room.Hist
	}
	if n := room.Note(); n != "" {
		evs = append(evs, nota(n))
	}
	if room.Err != nil {
		evs = append(evs, notaClau("compact.fallida", room.Err.Error()))
	}
	return evs
}

// RepAmpliacio alimenta la resposta del model a «CONTINUA <n> o FINAL».
func (t *Torn) RepAmpliacio(resp string, err error) []Event {
	if err == nil {
		if n, ok := ParseAmpliacio(resp); ok {
			t.extres += n
			t.ampliacions++
			t.execsSnap = t.execs
			return []Event{notaClau("app.ampliaSegueix", n, t.Limit())}
		}
	}
	// No s'ha entès (o la crida ha petat) i la checklist és a mitges: es
	// continua igualment. Tancar amb un resum perquè el model no ha dit la
	// paraula exacta és deixar la feina a mig fer per una formalitat; el
	// que protegeix del bucle etern és potAmpliar (execs noves des de
	// l'última), no la paraula.
	if fets, total := t.todoEstat(); total > 0 && fets < total {
		t.extres += ChunkAmpliacio
		t.ampliacions++
		t.execsSnap = t.execs
		motiu := "error"
		if err == nil {
			motiu = "«" + retallaText(resp, 60) + "»"
		}
		return []Event{notaClau("app.ampliaPerChecklist", motiu, fets, total, ChunkAmpliacio, t.Limit())}
	}
	// FINAL, buit o error: síntesi com sempre.
	t.sintesiDemanada = true
	t.esgotat = true
	return []Event{avisClau("app.passosEsgotats", t.Limit())}
}

// retallaText deixa el text en una línia de n runes com a molt.
func retallaText(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// Aprovacio és la crida que espera permís ara mateix (zero si cap).
func (t *Torn) Aprovacio() llm.ToolCall {
	if !t.teActual {
		return llm.ToolCall{}
	}
	return t.actual
}

// Retalla fa lloc a l'historial en dur (sortides d'eina velles fora i
// només els missatges recents). Torna quants n'hi havia i quants en
// queden. És l'últim recurs quan el proveïdor diu que el context no hi
// cap i no hi ha temps de demanar un resum.
func (t *Torn) Retalla() (abans, ara int) {
	abans = len(t.Hist)
	podat, _ := PruneToolOutputs(t.Hist)
	t.Hist = TrimKeepSafe(podat, KeepRecentOnCompact)
	return abans, len(t.Hist)
}
