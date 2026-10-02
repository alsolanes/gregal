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
const FinalPrompt = "Les accions ja han acabat. Dona ara una resposta final clara per a l'usuari amb el que has trobat o fet. Si no has pogut acabar-ho tot, resumeix què has fet i què falta per fer. No facis servir eines ni expliquis el teu raonament intern."

// FinalPromptLlarg és la síntesi quan el torn s'ha fet sospitosament llarg:
// ampliacions gastades o ratxa de comprovacions vermelles. Aquí no basta un
// «què he fet»: l'usuari ha de poder decidir si val la pena continuar, i per
// això cal l'estat real del verd i del vermell, amb l'error concret i sense
// amagar el fracàs preexistent (que no és culpa del torn, però sí cal
// explicar-lo).
func FinalPromptLlarg(passos, fracasos int) string {
	p := fmt.Sprintf("Les accions s'aturen aquí: aquesta tasca ja ha consumit molt de pressupost (%d passos", passos)
	if fracasos > 0 {
		p += fmt.Sprintf(", %d comprovacions vermelles seguides", fracasos)
	}
	p += `) i en comptes de continuar a cegues cal que responguis a l'usuari, sense eines.
Escriu la síntesi final amb això, per aquest ordre:
1. Què has canviat exactament (fitxers, una línia per fitxer).
2. Què està verificat i funciona (quina comprovació ha passat).
3. Què queda vermell: l'error exacte, copiat de la sortida, i si és preexistent (falla igual amb els teus canvis apartats) o teu.
4. El següent pas concret si l'usuari vol continuar.
No prometis feina que no has fet ni amaguis fracassos: l'usuari veurà la teva resposta i decidirà si continua.`
	return p
}

// GuiaFracasRepetit és el que rep el model quan una comprovació li ha
// fallat tres cops seguits en aquest torn sense que n'hi hagi passat cap.
// És el bucle de l'«arregla-ho a cegues»: l'error real sovint ja és a la
// sortida (primera línia, abans del resum de tests) o no és seu (test
// preexistent). La guia es repeteix cada tres fracassos: si amb això el
// model no canvia d'estratègia, potAmpliar bloqueja l'ampliació i el torn
// tanca amb síntesi honesta.
const GuiaFracasRepetit = "COMPROVACIÓ VERMELLA REPETIDA: ja has executat una comprovació que ha fallat 3 cops en aquest torn sense que n'hi hagi passat cap. Abans de tornar-hi, fes això, per ordre:\n1. Rellegeix la sortida de l'error SENCERA, des de la primera línia: els errors de compilació i de vet surten ABANS del resum de tests, i la dada clau sol ser la primera línia, no l'última.\n2. Comprova si el fracàs és preexistent: aparta els teus canvis (git stash), reprodueix la comprovació, i retorna'ls (git stash pop). Si falla igual, NO és teu: no pots arreglar-lo des d'aquesta tasca.\n3. Si és teu, canvia d'estratègia: llegeix el fitxer i línia exactes de l'error en comptes de fer retocs a l'atzar. Si és preexistent, digue-ho a l'usuari (quina comprovació, quin error, i que ja fallava abans) i tanca el torn.\nRepetir la mateixa comprovació esperant un resultat diferent no és progrés."

// DoomTracker detecta el "doom loop" d'opencode: la mateixa eina amb els
// mateixos arguments 3 vegades seguides = el model està encallat. En ese
// cas no s'executa: es retorna una guia perquè canviï d'estratègia.
// A més caça la variant amb comptador (repro8.mjs → repro9.mjs…): mateixa
// plantilla 4 seguides, encara que el cos variï.
//
// NoteOut fa el complement amb RESULTATS: la mateixa sortida exacta 3 cops
// seguits (típic: rellegir el mateix amb bash_output) és repetició encara
// que la crida variï una mica. L'alternança A-B-A-B en canvi NO és doom
// (decisió provada a TestDoomAlternatNoDispara): eines diferents poden ser
// progrés, i el cas patològic queda acotat per MaxSteps + ampliacions.
type DoomTracker struct {
	lastSig   string
	count     int
	lastNorm  string
	normCount int
	ultOut    string
	outCount  int
}

// normDoomLen és el tros comparat en mode difús: la plantilla de la
// comanda, no el cos sencer (que varia a cada intent).
const normDoomLen = 120

var reDigitsDoom = regexp.MustCompile(`[0-9]+`)

// normSig normalitza una signatura: dígits → # i tall a la plantilla.
func normSig(sig string) string {
	s := reDigitsDoom.ReplaceAllString(sig, "#")
	if r := []rune(s); len(r) > normDoomLen {
		return string(r[:normDoomLen])
	}
	return s
}

