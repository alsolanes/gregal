package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"gregal/internal/agent"
	serviceclient "gregal/internal/client"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/mcp"
	"gregal/internal/session"
	"gregal/internal/tools"
	"gregal/internal/usercmd"
	"gregal/internal/verify"
)

type streamTickMsg struct{}

type serviceStatusMsg struct {
	client *serviceclient.Client
	health serviceclient.Health
	err    error
}

type serviceEventsMsg struct {
	page       serviceclient.EventsPage
	err        error
	generation uint64
	stream     bool
}

type serviceCancelMsg struct{ err error }

// serviceSubmitMsg és la resposta de la cua del servei a un enviament del
// TUI delegat: l'execució creada (o l'error de connexió).
type serviceSubmitMsg struct {
	run serviceclient.Run
	err error
}

type streamDoneMsg struct {
	reply string
	think int // caràcters de raonament intern (0 = cap)
	// promptTokens és el consum real declarat pel proveïdor (0 = no el
	// declara): si hi és, val més que l'heurístic per al mesurador.
	promptTokens int
	cachedTokens int
	err          error
}

type parallelResult struct {
	role     string
	provider string
	model    string
	reply    string
	err      error
}

type parallelDoneMsg struct {
	prompt  string
	results []parallelResult
}

type bashDoneMsg struct {
	cmd string
	out string
	err error
}

type opDoneMsg struct {
	desc string
	out  string
	err  error
	next func(out string, err error) tea.Msg
}

type verifyDoneMsg struct {
	verdict verify.Verdict
	raw     string
	err     error
}

type agentStepMsg struct {
	content string
	calls   []llm.ToolCall
	err     error
	fb      string // model fallback que ha respost ("" = primari)
	// promptTokens és el consum real declarat pel proveïdor (0 = no el
	// declara): si hi és, val més que l'heurístic per al mesurador.
	promptTokens int
	cachedTokens int
}

type toolRes struct {
	call llm.ToolCall
	out  string
	diff string
	imgs []string
}

type agentExecMsg struct {
	results []toolRes
}

type autonomousCheckpointMsg struct {
	checks  []agent.AutonomousCheck
	verdict string
	err     error
}

// agentExtensioMsg és la resposta del model a si cal continuar (CONTINUA
// <n> o FINAL) en esgotar el pressupost de passos.
type agentExtensioMsg struct {
	resp string
	err  error
}

// agentCompactMsg arriba quan s'ha fet lloc a l'historial de l'agent a
// mig torn (retall de sortides velles + resum dels passos anteriors).
type agentCompactMsg struct {
	room agent.RoomResult
}

// pendingOp és una acció pendent de confirmació [s/n].
// next, si no és nil, continua el flux (p. ex. reprendre l'agent).
type pendingOp struct {
	desc string
	// agent marca les aprovacions que venen del motor: allà l'eina no
	// l'executa el diàleg sinó el camí normal d'execució del torn.
	agent bool
	// preview és el canvi que s'aprova, ja pintat (diff d'un edit, cos
	// d'un fitxer nou). Abans només es veia el JSON dels arguments
	// retallat a 300 caràcters i el diff arribava DESPRÉS d'aprovar:
	// un write d'un fitxer sencer s'aprovava a cegues.
	preview string
	// run rep el context del torn: cancel·lar-lo (Esc, Ctrl+C) atura la
	// comanda o l'eina a mitges en comptes de deixar-la corrent mentre
	// el TUI ja ha donat el torn per mort.
	run  func(ctx context.Context) (string, error)
	next func(out string, err error) tea.Msg
}

// streamer acumula tokens d'un stream per pintar-los en viu.
type streamer struct {
	mu    sync.Mutex
	text  string
	think string
	done  bool
	// note és l'avís de reintent del client («proveïdor HTTP 502 ·
	// reintent 2/6 d'aquí a 4 s»): es pinta al bloc de feina mentre no
	// arriba text, perquè l'espera no sigui muda.
	note string
	// inici i ultim: quan ha començat aquest pas i quan ha arribat l'últim
	// byte. L'avís de «proveïdor mut» es compta des d'aquí i no des de
	// l'inici del torn: en un torn de deu passos, el primer segon de cada
	// pas nou deia «no ha respost en 4 minuts» mentre el model escrivia.
	inici time.Time
	ultim time.Time
}

// nouStreamer engega el rellotge del pas.
func nouStreamer() *streamer {
	ara := time.Now()
	return &streamer{inici: ara, ultim: ara}
}

// silenci és quant fa que no arriba res en aquest pas.
func (s *streamer) silenci() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ultim.IsZero() {
		return 0
	}
	return time.Since(s.ultim)
}

func (s *streamer) setNote(n string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.note = n
}

func (s *streamer) add(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.text += t
	s.ultim = time.Now()
}

func (s *streamer) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
}

func (s *streamer) addThink(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.think += t
	s.ultim = time.Now()
}

func (s *streamer) snapshot() (text, think string, done bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text, s.think, s.done
}

func (s *streamer) getNote() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.note
}

// ambAvisReintent adjunta al context el hook que porta els reintents del
// client al bloc de feina.
func ambAvisReintent(ctx context.Context, s *streamer) context.Context {
	return llm.WithRetryHook(ctx, func(attempt, total int, wait time.Duration, err error) {
		s.setNote(llm.RetryNote(attempt, total, wait, err))
	})
}

// esperaMuda és el temps sense cap byte del proveïdor a partir del qual
// el bloc de feina deixa de dir «escrivint…» i diu que s'està esperant.
const esperaMuda = 8 * time.Second

// tailRunes retorna els últims n runes (cua d'un text).
func tailRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return "…" + string(r[len(r)-n:])
}

// filesPensament és quantes files del pensament es veuen en directe.
const filesPensament = 4

// pensamentEnViu pinta el pensament del model com les últimes files d'un
// text embolcallat: es llegeix com un registre que avança. Abans era una
// cua de 120 caràcters que es reescrivia a cada tick, i no hi havia manera
// de seguir-la.
func pensamentEnViu(think string, width int) string {
	w := max(width-6, 20)
	files := strings.Split(ansi.Wrap(strings.TrimSpace(think), w, ""), "\n")
	if len(files) > filesPensament {
		files = files[len(files)-filesPensament:]
	}
	for i, f := range files {
		marca := "  "
		if i == 0 {
			marca = "◌ "
		}
		files[i] = dimStyle.Render(marca + f)
	}
	return strings.Join(files, "\n")
}

// Model és l'estat del TUI.
type Model struct {
	cfg     *config.Config
	cfgPath string
	client  *llm.Client
	version string
	cwd     string

	vp    viewport.Model
	input textarea.Model

	lines []string
	convo []llm.Message
	// compacted és el resum de la conversa ja compactada (viu al system
	// prompt; vegeu /compact). La convo només duu el recent.
	compacted string

	role            string
	mouseOn         bool   // ratolí capturat (roda scroll; desactiu = selecció nativa)
	rolePinned      bool   // cert si l'usuari ha fixat el rol (/role)
	agentRole       string // rol rutejat del torn actiu ("" = rol de sessió)
	busy            bool
	status          string
	ready           bool
	cancel          context.CancelFunc // cancel·la la crida de model activa
	cancelRequested bool               // cancel·lació voluntària, no error del model
	pendingKeyFor   string             // provider esperant clau: la següent línia (no ordre) es menja com a secret
	// keyBuf és la clau que s'està escrivint. No passa pel textarea:
	// així no apareix mai en clar a la pantalla, ni queda al buffer
	// d'edició ni a cap desfer. Es pinta com a rodonetes.
	keyBuf      string
	permissive  bool // mode permissiu: les ask s'aproven soles (deny continua bloquejant)
	journalBase int  // Seq del journal a l'inici del torn: verificar només si ha fet coses reals

	stream      *streamer
	streamLine  int
	streamShown string
	// streamRender és quan s'ha repintat el text en viu per últim cop.
	// El markdown es refa cada intervalViu, no a cada tick de 60 ms:
	// glamour costa uns mil·lisegons i a 60 ms es notava.
	streamRender time.Time

	pending *pendingOp
	// confirmIdx és el botó ressaltat del diàleg d'aprovació.
	confirmIdx int // 0=permet, 1=denega

	writing   bool
	writePath string
	writeBuf  []string

	agentActive bool
	// torn és la màquina d'estats del torn en marxa (nil = cap).
	// Abans aquí hi havia tretze comptadors (passos, reintents de tota
	// mena, ampliacions, execucions, checkpoints…) que startAgent havia
	// de recordar de posar a zero un per un, i la lògica que els movia
	// estava escampada per sis casos d'Update. Ara tot és a
	// internal/agent/motor.go, que es prova sense terminal ni xarxa.
	torn *agent.Torn
	// pasMecanic és el Pas.Mecanic del pas de model en curs (think: auto).
	pasMecanic bool
	// seedTodos és el checklist inicial del proper torn (passos d'un
	// /plan aprovat); startAgent el publica després de netejar l'anterior.
	seedTodos []tools.TodoItem
	// pendingQ és una pregunta seleccionable del model (AskUserQuestion).
	pendingQ *questionPending
	// queued són missatges escrits mentre l'agent treballa: s'envien en
	// acabar el torn (estil Claude) en comptes de perdre's.
	queued []string
	// lastPrompt és l'últim text enviat (per /retry quan el model torna
	// buit o l'usuari vol reintentar sense reescriure).
	lastPrompt string
	// showTodos mostra la checklist (Ctrl+T, com Claude Code).
	showTodos bool
	// timeline conserva una vista estructurada de la feina del torn. El
	// transcript continua sent la font de veritat de la sessió, però aquesta
	// vista permet inspeccionar eines i resultats sense buscar-los fent scroll.
	timeline      []timelineEvent
	timelineOpen  bool
	timelineSel   int
	lastFind      string
	findLine      int
	reducedMotion bool
	asciiMode     bool
	// animacions: mar, mascota i marea del mesurador. Apagades si no es
	// demanen. No té res a veure amb reducedMotion, que congela també el
	// spinner (accessibilitat).
	animacions bool
	// cockpit: la columna de la dreta (tasca, canvis, cua, context,
	// validació) quan el terminal fa 120 columnes o més. Ve del config
	// i Ctrl+T la commuta i ho desa. Per sota de 120, Ctrl+T commuta la
	// caixa de sempre (showTodos), que no es desa.
	cockpit bool
	// ample i alt són la mida del terminal, per recalcular el viewport
	// quan es commuta la columna sense esperar un WindowSizeMsg.
	ample, alt int

	// benvingudaN són les línies inicials (targeta de benvinguda i pista
	// de sessions) que ocupen mitja pantalla i no marxaven mai. Al primer
	// missatge es treuen: a partir d'aquí el que importa és la conversa.
	benvingudaN int

	// retallades compta les línies velles tretes de la pantalla pel sostre
	// de MaxLinies. Vegeu retallaSiCal.
	retallades int

	planning bool // exploració de pla en marxa (read-only)
	// flow és el graf que s'està executant (nil si cap) i flowCancel el
	// talla. Vegeu flow.go.
	flow        *flowRun
	flowCancel  context.CancelFunc
	planSteps   int           // passos d'exploració consumits
	planRetries int           // reintents d'error de servidor del pla
	planHist    []llm.Message // historial de l'exploració
	pendingPlan string        // pla pendent d'aprovar ([e]/[d])

	policy    *agent.Policy
	remembers *agent.Remember

	mcp *mcp.Manager // servidors MCP (nil si inactiu)
	// serviceClient és la connexió negociada amb el backend compartit. Amb
	// ell connectat, els torns es deleguen al servei: el TUI encua i pinta
	// events, i l'executor local queda en pausa.
	serviceClient     *serviceclient.Client
	serviceStatus     string
	serviceCursor     uint64
	serviceSession    string
	serviceStream     bool
	serviceGeneration uint64
	// serviceRunID és l'execució delegada en curs (0 = cap) i serviceTurn
	// marca que el torn que es pinta és del servei, no local.
	serviceRunID int64
	serviceTurn  bool

	mode string // "code" (total) o "chat" (només lectura)

	lastGoalID string  // últim objectiu creat o executat
	deferred   tea.Cmd // feina asíncrona demanada des d'un menú

	// sessDir és el directori llistat per l'últim /sessions: clicar una
	// fila numerada ("  N · ...") reprèn la N (buit = directori per defecte).
	sessDir string
	// sessionFile identifica la conversa actual; evita el «autosave» global.
	sessionFile string
	// pinned marca la conversa actual a la barra lateral del desktop/TUI.
	pinned bool

	spin   int    // frame del spinner
	branch string // branca git precalculada; View mai fa I/O

	// turnStart és quan ha començat el torn en curs (zero si no n'hi ha).
	// El porta el tick i no els vint llocs que posen busy=true: així no hi
	// ha cap camí que se'l deixi i mostri un rellotge parat.
	turnStart time.Time

	usedTokens int
	tokUp      int
	tokDown    int
	tokCached  int
	promptEst  int

	histEntries []string
	histIdx     int
	// histCerca és la cerca a l'historial (Ctrl+R). nil = tancada.
	histCerca *histCerca
	// ajuda: el diàleg de tecles (?) és obert. Qualsevol tecla el tanca.
	ajuda     bool
	histDraft string
	menu      *menuState
	suggIdx   int
	lastInput string

	editing   bool
	editPath  string
	editStart int
	editEnd   int
	editBuf   []string
}

// New crea el model inicial (rol actiu: chat). cfgPath és on es desa (/providers).
func New(cfg *config.Config, cfgPath string, c *llm.Client, v string) Model {
	// L'idioma es fixa aquí i no canvia: el TUI és un procés amb una sessió.
	SetIdioma(cfg.Lang())
	// El tema, abans de construir res: els estils es calculen aquí i tot
	// el que es pinti després els fa servir.
	SetTema(cfg.Tema())
	ti := textarea.New()
	ti.Placeholder = T("composer.placeholder")
	ti.CharLimit = 4000
	ti.ShowLineNumbers = false
	ti.SetHeight(1)
	ti.Prompt = "❯ "
	ti.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(escuma).Bold(true)
	ti.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(slate)
	// La fila del cursor del textarea porta un fons propi per defecte, que
	// dins del composer (que ja té el seu) es veia com una franja d'un
	// altre color davant del text. El fons és el del composer i prou.
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ti.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ti.FocusedStyle.Base = lipgloss.NewStyle()
	ti.BlurredStyle.Base = lipgloss.NewStyle()
	ti.Focus()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	role := initialRole(cfg)
	m := Model{cfg: cfg, cfgPath: cfgPath, client: c, version: v, cwd: cwd, input: ti, role: role, status: T("barra.llest"), mode: cfg.Mode,
		mcp:         mcp.Setup(toMCPServers(cfg)),
		policy:      &agent.Policy{Tools: cfg.Permissions.Tools, BashAllow: cfg.Permissions.BashAllow, BashDeny: cfg.Permissions.BashDeny, ProjectDir: cwd},
		remembers:   agent.NewRemember(),
		histEntries: session.LoadHistory(), histIdx: -1,
		mouseOn: cfg.MouseOn()} // el programa arrenca amb WithMouseCellMotion
	m.reducedMotion = strings.TrimSpace(os.Getenv("GREGAL_REDUCED_MOTION")) != ""
	m.animacions = cfg.AnimacionsOn() && !m.reducedMotion
	m.cockpit = cfg.CockpitOn()
	m.asciiMode = strings.TrimSpace(os.Getenv("GREGAL_ASCII")) != ""
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetupPrices(cfg)
	agent.SetupDelegate(cfg, c)
	m.refreshGitBranch()
	return m
}

// initialRole evita que una configuració amb mode code arrenqui per error amb
// el model de xat. Si una plantilla mínima no té think/code, conserva chat o
// el primer rol usable en lloc de fabricar una referència buida.
func initialRole(cfg *config.Config) string {
	return cfg.InitialRole()
}

// toMCPServers adapta el config al paquet mcp.
func toMCPServers(cfg *config.Config) map[string]mcp.Server {
	out := map[string]mcp.Server{}
	for name, s := range cfg.MCP {
		out[name] = mcp.Server{Command: s.Command, Args: s.Args, Env: s.Env}
	}
	return out
}

// Init anima el cursor (i apaga el ratolí si el config el vol desactivat;
// el programa arrenca amb captura activada).
func (m Model) Init() tea.Cmd {
	if m.mouseOn {
		return tea.Batch(textarea.Blink, tickIdle(), m.detectWindowsCmd())
	}
	return tea.Batch(textarea.Blink, tickIdle(), tea.DisableMouse, m.detectWindowsCmd())
}

func tick() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg { return streamTickMsg{} })
}

// tickIdle manté la mascota viva en repòs (mig segon; prou per veure-la
// respirar sense cremar CPU).
func tickIdle() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return streamTickMsg{} })
}

func (m Model) pollServiceEvents() tea.Cmd {
	if m.serviceClient == nil {
		return nil
	}
	c := m.serviceClient
	after, sessionID := m.serviceCursor, m.serviceSession
	generation := m.serviceGeneration
	return tea.Tick(750*time.Millisecond, func(time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		page, err := c.Events(ctx, after, 100, sessionID)
		return serviceEventsMsg{page: page, err: err, generation: generation}
	})
}

