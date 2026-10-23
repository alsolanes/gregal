package agent

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gregal/internal/tools"
)

// FinalPrompt demana la síntesi final sense eines quan s'esgota el
// pressupost de passos (estil opencode: resumeix + què falta, no buit).
// Com totes les guies, és en anglès; la llengua de la resposta la fixa la
// directiva LANGUAGE que tanca el system prompt (config.LanguageDirective),
// i aquí només s'hi remet.
const FinalPrompt = "The actions are over. Now give the user a clear final answer with what you found or did. If you could not finish everything, summarize what you did and what is still left to do. Do not use tools and do not explain your internal reasoning. " + SkillSuggestionGuidance + " " + RecordaLlengua

// SkillSuggestionGuidance fa que l'agent ofereixi una skill només quan la
// tasca completada hagi revelat un procediment reutilitzable i específic.
// La proposta queda a la resposta perquè la persona la revisi abans de desar.
const SkillSuggestionGuidance = `After a successfully completed, non-autonomous task, consider whether it revealed a concrete, reusable workflow or decision rule specific to this user's project or repeated work. Suggest at most one skill, and only when it adds knowledge not already covered by the enabled skill index, existing project instructions, or a saved skill. Never turn a one-off request, generic expertise, or sensitive information into a suggestion. Usually make no suggestion. When clearly worthwhile, add a short "Skill suggestion" section with a proposed name, why it is reusable, a compact draft of its trigger and key steps, and whether it belongs to this project or the user's personal skills. Ask whether the user wants it saved. Do not create or save it until the user explicitly agrees.`

// RecordaLlengua tanca les consignes de síntesi: remet a la directiva
// d'idioma del system prompt, que és la que sap la llengua de la sessió.
const RecordaLlengua = "Write it in the language required by the LANGUAGE rule at the end of the system prompt."

// FinalPromptLlarg és la síntesi quan el torn s'ha fet sospitosament llarg:
// ampliacions gastades o ratxa de comprovacions vermelles. Aquí no basta un
// «què he fet»: l'usuari ha de poder decidir si val la pena continuar, i per
// això cal l'estat real del verd i del vermell, amb l'error concret i sense
// amagar el fracàs preexistent (que no és culpa del torn, però sí cal
// explicar-lo).
func FinalPromptLlarg(passos, fracasos int) string {
	p := fmt.Sprintf("The actions stop here: this task has already used a lot of budget (%d steps", passos)
	if fracasos > 0 {
		p += fmt.Sprintf(", %d failing checks in a row", fracasos)
	}
	p += `), so instead of continuing blindly you must answer the user now, without tools.
Write the final summary with these points, in this order:
1. What exactly you changed (files, one line per file).
2. What is verified and works (which check passed).
3. What is still failing: the exact error, copied from the output, and whether it is pre-existing (it also fails without your changes) or yours.
4. The concrete next step if the user wants to continue.
Do not promise work you have not done and do not hide failures: the user will read your answer and decide whether to continue. ` + RecordaLlengua
	return p
}

// GuiaFracasRepetit és el que rep el model quan una comprovació li ha
// fallat tres cops seguits en aquest torn sense que n'hi hagi passat cap.
// És el bucle de l'«arregla-ho a cegues»: l'error real sovint ja és a la
// sortida (primera línia, abans del resum de tests) o no és seu (test
// preexistent). La guia es repeteix cada tres fracassos: si amb això el
// model no canvia d'estratègia, potAmpliar bloqueja l'ampliació i el torn
// tanca amb síntesi honesta.
// Per saber si és preexistent ja no es proposa git stash: amb feina de
// l'usuari sense desar, apartar-la i tornar-la és perillós.
const GuiaFracasRepetit = "REPEATED FAILING CHECK: a check has failed 3 times in this turn and none has passed. Before running it again, do this, in order:\n1. Re-read the WHOLE error output, from the first line: compile and vet errors appear BEFORE the test summary, and the key line is usually the first one, not the last.\n2. Check whether the failure is pre-existing: review your own changes with `git diff`, or run the same check in a clean copy of HEAD (`git worktree add`). Never use `git stash`, `git checkout` or `git reset` on the user's files. If it also fails without your changes, it is NOT yours and you cannot fix it from this task.\n3. If it is yours, change strategy: read the exact file and line of the error instead of making random tweaks. If it is pre-existing, tell the user (which check, which error, and that it was already failing) and end the turn.\nRunning the same check again and expecting a different result is not progress."

