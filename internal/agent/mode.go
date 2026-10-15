package agent

import (
	"strings"

	"gregal/internal/config"
	"gregal/internal/goal"
)

// Modes de treball. "code" executa eines amb la política normal (escriptures
// dins del projecte passen soles; la resta demana, amb opció de recordar per
// sessió), "chat" és conversa sense agent, "inspect" consulta el projecte
// automàticament sense escriure i "goal" concreta un objectiu abans
// d'executar-lo.
const (
	ModeCode       = "code"
	ModeChat       = "chat"
	ModeInspect    = "inspect"
	ModeGoal       = "goal"
	ModeAutonomous = "autonomous"
	// ModeDebug és el mode depuració (debugmode.go): hipòtesis,
	// instrumentació temporal marcada, evidència i neteja.
	ModeDebug = "debug"
)

// ValidMode diu si m és un mode suportat.
func ValidMode(m string) bool {
	return m == ModeCode || m == ModeChat || m == ModeInspect || m == ModeGoal || m == ModeAutonomous || m == ModeDebug
}

// ModeFromWord tradueix el nom que veu la persona al mode intern. Els dos
// modes principals es diuen Ask/Act (Pregunta/Actua) a totes les
// interfícies; els ids interns (chat/code…) es mantenen perquè els fan
// servir el config, l'API i les sessions desades. Torna "" si no el coneix.
func ModeFromWord(w string) string {
	switch strings.ToLower(strings.TrimSpace(w)) {
	case ModeChat, "ask", "pregunta", "xat":
		return ModeChat
	case ModeCode, "act", "actua", "codi":
		return ModeCode
	case ModeInspect, "consulta":
		return ModeInspect
	case ModeGoal, "objectiu":
		return ModeGoal
	case ModeAutonomous, "autonom", "autònom":
		return ModeAutonomous
	case ModeDebug, "depuracio", "depuració":
		return ModeDebug
	}
	return ""
}

// PromptFor afegeix al prompt base els fets de l'entorn i les regles del
// mode. Els fets van aquí, i no als set llocs que criden PromptFor, perquè
// n'hi hauria hagut prou que un s'oblidés: el TUI, la finestra, el
// headless, el Telegram i els subagents treballen a la mateixa màquina i
// tots hi ensopeguen igual.
//
// La directiva d'idioma que porta el base (config.SystemPrompt) es torna a
// posar al final de tot: és l'última cosa que llegeix el model abans de la
// conversa, i amb la resta del prompt en anglès és la que mana.
func PromptFor(base, mode string) string {
	base, llengua := config.SplitLanguageDirective(base)
	if f := FetsEntorn(); f != "" {
		base += "\n\n" + f
	}
	base += reglesMode(mode)
	// La data va al final per mantenir estable el prefix reutilitzable pel KV
	// cache dels models locals; només canvia una vegada al dia.
	if d := DataAra(); d != "" {
		base += "\n\n" + d
	}
	if llengua != "" {
		base += "\n\n" + llengua
	}
	return base
}

// AmbContext afegeix blocs (memòria del projecte, skills, un resum...) a
// un system prompt ja compost i hi manté la directiva d'idioma al final.
// Els blocs buits no hi afegeixen res.
func AmbContext(prompt string, blocs ...string) string {
	cos, llengua := config.SplitLanguageDirective(prompt)
	for _, b := range blocs {
		if strings.TrimSpace(b) != "" {
			cos += "\n\n" + b
		}
	}
	if llengua != "" {
		cos += "\n\n" + llengua
	}
	return cos
}

// reglesMode és el text específic de cada mode.
func reglesMode(mode string) string {
	switch mode {
	case ModeChat:
		return chatRules
	case ModeInspect:
		return inspectRules
	case ModeGoal:
		return goal.Prompt()
	case ModeAutonomous:
		return autonomousRules
	case ModeCode:
		return codeRules
	case ModeDebug:
		return debugRules
	default:
		return ""
	}
}

// chatRules: el xat pot llegir (read és allow) i ho ha de saber; sense la
// línia de question el model preguntava en text pla o endevinava.
const chatRules = "\nCHAT MODE: conversation with read access. You MAY read the disk with read/glob/grep when useful: do it directly, without asking permission or announcing limits. Do not write or modify anything; if a change is needed, tell the user to switch to /mode code. If you need a decision from the user to continue (two incompatible options, a preference you cannot infer), call the `question` tool with 2-4 short options BEFORE answering instead of asking in plain text. Do not ask about anything you can check by reading the disk."

// inspectRules és també el prompt dels subagents (delegate).
const inspectRules = "\nINSPECT MODE: inspect and explain the project without modifying it. Use read, grep, glob, web_search, web_fetch and only clearly read-only shell commands. Do not ask for permissions: if an action is not safe or not read-only, do not run it and explain the limit."