func (m Model) serviceEventsCmd() tea.Cmd {
	if m.serviceClient == nil {
		return nil
	}
	if !m.serviceStream {
		return m.pollServiceEvents()
	}
	c := m.serviceClient
	after, sessionID := m.serviceCursor, m.serviceSession
	generation := m.serviceGeneration
	return tea.Tick(1*time.Millisecond, func(time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		page, err := c.EventsStream(ctx, after, 100, sessionID)
		return serviceEventsMsg{page: page, err: err, generation: generation, stream: true}
	})
}

// pushServiceEvent tradueix un event durable del servei a línies del TUI.
// El text final es pinta com a resposta de l'agent; la resta, com a
// activitat; el done allibera el torn delegat.
func (m *Model) pushServiceEvent(e serviceclient.Event) {
	text := strings.TrimSpace(e.Text)
	switch e.Kind {
	case "text":
		m.push(assistantMD(text, m.vp.Width))
	case "done":
		m.serviceTurn = false
		m.serviceRunID = 0
		m.status = T("barra.llest")
		m.push(dimStyle.Render("✓ " + T("est.serveiAcabat")))
	case "error":
		m.push(warnStyle.Render(text))
	default:
		// activity, tool_call, tool_result, status, route, budget,
		// run_queued/started/completed...: activitat, no transcript.
		m.push(dimStyle.Render("servei · " + text))
	}
}

// mascotFrames adapta Ventet al terminal: dues cues i els dos ulls.
// bressolen mentre l'agent espera o treballa. En repòs avança un frame a cada tickIdle (500 ms)
var mascotFrames = []string{"≋‹••›", "≈‹••›", "∼‹––›", "≈‹••›"}

// spinnerFrames és més visible que un simple canvi de text: el composer i la
// barra d'estat comparteixen aquest pols mentre hi ha una operació en curs.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Injectable només per provar que View no dispara processos externs.
var gitBranchFn = tools.GitBranch

// refreshGitBranch consulta Git només en esdeveniments, mai des de View.
func (m *Model) refreshGitBranch() { m.branch = gitBranchFn(m.cwd) }

// cmdHint és una ordre amb descripció per l'autocomplete.
type cmdHint struct {
	name string
	desc string
}

// ordres és la llista completa d'ordres (popup amb Tab).
//
// Funció i no variable: com a var, els T() s'avaluaven a la
// inicialització del paquet, abans que New() fixés l'idioma, i el
// desplegable es quedava en català per sempre.
func ordres() []cmdHint {
	return []cmdHint{
		{"agent", T("cmd.agent")},
		{"bash", T("cmd.bash")},
		{"cancel", T("cmd.cancel")},
		{"clear", T("cmd.clear")},
		{"edit", T("cmd.edit")},
		{"end", T("cmd.end")},
		{"find", T("cmd.find")},
		{"graf", T("cmd.graf")},
		{"grafs", T("cmd.grafs")},
		{"help", T("cmd.help")},
		{"key", T("cmd.key")},
		{"mode", T("cmd.mode")},
		{"model", T("cmd.model")},
		{"permissions", T("cmd.permissions")},
		{"plan", T("cmd.plan")},
		{"provider", T("cmd.provider")},
		{"queue", T("cmd.queue")},
		{"reviewer", T("cmd.reviewer")},
		{"review", T("cmd.review")},
		{"settings", T("cmd.settings")},
		{"stats", T("cmd.stats")},
		{"quit", T("cmd.quit")},
		{"read", T("cmd.read")},
		{"checkpoints", T("cmd.checkpoints")},
		{"diff", T("cmd.diff")},
		{"compact", T("cmd.compact")},
		{"connect", T("cmd.connect")},
		{"parallel", T("cmd.parallel")},
		{"attach", T("cmd.attach")},
		{"permissiu", T("cmd.permissiu")},
		{"remember", T("cmd.remember")},
		{"recall", T("cmd.recall")},
		{"retry", T("cmd.retry")},
		{"forget", T("cmd.forget")},
		{"nota", T("cmd.nota")},
		{"notes", T("cmd.notes")},
		{"oblida-nota", T("cmd.oblida-nota")},
		{"mouse", T("cmd.mouse")},
		{"theme", T("cmd.theme")},
		{"copy", T("cmd.copy")},
		{"mcp", T("cmd.mcp")},
		{"pin", T("cmd.pin")},
		{"resume", T("cmd.resume")},
		{"rewind", T("cmd.rewind")},
		{"role", T("cmd.role")},
		{"roles", T("cmd.roles")},
		{"save", T("cmd.save")},
		{"sessions", T("cmd.sessions")},
		{"verify", T("cmd.verify")},
		{"write", T("cmd.write")},
	}
}

// filterCommands retorna les ordres que comencen per prefix (amb / o sense).
// maxSugg és l'alçada FIXA del popup de suggeriments: sempre pinta aquestes
// files quan és actiu (farcit amb buides) perquè el text de dalt no balli
// mentre filtres lletra a lletra.
const maxSugg = 5

func filterCommands(prefix string) []cmdHint {
	prefix = strings.TrimPrefix(prefix, "/")
	var out []cmdHint
	for _, c := range ordres() {
		if strings.HasPrefix(c.name, prefix) {
			out = append(out, c)
			if len(out) >= maxSugg {
				break
			}
		}
	}
	return out
}

// filterUserCommands hi afegeix les personalitzades que encaixen.
func filterUserCommands(prefix, cwd string, out []cmdHint) []cmdHint {
	prefix = strings.TrimPrefix(prefix, "/")
	for _, c := range usercmd.List(cwd) {
		if len(out) >= maxSugg {
			break
		}
		if strings.HasPrefix(c.Name, prefix) {
			dup := false
			for _, o := range out {
				if o.name == c.Name {
					dup = true
					break
				}
			}
			if !dup {
				out = append(out, cmdHint{name: c.Name, desc: c.Desc + " (" + c.Source + ")"})
			}
		}
	}
	return out
}

// suggestions retorna el popup si l'input és una ordre a mitges.
func (m Model) suggestions() []cmdHint {
	if strings.Contains(m.input.Value(), "\n") {
		return nil
	}
	if m.writing || m.editing || m.pending != nil || m.busy {
		return nil
	}
	v := m.input.Value()
	if !strings.HasPrefix(v, "/") || strings.Contains(v, " ") {
		return nil
	}
	return filterUserCommands(v, m.cwd, filterCommands(v))
}

// tryComplete completa la seleccionada (↑↓) + espai (Tab). Torna fals si no hi ha.
func (m *Model) tryComplete() bool {
	sug := m.suggestions()
	if len(sug) == 0 {
		return false
	}
	i := m.suggIdx
	if i < 0 || i >= len(sug) {
		i = 0
	}
	m.input.SetValue("/" + sug[i].name + " ")
	m.suggIdx = 0
	return true
}

// mentionRe detecta @fitxer al xat (estil Claude Code).
var mentionRe = regexp.MustCompile(`@([A-Za-z0-9_./~-][^\s]*)`)

// extractMentions retorna els paths únics mencionats, en ordre
// (sense puntuació final: "@x.go," → "x.go").
func extractMentions(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mentionRe.FindAllStringSubmatch(text, -1) {
		ref := strings.TrimRight(m[1], ".,;:!?)}")
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

// fitInput ajusta l'alçada de l'entrada multilínia (1..6 files).
func (m *Model) fitInput() {
	n := 1 + strings.Count(m.input.Value(), "\n")
	if n < 1 {
		n = 1
	}
	if n > 6 {
		n = 6
	}
	m.input.SetHeight(n)
}

// ampleAmbColumna és des de quantes columnes el cockpit va a la dreta;
// ampleColumna és el que ocupa: 32 de contingut, la línia i un espai.
const (
	ampleAmbColumna = 120
	ampleColumna    = 34
)

// columnaActiva diu si el cockpit es pinta com a columna ara mateix.
func (m Model) columnaActiva() bool {
	return m.cockpit && m.ample >= ampleAmbColumna
}

// midaViewport és vpSize tenint en compte la columna del cockpit.
func (m Model) midaViewport(w, h int) (int, int) {
	if m.cockpit && w >= ampleAmbColumna {
		w -= ampleColumna
	}
	return vpSize(w, h)
}

// redimensiona recalcula el viewport i el composer amb la mida coneguda
// del terminal: és el que passa en commutar la columna amb Ctrl+T.
func (m *Model) redimensiona() {
	if !m.ready || m.ample == 0 {
		return
	}
	vw, vh := m.midaViewport(m.ample, m.alt)
	m.vp.Width, m.vp.Height = vw, vh
	m.input.SetWidth(vw - 4)
	m.refresh()
}

func vpSize(w, h int) (int, int) {
	w -= 4
	// Files fixes que View() pinta al voltant del viewport:
	// capçalera(1)+onada(1)+blanc(1)+blanc(1)+composer(4)+barra(1) = 9.
	// Reserves 11 deixaven dues files mortes sota la barra: el viewport no
	// arribava mai al final del terminal i la barra quedava penjada.
	h -= 9
	if w < 20 {
		w = 20
	}
	if h < 3 {
		h = 3
	}
	return w, h
}

// MaxLinies acota la conversa que es té a la pantalla. No és per estalviar
// memòria: el temps se'n va tot a vp.SetContent, que amb 20.000 línies
// costa 6,3 ms cada cop (el strings.Join, per comparar, en costa 0,5). Amb
// el sostre, el pitjor cas queda fix i el terminal no es va tornant espès a
// mesura que avança la sessió.
//
// Retallar no perd res: la conversa sencera es desa a disc i /resume la
// torna. Això només és el que es pot recórrer amb el scroll.
const (
	MaxLinies   = 6000
	ExcesLinies = 600 // es retalla a blocs; moure el slice també costa
)

func (m *Model) push(line string) {
	m.lines = append(m.lines, line)
	m.retallaSiCal()
	m.refresh()
}

// retallaSiCal treu les línies més velles quan se superi el sostre. L'avís
// no viu a m.lines sinó que es pinta a refresh(): així els índexs (sobretot
// m.streamLine) no s'han de tornar a quadrar cada cop.
func (m *Model) retallaSiCal() {
	if len(m.lines) <= MaxLinies+ExcesLinies {
		return
	}
	tall := len(m.lines) - MaxLinies
	nou := make([]string, MaxLinies, MaxLinies+ExcesLinies+1)
	copy(nou, m.lines[tall:])
	m.lines = nou
	m.retallades += tall
	// L'índex de la línia que creix es desplaça amb la resta, i també el
	// de cada crida d'eina: el seu resultat hi escriu a sobre, i amb
	// l'índex vell hauria anat a parar a una altra fila.
	if m.streamLine -= tall; m.streamLine < 0 {
		m.streamLine = 0
	}
	for i := range m.timeline {
		if m.timeline[i].linia >= 0 {
			if m.timeline[i].linia -= tall; m.timeline[i].linia < 0 {
				m.timeline[i].linia = -1
			}
		}
	}
}

// visual és la conversa tal com va al viewport: cada línia embolcallada
// a l'amplada. El viewport no embolcalla res, talla: un pas del pla o una
// ruta llarga quedaven amagats per la dreta i calia eixamplar la finestra
// per llegir-los. L'embolcall respecta els codis de color.
func (m Model) visual() string {
	if m.vp.Width <= 0 {
		return strings.Join(m.lines, "\n")
	}
	out := make([]string, len(m.lines))
	for i, l := range m.lines {
		// Un \r (sortida de Windows) fa tornar el cursor a la columna 0 i
		// la resta de la fila, columna del cockpit inclosa, es pinta sobre
		// el principi. Un tabulador ocupa el que vol el terminal, no el que
		// compta lipgloss. Cap dels dos pot arribar a la pantalla.
		l = strings.ReplaceAll(l, "\r", "")
		l = strings.ReplaceAll(l, "\t", "    ")
		out[i] = ansi.Wrap(l, m.vp.Width, "")
	}
	return strings.Join(out, "\n")
}

func (m *Model) refresh() {
	if m.ready {
		// Si l'usuari ha pujat per llegir, no li robem la posició quan arriba
		// sortida nova. Només seguim automàticament quan ja era al final.
		follow := m.vp.AtBottom()
		offset := m.vp.YOffset
		m.vp.SetContent(m.visual())
		if follow {
			m.vp.GotoBottom()
		} else {
			m.vp.SetYOffset(offset)
		}
	}
}

func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncat)"
}

func shortArgs(args string, n int) string {
	one := strings.Join(strings.Fields(args), " ")
	if len(one) <= n {
		return one
	}
	return one[:n] + "…"
}

func (m Model) roleRef() config.Role                 { return m.cfg.Roles[m.role] }
func (m Model) provOf(r config.Role) config.Provider { return m.cfg.Providers[r.Provider] }