// DoomTracker detecta el "doom loop" d'opencode: la mateixa eina amb els
// mateixos arguments 3 vegades seguides = el model està encallat. En ese
// cas no s'executa: es retorna una guia perquè canviï d'estratègia.
// A més caça la variant amb comptador (repro8.mjs → repro9.mjs…): mateixa
// plantilla 4 seguides, encara que el cos variï.
//
// NoteOut fa el complement amb RESULTATS: la mateixa sortida exacta 3 cops
// seguits és repetició encara que la crida variï una mica, excepte si la
// feina de fons encara corre (EN MARXA: esperar és legítim). L'alternança
// A-B-A-B en canvi NO és doom
// (decisió provada a TestDoomAlternatNoDispara): eines diferents poden ser
// progrés, i el cas patològic queda acotat per MaxSteps + ampliacions.
type DoomTracker struct {
	lastSig   string
	count     int
	lastNorm  string
	normCount int
	ultOut    string
	outCount  int
	// ultOutSig és la signatura normalitzada de la crida que ha donat
	// ultOut (NoteOutCall): la sortida només compta com a repetida si la
	// crida també es repeteix.
	ultOutSig string
}

// normDoomLen és el tros comparat en mode difús: la plantilla de la
// comanda, no el cos sencer (que varia a cada intent).
const normDoomLen = 120

var reDigitsDoom = regexp.MustCompile(`[0-9]+`)

// normSig normalitza una signatura: dígits → # i tall a la plantilla.
func normSig(sig string) string {
	// Reading numbered files or successive offsets is ordinary progress.
	// Keep the complete arguments here; exact repeats still trip Note.
	if strings.HasPrefix(sig, "read\x00") || strings.HasPrefix(sig, "mcp_") {
		// MCP: les consultes difereixen sovint al final (group_by, filtres).
		return sig
	}
	s := reDigitsDoom.ReplaceAllString(sig, "#")
	if r := []rune(s); len(r) > normDoomLen {
		return string(r[:normDoomLen])
	}
	return s
}

func (d *DoomTracker) Note(sig string) bool {
	// Esperar un procés de fons és el patró oficial (bash_background +
	// polls de bash_output amb els mateixos arguments): comptar-lo com a
	// repetició trencaria les comandes llargues a la tercera lectura. El
	// que sí es caça és rellegir una feina ACABADA (vegeu NoteOut).
	if strings.HasPrefix(sig, "bash_output\x00") {
		return false
	}
	if sig == d.lastSig {
		d.count++
	} else {
		d.lastSig = sig
		d.count = 1
	}
	if n := normSig(sig); n == d.lastNorm {
		d.normCount++
	} else {
		d.lastNorm = n
		d.normCount = 1
	}
	return d.count >= 3 || d.normCount >= 4
}

// NoteOut registra la sortida d'una eina executada: 3 resultats idèntics
// seguits (buits o no) són repetició. Torna true quan cal guiar.
// Les sortides buides o "sense sortida" no compten: moltes eines
// legítimes no tornen res i castigar-les trencaria fluxos normals.
// Tampoc compta llegir una feina EN MARXA: el procés encara corre i la
// dada pot arribar al poll següent. Rellegir-la un cop ACABADA, en canvi,
// sí que és donar voltes (3 cops = guia).
func (d *DoomTracker) NoteOut(out string) bool {
	return d.NoteOutCall("", out)
}

// NoteOutCall és NoteOut sabent quina crida ha donat la sortida (DoomSig).
// Tres cerques DIFERENTS que tornen «sense coincidències» no són un bucle,
// són una cerca que avança: abans disparaven la guia. Ara la sortida
// idèntica només compta si la crida (normalitzada, normSig) també és la
// mateixa, i les sortides sense contingut (cap coincidència, cap
// resultat) no compten mai. Amb sig buit, es compara només la sortida.
func (d *DoomTracker) NoteOutCall(sig, out string) bool {
	t := strings.TrimSpace(out)
	if t == "" || sortidaSenseContingut(t) || procesEnMarxa(t) {
		d.ultOut, d.outCount, d.ultOutSig = "", 0, ""
		return false
	}
	ns := ""
	if sig != "" {
		ns = normSig(sig)
	}
	if t == d.ultOut && ns == d.ultOutSig {
		d.outCount++
	} else {
		d.ultOut, d.ultOutSig = t, ns
		d.outCount = 1
	}
	return d.outCount >= 3
}