func (d *DoomTracker) Note(sig string) bool {
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
func (d *DoomTracker) NoteOut(out string) bool {
	t := strings.TrimSpace(out)
	if t == "" || t == "(sense sortida)" {
		d.ultOut, d.outCount = "", 0
		return false
	}
	if t == d.ultOut {
		d.outCount++
	} else {
		d.ultOut = t
		d.outCount = 1
	}
	return d.outCount >= 3
}

// DoomGuide és el que rep el model quan cau al forat.
const DoomGuide = "EINA REPETIDA: ja has cridat aquesta eina 3-4 vegades igual o gairebé igual (només canvien números) sense avançar. Canvia d'estratègia (llegeix un altre fitxer, afina el patró, o resumeix el que tens) en comptes de repetir-la."

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
	TimeoutDecisio = 180 * time.Second
)

// PreguntaAmpliacio demana CONTINUA <n> o FINAL (%d = topall del tros).
const PreguntaAmpliacio = "Has exhaurit el tros de passos actual. Mira l'estat (checklist i últims resultats). Si la tasca no està acabada i estàs avançant, respon EXACTAMENT «CONTINUA <n>» amb n entre 1 i %d (passos extra que demanes): la feina s'ha d'acabar, no importa quants passos calguin mentre avancis. Respon EXACTAMENT «FINAL» només si la tasca ja està feta o estàs encallat repetint el mateix sense progrés; a la següent resposta faràs la síntesi final sense eines."

// PreguntaAmbEstat és la pregunta amb el checklist actual adjunt (si n'hi
// ha): el model decideix amb dades, no a cegues.
func PreguntaAmbEstat() string {
	p := fmt.Sprintf(PreguntaAmpliacio, ChunkAmpliacio)
	if _, total := tools.TodoStats(); total > 0 {
		p += "\nChecklist actual:\n" + tools.TodoRender()
	}
	return p
}

// PreguntaAmbLlista és PreguntaAmbEstat amb una llista donada (sessions web).
func PreguntaAmbLlista(llista []tools.TodoItem) string {
	p := fmt.Sprintf(PreguntaAmpliacio, ChunkAmpliacio)
	if _, total := tools.StatsTodos(llista); total > 0 {
		p += "\nChecklist actual:\n" + tools.RenderTodos(llista)
	}
	return p
}

// ParseAmpliacio llegeix la resposta: CONTINUA <n> (n=Chunk si no en diu)
// o res (FINAL, buit, xerrameca) = tancar.
// o res (FINAL, buit, xerrameca) = tancar.
func ParseAmpliacio(resp string) (int, bool) {
	// La decisió es busca dins del text, no al principi: un model pensador
	// hi posa una frase abans («La checklist és a 9/11, així que CONTINUA
	// 10») i abans això comptava com a FINAL. Si hi ha les dues paraules,
	// mana l'última: és la conclusió.
	up := strings.ToUpper(resp)
	iCont := ultimaParaula(up, "CONTINUA")
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
const GuiaTodosPendents = "Tens passos pendents al checklist (todowrite) i has tancat el torn sense fer-los: no donis la feina per acabada i no preguntis si has de continuar. Actualitza el checklist i continua executant el següent pas pendent amb eines, sense tornar a anunciar el pla."

// GuiaVerificacioFallida és el que rep el model quan tanca el torn (sense
// eines) just després que l'última eina executada hagi fallat: en comptes
// d'afirmar èxit, ha de continuar arreglant-ho o explicar què falla.
// Mesurat en local el 2026-09-17: sense això, el model veu el test en vermell
// i respon "TOT BÉ" igualment. Acotat com les altres guies (verifRetries).
// La sortida del preexistent hi és per la tirada del 2026-09-28: un vermell
// que ja era allà abans de tocar res no es pot arreglar des de la tasca, i
// exigir-ho és demanar-li un bucle infinit.
const GuiaVerificacioFallida = "L'última eina executada ha FALLAT (mira la seva sortida) i has tancat el torn sense arreglar-ho: no afirmis que funciona. Continua amb eines fins que la comprovació passi de debò, o explica exactament quin error queda i què caldria per resoldre'l. Si el fracàs és preexistent (es reprodueix igual amb els teus canvis apartats: git stash → reprodueix → git stash pop), digue-ho explícitament i tanca: aquell vermell no és teu."

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
func TascaDeReparacio(tasca string) bool {
	t := strings.ToLower(tasca)
	for _, v := range []string{"arregla", "fes que", "aconsegueix que", "fix", "repara", "soluciona", "fes passar", "afegeix", "canvia", "implementa"} {
		if strings.Contains(t, v) {
			return true
		}
	}
	return false
}