// transcriptTail retorna els últims n missatges com a text pel revisor.
func (m Model) transcriptTail(n int) string {
	msgs := m.convo
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	var b strings.Builder
	for _, msg := range msgs {
		if msg.Role == "tool" {
			b.WriteString("tool(" + msg.Name + "): ")
		} else {
			b.WriteString(msg.Role + ": ")
		}
		b.WriteString(msg.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) sendChatStream(hist []llm.Message, s *streamer) (tea.Cmd, context.CancelFunc) {
	r := m.roleRef()
	p := m.provOf(r)
	c := m.client
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	return func() tea.Msg {
		defer cancel()
		defer s.finish()
		reply, err := c.ChatStream(ctx, p.BaseURL, p.APIKey, r.Model, hist, r.Temperature, r.MaxTokens, s.add, s.addThink)
		_, think, _ := s.snapshot()
		prompt := 0
		cached := 0
		if u, ok := c.LastUsage(); ok {
			prompt = u.PromptTokens
			cached = u.PromptTokensDetails.CachedTokens
		}
		return streamDoneMsg{reply: reply, think: len([]rune(think)), promptTokens: prompt, cachedTokens: cached, err: err}
	}, cancel
}

// parallelRoleNames retorna els rols que es compararan, sense repetir el
// mateix provider/model. L'actiu sempre hi és; chat, think i reviewer s'hi
// afegeixen si existeixen. Això permet combinar models locals amb un revisor
// cloud sense haver d'editar cap YAML.
func parallelRoleNames(cfg *config.Config, active string) []string {
	order := []string{active, "chat", "think", "reviewer", "code"}
	seenTargets := map[string]bool{}
	seenRoles := map[string]bool{}
	var out []string
	for _, name := range order {
		if name == "" || seenRoles[name] {
			continue
		}
		r, ok := cfg.Roles[name]
		if !ok || name == "reviewer" && strings.TrimSpace(r.Model) == "" {
			continue
		}
		seenRoles[name] = true
		key := r.Provider + "\x00" + r.Model
		if seenTargets[key] {
			continue
		}
		seenTargets[key] = true
		out = append(out, name)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

// startParallel consulta diversos models alhora. És deliberadament una
// operació explícita (/parallel), perquè un torn normal no dupliqui cost cloud
// sense que l'usuari ho demani.
func (m Model) startParallel(prompt string) (tea.Model, tea.Cmd) {
	if m.busy || m.agentActive {
		m.push(systemLine(T("app.feinaEnMarxa")))
		return m, nil
	}
	names := parallelRoleNames(m.cfg, m.role)
	if len(names) < 2 {
		m.push(systemLine(T("app.calenRols")))
		return m, nil
	}
	// Desa també la pregunta al transcript, com fan /chat i /agent. Això fa
	// que una sessió reprenguda conservi el context de la comparació.
	m.convo = append(m.convo, llm.Message{Role: "user", Content: prompt})
	hist := append([]llm.Message{{Role: "system", Content: m.sysPrompt()}}, m.convo...)
	m.push(userLine("⇄ " + prompt))
	m.push(dimStyle.Render("consultant " + strings.Join(names, " · ") + T("app.enParallel")))
	m.busy = true
	m.status = fmt.Sprintf("comparant %d models…", len(names))
	m.promptEst = llm.EstimateTokens(hist)
	cfg, client := m.cfg, m.client
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	m.cancel = cancel
	m.cancelRequested = false
	return m, func() tea.Msg {
		defer cancel()
		results := make([]parallelResult, len(names))
		var wg sync.WaitGroup
		for i, name := range names {
			wg.Add(1)
			go func(i int, name string) {
				defer wg.Done()
				r := cfg.Roles[name]
				p, ok := cfg.Providers[r.Provider]
				results[i] = parallelResult{role: name, provider: r.Provider, model: r.Model}
				if !ok {
					results[i].err = fmt.Errorf("provider desconegut: %s", r.Provider)
					return
				}
				answer, _, err := client.ChatFO(ctx, cfg.PrimTarget(p, r), cfg.FallbackTarget(r), hist,
					r.Temperature, r.MaxTokens, func(string) {})
				results[i].reply, results[i].err = strings.TrimSpace(answer), err
			}(i, name)
		}
		wg.Wait()
		return parallelDoneMsg{prompt: prompt, results: results}
	}
}

func (m Model) sendVerify() (tea.Cmd, context.CancelFunc) {
	r := m.cfg.Roles["reviewer"]
	p := m.cfg.Providers[r.Provider]
	// Si el reviewer apunta a un proveïdor remot i no té clau configurada,
	// o si el model actiu és local i no hi ha connexió remota configurada,
	// no bloquegem el TUI amb una crida condemnada a fallar.
	if p.APIKey == "" && !strings.Contains(p.BaseURL, "localhost") && !strings.Contains(p.BaseURL, "127.0.0.1") {
		return func() tea.Msg {
			return verifyDoneMsg{verdict: verify.Verdict{Approved: true, Summary: "verificació omesa (reviewer remot sense clau)"}, raw: ""}
		}, func() {}
	}
	transcript := m.transcriptTail(6)
	diff := tools.GitDiff(m.cwd)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	return func() tea.Msg {
		defer cancel()
		v, raw, err := verify.Run(ctx, m.client, p, r, transcript, diff)
		return verifyDoneMsg{verdict: v, raw: raw, err: err}
	}, cancel
}

// autonomousCheckpointCmd executa les comprovacions declarades i, cada cert
// nombre de checkpoints, una revisió curta del diff. El missatge s'afegeix a
// l'historial perquè el mateix agent pugui corregir o replanificar.
func (m Model) autonomousCheckpointCmd() (tea.Cmd, context.CancelFunc) {
	a := m.cfg.AutonomousConfig()
	hist := append([]llm.Message(nil), m.histAgent()...)
	cwd := m.cwd
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	return func() tea.Msg {
		defer cancel()
		checks := agent.RunAutonomousChecks(ctx, cwd, a.Verify)
		verdictText := ""
		if a.ReviewEvery > 0 && (m.checkpointNum()+1)%a.ReviewEvery == 0 {
			vctx, vcancel := context.WithTimeout(ctx, 60*time.Second)
			v, _, err := agent.AutonomousReview(vctx, m.client, m.cfg, agent.AutonomousTranscript(hist), agent.AutonomousDiff(cwd))
			vcancel()
			if err != nil {
				verdictText = "error: " + err.Error()
			} else if v.Approved {
				verdictText = "APROVAT"
			} else {
				verdictText = "CAL REVISAR: " + v.Summary
			}
		}
		return autonomousCheckpointMsg{checks: checks, verdict: verdictText}
	}, cancel
}

// agentStepCmd demana el següent pas al model del rol ACTIU amb eines. El
// text arriba en directe a s (i el raonament també): la via amb eines ja
// fa streaming, així que cap mode ha de triar entre veure el text créixer i
// poder cridar eines.
func (m Model) agentStepCmd(hist []llm.Message, s *streamer) (tea.Cmd, context.CancelFunc) {
	r := m.roleRef()
	if m.agentActive && m.agentRole != "" {
		r = m.cfg.Roles[m.agentRole]
	}
	p := m.cfg.Providers[r.Provider]
	c := m.client
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	ctx = ambAvisReintent(ctx, s)
	// think del rol (sense això només el headless el respectava), pas a
	// pas amb think: auto: m.pasMecanic el posa avanca amb el Pas.
	ctx = llm.WithThink(ctx, agent.ThinkPerPas(r.Think, agent.Pas{Ordre: agent.OrdrePasModel, Mecanic: m.pasMecanic}))
	return func() tea.Msg {
		defer cancel()
		var fb string
		content, calls, _, err := c.ChatStreamWithToolsFO(ctx, m.cfg.PrimTarget(p, r), m.cfg.FallbackTarget(r), hist, r.Temperature, r.MaxTokens, agent.SpecsAll(),
			s.add, s.addThink, func(model string) { fb = model })
		s.finish()
		prompt := 0
		cached := 0
		if u, ok := c.LastUsage(); ok {
			prompt = u.PromptTokens
			cached = u.PromptTokensDetails.CachedTokens
		}
		return agentStepMsg{content: content, calls: calls, err: err, fb: fb, promptTokens: prompt, cachedTokens: cached}
	}, cancel
}

// finestraAgent és la finestra de context (el límit del model) del rol de
// l'agent. Per decidir compactació cal agent.Budget, que en descompta la
// reserva de resposta.
func (m Model) finestraAgent() int {
	return agent.Window(m.cfg, m.rolAgent())
}

// windowsMsg arriba quan s'han detectat les finestres dels proveïdors.
type windowsMsg struct{}

// detectWindowsCmd demana a cada proveïdor la finestra dels seus models
// (fora del bucle d'esdeveniments; la barra i la compactació les fan
// servir al pas següent).
func (m Model) detectWindowsCmd() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		agent.DetectWindows(ctx, cfg)
		return windowsMsg{}
	}
}

// agentCompactCmd fa lloc a l'historial de l'agent amb resum (MakeRoom)
// fora del bucle d'esdeveniments. budget és la finestra efectiva per al
// prompt (agent.Budget), no la finestra del model.
func (m Model) agentCompactCmd(budget int) (tea.Cmd, context.CancelFunc) {
	r := m.rolAgent()
	p := m.cfg.Providers[r.Provider]
	c := m.client
	sys := m.sysPrompt()
	hist := append([]llm.Message(nil), m.histAgent()...)
	prim, fb := m.cfg.PrimTarget(p, r), m.cfg.FallbackTarget(r)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	return func() tea.Msg {
		defer cancel()
		return agentCompactMsg{room: agent.MakeRoom(ctx, c, prim, fb, sys, hist, budget)}
	}, cancel
}

// turnDidRealWork diu si el torn ha tocat fitxers (journal o git diff).
// La verificació automàtica només corre en aquest cas: en xat pur,
// sense eines ni canvis, no hi ha res a revisar i el veredicte només
// fa soroll.
func (m Model) turnDidRealWork() bool {
	return verify.DidRealWork(m.journalBase, tools.Active.Seq(), tools.GitDiff(m.cwd))
}

func (m Model) autoVerify() bool {
	return m.cfg.Verify.Mode == "auto" || m.cfg.Verify.Mode == "both" || m.cfg.Verify.Mode == "strict"
}

func (m Model) visualFrame() int {
	if m.reducedMotion {
		return 0
	}
	return m.spin
}

func (m Model) manualVerify() bool {
	return m.cfg.Verify.Mode == "manual" || m.cfg.Verify.Mode == "both"
}

// Update gestiona missatges i tecles.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Clic esquerre a una fila numerada de /sessions la reprèn.
		// La resta (roda, moviment) cau al tractament comú de sota.
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			if m.clicSessio(msg.X, msg.Y) {
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.ample, m.alt = msg.Width, msg.Height
		vw, vh := m.midaViewport(msg.Width, msg.Height)
		if !m.ready {
			m.vp = viewport.New(vw, vh)
			// Sense el KeyMap de pager del viewport (u/d/k/j/b/f/espai…):
			// cada lletra que escriuries mouria la conversa. L'scroll va
			// per dreceres explícites (PgUp/PgDn, Ctrl+Inici/Fi, Ctrl+U/D
			// en buit, Shift+↑/↓) i la roda, que no passen pel KeyMap.
			m.vp.KeyMap = viewport.KeyMap{}
			m.vp.SetContent(strings.Join(m.lines, "\n"))
			// -4: vora del box (2) + padding (0,1) = 2 interior.
			// Sense el -4 el textarea feia línies de 2 columnes més
			// que l'interior i lipgloss les embolcallava: en escriure
			// text llarg, el composer saltava una fila fantasma.
			m.input.SetWidth(vw - 4)
			m.ready = true
			m.push(welcome(m.role, m.vp.Width))
			// Si hi ha feina desada (p. ex. d'abans d'actualitzar l'exe),
			// es diu a l'inici: si no, ningú troba /resume.
			if h := sessionsHint(session.DefaultDir()); h != "" {
				m.push(h)
			}
			m.benvingudaN = len(m.lines)
		} else {
			m.vp.Width = vw
			m.vp.Height = vh
			m.input.SetWidth(vw - 4)
			// L'embolcall depèn de l'amplada: es torna a fer.
			m.refresh()
		}

	case tea.KeyMsg:
		// L'ajuda de tecles es tanca amb la primera tecla, sigui quina
		// sigui; la tecla no fa res més (si no, un Enter la tancaria i
		// enviaria el que hi hagués escrit).
		if m.ajuda {
			m.ajuda = false
			return m, nil
		}
		// La cerca a l'historial menja les tecles mentre és oberta: si no,
		// escriure-hi dispararia les dreceres de sota.
		if m.histCerca != nil && m.tecleaCercaHist(msg.String(), msg.Runes) {
			return m, nil
		}
		// Clau d'API en curs: les tecles no arriben al composer, que la
		// pintaria en clar. S'acumulen a keyBuf i la pantalla mostra
		// rodonetes.
		if m.pendingKeyFor != "" && msg.String() != "ctrl+c" {
			switch msg.String() {
			case "enter":
				nom, clau := m.pendingKeyFor, m.keyBuf
				m.pendingKeyFor, m.keyBuf = "", ""
				if strings.TrimSpace(clau) == "" {
					m.push(systemLine(T("app.clauCancel") + nom + T("app.senseClau")))
					return m, nil
				}
				m.push(m.applyProviderKey(nom, clau))
				return m, nil
			case "esc":
				m.push(systemLine(T("app.clauCancel") + m.pendingKeyFor + T("app.senseClau")))
				m.pendingKeyFor, m.keyBuf = "", ""
				return m, nil
			case "backspace":
				if r := []rune(m.keyBuf); len(r) > 0 {
					m.keyBuf = string(r[:len(r)-1])
				}
				return m, nil
			}
			if len(msg.Runes) > 0 {
				m.keyBuf += string(msg.Runes)
			}
			return m, nil
		}
		if m.pendingPlan != "" && m.pending == nil && m.menu == nil && !m.busy && strings.TrimSpace(m.input.Value()) == "" {
			switch msg.String() {
			case "e":
				return m.answerPlan("e")
			case "d", "esc":
				return m.answerPlan("d")
			}
		}
		if m.pending != nil && msg.String() != "ctrl+c" {
			switch msg.String() {
			case "left", "up", "shift+tab":
				if m.confirmIdx > 0 {
					m.confirmIdx--
				}
				return m, nil
			case "right", "down", "tab":
				if m.confirmIdx < len(m.botonsConfirm())-1 {
					m.confirmIdx++
				}
				return m, nil
			case "enter":
				return m.answerPending(m.botonsConfirm()[m.confirmIdx].resposta)
			case "esc", "n":
				return m.answerPending("n")
			case "s", "y":
				return m.answerPending("s")
			case "r", "a":
				return m.answerPending("r")
			}
			return m, nil
		}
		if m.menu != nil && msg.String() != "ctrl+c" {
			m.menuKeys(msg.String())
			if cmd := m.takeDeferred(); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		if m.timelineOpen && msg.String() != "ctrl+c" {
			switch msg.String() {
			case "esc", "ctrl+l":
				m.timelineOpen = false
			case "up", "k", "ctrl+p":
				m.timelineMove(-1)
			case "down", "j", "ctrl+n":
				m.timelineMove(1)
			case "enter", " ":
				m.timelineToggle()
			case "home":
				m.timelineSel = 0
			case "end":
				if len(m.timeline) > 0 {
					m.timelineSel = len(m.timeline) - 1
				}
			}
			return m, nil
		}
		if cur := m.input.Value(); cur != m.lastInput {
			m.lastInput = cur
			m.suggIdx = 0
		}
		switch msg.String() {
		case "esc":
			// Esc atura la feina en curs (com la 1a pulsació de Ctrl+C).
			if m.pendingQ != nil {
				// A la pregunta: Esc = sense resposta, el model continua
				// sol amb el seu criteri (no cancel·la el torn).
				return m.answerQuestion(-1, "")
			}
			// Un graf pot durar minuts: Esc el talla allà on sigui. El pas
			// en marxa s'acaba (no es pot desfer a mitges) i no se n'engega
			// cap més.
			if m.flow != nil && m.flowCancel != nil {
				m.flowCancel()
				m.push(systemLine("graf: aturant… (s'acaba el pas en marxa)"))
				return m, nil
			}
			if m.busy && m.cancel != nil {
				m.cancelRequested = true
				m.cancel()
				m.status = T("est.cancellant")
				return m, nil
			}
		case "alt+enter", "shift+enter":
			// El textarea ignora l'enter amb modificador: insereix el
			// salt a la posició del cursor (entrada multilínia).
			m.input.InsertRune('\n')
			m.fitInput()
			return m, nil
		case "ctrl+c":
			// El primer atura la feina; el segon surt. Abans, un cop
			// demanada la cancel·lació, m.cancel seguia sent no-nil i
			// cap Ctrl+C posterior no tancava mai el programa.
			if m.cancel != nil && !m.cancelRequested {
				m.cancelRequested = true
				m.cancel()
				m.status = T("est.cancellant")
				return m, nil
			}
			m.autoSave()
			return m, tea.Quit
		case "ctrl+,":
			if !m.writing && !m.editing && m.pending == nil && !m.busy {
				m.menu = settingsMenu(&m)
			}
			return m, nil
		// Scroll de la conversa. PageUp/PageDown són les que la barra d'estat
		// anuncia des de sempre i no estaven implementades enlloc: llegir el
		// que havia passat amunt era, senzillament, impossible.
		case "pgup":
			m.vp.ViewUp()
			return m, nil
		case "pgdown":
			m.vp.ViewDown()
			return m, nil
		// home/end es queden per al cursor dins del text (són del composer).
		case "ctrl+home":
			m.vp.GotoTop()
			return m, nil
		case "ctrl+end":
			m.vp.GotoBottom()
			return m, nil
		case "shift+up", "ctrl+up":
			m.vp.LineUp(3)
			return m, nil
		case "shift+down", "ctrl+down":
			m.vp.LineDown(3)
			return m, nil
		case "ctrl+u", "ctrl+d":
			// Mitja pàgina NOMÉS amb el composer buit: amb text escrit,
			// ctrl+u esborra la línia i ctrl+d el caràcter (composer),
			// no pas scroll. El viewport també els porta de sèrie, però
			// el seu KeyMap és buit (ho mou tot en escriure).
			if strings.TrimSpace(m.input.Value()) == "" {
				if msg.String() == "ctrl+u" {
					m.vp.HalfPageUp()
				} else {
					m.vp.HalfPageDown()
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.fitInput()
			return m, cmd
		case "up", "down", "ctrl+p", "ctrl+n":
			// Amb pregunta oberta i composer buit, les fletxes trien
			// opció (estil Claude Code): mouen el ressaltat i l'Enter el
			// confirma. Si hi ha text escrit, manen el composer (cursor)
			// i l'Enter l'envia com a resposta lliure.
			if m.pendingQ != nil && strings.TrimSpace(m.input.Value()) == "" {
				d := 1
				if k := msg.String(); k == "up" || k == "ctrl+p" {
					d = -1
				}
				n := len(m.pendingQ.options)
				m.pendingQ.sel = (m.pendingQ.sel + d + n) % n
				return m, nil
			}
			// ↑/↓ és historial d'entrada (com la shell), també mentre
			// l'agent treballa: el que s'escrigui llavors s'encua. La
			// primera ↑ exigeix composer d'una línia (si no, mana el
			// cursor); un cop navegant (histIdx >= 0), les fletxes
			// continuen navegant encara que l'entrada mostrada en tingui:
			// si no, et quedaves encallat a la primera multilínia.
			if !m.writing && !m.editing && m.pending == nil && m.menu == nil && m.pendingQ == nil && m.histCerca == nil && (m.histIdx >= 0 || !strings.Contains(m.input.Value(), "\n")) {
				// Mentre es navega l'historial (histIdx >= 0), les fletxes
				// són SEMPRE de l'historial: si el missatge repescat
				// comença per "/" (una ordre antiga), el popup de
				// suggeriments se les quedava i la navegació quedava
				// clavada (la ↓ no avançava mai al missatge posterior).
				if m.histIdx < 0 {
					if sug := m.suggestions(); len(sug) > 0 {
						k := msg.String()
						if k == "up" || k == "ctrl+p" {
							m.suggIdx--
						} else {
							m.suggIdx++
						}
						if m.suggIdx < 0 {
							m.suggIdx = 0
						}
						if m.suggIdx >= len(sug) {
							m.suggIdx = len(sug) - 1
						}
						return m, nil
					}
				}
				k := msg.String()
				m.histNav(k == "up" || k == "ctrl+p")
				return m, nil
			}
		case "tab":
			if m.tryComplete() {
				return m, nil
			}
			// Tab ja no canvia de mode sol: un Tab involuntari et podia
			// deixar a "goal" (sense eines) i semblava que l'agent "no
			// acaba les tasques". El mode es canvia amb /mode o Shift+Tab.
			// I només ensenya amb el composer buit: a mig escriure, el
			// Tab és gairebé segur involuntari i la línia mouria la conversa.
			if strings.TrimSpace(m.input.Value()) == "" {
				m.push(systemLine(T("app.pistaOrdres")))
			}
			return m, nil
		case "ctrl+r":
			if len(m.histEntries) == 0 {
				m.push(systemLine(T("cerca.buida")))
				return m, nil
			}
			m.obreCercaHist()
			return m, nil
		case "ctrl+f":
			m.input.SetValue("/find ")
			m.fitInput()
			return m, nil
		case "ctrl+t":
			// Ctrl+T mostra/amaga el cockpit. En un terminal ample és la
			// columna de la dreta, i l'elecció es desa al config; en un
			// d'estret és la caixa sobre el composer, i no es desa.
			if m.ample >= ampleAmbColumna {
				m.cockpit = !m.cockpit
				m.cfg.Cockpit = "off"
				if m.cockpit {
					m.cfg.Cockpit = "on"
				}
				if m.cfgPath != "" {
					if err := m.cfg.Save(m.cfgPath); err != nil {
						m.push(systemLine(err.Error()))
					}
				}
				m.redimensiona()
				return m, nil
			}
			m.showTodos = !m.showTodos
			return m, nil
		case "ctrl+l":
			// Timeline del torn: ↑↓ navega i Enter desplega el resultat.
			m.timelineOpen = true
			if len(m.timeline) > 0 {
				m.timelineSel = len(m.timeline) - 1
			}
			return m, nil
		case "shift+tab":
			// Cicle explícit de mode (estil Claude: Shift+Tab).
			m.cycleMode()
			return m, nil
		case "?":
			// Amb el composer buit, ? és l'ajuda; amb text, és un signe
			// d'interrogació com qualsevol altre. Les pistes de tecles han
			// sortit de la barra d'estat i viuen aquí.
			if strings.TrimSpace(m.input.Value()) == "" && !m.busy && m.menu == nil && m.pending == nil && m.pendingQ == nil {
				m.ajuda = true
				return m, nil
			}
		case "enter":
			if m.pendingQ != nil {
				text := strings.TrimSpace(m.input.Value())
				if text == "" {
					// Enter buit confirma l'opció ressaltada (↑↓).
					return m.answerQuestion(m.pendingQ.sel, "")
				}
				m.input.SetValue("")
				m.input.SetHeight(1)
				if n, err := strconv.Atoi(text); err == nil && n >= 1 && n <= len(m.pendingQ.options) {
					return m.answerQuestion(n-1, "")
				}
				return m.answerQuestion(-1, text)
			}
			if sug := m.suggestions(); len(sug) > 0 {
				// Ordre exacta (/mode) → executa; prefix (/mo) → completa.
				exact := false
				v := strings.TrimSpace(m.input.Value())
				for _, c := range sug {
					if "/"+c.name == v {
						exact = true
						break
					}
				}
				if !exact {
					i := m.suggIdx
					if i < 0 || i >= len(sug) {
						i = 0
					}
					m.input.SetValue("/" + sug[i].name + " ")
					m.suggIdx = 0
					return m, nil
				}
			}
			text := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			m.input.SetHeight(1)
			if m.writing {
				return m.writeLine(text)
			}
			if m.editing {
				return m.editLine(text)
			}
			if m.pending != nil {
				return m.answerPending(strings.ToLower(text))
			}
			if m.busy || m.agentActive || m.planning || m.serviceTurn {
				// Com Claude Code: escriure mentre treballa encua el
				// missatge (s'envia en acabar) en comptes de perdre'l.
				// Amb el servei connectat la cua és la del servei: els
				// missatges queden visibles a totes les vistes.
				if text == "" {
					return m, nil
				}
				m.recordHist(text)
				if m.serviceClient != nil {
					return m.submitServiceTurn(text)
				}
				m.enqueue(text)
				m.input.SetValue("")
				m.input.SetHeight(1)
				return m, nil
			}
			if text == "" {
				return m, nil
			}
			m.recordHist(text)
			if strings.HasPrefix(text, "!") && !strings.HasPrefix(text, "!=") {
				// Mode shell (estil Claude): !comanda corre i la sortida
				// queda a la sessió com a context.
				return m.runCommand("/bash " + strings.TrimSpace(strings.TrimPrefix(text, "!")))
			}
			if strings.HasPrefix(text, "/") {
				return m.runCommand(text)
			}
			return m.submitText(text)
		}

	case flowTickMsg:
		return m.flowTick()
	case serviceStatusMsg:
		if msg.err != nil {
			m.serviceClient = nil
			m.serviceStatus = "desconnectat"
			m.serviceTurn = false
			m.serviceRunID = 0
			m.push(systemLine("servei: " + msg.err.Error()))
			return m, nil
		}
		m.serviceClient = msg.client
		m.serviceStatus = "connectat " + msg.health.InstanceID
		m.serviceGeneration++
		m.serviceCursor = 0
		m.serviceSession = "default"
		m.serviceStream = msg.health.Capabilities["event_stream"]
		m.serviceTurn = false
		m.serviceRunID = 0
		m.push(systemLine("servei compartit connectat: " + msg.health.InstanceID))
		return m, m.serviceEventsCmd()
	case serviceEventsMsg:
		if msg.generation != m.serviceGeneration {
			return m, nil
		}
		if msg.err != nil {
			if msg.stream && errors.Is(msg.err, serviceclient.ErrEventsStreamUnsupported) {
				// Servidors antics poden anunciar events durables però no tenir
				// encara la variant SSE; el cursor es conserva i passem a polling.
				m.serviceStream = false
				return m, m.pollServiceEvents()
			}
			// Una desconnexió temporal no desfà la negociació: el següent tick
			// torna a provar amb el mateix client, cursor i stream.
			if m.serviceClient != nil {
				return m, m.serviceEventsCmd()
			}
			return m, nil
		}
		for _, e := range msg.page.Events {
			if e.ID <= m.serviceCursor {
				continue
			}
			m.pushServiceEvent(e)
			if e.ID > m.serviceCursor {
				m.serviceCursor = e.ID
			}
		}
		if msg.page.Next > m.serviceCursor {
			m.serviceCursor = msg.page.Next
		}
		return m, m.serviceEventsCmd()
	case serviceSubmitMsg:
		if msg.err != nil {
			// El servei no ha acceptat el missatge: res silenciós. El torn
			// no corre enlloc i l'usuari pot reenviar o /connect.
			m.serviceTurn = false
			m.status = T("barra.llest")
			m.push(warnStyle.Render("servei: " + msg.err.Error()))
			return m, nil
		}
		m.serviceRunID = msg.run.ID
		if msg.run.State == "running" {
			m.status = T("est.serveiExecutant")
		}
		return m, m.serviceEventsCmd()
	case serviceCancelMsg:
		if msg.err != nil {
			m.push(systemLine("servei: " + msg.err.Error()))
		} else {
			m.push(systemLine("torn del servei aturat"))
		}
		return m, nil
	case streamTickMsg:
		m.spin++
		if m.stream != nil {
			text, think, _ := m.stream.snapshot()
			// El text del model es pinta com a markdown a mesura que
			// arriba: títols, llistes i codi es llegeixen des del primer
			// moment. Abans era una cua de 160 caràcters retallada per
			// l'esquerra, que amb paràgrafs ballava i no es podia llegir.
			// El pensament i els avisos segueixen al rail, que és feina.
			shown := ""
			if text != "" {
				shown = m.textEnViu(text)
			} else if think != "" {
				// El pensament en directe va al rail de feina, com la resta:
				// així no es confon amb el principi d'una resposta.
				shown = workRail(pensamentEnViu(think, m.vp.Width))
			} else if n := m.stream.getNote(); n != "" {
				// El proveïdor no respon i el client reintenta: dir-ho.
				shown = workRail(warnStyle.Render("◌ " + n))
			} else if sil := m.stream.silenci(); sil >= esperaMuda {
				// Ni un byte en tota aquesta estona. Sense això, la pantalla
				// deia «escrivint…» durant minuts amb el motor d'inferència
				// caigut darrere d'un gateway, i semblava que el TUI s'havia
				// penjat. Ara diu què passa, quant fa i com sortir-ne.
				shown = workRail(warnStyle.Render(fmt.Sprintf("◌ "+T("est.proveidorMut"), int(sil.Seconds()))))
			}
			if shown != "" && shown != m.streamShown {
				m.streamShown = shown
				m.lines[m.streamLine] = shown
				m.refresh()
			}
		}
		// Batec permanent: ràpid amb feina, suau en repòs. Mai no es
		// deixa morir (abans moria i la pàgina inicial quedava morta).
		if m.busy || m.agentActive {
			if m.turnStart.IsZero() {
				m.turnStart = time.Now()
			}
			return m, tick()
		}
		m.turnStart = time.Time{}
		// Missatges encuats mentre hi havia feina: s'envien ara.
		if mm, cmd := m.drainQueue(); cmd != nil {
			return mm, tea.Batch(cmd, tickIdle())
		} else {
			m = mm.(Model)
		}
		return m, tickIdle()

	case streamDoneMsg:
		m.busy = false
		m.cancel = nil
		m.stream = nil
		m.streamShown = ""
		if m.cancelRequested {
			m.cancelRequested = false
			m.lines[m.streamLine] = systemLine(T("est.respostaCancel"))
			m.refresh()
			m.status = T("barra.llest")
			return m, nil
		}
		if msg.err != nil {
			// Error de finestra en una crida sense eines (síntesi/extensió):
			// el camí d'agent ja el gestiona, però aquest no. Aprenem la
			// mida real i fem lloc perquè el proper torn no torni a petar.
			if ok, actualCtx := agent.ParseContextExceeded(msg.err); ok {
				if actualCtx > 0 {
					if rcfg, ok := m.cfg.Roles[m.role]; ok {
						agent.LearnWindow(m.cfg.Providers[rcfg.Provider].BaseURL, rcfg.Model, actualCtx)
						rcfg.ContextWindow = actualCtx
						m.cfg.Roles[m.role] = rcfg
					}
				}
				if m.torn != nil && len(m.torn.Hist) > agent.KeepRecentOnCompact+1 {
					abans, ara := m.torn.Retalla()
					m.lines[m.streamLine] = systemLine(fmt.Sprintf(
						"error: s'ha excedit el context; historial retallat %d → %d missatges — torna-ho a provar o /compact.",
						abans, ara))
					m.refresh()
					m.status = T("barra.llest")
					return m, nil
				}
			}
			m.lines[m.streamLine] = systemLine("error: " + msg.err.Error())
			m.refresh()
			m.status = "error"
			return m, nil
		}
		if msg.promptTokens > 0 {
			m.promptEst = msg.promptTokens
		}
		m.tokCached += msg.cachedTokens
		if strings.TrimSpace(msg.reply) == "" {
			// Cap client ha de pintar mai un torn en blanc: si no hi ha
			// resposta, s'ha de dir. (El cas normal ja arriba com a error
			// des del client; això és la xarxa de seguretat.)
			m.lines[m.streamLine] = systemLine(T("app.senseResposta"))
		} else {
			m.lines[m.streamLine] = assistantMD(msg.reply, m.vp.Width)
		}
		if msg.think > 0 {
			m.push(thinkLine(fmt.Sprintf(T("app.haPensat"), llm.FmtCount(msg.think))))
		}
		m.refresh()
		if strings.TrimSpace(msg.reply) != "" {
			m.convo = append(m.convo, llm.Message{Role: "assistant", Content: msg.reply})
		}
		tools.Active.MarkConvo(len(m.convo))
		if m.mode == agent.ModeGoal {
			// En mode objectiu, la resposta pot portar el bloc ```goal.
			m.recordGoal(msg.reply)
		}
		m.usedTokens += m.promptEst + llm.EstimateTokens([]llm.Message{{Role: "assistant", Content: msg.reply}})
		m.tokUp += m.promptEst
		m.tokDown += llm.EstimateTokens([]llm.Message{{Role: "assistant", Content: msg.reply}})
		if m.autoVerify() && m.turnDidRealWork() {
			m.busy = true
			m.status = T("est.verificant")
			cmd, cancel := m.sendVerify()
			m.cancel = cancel
			m.cancelRequested = false
			return m, cmd
		}
		m.status = T("barra.llest")
		return m.maybeCompactCmd(false)

	case parallelDoneMsg:
		m.busy = false
		m.cancel = nil
		m.status = T("barra.llest")
		if m.cancelRequested {
			m.cancelRequested = false
			m.push(systemLine(T("app.compCancel")))
			return m, nil
		}
		m.usedTokens += m.promptEst * len(msg.results)
		m.tokUp += m.promptEst * len(msg.results)
		var combined strings.Builder
		for _, result := range msg.results {
			label := fmt.Sprintf("◆ %s · %s/%s", result.role, result.provider, result.model)
			m.push(headStyle.Render(label))
			if result.err != nil {
				m.push(badStyle.Render("error: " + result.err.Error()))
				fmt.Fprintf(&combined, "%s: ERROR: %s\n\n", result.role, result.err)
				continue
			}
			if strings.TrimSpace(result.reply) == "" {
				m.push(dimStyle.Render(T("app.buida")))
				fmt.Fprintf(&combined, T("app.rolBuit"), result.role)
				continue
			}
			m.push(assistantMD(result.reply, m.vp.Width))
			fmt.Fprintf(&combined, "%s (%s/%s):\n%s\n\n", result.role, result.provider, result.model, result.reply)
			m.tokDown += llm.EstimateTokens([]llm.Message{{Role: "assistant", Content: result.reply}})
		}
		if combined.Len() > 0 {
			m.convo = append(m.convo, llm.Message{Role: "assistant", Content: strings.TrimSpace(combined.String())})
			tools.Active.MarkConvo(len(m.convo))
		}
		m.push(dimStyle.Render(T("app.compFeta")))
		return m.maybeCompactCmd(false)

	case compactDoneMsg:
		return m.handleCompactDone(msg)
	case bashDoneMsg:
		m.busy = false
		m.status = T("barra.llest")
		m.refreshGitBranch()
		if msg.err != nil {
			m.push(systemLine("$ " + msg.cmd + " → error: " + msg.err.Error()))
			return m, nil
		}
		m.push(systemLine("$ " + msg.cmd))
		m.push(capText(msg.out, 4000))
		return m, nil

	case opDoneMsg:
		m.busy = false
		m.cancel = nil
		m.status = T("barra.llest")
		m.refreshGitBranch()
		if msg.err != nil {
			m.push(systemLine(msg.desc + " → error: " + msg.err.Error()))
		} else {
			m.push(okStyle.Render("✓ " + msg.desc))
			if strings.TrimSpace(msg.out) != "" {
				m.push(capText(msg.out, 4000))
			}
		}
		if msg.next != nil {
			return m.Update(msg.next(msg.out, msg.err))
		}
		return m, nil

	case agentStepMsg:
		m.cancel = nil
		m.stream = nil
		m.streamShown = ""
		if m.cancelRequested {
			return m.cancellaTorn()
		}
		if m.planning {
			return m.passPla(msg)
		}
		if m.torn == nil || m.torn.Acabat() {
			// Un missatge endarrerit d'un torn ja tancat no el pot
			// reobrir ni repetir-ne el resum.
			return m, nil
		}
		m.comptaTokens(msg.promptTokens, msg.cachedTokens, msg.content, msg.calls)
		if msg.fb != "" {
			// Aquest avís és dels que no es poden perdre: si respon el model
			// de reserva vol dir que el que havies triat no ha funcionat, i
			// la resposta pot ser més lenta o pitjor. Abans anava en gris
			// apagat i es perdia al rotlle: exactament el cas en què més
			// importa saber-ho.
			m.push(warnStyle.Render("⚠ " + T("app.fallback") + msg.fb))
		}
		// El text que el model ha escrit abans de cridar eines es queda
		// pintat tal com s'ha llegit. Si no n'hi ha cap, la fila del bloc
		// de feina no diu res i es treu: abans hi quedava un «◌
		// escrivint…» per sempre. Si el pas porta la resposta final,
		// pinta() hi escriurà a sobre.
		if m.streamLine >= 0 && m.streamLine < len(m.lines) {
			if strings.TrimSpace(msg.content) == "" {
				m.esborraLinia(m.streamLine)
			} else {
				m.lines[m.streamLine] = assistantMD(msg.content, m.vp.Width)
			}
		}
		m.pinta(m.torn.RepPas(msg.content, msg.calls, msg.err))
		return m.avanca()

	case windowsMsg:
		return m, nil

	case agentSintesiMsg:
		m.cancel = nil
		m.stream = nil
		m.streamShown = ""
		if m.cancelRequested {
			return m.cancellaTorn()
		}
		if m.torn == nil || m.torn.Acabat() {
			// Un missatge endarrerit d'un torn ja tancat no el pot
			// reobrir ni repetir-ne el resum.
			return m, nil
		}
		m.comptaTokens(0, 0, msg.text, nil)
		m.pinta(m.torn.RepSintesi(msg.text, msg.err))
		return m.avanca()

	case agentCompactMsg:
		m.cancel = nil
		if m.cancelRequested {
			return m.cancellaTorn()
		}
		if m.torn == nil || m.torn.Acabat() {
			// Un missatge endarrerit d'un torn ja tancat no el pot
			// reobrir ni repetir-ne el resum.
			return m, nil
		}
		m.pinta(m.torn.RepCompactacio(msg.room))
		return m.avanca()

	case agentExtensioMsg:
		m.cancel = nil
		if m.cancelRequested {
			return m.cancellaTorn()
		}
		if m.torn == nil || m.torn.Acabat() {
			// Un missatge endarrerit d'un torn ja tancat no el pot
			// reobrir ni repetir-ne el resum.
			return m, nil
		}
		m.pinta(m.torn.RepAmpliacio(msg.resp, msg.err))
		return m.avanca()

	case agentExecMsg:
		m.cancel = nil
		if m.cancelRequested {
			return m.cancellaTorn()
		}
		m.refreshGitBranch()
		if m.planning {
			for _, r := range msg.results {
				m.planHist = append(m.planHist, agent.ToolMsg(r.call, r.out, r.imgs...))
				m.pushToolResult(r.call.Function.Name, r.out, strings.HasPrefix(r.out, "ERROR:"))
			}
			return m.dispatchPlanStep()
		}
		if m.torn == nil || m.torn.Acabat() {
			// Un missatge endarrerit d'un torn ja tancat no el pot
			// reobrir ni repetir-ne el resum.
			return m, nil
		}
		execs := make([]agent.Execucio, 0, len(msg.results))
		for _, r := range msg.results {
			execs = append(execs, agent.Execucio{Call: r.call, Sortida: r.out, Imatges: r.imgs, Diff: r.diff})
		}
		m.pinta(m.torn.RepExecucions(execs))
		return m.avanca()

	case verifyDoneMsg:
		m.busy = false
		m.cancel = nil
		m.status = T("barra.llest")
		if m.cancelRequested {
			m.cancelRequested = false
			m.push(systemLine(T("app.revisioCancel")))
			return m, nil
		}
		m.usedTokens += llm.EstimateTokens([]llm.Message{{Role: "user", Content: m.transcriptTail(6) + tools.GitDiff(m.cwd)}}) + llm.EstimateTokens([]llm.Message{{Role: "assistant", Content: msg.raw}})
		m.tokUp += llm.EstimateTokens([]llm.Message{{Role: "user", Content: m.transcriptTail(6) + tools.GitDiff(m.cwd)}})
		m.tokDown += llm.EstimateTokens([]llm.Message{{Role: "assistant", Content: msg.raw}})
		if msg.err != nil {
			m.push(systemLine("revisor: error: " + msg.err.Error()))
			return m, nil
		}
		if msg.verdict.Approved {
			m.push(okStyle.Render("✓ VEREDICTE: APROVAT"))
		} else {
			m.push(badStyle.Render("✗ VEREDICTE: CAL REVISAR"))
		}
		m.push(systemStyle.Render(msg.verdict.Summary))
		return m, nil

	case autonomousCheckpointMsg:
		m.cancel = nil
		if m.torn == nil || m.torn.Acabat() {
			// Un missatge endarrerit d'un torn ja tancat no el pot
			// reobrir ni repetir-ne el resum.
			return m, nil
		}
		m.busy = false
		resum := agent.RenderCheckpoint(msg.checks, msg.verdict)
		m.pinta(m.torn.RepCheckpoint(resum, msg.verdict))
		return m.avanca()

	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	cmds = append(cmds, cmd)
	m.input, cmd = m.input.Update(msg)
	m.fitInput()
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// startAgent engega el loop d'agent per una tasca (eines amb permís).
func (m *Model) startAgent(task string) (tea.Model, tea.Cmd) {
	if m.agentActive {
		m.push(systemLine(T("app.agentEnMarxa")))
		return m, nil
	}
	m.treuBenvinguda()
	m.lastPrompt = task
	m.agentActive = true
	m.pendingQ = nil
	m.timeline = nil
	m.timelineSel = 0
	tools.TodoClear()
	m.showTodos = false
	if len(m.seedTodos) > 0 {
		// Pla aprovat: el checklist surt abans del primer pas del model.
		tools.TodoSet(m.seedTodos)
		m.seedTodos = nil
		m.showTodos = true
	}
	m.journalBase = tools.Active.Seq() // verificar només si el torn fa coses reals
	// El model rep el mapa del projecte amb la tasca; a la pantalla surt
	// només el que ha escrit la persona.
	m.convo = append(m.convo, llm.Message{Role: "user", Content: agent.TascaAmbMapa(m.mode, m.cwd, task, m.convo)})
	m.agentRole = ""
	if !m.rolePinned {
		hasImages := false
		for _, msg := range m.convo {
			if len(msg.Images) > 0 {
				hasImages = true
				break
			}
		}
		if routed, why := m.cfg.Route(task, hasImages, m.role); routed != m.role && why != "" {
			m.agentRole = routed
			m.push(systemLine("\u21c4 " + why))
		}
	}
	m.push(userLine(task))
	// Tot l'estat del torn (passos, reintents, pressupost, permisos
	// pendents) viu al motor: aquí ja no hi ha res a posar a zero.
	m.torn = m.nouTorn(task)
	mm, cmd := m.avanca()
	return mm, tea.Batch(cmd, tick(), m.detectWindowsCmd())
}

// writeLine acumula una línia al buffer de /write.
func (m Model) writeLine(text string) (tea.Model, tea.Cmd) {
	if text == "/end" {
		return m.finishWrite()
	}
	if text == "/cancel" {
		m.writing = false
		m.writeBuf = nil
		m.status = T("barra.llest")
		m.push(systemLine(T("app.escripturaCancel")))
		return m, nil
	}
	m.writeBuf = append(m.writeBuf, text)
	m.status = fmt.Sprintf(T("app.escrivintN"), m.writePath, len(m.writeBuf))
	return m, nil
}

// finishWrite mostra vista prèvia i demana confirmació.
func (m Model) finishWrite() (tea.Model, tea.Cmd) {
	m.writing = false
	if len(m.writeBuf) == 0 {
		m.status = T("barra.llest")
		m.push(systemLine("res a escriure"))
		return m, nil
	}
	content := strings.Join(m.writeBuf, "\n") + "\n"
	m.writeBuf = nil
	path := m.writePath
	split := strings.Split(content, "\n")
	preview := strings.Join(split[:min(15, len(split))], "\n")
	m.push(systemLine(fmt.Sprintf(T("app.escriuras"), path, len(content))))
	m.push(dimStyle.Render(capText(preview, 1500)))
	m.confirmIdx = 0
	m.pending = &pendingOp{
		desc: fmt.Sprintf("escrit %s", path),
		run: func(ctx context.Context) (string, error) {
			n, err := tools.Write(path, []byte(content))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s (%d bytes)", path, n), nil
		},
	}
	m.status = T("est.confirmaCurt")
	m.push(warnStyle.Render(T("app.confirmaSN")))
	if m.permissive {
		return m.answerPending("s")
	}
	return m, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func workspacePath(dir, name string) string {
	if strings.TrimSpace(name) == "" || filepath.IsAbs(name) || strings.TrimSpace(dir) == "" {
		return name
	}
	return filepath.Join(dir, name)
}

// recordHist desa una entrada (sense duplicats consecutius).
func (m *Model) recordHist(text string) {
	if text == "" {
		return
	}
	if n := len(m.histEntries); n > 0 && m.histEntries[n-1] == text {
		m.histIdx = -1
		m.histDraft = ""
		return
	}
	m.histEntries = append(m.histEntries, text)
	if len(m.histEntries) > session.MaxHistory {
		m.histEntries = m.histEntries[len(m.histEntries)-session.MaxHistory:]
	}
	m.histIdx = -1
	m.histDraft = ""
	// Persistència immediata: si el procés mor sense /quit, la ↑ del
	// proper arrenc encara troba el que s'havia escrit.
	_ = session.SaveHistory(m.histEntries)
}

// histNav navega l'historial amb ↑/↓.
func (m *Model) histNav(up bool) {
	if len(m.histEntries) == 0 {
		return
	}
	if m.histIdx < 0 {
		if !up {
			return
		}
		m.histDraft = m.input.Value()
		m.histIdx = len(m.histEntries) - 1
	} else if up {
		if m.histIdx > 0 {
			m.histIdx--
		}
	} else {
		m.histIdx++
		if m.histIdx >= len(m.histEntries) {
			m.histIdx = -1
			m.input.SetValue(m.histDraft)
			m.fitInput()
			return
		}
	}
	m.input.SetValue(m.histEntries[m.histIdx])
	m.fitInput()
}

// editLine acumula una línia al buffer de /edit.
func (m Model) editLine(text string) (tea.Model, tea.Cmd) {
	if text == "/end" {
		return m.finishEdit()
	}
	if text == "/cancel" {
		m.editing = false
		m.editBuf = nil
		m.status = T("barra.llest")
		m.push(systemLine(T("app.edicioCancel")))
		return m, nil
	}
	m.editBuf = append(m.editBuf, text)
	m.status = fmt.Sprintf(T("app.editant"), m.editPath, m.editStart, m.editEnd, len(m.editBuf))
	return m, nil
}

// finishEdit mostra vista prèvia i demana confirmació.
func (m Model) finishEdit() (tea.Model, tea.Cmd) {
	m.editing = false
	raw, err := os.ReadFile(m.editPath)
	if err != nil {
		m.status = T("barra.llest")
		m.push(systemLine("edit: " + err.Error()))
		return m, nil
	}
	path, start, end := m.editPath, m.editStart, m.editEnd
	buf := append([]string{}, m.editBuf...)
	m.editBuf = nil
	newContent, err := tools.ReplaceLines(string(raw), start, end, buf)
	if err != nil {
		m.status = T("barra.llest")
		m.push(systemLine("edit: " + err.Error()))
		return m, nil
	}
	m.push(systemLine(fmt.Sprintf(T("app.editaras"), path, start, end)))
	m.push(dimStyle.Render(capText(tools.PreviewEdit(blockLines(string(raw), start, end), strings.Join(buf, "\n")), 1500)))
	m.confirmIdx = 0
	m.pending = &pendingOp{
		desc: fmt.Sprintf("edit %s %d-%d", path, start, end),
		run: func(ctx context.Context) (string, error) {
			n, werr := tools.Write(path, []byte(newContent))
			if werr != nil {
				return "", werr
			}
			return fmt.Sprintf("%s (%d bytes)", path, n), nil
		},
	}
	m.status = T("est.confirmaCurt")
	m.push(warnStyle.Render(T("app.confirmaSN")))
	if m.permissive {
		return m.answerPending("s")
	}
	return m, nil
}

// blockLines retorna les línies start..end per la preview.
func blockLines(content string, start, end int) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}

// parseRange accepta "12" o "12-15".
func parseRange(s string) (int, int, bool) {
	if i := strings.Index(s, "-"); i >= 0 {
		a, err1 := strconv.Atoi(s[:i])
		b, err2 := strconv.Atoi(s[i+1:])
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return a, b, true
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, false
	}
	return n, n, true
}

// sysPrompt afegeix les regles del mode (xat: només lectura; objectiu: preguntes)
// més la memòria del projecte (AGENTS.md) si n'hi ha.
func (m Model) sysPrompt() string {
	p := agent.PromptFor(m.cfg.SystemPrompt(), m.mode)
	if info := agent.ContextProjecte(m.cwd); info != "" {
		p += "\n\n" + info
	}
	if m.compacted != "" {
		p += "\n\n[Resum de la conversa anterior — continua amb aquest context]\n" + m.compacted
	}
	return p
}

// answerPending resol una confirmació [s/n].
func (m Model) answerPending(a string) (tea.Model, tea.Cmd) {
	switch a {
	case "r":
		// Recorda per la sessió i aplica: les pròximes crides iguals passen soles.
		if m.pending != nil && m.pending.agent && m.torn != nil {
			c := m.torn.Aprovacio()
			m.remembers.Allow("tui", agent.Sig(c.Function.Name, c.Function.Arguments))
			m.push(dimStyle.Render(T("app.sempreSessio") + c.Function.Name))
		}
		return m.answerPending("s")
	case "s", "si", "sí", "y", "yes":
		op := m.pending
		m.pending = nil
		if op.agent {
			// L'eina no la corre el diàleg: el motor la torna a treure
			// per OrdreExecuta, que és el camí amb cancel·lació i diff.
			if m.torn != nil {
				m.pinta(m.torn.RepAprovacio(true))
			}
			return m.avanca()
		}
		m.push(dimStyle.Render("s → " + op.desc))
		m.busy = true
		m.status = T("est.aplicant")
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		m.cancelRequested = false
		return m, func() tea.Msg {
			defer cancel()
			out, err := op.run(ctx)
			return opDoneMsg{desc: op.desc, out: out, err: err, next: op.next}
		}
	case "n", "no", "q":
		op := m.pending
		m.pending = nil
		m.status = T("barra.llest")
		// Si era una eina de l'agent, el motor respon el tool_call i
		// continua: cap crida no pot quedar sense resposta.
		if op != nil && op.agent && m.torn != nil {
			m.pinta(m.torn.RepAprovacio(false))
			return m.avanca()
		}
		m.push(systemLine(T("est.cancellat")))
		return m, nil
	default:
		m.push(systemLine("respon s, n o r"))
		return m, nil
	}
}

// autoSave desa historial + conversa (best-effort, silenciós). Es crida
// en acabar cada torn i en sortir: la conversa és a disc encara que el
// procés mori sense passar per /quit.
func (m *Model) autoSave() {
	_ = session.SaveHistory(m.histEntries)
	if len(m.convo) == 0 {
		return
	}
	cwd, _ := os.Getwd()
	if strings.TrimSpace(m.cwd) != "" {
		cwd = m.cwd
	}
	name := m.sessionFile
	if name == "" {
		name = session.NewName()
		m.sessionFile = name
	}
	_, _ = session.SaveSession(session.DefaultDir(), session.Session{
		Name:      name,
		Role:      m.role,
		Pinned:    m.pinned,
		Workspace: cwd,
		Compacted: m.compacted,
		Convo:     m.convo,
		Activity:  timelineToSession(m.timeline),
	})
}

// dedupConvo treu missatges idèntics seguits (mateix rol i contingut):
// el doble Enter d'abans els desava duplicats i en reprendre es veien
// dos cops. Només adjacents: repetir de debò amb resposta al mig queda.
func dedupConvo(convo []llm.Message) []llm.Message {
	out := convo[:0:0]
	var ant llm.Message
	teAnt := false
	for _, m := range convo {
		if teAnt && m.Role == ant.Role && m.Content == ant.Content && m.Name == ant.Name && m.ToolCallID == ant.ToolCallID {
			continue
		}
		out = append(out, m)
		ant, teAnt = m, true
	}
	return out
}

// resumeSession carrega una sessió i la pinta.
func (m *Model) resumeSession(name string) {
	m.resumeSessionDir(session.DefaultDir(), name)
}

// resumeSessionDir és el mateix amb directori explícit (tests).
func (m *Model) resumeSessionDir(dir, name string) {
	s, err := session.Load(dir, name)
	if err != nil {
		m.push(systemLine("resume: " + err.Error()))
		return
	}
	// Una conversa represa conserva el seu projecte. Si la carpeta ja no hi
	// és, no substituïm la conversa activa ni reassignem el fitxer original al
	// directori actual: això podria fer actuar eines sobre el projecte erroni.
	restoredWorkspace := m.cwd
	workspaceNotice := ""
	if strings.TrimSpace(s.Workspace) != "" {
		restoredWorkspace, err = filepath.Abs(s.Workspace)
		if err == nil {
			var info os.FileInfo
			info, err = os.Stat(restoredWorkspace)
			if err == nil && !info.IsDir() {
				err = fmt.Errorf("no és una carpeta")
			}
		}
		if err != nil {
			m.push(systemLine("resume: workspace no disponible: " + s.Workspace))
			return
		}
	}
	m.convo = dedupConvo(s.Convo)
	m.sessionFile = s.Name
	m.pinned = s.Pinned
	m.compacted = s.Compacted
	if restoredWorkspace != "" && filepath.Clean(restoredWorkspace) != filepath.Clean(m.cwd) {
		antic := m.cwd
		m.cwd = restoredWorkspace
		if m.policy != nil {
			m.policy.ProjectDir = restoredWorkspace
		}
		m.refreshGitBranch()
		workspaceNotice = fmt.Sprintf(T("app.resumeAltreDir"), restoredWorkspace, antic)
	}
	// Sense això la barra arrencava a "ctx 0" fins al primer torn, tot i
	// que la conversa represa ja ocupa context.
	m.recalcCtx()
	m.timeline = timelineFromSession(s.Activity)
	m.timelineSel = max(0, len(m.timeline)-1)
	m.lines = nil
	m.vp.SetContent("")
	m.push(systemLine(fmt.Sprintf("reprenent %s (%d missatges, rol %s)", s.Name, len(m.convo), m.role)))
	if workspaceNotice != "" {
		m.push(systemLine(workspaceNotice))
	}
	// El rol desat mana si encara existeix: si no, seguir amb un altre
	// model sense dir-ho és com canviar de cotxe a mitja autopista.
	// (Després de netejar lines: si no, l'avís s'esborra.)
	if s.Role != "" {
		if _, ok := m.cfg.Roles[s.Role]; ok {
			if s.Role != m.role {
				m.role = s.Role
				m.push(systemLine(fmt.Sprintf(T("app.resumeRol"), s.Role)))
			}
		} else {
			m.push(systemLine(fmt.Sprintf(T("app.resumeRolPerdut"), s.Role, m.role)))
		}
	}
	for _, msg := range m.convo {
		switch msg.Role {
		case "user":
			m.push(userLine(agent.SenseMapa(msg.Content)))
		case "assistant":
			if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) > 0 {
				m.push(dimStyle.Render(fmt.Sprintf(T("app.cridesEina"), len(msg.ToolCalls))))
			} else {
				m.push(assistantMD(msg.Content, m.vp.Width))
			}
		case "tool":
			// els resultats ja es van veure en el seu moment
		}
	}
}

// sessioText resumeix una sessió en una línia llegible (títol si n'hi ha,
// si no el nom): menú, /sessions i pista inicial el comparteixen.
func sessioText(in session.Info) string {
	titol := in.Title
	if titol == "" {
		titol = in.Name
	}
	if r := []rune(titol); len(r) > 40 {
		titol = strings.TrimSpace(string(r[:39])) + "…"
	}
	return fmt.Sprintf("%s (%s, %d missatges, %s)", titol, in.Name, in.Msgs, in.SavedAt.Format("02-01 15:04"))
}

// llistaSessions pinta les desades numerades (1 = la més recent): una
// per línia, amb títol llegible i nom tècnic apagat (els slugs són
// llargs i ningú els hauria de teclejar). /resume N continua la N.
func llistaSessions(dir string) string {
	infos, err := session.List(dir)
	if err != nil || len(infos) == 0 {
		return systemLine(T("app.capSessio"))
	}
	var b strings.Builder
	b.WriteString(systemLine(T("app.sessionsTitol")))
	for i, in := range infos {
		titol := in.Title
		if titol == "" {
			titol = in.Name
		}
		if r := []rune(titol); len(r) > 40 {
			titol = strings.TrimSpace(string(r[:39])) + "…"
		}
		fmt.Fprintf(&b, "\n  %d · %s  %s", i+1, titol,
			dimStyle.Render(fmt.Sprintf("(%s · %d missatges · %s)", in.Name, in.Msgs, in.SavedAt.Format("02-01 15:04"))))
	}
	return b.String()
}

// sessioPerNumero resol el número de /resume N (1 = recent primer).
func sessioPerNumero(dir string, n int) (session.Info, bool) {
	infos, err := session.List(dir)
	if err != nil || n < 1 || n > len(infos) {
		return session.Info{}, false
	}
	return infos[n-1], true
}

// reFilaSessio detecta les files numerades de /sessions ("  N · ...").
var reFilaSessio = regexp.MustCompile(`^\s*(\d+)\s*·`)

// clicSessio reprèn la sessió clicada (botó esquerre sobre una fila
// "N · ..." del viewport). Torna fals si no era cap fila (el clic cau
// al tractament comú). Mai en feina ni amb popups: ni roba torns ni
// encerta files mogudes.
func (m *Model) clicSessio(x, y int) bool {
	if x < 0 || y < 0 {
		return false
	}
	if m.busy || m.agentActive || m.planning || m.flow != nil {
		return false
	}
	if m.menu != nil || m.pending != nil || m.pendingQ != nil || m.histCerca != nil || m.showTodos {
		return false
	}
	if m.writing || m.editing || m.pendingPlan != "" {
		return false
	}
	if len(m.suggestions()) > 0 {
		return false
	}
	// Files 0-1 capçalera i onada, 2 en blanc, 3.. viewport.
	rel := y - 3
	if rel < 0 || rel >= m.vp.Height {
		return false
	}
	// Files VISUALS (partides pels \n interns): l'índex del viewport no
	// és l'índex de m.lines (un missatge en té moltes).
	visuals := strings.Split(m.visual(), "\n")
	idx := m.vp.YOffset + rel
	if idx < 0 || idx >= len(visuals) {
		return false
	}
	sub := reFilaSessio.FindStringSubmatch(visuals[idx])
	if sub == nil {
		return false
	}
	n, err := strconv.Atoi(sub[1])
	if err != nil {
		return false
	}
	dir := m.sessDir
	if dir == "" {
		dir = session.DefaultDir()
	}
	if in, ok := sessioPerNumero(dir, n); ok {
		m.resumeSessionDir(dir, in.Name)
		return true
	}
	return false
}

// resumeTriada resol número (1-based, recent primer) o nom de sessió.
func (m *Model) resumeTriada(dir, arg string) {
	if n, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil {
		if in, ok := sessioPerNumero(dir, n); ok {
			m.resumeSessionDir(dir, in.Name)
			return
		}
		m.push(systemLine(fmt.Sprintf(T("app.resumeMalNumero"), arg)))
		return
	}
	m.resumeSessionDir(dir, arg)
}

// sessionsHint diu a l'inici que hi ha feina per reprendre (directori
// explícit pels tests; buit si no hi ha res).
func sessionsHint(dir string) string {
	infos, err := session.List(dir)
	if err != nil || len(infos) == 0 {
		return ""
	}
	txt := fmt.Sprintf(T("benvinguda.sessions"), len(infos), nomSessioCurt(infos[0].Name))
	return "  " + faintStyle.Render("↺ ") + dimStyle.Render(strings.TrimPrefix(txt, "  ↺ "))
}

// nomSessioCurt deixa «sessio-20260918-112808-312» en «18/09 11:28»: a la
// pantalla d'entrada l'identificador sencer era la línia més llarga i la
// menys llegible.
func nomSessioCurt(name string) string {
	n := strings.TrimPrefix(name, "sessio-")
	if len(n) >= 15 && n[8] == '-' {
		d, h := n[:8], n[9:15]
		return d[6:8] + "/" + d[4:6] + " " + h[0:2] + ":" + h[2:4]
	}
	return name
}

// submitText processa text d'usuari (l'historial ja l'ha enregistrat
// l'enter): mode code → agent, altrament xat amb @adjunts.
func (m Model) submitText(text string) (tea.Model, tea.Cmd) {
	m.treuBenvinguda()
	m.lastPrompt = text
	// Cortesia pura ("hola") es respon en local, sense model ni eines:
	// abans cremava un torn d'agent sencer explorant el repo.
	if reply, ok := agent.SmallTalk(text); ok {
		m.push(userLine(text))
		m.convo = append(m.convo, llm.Message{Role: "user", Content: text})
		m.convo = append(m.convo, llm.Message{Role: "assistant", Content: reply})
		m.push(assistantMD(reply, m.vp.Width))
		m.status = T("barra.llest")
		return m, nil
	}
	// Tots els modes passen per l'agent: code (amb permís), chat i
	// consulta (la política ja hi denega escriure) i goal (només lectura,
	// com chat: el prompt li promet read/grep/glob). Abans chat, consulta
	// i goal anaven per ChatStream, que no porta `tools`, i els models
	// intentaven llegir escrivint la crida en text (`<tool_call>…`, DSML),
	// que no s'executa i matava el torn aquí mateix.
	// Amb el servei compartit connectat, el torn corre al servei: el TUI
	// encua i pinta els events durables, no executa res localment.
	if m.serviceClient != nil {
		return m.submitServiceTurn(text)
	}
	return m.startAgent(text)
}

// submitServiceTurn delega el torn al servei compartit: POST a la cua v2 i
// la resposta arribarà pels events (pollServiceEvents). S'hi envia el
// directori del TUI perquè el torn treballi on l'usuari és, no al cwd de
// la sessió del servei (validat pel guard del servei).
func (m Model) submitServiceTurn(text string) (tea.Model, tea.Cmd) {
	m.lastPrompt = text
	m.push(userLine(text))
	m.serviceTurn = true
	m.serviceRunID = 0
	m.status = T("est.serveiEncua")
	c := m.serviceClient
	sess := m.serviceSession
	ws := m.cwd
	// Només els modes que restringeixen el torn viatgen (mateixa semàntica
	// que el complement d'Office): la delegació mai no ha d'executar amb
	// més permisos dels que mostra el TUI. Els altres modes els governa la
	// sessió compartida del servei; sincronitzar-los és un lliurament posterior.
	mode := m.mode
	if mode != "chat" && mode != "inspect" && mode != agent.ModeAutonomous {
		mode = ""
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		run, err := c.SubmitRun(ctx, sess, text, mode, "", ws)
		return serviceSubmitMsg{run: run, err: err}
	}
}

// retryLast torna a enviar l'últim text (el "try again" del chat): quan
// el model torna buit o la xarxa falla, reescriure el mateix missatge a
// mà és el que tothom prova primer. No duplica l'usuari al transcript:
// si encara és l'últim missatge, es treu i submitText el torna a afegir.
func (m Model) retryLast() (tea.Model, tea.Cmd) {
	if m.busy || m.agentActive || m.planning {
		m.push(systemLine(T("app.feinaEnMarxa")))
		return m, nil
	}
	text := strings.TrimSpace(m.lastPrompt)
	if text == "" {
		m.push(systemLine(T("app.resRetry")))
		return m, nil
	}
	if n := len(m.convo); n > 0 && m.convo[n-1].Role == "user" && m.convo[n-1].Content == text {
		m.convo = m.convo[:n-1]
	}
	// El torn que ha anat malament no es reaprofita: submitText en
	// construeix un de nou a partir de la conversa, que ja s'ha netejat.
	m.torn = nil
	m.push(dimStyle.Render("↻ " + T("app.retryFet")))
	return m.submitText(text)
}

// runCommand executa ordres locals.
// truncConvo retalla l'historial a n (E2b). n<0 = sense informació (no
// es toca); n>len = no es toca.
func truncConvo[T any](convo []T, n int) []T {
	if n < 0 || n > len(convo) {
		return convo
	}
	return convo[:n]
}

func (m Model) runCommand(text string) (tea.Model, tea.Cmd) {
	m.treuBenvinguda()
	parts := strings.Fields(text)
	m.push(dimStyle.Render(text))
	switch parts[0] {
	case "/quit", "/q":
		m.autoSave()
		return m, tea.Quit
	case "/help":
		m.push(helpBlock())
		return m, nil
	case "/agent":
		task := strings.TrimSpace(strings.TrimPrefix(text, "/agent"))
		if task == "" {
			m.push(systemLine(T("app.usAgent")))
			return m, nil
		}
		return m.startAgent(task)
	case "/parallel":
		task := strings.TrimSpace(strings.TrimPrefix(text, "/parallel"))
		if task == "" {
			m.push(systemLine(T("app.usParallel")))
			return m, nil
		}
		return m.startParallel(task)
	case "/retry", "/reintenta":
		return m.retryLast()
	case "/cancel":
		if m.serviceClient == nil {
			m.push(systemLine("no hi ha cap servei compartit connectat"))
			return m, nil
		}
		c := m.serviceClient
		sessionID := m.serviceSession
		runID := m.serviceRunID
		m.push(systemLine("aturant el torn del servei…"))
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// Amb run conegut, la cancel·lació identifica EL torn; si el
			// desconeixem (TUI acabat de connectar), cau al cancel de sessió.
			if runID > 0 {
				if err := c.CancelRun(ctx, runID); err != nil {
					return serviceCancelMsg{err: err}
				}
				return serviceCancelMsg{}
			}
			if err := c.Cancel(ctx, sessionID); err != nil {
				return serviceCancelMsg{err: err}
			}
			return serviceCancelMsg{}
		}
	case "/review":
		files, err := tools.GitDiffStructured(m.cwd)
		if err != nil {
			m.push(systemLine("review: " + err.Error()))
			return m, nil
		}
		m.menu = reviewMenu(files)
		if m.menu == nil {
			m.push(systemLine(T("review.net")))
		}
		return m, nil
	case "/stats":
		m.push(m.statsBlock())
		return m, nil
	case "/find":
		query := strings.TrimSpace(strings.TrimPrefix(text, "/find"))
		if query == "" {
			query = m.lastFind
		}
		if query == "" {
			m.push(systemLine(T("find.us")))
			return m, nil
		}
		if !m.findNext(query) {
			m.push(systemLine(fmt.Sprintf(T("find.cap"), query)))
		}
		return m, nil
	case "/queue":
		if len(parts) == 1 {
			if len(m.queued) == 0 {
				m.push(systemLine(T("cua.buida")))
				return m, nil
			}
			for i, item := range m.queued {
				m.push(systemLine(fmt.Sprintf("%d · %s", i+1, item)))
			}
			m.push(systemLine(T("cua.pista")))
			return m, nil
		}
		switch parts[1] {
		case "clear":
			m.queued = nil
			m.push(systemLine(T("cua.neta")))
			return m, nil
		case "rm":
			if len(parts) != 3 {
				m.push(systemLine(T("cua.us")))
				return m, nil
			}
			n, err := strconv.Atoi(parts[2])
			if err != nil || n < 1 || n > len(m.queued) {
				m.push(systemLine(T("cua.index")))
				return m, nil
			}
			m.queued = append(m.queued[:n-1], m.queued[n:]...)
			m.push(systemLine(T("cua.treta")))
			return m, nil
		default:
			m.push(systemLine(T("cua.us")))
			return m, nil
		}
	case "/roles":
		var names []string
		for n, r := range m.cfg.Roles {
			names = append(names, fmt.Sprintf("%s→%s/%s", n, r.Provider, r.Model))
		}
		m.push(systemLine("rols: " + strings.Join(names, " · ")))
		return m, nil
	case "/role":
		if len(parts) < 2 {
			m.menu = roleMenu(&m)
			return m, nil
		}
		name := parts[1]
		r, ok := m.cfg.Roles[name]
		if !ok || name == "reviewer" {
			m.push(systemLine("rol desconegut (tria: chat, think, code)"))
			return m, nil
		}
		m.role = name
		m.rolePinned = true
		m.status = T("barra.llest")
		m.push(systemLine(fmt.Sprintf("rol actiu: %s (%s/%s)", name, r.Provider, r.Model)))
		// La finestra és del model del rol: refresca-la (throttled) perquè la
		// barra i la compactació no quedin amb la del rol anterior.
		return m, m.detectWindowsCmd()
	case "/model":
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/model"))
		if arg == "" {
			m.menu = modelMenu(&m)
			return m, nil
		}
		prov, model := m.roleRef().Provider, arg
		if i := strings.Index(arg, "/"); i >= 0 {
			prov, model = arg[:i], arg[i+1:]
		} else if f := strings.Fields(arg); len(f) == 2 {
			if _, ok := m.cfg.Providers[f[0]]; ok {
				// Dues paraules amb provider primer: /model strix Nex-2.5-mini.
				prov, model = f[0], f[1]
			}
		} else if _, ok := m.cfg.Providers[arg]; ok {
			// Només un nom de provider: conserva el model del rol.
			prov, model = arg, m.roleRef().Model
		}
		if _, ok := m.cfg.Providers[prov]; !ok {
			m.push(systemLine("provider desconegut: " + prov))
			return m, nil
		}
		if strings.TrimSpace(model) == "" {
			m.push(systemLine(T("app.usModel")))
			return m, nil
		}
		r := m.roleRef()
		r.Provider, r.Model = prov, model
		m.cfg.Roles[m.role] = r
		m.status = T("barra.llest")
		msg := systemLine(fmt.Sprintf(T("rol.assignat"), m.role, prov, model))
		if kp := m.keyPromptIfMissing(prov); kp != "" {
			msg += "\n" + kp
		}
		m.push(msg)
		if ids, err := fetchModelIDs(m.cfg.Providers[prov].BaseURL, m.cfg.Providers[prov].APIKey); err == nil && len(ids) > 0 {
			if real, ok := matchModelFold(ids, model); ok && real != model {
				// El backend distingeix majúscules: corregeix sol.
				r.Model = real
				m.cfg.Roles[m.role] = r
				m.push(systemLine(fmt.Sprintf(T("app.majuscules"), model, real)))
			} else if !ok {
				m.push(warnStyle.Render("⚠ "+prov+" no llista "+model) + dimStyle.Render(" — disponibles: "+shortIDs(ids)))
			}
		}
		// El model nou pot tenir una finestra diferent: refresca-la.
		return m, m.detectWindowsCmd()
	case "/mode":
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/mode"))
		if arg == "" {
			m.menu = modeMenu()
			return m, nil
		}
		if !agent.ValidMode(arg) {
			m.push(systemLine(T("app.usMode")))
			return m, nil
		}
		m.setMode(arg)
		return m, nil
	case "/goal":
		return m.goalCommand(strings.TrimSpace(strings.TrimPrefix(text, "/goal")))
	case "/plan":
		return m.startPlan(strings.TrimSpace(strings.TrimPrefix(text, "/plan")))
	case "/graf", "/grafs":
		return m.flowCommand(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "/grafs"), "/graf")))
	case "/rewind":
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/rewind"))
		if arg == "" {
			files := tools.Active.Files()
			if len(files) == 0 {
				m.push(systemLine(T("app.resADesfer")))
				return m, nil
			}
			m.confirmIdx = 0
			m.pending = &pendingOp{
				desc: fmt.Sprintf(T("app.desferAvis"), len(files)),
				run: func(ctx context.Context) (string, error) {
					out, err := tools.Active.Rewind()
					if err == nil {
						m.convo = truncConvo(m.convo, tools.Active.ConvoLenAt(0))
					}
					return out, err
				},
			}
			m.push(systemLine(T("app.desfaras") + strings.Join(files, ", ")))
			return m, nil
		}
		// /rewind N → torna al checkpoint N (vegeu /checkpoints).
		var seq int
		if _, err := fmt.Sscanf(arg, "%d", &seq); err != nil {
			m.push(systemLine(T("app.usRewind")))
			return m, nil
		}
		m.confirmIdx = 0
		m.pending = &pendingOp{
			desc: fmt.Sprintf(T("app.tornarCheckpoint"), seq),
			run: func(ctx context.Context) (string, error) {
				out, err := tools.Active.RewindTo(seq)
				if err == nil {
					m.convo = truncConvo(m.convo, tools.Active.ConvoLenAt(seq))
				}
				return out, err
			},
		}
		m.push(systemLine(fmt.Sprintf(T("app.tornarasCheckpoint"), seq)))
		return m, nil
	case "/diff":
		cps := tools.Active.Checkpoints()
		if len(cps) == 0 {
			m.push(systemLine(T("app.capCanviFitxers")))
		} else {
			var rows []string
			for _, c := range cps {
				rows = append(rows, fmt.Sprintf("#%d %s %s", c.Seq, c.Op, c.Path))
			}
			m.push(hunkStyle.Render(T("app.canvisSessio")) + "\n" + indentTool(strings.Join(rows, "\n")))
		}
		if gd := strings.TrimSpace(tools.GitDiff(m.cwd)); gd != "" {
			m.push(indentTool(capText(gd, 2000)))
		}
		return m, nil
	case "/compact":
		if len(m.convo) == 0 {
			m.push(systemLine("res a compactar: conversa buida"))
			return m, nil
		}
		return m.maybeCompactCmd(true)
	case "/remember":
		if err := config.Remember(strings.TrimSpace(strings.TrimPrefix(text, "/remember"))); err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		m.push(systemLine(T("app.apuntat")))
		return m, nil
	case "/recall":
		lines := config.Recall(strings.TrimSpace(strings.TrimPrefix(text, "/recall")))
		if len(lines) == 0 {
			m.push(systemLine(T("app.memBuida")))
			return m, nil
		}
		m.push(systemLine(T("app.memoria") + strings.Join(lines, "\n")))
		return m, nil
	case "/forget":
		n, err := config.Forget(strings.TrimSpace(strings.TrimPrefix(text, "/forget")))
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		if n == 0 {
			m.push(systemLine(T("app.capResultat")))
			return m, nil
		}
		m.push(systemLine(fmt.Sprintf("oblidades %d entrades", n)))
		return m, nil
	case "/nota":
		if err := config.Note(m.cwd, strings.TrimSpace(strings.TrimPrefix(text, "/nota"))); err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		m.push(systemLine("nota desada al projecte"))
		return m, nil
	case "/notes":
		lines := config.Notes(m.cwd, strings.TrimSpace(strings.TrimPrefix(text, "/notes")))
		if len(lines) == 0 {
			m.push(systemLine(T("app.senseNotes")))
			return m, nil
		}
		m.push(systemLine(T("app.notesTitol") + strings.Join(lines, "\n")))
		return m, nil
	case "/oblida-nota":
		n, err := config.ForgetNote(m.cwd, strings.TrimSpace(strings.TrimPrefix(text, "/oblida-nota")))
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		if n == 0 {
			m.push(systemLine(T("app.capNota")))
			return m, nil
		}
		m.push(systemLine(fmt.Sprintf("oblidades %d notes", n)))
		return m, nil
	case "/theme":
		arg := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(text, "/theme")))
		if arg == "menu" || arg == "" {
			m.menu = themeMenu(&m)
			return m, nil
		}
		nou := ""
		if arg == "dark" {
			arg = "fosc"
		} else if arg == "light" {
			arg = "clar"
		}
		for _, t := range config.TemesSuportats {
			if arg == t {
				nou = t
				break
			}
		}
		if nou == "" {
			m.push(systemLine(T("app.temaNo") + arg + T("app.temaNoms") + strings.Join(config.TemesSuportats, ", ") + ")"))
			return m, nil
		}
		m.cfg.Theme = nou
		SetTema(nou)
		missatge := T("app.tema") + nou
		if err := m.cfg.Save(m.cfgPath); err != nil {
			missatge += T("app.noDesat") + err.Error() + ")"
		}
		m.push(systemLine(missatge))
		m.refresh()
		return m, nil
	case "/mouse":
		arg := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(text, "/mouse")))
		switch arg {
		case "on":
			m.mouseOn = true
		case "off":
			m.mouseOn = false
		case "":
			m.mouseOn = !m.mouseOn
		default:
			m.push(systemLine(T("app.usMouse")))
			return m, nil
		}
		if m.mouseOn {
			m.cfg.Mouse = "on"
		} else {
			m.cfg.Mouse = "off"
		}
		estat := T("app.ratoliOn")
		if !m.mouseOn {
			estat = T("app.ratoliOff")
		}
		if err := m.cfg.Save(m.cfgPath); err != nil {
			m.push(systemLine(estat + T("app.noDesat") + err.Error() + ")"))
		} else {
			m.push(systemLine(estat + " (desat)"))
		}
		if m.mouseOn {
			return m, tea.EnableMouseCellMotion
		}
		return m, tea.DisableMouse
	case "/copy":
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/copy"))
		n := 1
		if arg != "" {
			fmt.Sscanf(arg, "%d", &n)
			if n < 1 {
				n = 1
			}
		}
		found := 0
		body := ""
		for i := len(m.convo) - 1; i >= 0; i-- {
			if m.convo[i].Role == "assistant" && strings.TrimSpace(m.convo[i].Content) != "" {
				found++
				if found == n {
					body = m.convo[i].Content
					break
				}
			}
		}
		if body == "" {
			m.push(systemLine(T("app.resACopiar")))
			return m, nil
		}
		how, err := copyToClipboard(body)
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		m.push(systemLine(fmt.Sprintf(T("app.copiat"), len([]rune(body)), how)))
		return m, nil
	case "/attach":
		// /attach <ruta> [text]: afegeix la imatge a la conversa (val per
		// al xat i per al pròxim torn d'agent, que se sembra de m.convo).
		rest := strings.TrimSpace(strings.TrimPrefix(text, "/attach"))
		if rest == "" {
			m.push(systemLine(T("app.usAttach")))
			return m, nil
		}
		fields := strings.Fields(rest)
		caption := "imatge adjunta: " + fields[0]
		if len(fields) > 1 {
			caption = strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
		}
		u, err := tools.ReadImageDataURL(fields[0])
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		m.convo = append(m.convo, llm.Message{Role: "user", Content: caption, Images: []string{u}})
		m.push(systemLine("▣ imatge adjunta (" + fields[0] + ")"))
		return m, nil
	case "/checkpoints":
		cps := tools.Active.Checkpoints()
		if len(cps) == 0 {
			m.push(systemLine(T("app.capCheckpoint")))
			return m, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("checkpoints (%d):\n", len(cps)))
		for _, e := range cps {
			fmt.Fprintf(&b, "  %d · %s %s\n", e.Seq, e.Op, e.Path)
		}
		b.WriteString(T("app.rewindPista"))
		m.push(systemLine(b.String()))
		return m, nil
	case "/mcp":
		if m.mcp == nil || m.mcp.Summary() == "inactiu" {
			m.push(systemLine(T("app.mcpInactiu")))
			return m, nil
		}
		m.push(systemLine("mcp: " + m.mcp.Summary()))
		for _, t := range m.mcp.Tools {
			m.push(dimStyle.Render("  " + t.Spec.Name + " — " + shortArgs(t.Spec.Description, 100)))
		}
		for srv, msg := range m.mcp.Errors {
			m.push(badStyle.Render("  ✗ " + srv + ": " + msg))
		}
		return m, nil
	case "/connect":
		m.push(systemLine("connectant amb el servei compartit…"))
		return m, func() tea.Msg {
			c, health, err := serviceclient.Discover(context.Background())
			return serviceStatusMsg{client: c, health: health, err: err}
		}
	case "/pin":
		if len(m.convo) == 0 {
			m.push(systemLine("res a fixar encara"))
			return m, nil
		}
		cwd := m.cwd
		if strings.TrimSpace(cwd) == "" {
			cwd, _ = os.Getwd()
		}
		name := m.sessionFile
		if name == "" {
			name = session.NewName()
		}
		m.pinned = !m.pinned
		path, err := session.SaveSession(session.DefaultDir(), session.Session{
			Name: name, Role: m.role, Pinned: m.pinned, Workspace: cwd,
			Compacted: m.compacted, Convo: m.convo, Activity: timelineToSession(m.timeline),
		})
		if err != nil {
			m.pinned = !m.pinned
			m.push(systemLine("pin: " + err.Error()))
			return m, nil
		}
		m.sessionFile = strings.TrimSuffix(filepath.Base(path), ".json")
		estat := "desfixada"
		if m.pinned {
			estat = "fixada"
		}
		m.push(systemLine("conversa " + estat))
		return m, nil
	case "/permissions":
		if len(parts) == 1 {
			m.menu = permissionsMenu(&m)
			return m, nil
		}
		if len(parts) != 2 || parts[1] != "show" {
			m.push(systemLine(T("app.usPerms")))
			return m, nil
		}
		var rows []string
		for _, n := range []string{"read", "bash", "write", "edit", "patch", "grep", "glob", "web_search", "web_fetch", "delegate", "read_image", "gh_issue", "gh_pr", "office_read", "office_edit"} {
			d, why := m.policy.Decide(m.mode, n, "{}")
			rows = append(rows, fmt.Sprintf("%s→%s (%s)", n, d, why))
		}
		m.push(systemLine(fmt.Sprintf(T("app.modePermisos"), m.mode, strings.Join(rows, " · "))))
		if len(m.policy.BashAllow) > 0 {
			m.push(systemLine("bash_allow: " + strings.Join(m.policy.BashAllow, ", ")))
		}
		if len(m.policy.BashDeny) > 0 {
			m.push(systemLine("bash_deny: " + strings.Join(m.policy.BashDeny, ", ")))
		}
		return m, nil
	case "/reviewer":
		m.menu = reviewerMenu(&m)
		return m, nil
	case "/settings", "/config":
		m.menu = settingsMenu(&m)
		return m, nil
	case "/key", "/apikey":
		arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "/apikey"), "/key"))
		return m.handleKeyCommand(arg)
	case "/provider":
		m.push(m.providerCmd(strings.TrimSpace(strings.TrimPrefix(text, "/provider"))))
		return m, nil
	case "/permissiu":
		arg := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(text, "/permissiu")))
		switch arg {
		case "on", "sí", "si", "s":
			m.permissive = true
		case "off", "no", "n":
			m.permissive = false
		case "":
			m.permissive = !m.permissive
		default:
			m.push(systemLine(T("app.usPermissiu")))
			return m, nil
		}
		if m.permissive {
			m.push(warnStyle.Render(T("app.permissiuActiu")) + dimStyle.Render(T("app.denyBloqueja")))
		} else {
			m.push(systemLine(T("app.permissiuOff")))
		}
		return m, nil
	case "/read":
		if len(parts) < 2 {
			m.push(systemLine(T("app.usRead")))
			return m, nil
		}
		off, lim := 1, 0
		if len(parts) > 2 {
			off, _ = strconv.Atoi(parts[2])
		}
		if len(parts) > 3 {
			lim, _ = strconv.Atoi(parts[3])
		}
		// Word/Excel/PowerPoint: lector propi (el Read pla
		// tornaria brossa binària del zip).
		var out string
		var err error
		if tools.OfficeKind(parts[1]) != "" {
			out, err = tools.OfficeRead(workspacePath(m.cwd, parts[1]))
		} else {
			out, err = tools.Read(workspacePath(m.cwd, parts[1]), off, lim)
		}
		if err != nil {
			m.push(systemLine("read: " + err.Error()))
			return m, nil
		}
		m.push(systemLine("llegit " + parts[1]))
		m.push(capText(out, 4000))
		return m, nil
	case "/bash":
		cmd := strings.TrimSpace(strings.TrimPrefix(text, "/bash"))
		if cmd == "" {
			m.push(systemLine(T("app.usBash")))
			return m, nil
		}
		if m.mode == "chat" {
			if class, reason := tools.Classify(cmd); class != "allow" {
				m.push(badStyle.Render(T("app.xatLecturaCurt") + reason + ")"))
				return m, nil
			}
		}
		switch class, reason := tools.Classify(cmd); class {
		case "deny":
			m.push(badStyle.Render("✗ bloquejat: " + reason))
			return m, nil
		case "allow":
			m.push(dimStyle.Render("$ " + cmd))
			m.busy = true
			m.status = T("est.executant")
			return m, func() tea.Msg {
				out, err := tools.BashIn(m.cwd, cmd, tools.DefaultTimeout)
				return bashDoneMsg{cmd: cmd, out: out, err: err}
			}
		default:
			m.confirmIdx = 0
			m.pending = &pendingOp{
				desc: "bash: " + cmd,
				run: func(ctx context.Context) (string, error) {
					return tools.BashCtx(ctx, m.cwd, cmd, tools.DefaultTimeout)
				},
			}
			m.status = T("est.confirmaCurt")
			m.push(warnStyle.Render("$ " + cmd + " — fora de la llista segura. confirma amb s/n"))
			return m, nil
		}
	case "/edit":
		if m.mode == "chat" {
			m.push(systemLine(T("app.xatNomesLectura")))
			return m, nil
		}
		if len(parts) < 3 {
			m.push(systemLine(T("app.usEdit")))
			return m, nil
		}
		start, end, ok := parseRange(parts[2])
		if !ok {
			m.push(systemLine(T("app.rangInvalid")))
			return m, nil
		}
		path := workspacePath(m.cwd, parts[1])
		raw, err := os.ReadFile(path)
		if err != nil {
			m.push(systemLine("edit: " + err.Error()))
			return m, nil
		}
		total := len(strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n"))
		if start < 1 || end < start || end > total {
			m.push(systemLine(fmt.Sprintf(T("app.rangFora"), total)))
			return m, nil
		}
		m.editing = true
		m.editPath = path
		m.editStart, m.editEnd = start, end
		m.editBuf = nil
		m.status = fmt.Sprintf("editant %s %d-%d — /end per acabar", parts[1], start, end)
		m.push(systemLine(fmt.Sprintf(T("app.editantActual"), parts[1], start, end)))
		m.push(dimStyle.Render(capText(blockLines(string(raw), start, end), 1500)))
		m.push(systemLine(T("app.escriuReempl")))
		return m, nil
	case "/write":
		if m.mode == "chat" {
			m.push(systemLine(T("app.xatNomesLectura")))
			return m, nil
		}
		if len(parts) < 2 {
			m.push(systemLine(T("app.usWrite")))
			return m, nil
		}
		m.writing = true
		m.writePath = workspacePath(m.cwd, parts[1])
		m.writeBuf = nil
		m.status = "escrivint " + parts[1] + " — /end per acabar"
		m.push(systemLine("escrivint " + parts[1] + T("app.finsEnd")))
		return m, nil
	case "/verify":
		if len(parts) >= 3 && parts[1] == "mode" {
			switch parts[2] {
			case "off", "manual", "auto", "both", "strict":
				m.cfg.Verify.Mode = parts[2]
				if err := m.cfg.Save(m.cfgPath); err != nil {
					m.push(systemLine("verify.mode: " + parts[2] + T("app.noDesat") + err.Error() + ")"))
				} else {
					m.push(systemLine("verify.mode: " + parts[2] + " (desat)"))
				}
			default:
				m.push(systemLine(T("app.usVerify")))
			}
			return m, nil
		}
		// Interruptor ràpid: on = verifica sol després de feina real,
		// off = res de verificació. Es desa al config com el mode.
		if len(parts) >= 2 && (parts[1] == "on" || parts[1] == "off" || parts[1] == "toggle") {
			mode := parts[1]
			if mode == "toggle" {
				if m.autoVerify() {
					mode = "off"
				} else {
					mode = "on"
				}
			}
			if mode == "on" {
				m.cfg.Verify.Mode = "auto"
			} else {
				m.cfg.Verify.Mode = "off"
			}
			if err := m.cfg.Save(m.cfgPath); err != nil {
				m.push(systemLine("verify: " + mode + T("app.noDesat") + err.Error() + ")"))
			} else {
				clau := "app.verifyDesactivada"
				if mode == "on" {
					clau = "app.verifyActivada"
				}
				m.push(systemLine(T(clau) + " (verify.mode=" + m.cfg.Verify.Mode + ", desat)"))
			}
			return m, nil
		}
		if !m.manualVerify() {
			m.push(systemLine(T("app.verifyOff") + m.cfg.Verify.Mode + ")"))
			return m, nil
		}
		if len(m.convo) == 0 {
			m.push(systemLine("res a verificar encara"))
			return m, nil
		}
		m.busy = true
		m.status = T("est.verificant")
		cmd, cancel := m.sendVerify()
		m.cancel = cancel
		m.cancelRequested = false
		return m, cmd
	case "/save":
		name := strings.TrimSpace(strings.TrimPrefix(text, "/save"))
		if len(m.convo) == 0 {
			m.push(systemLine("res a desar"))
			return m, nil
		}
		cwd, _ := os.Getwd()
		path, err := session.SaveSession(session.DefaultDir(), session.Session{
			Name:   name,
			Role:   m.role,
			Pinned: m.pinned,
			Workspace: func() string {
				if strings.TrimSpace(m.cwd) != "" {
					return m.cwd
				}
				return cwd
			}(),
			Compacted: m.compacted,
			Convo:     m.convo,
			Activity:  timelineToSession(m.timeline),
		})
		if err != nil {
			m.push(systemLine("save: " + err.Error()))
			return m, nil
		}
		m.sessionFile = strings.TrimSuffix(filepath.Base(path), ".json")
		m.push(okStyle.Render("✓ desat: " + path))
		return m, nil
	case "/sessions":
		// Un selector amb fletxes, amb l'última sessió ja triada: nou de
		// cada deu cops el que es vol és reprendre l'última. Sense cap
		// sessió desada, es diu i prou.
		m.sessDir = session.DefaultDir()
		if mn := resumeMenu(); mn != nil {
			mn.title = T("menu.sessions.llista")
			m.menu = mn
			return m, nil
		}
		m.push(llistaSessions(m.sessDir))
		return m, nil
	case "/resume":
		if len(parts) < 2 {
			if mn := resumeMenu(); mn != nil {
				m.menu = mn
			} else {
				m.push(systemLine(T("app.capSessio")))
			}
			return m, nil
		}
		m.resumeTriada(session.DefaultDir(), parts[1])
		return m, nil
	case "/clear":
		m.lines = nil
		m.convo = nil
		m.sessionFile = ""
		m.pinned = false
		m.vp.SetContent("")
		m.push(systemLine("conversa netejada"))
		return m, nil
	default:
		// Ordres personalitzades (~/.config/gregal/commands/*.md,
		// <projecte>/.gregal/commands/*.md): s'expandeixen i entren com
		// a text d'usuari normal (l'historial ja duu la línia /nom).
		if c, args, ok := usercmd.Resolve(m.cwd, text); ok {
			m.push(dimStyle.Render("⟳ /" + c.Name + " (" + c.Source + ")"))
			return m.submitText(usercmd.Expand(c, args))
		}
		m.push(warnStyle.Render("ordre desconeguda: " + parts[0]))
		return m, nil
	}
}