// sortidaSenseContingut reconeix els resultats «buits» de les eines (en
// català i en anglès, perquè els textos de les eines es poden traduir).
func sortidaSenseContingut(t string) bool {
	switch strings.ToLower(strings.Trim(t, "()[]. ")) {
	case "sense sortida", "sense coincidències", "sense coincidencies", "sense resultats",
		"no output", "no matches", "no matches found", "no results", "no results found":
		return true
	}
	return false
}

// procesEnMarxa: llegir una feina de fons que encara corre és esperar, no
// repetir (Proc.Describe: «[id] EN MARXA · …»).
func procesEnMarxa(t string) bool {
	return strings.Contains(t, "] EN MARXA ·") || strings.Contains(t, "] RUNNING ·")
}

// DoomGuide és el que rep el model quan cau al forat.
const DoomGuide = "REPEATED TOOL CALL: you have called this tool 3-4 times with the same or nearly the same arguments (only numbers change) without making progress. Change strategy (read a different file, refine the pattern, or summarize what you have) instead of repeating it."

// Ampliació de pressupost decidida pel model.
//
// Quan s'acaba el límit de passos no es tanca directament: es demana al
// model si val la pena continuar (ell veu l'estat real). El pressupost és
// tou, com a Claude Code o opencode: mentre hi hagi progrés mesurable des
// de l'última ampliació (eines executades de noves) i el model no estigui
// encallat repetint, se li concedeix un tros més. El que atura la feina
// és la manca d'avenç, no un comptador. Abans eren dos trossos de deu, i
// tasques llargues es quedaven a mitges amb el checklist a la meitat.
//
// Però «progrés» s'ha de mesurar bé, i una tirada real de 2026-09-28 va
// demostrar que «eines noves» no basta: una tasca trivial (afegir la
// carpeta a la barra del TUI) va cremar 146 passos i 50 minuts perquè el
// model corria `go test` una i altra vegada contra una suite que no podia
// quedar verda (un error de vet seu + un test live preexistent contra un
// endpoint caigut). Cada corrida era una eina nova: l'ampliació es
// concedia sempre i ningú consultava l'usuari. D'aquí dues dents més:
//
//   - FracasTracker (motor.RepExecucions): una comprovació que falla tres
//     cops sense que n'hi passi cap és un bucle, encara que entre mig hi
//     hagi hagut edicions que «triomfen». Guia el model (llegeix l'error
//     sencer, comprova si és preexistent amb git stash) i, a la ratxa,
//     bloqueja l'ampliació: amb la suite impossible de deixar verda,
//     el temps de tothom. Millor síntesi honesta i que l'usuari decideixi.
//   - MaxAmpliacionsInteractiu: en una sessió amb persona a l'altra
//     banda de la pantalla, el topall de seguretat és molt més curt que el
//     de l'autònom (que té checkpoints propis): tres trossos (~100 passos
//     amb el MaxSteps per defecte) i a parar amb síntesi. L'alternativa
//     no és «la tasca es queda a mitges»: és «l'usuari llegeix què ha
//     passat i diu continua», que és el checkpoint que mai ningú li feia.
const (
	ChunkAmpliacio = 20
	MaxAmpliacions = 25
	// MaxAmpliacionsInteractiu és el topall d'ampliacions d'un torn en
	// mode interactiu (TUI, headless): l'usuari hi és, i després de tres
	// trossos tocava parlar amb ell, no amb el model.
	MaxAmpliacionsInteractiu = 3
	// TimeoutDecisio és el que s'espera l'ampliació i la síntesi: crides
	// sense eines, però un model pensador amb 24 % de context darrere hi
	// pot trigar més d'un minut i mig. Amb 90 s, l'error es prenia per un
	// FINAL i el torn es tancava a mitges.
	TimeoutDecisio = 600 * time.Second
)

// PreguntaAmpliacio demana CONTINUE <n> o FINAL (%d = topall del tros).
// ParseAmpliacio accepta també CONTINUA (el prompt d'abans).
const PreguntaAmpliacio = "You have used up the current chunk of steps. Look at the state (checklist and latest results). If the task is not finished and you are making progress, reply EXACTLY \"CONTINUE <n>\" with n between 1 and %d (the extra steps you need). Reply EXACTLY \"FINAL\" only if the task is done or you are stuck repeating the same thing without progress; in your next reply you will write the final summary without tools."