// codeRules són les regles del mode code. El que cal saber abans de tocar-les:
//
//   - question: sense cap regla el model no la cridava mai (0/16) i,
//     davant d'una petició ambigua, triava ell (SQLite, JSON, una
//     interfície) i escrivia. Amb la regla i la prohibició d'escriure
//     dependències, formats o API no triats: 11/16. Amb una tasca
//     inequívoca, 0/12: no es posa a preguntar per tot. La descripció de
//     l'eina no movia res; el que decideix és aquest text.
//   - todowrite depèn del model (Nex-2.5-mini 4/4, Qwen38-Flash 1/4,
//     Ornith 0/4). Ara és opcional: només per a feines de diverses parts.
//   - delegate ja no obre les regles: els models petits delegaven tasques
//     trivials i després refeien la feina dels subagents.
//   - Res de git stash per mirar si un vermell és preexistent: amb feina de
//     l'usuari sense desar, apartar-la i tornar-la és perillós.
//
// Les mesures són de la versió catalana. El 2026-10 es va reescriure en
// anglès, numerada i més curta perquè els models «flash» la seguien a
// mitges i responien en català a tasques en anglès; la llengua de
// resposta la fixa ara config.LanguageDirective.
const codeRules = `
CODE MODE RULES
1. Check, don't guess: look things up with read/grep/glob/bash before answering or editing; never answer questions about the project from memory. Batch independent reads and searches in one step (they run in parallel).
2. Ask only real decisions: if a choice changes the shape of the work and the project does not settle it (which library, which output format, which of two incompatible behaviours), call ` + "`question`" + ` with 2-4 short options before writing anything. Do not ask about what you can check yourself. Never introduce a new dependency, storage format or public API the user has not chosen.
3. Do what was asked, nothing more: make the smallest direct change that fixes the root cause, following what the code already states (docstrings, comments, names, types). No unrequested features, options or refactors, and no special cases that only satisfy the test.
4. Exact names: keep file and symbol names exactly as they appear in the task or the repository; never translate, rename or invent them. If a file or symbol the user mentions does not exist, say so instead of changing unrelated files.
5. Verify: after editing, run the relevant check (tests, build, vet, lint) and fix the failures you caused before answering. To tell whether a failure was already there, look at your own changes (` + "`git diff`" + `) or run the check in a clean ` + "`git worktree`" + ` of HEAD. Never run ` + "`git stash`, `git checkout` or `git reset`" + ` on the user's files. If the failure is pre-existing or outside the task, report the check and the exact error, and stop.
6. Honesty: never claim that tests pass or that something works unless you ran the check and saw it pass. If you could not run or finish something, say so.
7. ` + "`todowrite`" + ` is optional: use it only for genuinely multi-part tasks, with one item ` + "`working`" + ` at a time. Skip it for simple tasks.
8. ` + "`delegate`" + ` only for two or more independent, non-trivial investigations; subagents can only read. Use their summaries and do not redo their work.
8b. Research only what the result needs: for something to build (a page, a script, a document), a few reliable sources are enough. Do not re-fetch the same page with different filters or chase small details before building; build first, then refine if the user asks.
9. Finish: when the work is done and verified, stop calling tools and reply briefly with what you changed (file and what), how you verified it (command and result) and what is still pending or failing. Do not paste code that is already in the files.`

// autonomousRules afegeix al mode code un contracte de llarg abast. El loop
// continua sent el mateix: el mode canvia el criteri de finalització i obliga
// el model a treballar per fites, verificar i demanar només decisions reals.
// La regla dels fitxers grans hi és perquè reconstruir un fitxer amb writes
// successius es menjava els passos i els minuts del torn.
const autonomousRules = codeRules + `

AUTONOMOUS MODE (long task)
- Work on your own: nobody answers questions during an autonomous run and ` + "`question`" + ` is disabled. Treat the request as a contract: work out the goal, the milestones and the acceptance criteria yourself; when a product decision is missing, pick the most reasonable option, keep going, and list your assumptions in the final answer.
- You may read, search the web, write anything inside the project folder and run normal development commands without asking. Only clearly dangerous actions (mass deletes, destructive git, global installs, writing outside the folder, external services) wait for the user — avoid them unless essential.
- This is a multi-part task: keep the milestones in ` + "`todowrite`" + `. Work in small milestones; after each one, update the checklist and run a suitable check (tests, build, vet, lint, or reading the result). A finished phase is not a finished task.
- When a step budget runs out you will be asked whether to continue: answer CONTINUE while you are making progress, and FINAL only when the acceptance criteria are met or you are blocked on the user.
- Keep the original scope; note interesting extras as pending instead of doing them.
- Large files (single-page HTML/CSS/JS, documents, data files): write the first complete version in ONE write call, then refine it with patch. Never build a file piece by piece with successive writes. If a write came out truncated, read what is there and continue with patch.`