func (m Model) handleKeyCommand(arg string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		if len(m.cfg.Providers) == 0 {
			m.push(systemLine("cap provider configurat (/provider add <nom> <url>)"))
			return m, nil
		}
		m.menu = providerKeyMenu(&m)
		return m, nil
	}
	nom := fields[0]
	if len(fields) == 1 {
		// Si és un provider conegut, demana la clau de forma segura
		if _, ok := m.cfg.Providers[nom]; ok {
			m.pendingKeyFor = nom
			m.push(systemLine("escriu la clau de " + nom + T("prov.senseHistorial")))
			return m, nil
		}
		// Si no és un provider conegut, pot ser directament una clau per al provider actiu
		curProv := m.roleRef().Provider
		if _, ok := m.cfg.Providers[curProv]; ok {
			m.push(m.applyProviderKey(curProv, nom))
			return m, nil
		}
		m.push(systemLine("provider desconegut: " + nom))
		return m, nil
	}
	// /key <provider> <clau>
	if _, ok := m.cfg.Providers[nom]; !ok {
		m.push(systemLine("provider desconegut: " + nom))
		return m, nil
	}
	m.push(m.applyProviderKey(nom, fields[1]))
	return m, nil
}

// inputBox pinta un composer amb intenció explícita: actuar o conversar.
func (m Model) inputBox() string {
	// Dues files: l'entrada a dalt i un peu a baix amb el mode, el model i
	// les tecles. Abans l'etiqueta anava a dalt i repetia la insígnia de
	// mode de la barra amb una descripció («crea · edita · executa») que,
	// a la vint-i-cinquena vegada, ja no diu res a ningú.
	c := boxIdle
	badge := composerChat
	nom := T("composer.xat")
	switch m.mode {
	case "code":
		c, badge, nom = amber, composerCode, T("composer.actua")
	case agent.ModeInspect:
		c, badge, nom = blue, modeInspectStyle, T("composer.consulta")
	case agent.ModeGoal:
		c, badge, nom = lilaFons, composerGoal, T("composer.objectiu")
	case agent.ModeAutonomous:
		c, badge, nom = teal, composerAuto, T("composer.autonomous")
	}
	r := m.rolAgent()
	esquerra := badge.Render("● "+nom) + dimStyle.Render(" · "+r.Provider+"/"+r.Model)
	dreta := faintStyle.Render(T("composer.ajuda"))

	switch {
	case m.histCerca != nil:
		c = escuma
		esquerra, dreta = m.etiquetaCercaHist(), ""
	case m.ajuda:
		esquerra, dreta = faintStyle.Render(T("ajuda.pistes")), ""
	case m.menu != nil:
		c = voraApagada
		esquerra, dreta = faintStyle.Render(T("composer.selector")), ""
	case m.pending != nil:
		c = amber
		esquerra, dreta = warnStyle.Render(T("composer.confirma")), ""
	case m.busy:
		// Amb feina, el peu és l'únic lloc on hi ha moviment: el spinner,
		// què fa, quant fa que hi és i com aturar-ho.
		c = amber
		what := strings.TrimSpace(m.status)
		if what == "" || what == "llest" {
			what = "treballant…"
		}
		esquerra = spinnerStyle.Render(spinnerFrames[m.visualFrame()%len(spinnerFrames)]) + " " + spinnerStyle.Render(what)
		dreta = ""
		if !m.turnStart.IsZero() {
			dreta = faintStyle.Render(elapsedShort(time.Since(m.turnStart)))
		}
		if m.cancel != nil {
			if dreta != "" {
				dreta += "  "
			}
			dreta += faintStyle.Render(T("est.escAtura"))
		}
	}

	cos := m.input.View()
	if m.pendingKeyFor != "" {
		c = amber
		esquerra = warnStyle.Render("◆ "+T("clau.titol")+" "+m.pendingKeyFor) +
			faintStyle.Render("  "+T("clau.pista"))
		dreta = ""
		cos = lipgloss.NewStyle().Foreground(escuma).Bold(true).Render("❯ ") +
			dimStyle.Render(strings.Repeat("•", len([]rune(m.keyBuf))))
	}

	innerW := max(m.vp.Width-4, 10)
	peu := esquerra
	if gap := innerW - lipgloss.Width(esquerra) - lipgloss.Width(dreta); gap > 1 && dreta != "" {
		peu = esquerra + strings.Repeat(" ", gap) + dreta
	} else if dreta != "" {
		peu = esquerra + " " + dreta
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c).
		Background(night).
		// Width és el contingut; la vora hi suma 2. Sense restar-los, el
		// marc sortia dues columnes més ample que la capçalera.
		Width(max(m.vp.Width-2, 10)).
		Padding(0, 1).
		Render(cos + "\n" + peu)
}