// PreguntaAmbEstat és la pregunta amb el checklist actual adjunt (si n'hi
// ha): el model decideix amb dades, no a cegues.
func PreguntaAmbEstat() string {
	p := fmt.Sprintf(PreguntaAmpliacio, ChunkAmpliacio)
	if _, total := tools.TodoStats(); total > 0 {
		p += "\nCurrent checklist:\n" + tools.TodoRender()
	}
	return p
}

// PreguntaAmbLlista és PreguntaAmbEstat amb una llista donada (sessions web).
func PreguntaAmbLlista(llista []tools.TodoItem) string {
	p := fmt.Sprintf(PreguntaAmpliacio, ChunkAmpliacio)
	if _, total := tools.StatsTodos(llista); total > 0 {
		p += "\nCurrent checklist:\n" + tools.RenderTodos(llista)
	}
	return p
}

// ParseAmpliacio llegeix la resposta: CONTINUE <n> o CONTINUA <n> (n=Chunk
// si no en diu) o res (FINAL, buit, xerrameca) = tancar.
func ParseAmpliacio(resp string) (int, bool) {
	// La decisió es busca dins del text, no al principi: un model pensador
	// hi posa una frase abans («La checklist és a 9/11, així que CONTINUA
	// 10») i abans això comptava com a FINAL. Si hi ha les dues paraules,
	// mana l'última: és la conclusió. CONTINUE i CONTINUA fan 8 lletres.
	up := strings.ToUpper(resp)
	iCont := max(ultimaParaula(up, "CONTINUA"), ultimaParaula(up, "CONTINUE"))
	iFinal := ultimaParaula(up, "FINAL")
	if iCont < 0 || iFinal > iCont {
		return 0, false
	}
	resta := up[iCont+len("CONTINUA"):]
	if i := strings.IndexByte(resta, '\n'); i >= 0 {
		resta = resta[:i]
	}
	for _, camp := range strings.Fields(resta) {
		if n, err := strconv.Atoi(strings.Trim(camp, ".,;:*»«\"'()")); err == nil && n >= 1 {
			return min(n, ChunkAmpliacio), true
		}
	}
	return ChunkAmpliacio, true
}

// ultimaParaula és l'índex de l'última aparició de `mot` com a paraula
// sencera («continuaré» no compta com a CONTINUA), o -1.
func ultimaParaula(s, mot string) int {
	for i := strings.LastIndex(s, mot); i >= 0; i = strings.LastIndex(s[:i], mot) {
		ra, _ := utf8.DecodeLastRuneInString(s[:i])
		rd, _ := utf8.DecodeRuneInString(s[i+len(mot):])
		if !unicode.IsLetter(ra) && !unicode.IsLetter(rd) {
			return i
		}
		if i == 0 {
			break
		}
	}
	return -1
}

// GuiaTodosPendents és el que rep el model quan tanca el torn amb passos
// pendents al checklist: anuncia què farà i s'atura sense cridar res. No
// es pot donar per acabada: continua amb el següent pas pendent.
const GuiaTodosPendents = "Your checklist (todowrite) still has pending steps and you ended the turn without doing them: do not present the work as finished and do not ask whether to continue. Update the checklist and carry on with the next pending step using tools, without announcing the plan again."

// GuiaVerificacioFallida és el que rep el model quan tanca el torn (sense
// eines) just després que l'última eina executada hagi fallat: en comptes
// d'afirmar èxit, ha de continuar arreglant-ho o explicar què falla.
// Mesurat en local el 2026-09-17: sense això, el model veu el test en vermell
// i respon "TOT BÉ" igualment. Acotat com les altres guies (verifRetries).
// La sortida del preexistent hi és per la tirada del 2026-09-28: un vermell
// que ja era allà abans de tocar res no es pot arreglar des de la tasca, i
// exigir-ho és demanar-li un bucle infinit.
const GuiaVerificacioFallida = "The last tool you ran FAILED (see its output) and you ended the turn without fixing it: do not claim it works. Keep using tools until the check really passes, or explain exactly which error remains and what it would take to fix it. If the failure is pre-existing (it also fails without your changes: check with `git diff` or in a clean `git worktree` of HEAD, never with `git stash`), say so explicitly and stop: that failure is not yours."

