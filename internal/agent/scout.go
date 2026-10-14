package agent

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// L'investigador d'inici de torn (scout): quan una tasca de code,
// autonomous o debug demana investigar, abans del primer pas del
// coordinador corre un subagent real de només lectura (el mateix runner
// que delegate) que torna un resum amb camins de fitxer. El resum entra a
// l'historial del torn perquè el coordinador no torni a fer la mateixa
// exploració. Emet els mateixos events de delegate (la sala d'agents hi
// veu un fill real) i, si falla, el torn continua com sempre.

// ScoutAgentName és el nom amb què el scout surt als events de delegate
// (DelegateEvent.Agent) quan no hi ha cap subagent «researcher» definit.
const ScoutAgentName = "scout"

const (
	scoutMaxSteps = 8
	scoutTimeout  = 4 * time.Minute
	scoutMaxTask  = 1500
)

// scoutGreeting: cortesia o confirmacions curtes que mai no demanen
// investigar («hola», «gràcies», «ok, fes-ho»).
var scoutGreeting = regexp.MustCompile(`(?i)^\s*(hi|hello|hey|hola|bon dia|bona tarda|thanks|thank you|gràcies|merci|ok|okay|d'acord|vale|yes|no|sí|si|continue|continua|go on|segueix)\b`)

// scoutSignals: paraules o formes que indiquen feina sobre el codi, un
// error o una recerca (fitxers amb extensió, camins, «bug», «web»…).
var scoutSignals = regexp.MustCompile(`(?i)(\b(file|files|fitxer|fitxers|bug|bugs|error|errors|crash|fail|failing|fix|arregla|feature|implement|implementa|add|afegeix|refactor|investigate|investiga|research|recerca|why|per què|perquè|how|com|where|on|find|busca|search|web|docs?|documentation|test|tests|endpoint|api|function|funció|module|mòdul|class|component)\b|\w\.(go|py|js|ts|tsx|jsx|rs|java|kt|cs|cpp|c|h|rb|php|md|json|yaml|yml|toml|html|css|sql|sh)\b|[\w-]+/[\w./-]+)`)

// scoutSmallEdit: canvis petits i ja concrets on una investigació prèvia
// costaria més que la feina («rename x to y», «fix typo in README»).
var scoutSmallEdit = regexp.MustCompile(`(?i)\b(typo|rename|canvia el nom|bump|version|versió)\b`)

// ScoutWanted decideix si el torn comença amb l'investigador: només en els
// modes que actuen (code, autonomous, debug), amb agent.scout actiu i
// quan la tasca no és trivial (salutacions, confirmacions i edicions
// petites queden fora).
func ScoutWanted(cfg *config.Config, mode, task string) bool {
	if cfg == nil || !cfg.ScoutOn() {
		return false
	}
	switch mode {
	case ModeCode, ModeAutonomous, ModeDebug:
	default:
		return false
	}
	return scoutTaskNeedsResearch(task)
}

// scoutTaskNeedsResearch és l'heurística pura (sense config ni mode).
func scoutTaskNeedsResearch(task string) bool {
	t := strings.TrimSpace(task)
	words := strings.Fields(t)
	if len(words) < 4 {
		return false
	}
	if scoutGreeting.MatchString(t) && len(words) < 8 {
		return false
	}
	if scoutSmallEdit.MatchString(t) && len(words) <= 10 {
		return false
	}
	if len(words) > 8 {
		return true
	}
	return scoutSignals.MatchString(t)
}

// ScoutPrompt és l'encàrrec que rep el subagent investigador.
func ScoutPrompt(task string) string {
	r := []rune(strings.TrimSpace(task))
	if len(r) > scoutMaxTask {
		r = append(r[:scoutMaxTask], '…')
	}
	return "Investigate what the coordinator needs to do this task: the relevant files, the current behaviour, constraints (tests, conventions, dependencies) and facts from the web only if the task really needs them. Do not change anything. " +
		"Return a concise brief (at most ~25 lines): file paths (with line numbers where useful), what exists today, what has to change and any risk. No preamble.\n\nTASK:\n" + string(r)
}

// ScoutResult és el que torna RunScout. Brief buit = no hi ha res a
// injectar (falla, cancel·lació o resposta buida); Err en diu el motiu.
type ScoutResult struct {
	ID    string
	Agent string
	Brief string
	Err   error
}

// ScoutOptions: el directori i la sessió on treballa el fill. ID és
// opcional (el client el pot voler saber abans, per lligar-hi la sala).
type ScoutOptions struct {
	ID        string
	Cfg       *config.Config
	Task      string
	Workspace string
	Session   string
}