func (m Model) headerLine() string {
	project := filepath.Base(filepath.Clean(m.cwd))
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = m.cwd
	}
	// Sense el [45° NE]: la rosa dels vents és de la benvinguda. La
	// capçalera diu on ets i prou.
	logo := Logo()
	right := faintStyle.Render(m.version)
	branch := m.branch
	// El mode no hi és: ja mana al composer (color de la vora i etiqueta)
	// i surt a la barra d'estat; tres vegades a la mateixa pantalla era
	// soroll. La capçalera diu on ets: projecte i branca.
	build := func() string {
		left := logo + dimStyle.Render(" │ ") + projectStyle.Render(project)
		if branch != "" {
			left += dimStyle.Render(" │ ") + branchStyle.Render(branch)
		}
		return left
	}
	left := build()
	// Un projecte o una branca llargs no poden fer que la capçalera desbordi:
	// el terminal l'embolcallaria i tota la pantalla saltaria una fila. Es
	// retallen amb "…" (primer la branca, després el projecte) fins que hi
	// càpiguen amb la versió; si ni així, es talla en dur.
	for lipgloss.Width(left)+lipgloss.Width(right)+1 > m.vp.Width {
		switch {
		case len([]rune(branch)) > 8:
			branch = shorten(branch, len([]rune(branch))-4)
		case len([]rune(project)) > 6:
			project = shorten(project, len([]rune(project))-3)
		default:
			return lipgloss.NewStyle().MaxWidth(m.vp.Width).Render(left)
		}
		left = build()
	}
	gap := m.vp.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}
	// Mar de fons en feina: el buit entre projecte i versió porta aigua
	// que viatja (swellLine fa exactament gap cel·les). Només amb feina
	// en marxa; en repòs, mar plana (espais). És l'únic moviment de la
	// capçalera: el ritme el posa el tick (60 ms amb feina).
	if m.animacions && (m.busy || m.agentActive) && gap >= 6 {
		// Un espai a banda i banda: sense ells la mar sortia enganxada a
		// la branca («git:main~≈≈≋≋») i semblava part del nom.
		return left + " " + waveStyle.Render(swellLine(gap-2, m.visualFrame())) + " " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

// dirSegment torna la carpeta de treball i l'abast de permisos per a la
// barra d'estat. La carpeta és el final del camí (el nom del projecte és
// el que distingeix una sessió d'una altra; «C:/Users/usera/…» no diu
// res) i, si hi cap, el camí sencer. L'abast diu què pot tocar l'agent:
// el projecte, o tot el disc si el permís és permissiu.
//
// disponible és l'espai que queda a la barra un cop hi són el mode, el
// context i l'estat: l'estat mana (durant una aprovació «espera la teva
// aprovació» és el que s'ha de llegir), i la carpeta s'hi adapta. Abans es
// reservaven 24 columnes pensant en «llest», i a 80 columnes l'aprovació
// sortia tallada («◌ espera la t») i el ■ de permisos desapareixia.
func (m Model) dirSegment(disponible int) (string, string) {
	dir := strings.TrimSpace(m.cwd)
	if dir == "" || disponible < 4 {
		return "", ""
	}
	// Els separadors de Windows es mostren com els de la resta.
	dir = strings.ReplaceAll(dir, "\\", "/")
	curt := dir
	if i := strings.LastIndex(strings.TrimRight(dir, "/"), "/"); i >= 0 {
		if base := dir[i+1:]; base != "" {
			curt = base
		}
	}
	// Abast: el projecte és el directori de treball; si el permís deixa
	// escriure a fora, la barra ho ha de cridar.
	abast := T("barra.abastProjecte")
	if m.permissive {
		abast = T("barra.abastTot")
	}
	// De més ric a més pobre: camí sencer amb abast, nom del projecte amb
	// abast, nom sol, i el nom retallat si ni així hi cap.
	// Cada forma porta «  » davant i, amb abast, « » entre tots dos.
	wa := lipgloss.Width(abast)
	if lipgloss.Width(dir)+wa+3 <= disponible {
		return dir, abast
	}
	if lipgloss.Width(curt)+wa+3 <= disponible {
		return curt, abast
	}
	if lipgloss.Width(curt)+2 <= disponible {
		return curt, ""
	}
	r := []rune(curt)
	if n := disponible - 3; n > 0 && n < len(r) {
		return string(r[:n]) + "…", ""
	}
	return "", ""
}

// View pinta capçalera, historial, entrada i barra d'estat.
func (m Model) View() string {
	if !m.ready {
		return "carregant gregal…"
	}
	suggHL := dimStyle
	// Quatre segments i prou: mode, context, estat i permisos. Els
	// comptadors ↑↓ i el cost són a /stats; el checklist, la cua i la
	// verificació al cockpit (Ctrl+T); les tecles a ?. Abans hi havia nou
	// senyals i tres jocs de pistes competint per una fila.
	ctxSeg := ""
	ctxStyle := okStyle
	if w := m.finestraAgent(); w > 0 {
		pct := m.promptEst * 100 / w
		ctxSeg = fmt.Sprintf("ctx %d%%", pct)
		if m.animacions && m.vp.Width >= 96 {
			ctxSeg += " " + tideGauge(pct, 8, m.visualFrame())
		}
		switch {
		case pct >= 80:
			ctxStyle = badStyle
			suggHL = warnStyle
		case pct >= 50:
			ctxStyle = warnStyle
		}
	}
	estat := m.status
	estatStyle := okStyle
	mascota := mascotFrames[0]
	if m.animacions {
		mascota = mascotFrames[m.visualFrame()%len(mascotFrames)]
	}
	switch {
	case m.pendingQ != nil:
		estat = "? " + T("est.triaOpcio")
		estatStyle = warnStyle
	case m.pending != nil:
		// Curt: les tecles ja són al diàleg.
		estat = "◌ " + T("est.aprovacio")
		estatStyle = warnStyle
	case m.agentActive:
		estat = fmt.Sprintf("◈ agent %d/%d", m.passosAgent(), m.limitAgent())
		estatStyle = warnStyle
	case m.busy:
		estat = "◈ " + m.status
		estatStyle = warnStyle
	}
	estat = mascota + " " + estat
	// Quadrat ple = cada eina delicada demana permís; buit = permissiu.
	permisos := dimStyle.Render("■")
	if m.permissive {
		permisos = warnStyle.Render("□")
	}
	rightBar := estatStyle.Render(estat) + "  " + permisos
	if !m.vp.AtBottom() {
		rightBar = projectStyle.Render(fmt.Sprintf("↕ %d%%", int(m.vp.ScrollPercent()*100))) + "  " + rightBar
	}
	ctxPart := ""
	if ctxSeg != "" {
		ctxPart = "  " + ctxStyle.Render(ctxSeg)
	}
	// La carpeta és l'única cosa de la pantalla que diu on treballa
	// l'agent. Si la barra va ampla, el camí sencer; si no, el final del
	// camí (la part que distingeix un projecte d'un altre) i l'abast. Però
	// s'adapta a l'espai que deixen el mode, el context i l'estat (amb una
	// columna de marge entre esquerra i dreta), mai al revés.
	leftBar := modeBadge(m.mode)
	disponible := m.vp.Width - lipgloss.Width(leftBar) - lipgloss.Width(ctxPart) - lipgloss.Width(rightBar) - 1
	dirSeg, abastSeg := m.dirSegment(disponible)
	if dirSeg != "" {
		leftBar += "  " + dirStyle.Render(dirSeg)
		if abastSeg != "" {
			leftBar += " " + dimStyle.Render(abastSeg)
		}
	}
	leftBar += ctxPart
	gap := m.vp.Width - lipgloss.Width(leftBar) - lipgloss.Width(rightBar)
	bar := leftBar + " " + rightBar
	if gap > 0 {
		bar = leftBar + strings.Repeat(" ", gap) + rightBar
	} else {
		bar = lipgloss.NewStyle().MaxWidth(m.vp.Width).Render(bar)
	}
	sugg := ""
	if list := m.suggestions(); len(list) > 0 {
		sel := m.suggIdx
		if sel < 0 {
			sel = 0
		}
		if sel >= len(list) {
			sel = len(list) - 1
		}
		rows := make([]string, 0, maxSugg)
		for i, c := range list {
			row := "  /" + c.name + " — " + c.desc
			if i == sel {
				row = suggHL.MaxWidth(m.vp.Width).Render("▸ /" + c.name + " — " + c.desc)
			} else {
				row = dimStyle.MaxWidth(m.vp.Width).Render(row)
			}
			rows = append(rows, row)
		}
		for len(rows) < maxSugg {
			rows = append(rows, "")
		}
		sugg = strings.Join(rows, "\n") + "\n"
	}
	// El selector, l'aprovació, la pregunta, la cerca i l'ajuda ja no
	// s'encabeixen entre la conversa i el composer: se superposen a la
	// conversa (vegeu overlay.go). El viewport no cedeix cap fila i el text
	// no salta en obrir-los ni en tancar-los.
	vp := m.vp
	todos := ""
	if m.showTodos && !m.columnaActiva() {
		todos = m.cockpitBox(m.vp.Width) + "\n"
	}
	timeline := ""
	if m.timelineOpen {
		timeline = m.timelineBox(m.vp.Width) + "\n"
	}
	queue := ""
	if len(m.queued) > 0 && !m.showTodos {
		queue = m.queueBox(m.vp.Width) + "\n"
	}
	// filesVisuals: files reals que ocupa un bloc ja renderitzat.
	// Comptar "\n" no n'hi ha prou: els blocs amb Width (confirmació,
	// pregunta, todos, suggeriments llargs) i el composer s'embolcalla i
	// ocupen més files visuals de les que diuen els salts. Si el viewport
	// no cedeix exactament aquestes, el total supera l'alçada del terminal
	// i el terminal fa scroll: tot el text anterior balla (amaga i mostra
	// blocs com el veredicte) amb cada tecla.
	filesVisuals := func(block string) int {
		if block == "" {
			return 0
		}
		return lipgloss.Height(strings.TrimSuffix(block, "\n"))
	}
	ib := m.inputBox()
	vp.Height -= filesVisuals(sugg) + filesVisuals(todos) + filesVisuals(timeline) + filesVisuals(queue)
	// El composer creix en escriure (multilínia o línies llargues que
	// s'embolcalla): el viewport ha de cedir l'excés sobre la base 4 que
	// ja descompta vpSize, o el total desborda i tot salta.
	if exc := lipgloss.Height(ib) - 4; exc > 0 {
		vp.Height -= exc
	}
	if vp.Height < 3 {
		vp.Height = 3
	}
	// Encongir el viewport (obrir un popup) no pot moure la lectura. Abans
	// qualsevol canvi d'alçada forçava GotoBottom, o sigui que amb
	// l'autocomplete obert era impossible quedar-se amunt: cada tecla et
	// tornava al final. Només seguim el final si l'usuari ja hi era.
	if m.vp.AtBottom() {
		vp.GotoBottom()
	}
	conversa := vp.View()
	if caixa := m.dialegObert(); caixa != "" {
		conversa = superposa(conversa, caixa, m.vp.Width)
	}
	out := fmt.Sprintf("%s\n%s\n\n%s\n\n%s%s%s%s%s\n%s",
		m.headerLine(),
		waveLine(m.vp.Width),
		conversa,
		sugg,
		todos,
		timeline,
		queue,
		ib,
		bar,
	)
	// La columna del cockpit va a la dreta de tot, de dalt a baix: la
	// vista de l'esquerra ja fa `ample − 34` i això hi afegeix les 34.
	if m.columnaActiva() {
		out = lipgloss.JoinHorizontal(lipgloss.Top, out, m.cockpitColumna(ampleColumna, lipgloss.Height(out)))
	}
	if m.asciiMode {
		return asciiView(out)
	}
	return out
}

// contingut és el text del viewport: la conversa més, si s'ha retallat,
// una línia que ho diu. L'avís es pinta aquí i no es desa a m.lines perquè
// així els índexs de les línies (m.streamLine) no s'han de quadrar.
func (m *Model) contingut() string {
	cos := strings.Join(m.lines, "\n")
	if m.retallades == 0 {
		return cos
	}
	avis := dimStyle.Render(fmt.Sprintf(T("app.retallades"), m.retallades))
	if cos == "" {
		return avis
	}
	return avis + "\n" + cos
}

// treuBenvinguda esborra la targeta d'inici i la pista de sessions quan
// arriba el primer missatge. Ocupaven tretze de les trenta-quatre files
// d'una pantalla normal i no marxaven mai: la conversa començava a la
// meitat de baix i s'hi quedava tota la sessió.
func (m *Model) treuBenvinguda() {
	if m.benvingudaN <= 0 || len(m.lines) < m.benvingudaN {
		m.benvingudaN = 0
		return
	}
	m.lines = append([]string{}, m.lines[m.benvingudaN:]...)
	if m.streamLine -= m.benvingudaN; m.streamLine < 0 {
		m.streamLine = 0
	}
	m.benvingudaN = 0
	m.refresh()
}
