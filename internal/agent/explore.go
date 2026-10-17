package agent

import (
	"strconv"
	"strings"

	"gregal/internal/config"
)

// L'explorador: la versió del scout per a les preguntes (mode inspect).
// En un projecte gran, una pregunta com «com detecta Gregal un bucle de
// crides?» costava 7-24 passos seqüencials de grep/read al coordinador,
// cadascun reenviant tot el prompt (eines, mapa, historial): 86-733 k
// tokens d'entrada per pregunta (docs/research/explore.md). L'explorador fa
// aquesta cerca en un fill de només lectura, amb:
//
//   - només read, grep, glob i symbols (un prefix més curt a cada pas que
//     les ~20 eines del coordinador);
//   - un sostre de passos (exploreMaxSteps) i l'avís de tancament de
//     sempre: quan s'acaba, respon amb el que té, marcat com a parcial;
//   - el mapa del projecte al prompt, perquè el primer pas ja vagi a
//     fitxers concrets;
//   - una resposta amb cites camí:línia que el coordinador fa servir tal
//     qual: no torna a obrir els fitxers per comprovar-les.
//
// Les lectures queden dins del fill: a la web, al TUI o al Telegram, la
// pregunta següent de la mateixa conversa no les torna a enviar.

// ExploreAgentName és el nom de l'explorador als events de delegate.
const ExploreAgentName = "explorer"

// exploreMaxSteps: el banc de preguntes sobre aquest repositori (5
// preguntes, mediana de 14 passos al coordinador) es respon dins de 8
// passos quan cada pas porta diverses crides en paral·lel.
const exploreMaxSteps = 8

// exploreTools són les eines de l'explorador: les de cercar i llegir codi.
var exploreTools = []string{"read", "grep", "glob", "symbols"}

// ExploreWanted diu si una pregunta en mode inspect comença amb
// l'explorador: agent.scout actiu, una pregunta que demana mirar el codi
// (les mateixes regles que el scout) i un projecte que no és petit (en un
// de petit el coordinador ho llegeix tot en un pas). Amb agent.scout:
// always, també en projectes petits.
func ExploreWanted(cfg *config.Config, mode, task, workspace string) bool {
	if cfg == nil || !cfg.ScoutOn() || mode != ModeInspect {
		return false
	}
	if !scoutTaskNeedsResearch(task) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Agent.Scout), "always") {
		return true
	}
	return !projectePetit(workspace)
}

// exploreDef és la definició del fill explorador. Un subagent «explorer»
// definit per l'usuari mana (amb el seu model i prompt), però sempre de
// lectura i amb un sostre de passos.
func exploreDef(workspace string) *SubagentDef {
	if def, ok := FindSubagent(LoadSubagents(workspace), ExploreAgentName); ok && def.Mode != ModeCode {
		if def.MaxSteps <= 0 || def.MaxSteps > exploreMaxSteps*2 {
			def.MaxSteps = exploreMaxSteps
		}
		return &def
	}
	return &SubagentDef{
		Name:        ExploreAgentName,
		Description: "Explorer. You search the code base to answer a question and report the answer with path:line citations; you never edit.",
		Mode:        ModeInspect,
		MaxSteps:    exploreMaxSteps,
		Tools:       append([]string(nil), exploreTools...),
	}
}

// ExplorePrompt és l'encàrrec de l'explorador: la pregunta (o l'àrea que
// li toca, si n'hi ha diversos), com treballar barat i què ha de tornar.
// mapa és el mapa del projecte (pot ser buit).
func ExplorePrompt(task, focus, mapa string) string {
	r := []rune(strings.TrimSpace(task))
	if len(r) > scoutMaxTask {
		r = append(r[:scoutMaxTask], '…')
	}
	var b strings.Builder
	if f := strings.TrimSpace(focus); f != "" {
		b.WriteString("You are one of several explorers working in parallel on the same question; each covers one part. YOUR PART: " + f + "\nCover only your part.\n\n")
	}
	b.WriteString("Find in the code base the answer to the QUESTION below and report it.\n" +
		"Work in few steps: in every step send ALL the calls you can already think of at once (several greps, then reads of every hit you need). " +
		"grep takes text or a regex (a|b). Read only the lines you need (offset/limit around the grep hits), not whole files. " +
		"Stop searching as soon as you can answer. You have at most " + strconv.Itoa(exploreMaxSteps) + " steps.\n" +
		"Report, with no preamble: the answer, where every claim cites path:line (paths as in the project map, line numbers you saw in read or grep output). " +
		"Quote identifiers exactly. Mark with UNVERIFIED only what the question needs and you could not check. Keep it under ~40 lines.\n\nQUESTION:\n")
	b.WriteString(string(r))
	if m := strings.TrimSpace(mapa); m != "" {
		b.WriteString("\n\n" + m)
	}
	return b.String()
}

// ExploreRespondreAl és el pas del coordinador després del qual, amb un
// informe de l'explorador, se li recorda que respongui (OpcionsTorn.RespondreAl).
const ExploreRespondreAl = 2

// AvisRespondre és el recordatori: ja té l'informe i dos passos de lectures
// pròpies; el que falti es diu a la resposta.
const AvisRespondre = "You already have the explorer report and have done your own reads on top of it: answer the question now from what you know, with the path:line citations. Say plainly what is still unverified instead of searching further."

// ExploreNote és el text que rep el coordinador amb l'informe.
func ExploreNote(brief string) string {
	return "EXPLORER REPORT — read-only explorer subagents already searched the code for this question before you started (one per AREA when it is split). " +
		"Answer from this report and keep its path:line citations: they come from lines the explorer read. Do not repeat its searches and do not re-read files just to double-check it. " +
		"Answer now unless the report misses something the question cannot do without (marked UNVERIFIED or PARTIAL, or not covered): then spend at most one or two steps of targeted reads (offset/limit) on that point only. " +
		"A side detail that stays unverified is said as such in the answer, not chased.\n" +
		strings.TrimSpace(brief)
}