// SemblaFracas diu si la sortida d'una eina sembla un fracàs (test en
// vermell, error d'execució). Són marques estretes a posta: un grep que
// troba la paraula "error" en un log no és un fracàs de l'eina. En canvi,
// una sortida que COMENÇA per "ERROR:" sí: és el prefix que el loop posa
// als errors d'execució (eina fallada, no contingut trobat).
func SemblaFracas(out string) bool {
	t := strings.ToLower(out)
	if strings.HasPrefix(t, "error:") {
		return true
	}
	marques := []string{
		"assertionerror", "traceback (most recent call last)",
		"\nfailed", " test failed", "tests failed", "failed: ",
		"compilation failed", "build failed",
		"command not found", "no such file",
		// Go no diu mai "failed": un `go test` en vermell llista
		// «--- FAIL: TestX» i tanca amb «FAIL\tpkg». Sense aquestes
		// marques un vermell de Go era invisible (si l'ordre enmascara
		// l'exit code amb una canonada, ni el prefix ERROR: no hi ha)
		// i el model tancava el torn dient «fet» amb la suite vermella.
		// «\nfail» a seques era massa ample: un `ls` amb una carpeta
		// failover/ ja semblava un vermell.
		"--- fail", "\nfail\t", "\nfail\n",
	}
	for _, m := range marques {
		if strings.Contains(t, m) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimRight(t, " \r\n"), "\nfail")
}

// EsComprovacio diu si una ordre de bash és una comprovació (test, build,
// vet, lint): les úniques que compten per a la ratxa de vermelles. Sense
// aquest filtre, tres `grep` sense coincidències (exit 1 → «ERROR:»)
// disparaven la guia de fracàs repetit en plena exploració, i un `sed -i`
// verd entre dues corrides de test posava la ratxa a zero, que és
// justament el bucle «retoqueja i torna a córrer» que s'havia de caçar.
// Es mira el cap de cada tros de la canonada (|, &&, ;), saltant
// variables d'entorn i envoltoris com timeout.
func EsComprovacio(cmd string) bool {
	subs := map[string][]string{
		"go":     {"test", "vet", "build"},
		"cargo":  {"test", "check", "build", "clippy"},
		"dotnet": {"test", "build"},
		"npm":    {"test", "run", "t"},
		"pnpm":   {"test", "run"},
		"yarn":   {"test", "run", "lint", "build"},
		"python": {"-m"}, "python3": {"-m"}, "py": {"-m"},
		"node": {"--test"},
	}
	sols := map[string]bool{
		"make": true, "pytest": true, "tsc": true, "eslint": true, "ruff": true,
		"mypy": true, "flake8": true, "gradle": true, "./gradlew": true,
		"gradlew": true, "mvn": true, "ctest": true, "jest": true, "vitest": true,
		"golangci-lint": true, "staticcheck": true, "gofmt": true,
	}
	for _, tros := range strings.FieldsFunc(strings.ToLower(cmd), func(r rune) bool {
		return r == '|' || r == ';' || r == '&' || r == '\n'
	}) {
		f := strings.Fields(tros)
		for len(f) > 0 {
			if f[0] == "timeout" && len(f) > 1 {
				f = f[2:] // timeout 60s go test
				continue
			}
			if strings.Contains(f[0], "=") || f[0] == "env" || f[0] == "time" {
				f = f[1:]
				continue
			}
			break
		}
		if len(f) == 0 {
			continue
		}
		if sols[f[0]] {
			return true
		}
		if len(f) > 1 {
			for _, s := range subs[f[0]] {
				if f[1] == s {
					return true
				}
			}
		}
	}
	return false
}

// TascaDeReparacio diu si la tasca demana canviar el codi (i per tant té
// sentit exigir la verificació verda abans de tancar). Sense verb de
// canvi, un test en vermell pot ser el resultat correcte ("per què
// falla?") i no s'ha de forçar cap continuació. «Afegeix» i «canvia» hi
// són perquè la tasca trivial «revisa i afegeix X» va arribar a
// considerar-se no-reparació i el model es quedava amb la suite vermella
// sense guia: afegir una cosa és reparar-la si has de verificar-la.
// Els verbs anglesos (l'idioma per defecte) hi són pels mateixos motius.
func TascaDeReparacio(tasca string) bool {
	t := strings.ToLower(tasca)
	for _, v := range []string{"arregla", "fes que", "aconsegueix que", "fix", "repara", "soluciona", "fes passar", "afegeix", "canvia", "implementa",
		"repair", "solve", "make it ", "make the ", "make sure", "make all", "get the tests", "add ", "change ", "implement", "update "} {
		if strings.Contains(t, v) {
			return true
		}
	}
	return false
}