// scoutDef tria la definició del scout: un subagent «researcher» de només
// lectura definit per l'usuari si n'hi ha (amb el seu model i prompt);
// si no, el genèric de lectura amb 8 passos.
func scoutDef(workspace string) *SubagentDef {
	if def, ok := FindSubagent(LoadSubagents(workspace), "researcher"); ok && def.Mode != ModeCode {
		if def.MaxSteps <= 0 || def.MaxSteps > scoutMaxSteps {
			def.MaxSteps = scoutMaxSteps
		}
		return &def
	}
	return &SubagentDef{
		Name:        ScoutAgentName,
		Description: "Researcher. You investigate before the coordinator acts and hand it a brief; you never edit.",
		Mode:        ModeInspect,
		MaxSteps:    scoutMaxSteps,
	}
}

// RunScout corre l'investigador amb el runner de subagents del context (el
// del torn a la web; si no, el global) i torna el resum retallat. Comparteix
// els límits de delegate (semàfor del torn i sostre global de fills vius),
// penja del context del torn (cancel·lar el torn el mata) i té el seu propi
// comptador de tokens encadenat al del torn.
func RunScout(ctx context.Context, o ScoutOptions) ScoutResult {
	if ctx == nil {
		ctx = context.Background()
	}
	runner := currentDelegateRunner()
	var semaphore chan struct{}
	if scoped, ok := ctx.Value(delegateRunnerContextKey{}).(delegateRunnerContext); ok {
		runner, semaphore = scoped.run, scoped.semaphore
	}
	if runner == nil {
		return ScoutResult{Err: errors.New("subagents not available")}
	}
	def := scoutDef(o.Workspace)
	prompt := ScoutPrompt(o.Task)
	id := o.ID
	if id == "" {
		id = nextDelegateID()
	}
	res := ScoutResult{ID: id, Agent: def.Name}
	childCtx, cancel := context.WithTimeout(withDelegateScope(ctx, o.Session, o.Workspace), scoutTimeout)
	defer cancel()
	childCtx = withDelegateIdentity(childCtx, id, prompt, time.Now())
	childCtx = withSubagentDef(childCtx, def)
	childCtx = llm.WithUsage(childCtx, &llm.UsageMeter{})
	fail := func(err error) ScoutResult {
		emitDelegateEvent(childCtx, DelegateEvent{Status: delegateTerminalStatus(err, ctx.Err()), Message: err.Error(), MaxSteps: def.MaxSteps})
		res.Err = err
		return res
	}
	if semaphore != nil {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
		case <-childCtx.Done():
			return fail(childCtx.Err())
		}
	}
	select {
	case delegateGlobalLive <- struct{}{}:
		defer func() { <-delegateGlobalLive }()
	case <-childCtx.Done():
		return fail(childCtx.Err())
	}
	emitDelegateEvent(childCtx, DelegateEvent{Status: DelegateWorking, Message: "started", MaxSteps: def.MaxSteps})
	out, err := runDelegateSafely(runner, childCtx, prompt)
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(out) == "" {
		return fail(errors.New("empty brief"))
	}
	emitDelegateEvent(childCtx, DelegateEvent{Status: DelegateCompleted, Message: "completed", Output: delegatePreview(out)})
	res.Brief = retallaResum(out)
	return res
}

// NewDelegateID dona un id de fill nou (el mateix format que delegate).
func NewDelegateID() string { return nextDelegateID() }

// ScoutNote és el text que rep el coordinador amb el resum.
func ScoutNote(brief string) string {
	return "RESEARCHER BRIEF — a read-only researcher subagent already investigated this task before you started. " +
		"Use it as your starting point: do not repeat its exploration; read a file again only when you need its exact current lines to edit it.\n" +
		strings.TrimSpace(brief)
}

// InjectScoutBrief afegeix el resum a l'últim missatge de l'usuari (la
// tasca) d'una còpia de l'historial. Va al mateix missatge i no en un de
// nou perquè alguns proveïdors no accepten dos torns d'usuari seguits.
// Sense missatge d'usuari, n'afegeix un.
func InjectScoutBrief(hist []llm.Message, brief string) []llm.Message {
	if strings.TrimSpace(brief) == "" {
		return hist
	}
	out := append([]llm.Message(nil), hist...)
	note := ScoutNote(brief)
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role == "user" {
			out[i].Content = strings.TrimRight(out[i].Content, "\n") + "\n\n" + note
			return out
		}
	}
	return append(out, llm.Message{Role: "user", Content: note})
}
