package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/runs"
	"gregal/internal/session"
)

// Logf és el logger del bot (mai ha d'incloure el token).
type Logf func(format string, args ...any)

// Options és la configuració del bot.
type Options struct {
	Cfg          *config.Config
	CfgPath      string
	Cwd          string
	Token        string
	AllowedUsers []int64
	AllowGroups  bool
	SessionsDir  string
	GoalsDir     string
	Log          Logf
}

// chatState és l'estat d'una conversa de Telegram.
type chatState struct {
	mu         sync.Mutex
	name       string // nom de la sessió (al mateix magatzem que el TUI)
	role       string
	rolePinned bool // cert si l'usuari ha fixat el rol (/role)
	mode       string
	convo      []llm.Message
	// user és el compte gregal lligat ("" = convidat: projecte global).
	// dir és la seva carpeta de treball i pol la seva política (ProjectDir
	// propi: escriure fora de casa demana permís). Es resolen per missatge
	// perquè en un grup hi pot haver més d'un usuari.
	user   string
	dir    string
	pol    *agent.Policy
	busy   bool
	cancel context.CancelFunc
	// runID identifica el torn actiu de la cua; pendingRuns permet que /stop
	// cancel·li també una petició que encara no ha començat.
	runID       int64
	pendingRuns []int64
	created     time.Time
}

// Bot és el bot de Telegram de Gregal.
type Bot struct {
	cfg     *config.Config
	cfgPath string
	cwd     string
	api     *API
	client  *llm.Client
	policy  *agent.Policy
	queue   *runs.Queue
	logf    Logf
	// Ritme d\'edicio del missatge de resposta (vegeu limitador d\'edicions).
	editMu   sync.Mutex
	lastEdit map[int64]time.Time

	allowed     map[int64]bool
	allowGroups bool
	// tgUsers lliga id de Telegram → compte gregal (de users.*.telegram_id).
	tgUsers map[int64]string
	// unattended diu qui pot entrar al mode autònom (eines sense aprovació).
	// L'amo hi és per defecte; un usuari lligat només si ho demana al config.
	unattended map[int64]bool
	// pols desa una política per usuari (ProjectDir = la seva casa).
	polMu       sync.Mutex
	pols        map[string]*agent.Policy
	sessionsDir string
	goalsDir    string

	mu        sync.Mutex
	chats     map[int64]*chatState
	approve   map[string]approveReq
	remembers *agent.Remember
	apSeq     int
	// pendingPlans guarda el pla proposat per xat (/plan → Executa/Descarta).
	pendingPlans map[int64]string

	meOnce sync.Once
	meUser string

	// Models en viu + overrides per xat+rol (models.go).
	modelsMu    sync.Mutex
	modelsCache []ModelRef
	modelsAt    time.Time
	modelsOv    map[string]map[string]string // xat → rol → "provider/model"
	modelsFile  string                       // buit = telegram-models.json al costat de sessions/
}

// New crea el bot.
func New(o Options) *Bot {
	logf := o.Log
	if logf == nil {
		logf = func(string, ...any) {}
	}
	allowed := map[int64]bool{}
	for _, id := range o.AllowedUsers {
		allowed[id] = true
	}
	tgUsers := map[int64]string{}
	unattended := map[int64]bool{}
	if o.Cfg != nil {
		for name, u := range o.Cfg.Users {
			if u.TelegramID != 0 {
				tgUsers[u.TelegramID] = name
				// El mode autònom va sense aprovacions: només l'amo (o qui
				// el config marca explícitament) hi pot entrar.
				unattended[u.TelegramID] = u.Unattended || allowed[u.TelegramID]
			}
		}
	}
	api := NewAPI(o.Token)
	if o.Cfg != nil {
		api.language = o.Cfg.Lang()
	}
	b := &Bot{
		cfg:         o.Cfg,
		cfgPath:     o.CfgPath,
		cwd:         o.Cwd,
		api:         api,
		client:      llm.New(),
		policy:      policyFrom(o.Cfg, o.Cwd),
		queue:       runs.New(),
		logf:        logf,
		allowed:     allowed,
		allowGroups: o.AllowGroups,
		tgUsers:     tgUsers,
		unattended:  unattended,
		sessionsDir: o.SessionsDir,
		goalsDir:    o.GoalsDir,
		chats:       map[int64]*chatState{},
		approve:     map[string]approveReq{},
		remembers:   agent.NewRemember(),
	}
	if o.Cfg != nil {
		o.Cfg.ConfigureClient(b.client)
	}
	b.loadModelsOv()
	agent.SetupDelegate(o.Cfg, b.client)
	return b
}

// policyFrom llegeix la política de permisos del config.
func policyFrom(cfg *config.Config, cwd string) *agent.Policy {
	p := agent.DefaultPolicy()
	p.ProjectDir = cwd
	if cfg == nil {
		return p
	}
	p.Tools = cfg.Permissions.Tools
	p.BashAllow = cfg.Permissions.BashAllow
	p.BashDeny = cfg.Permissions.BashDeny
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetupPrices(cfg)
	return p
}

// API exposa el client (per proves i comprovacions de token).
func (b *Bot) API() *API { return b.api }

// tr selects Catalan or English for user-facing Telegram text. A nil config
// follows the documented English default.
func (b *Bot) tr(ca, en string) string {
	if b != nil && b.cfg != nil && b.cfg.Lang() == "ca" {
		return ca
	}
	return en
}

// identity resol qui ets a partir de l'id de Telegram: compte gregal,
// carpeta de treball i política. Sense lligam, el de sempre (projecte
// global, política global): ningú perd accés pel fet d'afegir el mapa.
func (b *Bot) identity(fromID int64) (user, dir string, pol *agent.Policy) {
	if name, ok := b.tgUsers[fromID]; ok {
		if u, ok := b.cfg.Users[name]; ok {
			home := u.Home
			if home == "" && len(u.Roots) > 0 {
				home = u.Roots[0]
			}
			if home == "" {
				home = b.cwd
			}
			return name, home, b.policyFor(name, home)
		}
	}
	return "", b.cwd, b.policy
}

// policyFor desa una política per usuari: igual que la global però amb
// ProjectDir a casa seva (escriure-hi dins en mode codi passa sol;
// fora, demana permís).
func (b *Bot) policyFor(user, home string) *agent.Policy {
	b.polMu.Lock()
	defer b.polMu.Unlock()
	if p, ok := b.pols[user]; ok {
		return p
	}
	p := policyFrom(b.cfg, home)
	if b.pols == nil {
		b.pols = map[string]*agent.Policy{}
	}
	b.pols[user] = p
	return p
}

// dirOf i polOf són la xarxa de seguretat: un estat construït a mà (proves)
// no té identitat i cau al projecte global, com abans.
func (b *Bot) dirOf(st *chatState) string {
	if st != nil && st.dir != "" {
		return st.dir
	}
	return b.cwd
}

func (b *Bot) polOf(st *chatState) *agent.Policy {
	if st != nil && st.pol != nil {
		return st.pol
	}
	return b.policy
}

// Run fa el bucle de long polling fins que el context es cancel·la.
func (b *Bot) Run(ctx context.Context) error {
	me, err := b.api.Me(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", b.tr("token rebutjat per Telegram", "Telegram rejected the bot token"), err)
	}
	b.logf("bot @%s (%d) en marxa · projecte %s · grups: %v", me.Username, me.ID, b.cwd, b.allowGroups)
	b.meUser = me.Username

	// Avisos de jobs programats (procés separat del servidor: via disc).
	go b.watchJobs(ctx)

	// Menú d'ordres al client (best effort: si falla, /help continua valent).
	if err := b.api.SetMyCommands(ctx, b.botMenu()); err != nil {
		b.logf("setMyCommands: %v", err)
	}

	var offset int64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ups, err := b.api.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			b.logf("getUpdates: %v", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range ups {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			switch {
			case u.Message != nil:
				// Cada update en una goroutine: si no, un torn llarg bloquejaria
				// el bucle i els botons d'aprovació no arribarien mai.
				go b.handleMessage(ctx, u.Message)
			case u.EditedMessage != nil:
				// Un missatge editat es tracta com de nou (p. ex. correcció
				// d'una ordre mal escrita).
				go b.handleMessage(ctx, u.EditedMessage)
			case u.CallbackQuery != nil:
				go b.handleCallback(ctx, u.CallbackQuery)
			}
		}
	}
}

// state retorna (o crea) l'estat de la conversa.
func (b *Bot) state(chatID int64) *chatState {
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.chats[chatID]
	if !ok {
		st = &chatState{
			name:    "tg-" + strconv.FormatInt(chatID, 10),
			role:    "chat",
			mode:    b.modeInicial(chatID),
			created: time.Now(),
		}
		if st.mode == "" {
			st.mode = agent.ModeCode
		}
		if s, err := session.Load(b.sessionsDir, st.name); err == nil {
			st.convo = s.Convo
			if s.Role != "" {
				st.role = s.Role
			}
		}
		b.chats[chatID] = st
	}
	return st
}

// send envia text pla pel xat (parteix les respostes llargues).
func (b *Bot) send(ctx context.Context, chatID int64, text string) {
	b.logReply(chatID, text)
	for _, chunk := range splitMessage(text, 3900) {
		if _, err := b.api.SendMessage(ctx, chatID, chunk, nil); err != nil {
			b.logf("sendMessage: %v", err)
			return
		}
	}
}

// splitMessage parteix un text en trossos respectant salts de línia.
func splitMessage(text string, max int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{"(buit)"}
	}
	if len(text) <= max {
		return []string{text}
	}
	var out []string
	for len(text) > max {
		cut := strings.LastIndex(text[:max], "\n")
		if cut < max/2 {
			cut = max
		}
		out = append(out, text[:cut])
		text = strings.TrimLeft(text[cut:], "\n")
	}
	if strings.TrimSpace(text) != "" {
		out = append(out, text)
	}
	return out
}

// authorized diu si l'usuari pot fer servir el bot en aquest xat.
func (b *Bot) authorized(u *User, chat Chat) bool {
	if u == nil {
		return false
	}
	// Lligat a un compte: entra (als grups, només si estan permesos).
	if _, ok := b.tgUsers[u.ID]; ok {
		return chat.Type == "private" || b.allowGroups
	}
	if len(b.allowed) == 0 {
		return chat.Type == "private"
	}
	if !b.allowed[u.ID] {
		return false
	}
	if chat.Type != "private" && !b.allowGroups {
		return false
	}
	return true
}

// logReply registra la resposta enviada (per diagnòstics).
func (b *Bot) logReply(chatID int64, text string) {
	b.logf("respost a %d: %q", chatID, truncate(strings.ReplaceAll(text, "\n", " "), 90))
}

// canApprove diu si l'usuari pot aprovar eines (només els autoritzats).
func (b *Bot) canApprove(u *User) bool {
	if u == nil {
		return false
	}
	if len(b.allowed) == 0 {
		return false
	}
	return b.allowed[u.ID]
}

// mayRunUnattended diu si l'usuari pot activar el mode autònom, que fa anar
// les eines sense cap diàleg. L'amo hi té dret; un usuari familiar lligat,
// només si ho demana al config (users.<nom>.unattended). El camí de text ho
// ha de comprovar explícitament: els botons ja passen per canApprove.
func (b *Bot) mayRunUnattended(userID int64) bool {
	if v, ok := b.unattended[userID]; ok {
		return v
	}
	// Sense compte lligat al config: només l'amo és a allowed_users.
	return b.allowed[userID]
}

// modeInicial diu amb quin mode neix un xat nou. El mode del config val per a
// tothom, amb una excepció: l'autònom és un permís i no s'hereta. Si el config
// el té desat (d'una versió anterior o d'un descuit), qui no hi tingui dret
// comença en codi. En un grup, chatID no és cap persona i per tant tampoc no
// s'hereta: allà el mode autònom s'ha de triar a posta.
func (b *Bot) modeInicial(chatID int64) string {
	m := b.cfg.Mode
	if m == agent.ModeAutonomous && !b.mayRunUnattended(chatID) {
		return agent.ModeCode
	}
	return m
}

func (b *Bot) handleMessage(ctx context.Context, m *Message) {
	// Cada missatge va en goroutine pròpia: un pànic no pot tombar el bot.
	defer func() {
		if r := recover(); r != nil {
			b.logf("pànic recuperat al torn: %v", r)
			if m != nil {
				b.send(ctx, m.Chat.ID, "⚠️ "+b.tr("El torn s'ha interromput per un error intern. Pots tornar-ho a provar.", "The turn stopped because of an internal error. Please try again."))
			}
		}
	}()
	if m.From == nil {
		return
	}
	if !b.authorized(m.From, m.Chat) {
		b.logf("ignorat: usuari %d (%s) al xat %d (%s)", m.From.ID, m.From.Username, m.Chat.ID, m.Chat.Type)
		if m.Chat.Type == "private" {
			b.send(ctx, m.Chat.ID, fmt.Sprintf("🔒 %s\n%s %d: %s", b.tr("No autoritzat.", "Not authorized."), b.tr("El teu id és", "Your ID is"), m.From.ID, b.tr("afegeix-lo a `telegram.allowed_users` del config de Gregal.", "add it to `telegram.allowed_users` in the Gregal config.")))
		}
		return
	}
	text := strings.TrimSpace(m.Text)
	st := b.state(m.Chat.ID)
	// Identitat per missatge: en un grup hi pot haver més d'un usuari i el
	// torn ha de treballar a casa de qui parla, no a la de l'anterior.
	if user, dir, pol := b.identity(m.From.ID); st.user != user {
		b.logf("xat %d: identitat %s (%s)", m.Chat.ID, user, dir)
		st.user, st.dir, st.pol = user, dir, pol
	}
	if st.dir == "" {
		_, st.dir, st.pol = b.identity(m.From.ID)
	}
	// Fotos: es descarreguen (la més gran) i van al torn com a imatge.
	// Les notes de veu es detecten però no es transcriuen (1.0: sense STT).
	if len(m.Photo) > 0 {
		b.handlePhoto(ctx, m, st, text)
		return
	}
	if m.Document != nil || m.Audio != nil {
		b.handleDocument(ctx, m, st, text)
		return
	}
	if m.Voice != nil {
		b.send(ctx, m.Chat.ID, "🎙️ "+b.tr("Les notes de veu encara no estan suportades: escriu-m'ho o envia una foto.", "Voice messages are not supported yet. Send text or a photo instead."))
		return
	}
	if text == "" {
		return
	}
	b.logf("rebut: usuari %d xat %d (%s) %q", m.From.ID, m.Chat.ID, m.Chat.Type, truncate(text, 70))
	if strings.HasPrefix(text, "/") {
		b.command(ctx, m, st, text)
		return
	}
	// En grups, el text pla només s'atén si respon al bot o el menciona
	// (Telegram no envia la resta de missatges si el mode privacitat és actiu).
	if m.Chat.Type != "private" && !b.addressed(m) {
		return
	}
	b.turn(ctx, m.Chat.ID, st, text)
}

// myUsername retorna el nom del bot, amb cau (una sola crida a getMe).
func (b *Bot) myUsername(ctx context.Context) string {
	b.meOnce.Do(func() {
		if me, err := b.api.Me(ctx); err == nil {
			b.meUser = me.Username
		}
	})
	return b.meUser
}

// addressed diu si el missatge va dirigit al bot.
func (b *Bot) addressed(m *Message) bool {
	if m.Text == "" {
		return false
	}
	if me := b.myUsername(context.Background()); me != "" {
		low := strings.ToLower(m.Text)
		if strings.Contains(low, "@"+strings.ToLower(me)) {
			return true
		}
		// Resposta directa a un missatge del bot.
		if m.ReplyTo != nil && m.ReplyTo.From != nil &&
			strings.EqualFold(m.ReplyTo.From.Username, me) {
			return true
		}
	}
	// Respostes al bot: el text comença pel que havíem dit nosaltres.
	for _, prefix := range []string{"≋", "◆", "⚠️", "🔒", "📊", "🗂"} {
		if strings.HasPrefix(strings.TrimSpace(m.Text), prefix) {
			return true
		}
	}
	return false
}

// approveReq és una aprovació pendent: canal + signatura + àmbit per al
// "sempre en aquesta sessió" (àmbit = xat).
type approveReq struct {
	ch    chan approveRes
	sig   string
	scope string
}

// approveRes distingeix el permís puntual del recordat (rebut diferent).
type approveRes struct {
	ok     bool
	always bool
}

// tgScope aïlla els records per xat.
func tgScope(chatID int64) string { return fmt.Sprintf("tg:%d", chatID) }

// askApproval envia els botons i espera la decisió (timeout = denegat).
// Amb "Sempre" la decisió es recorda per l'àmbit (sessió del xat).
func (b *Bot) askApproval(ctx context.Context, chatID int64, name, args, sig string) bool {
	b.mu.Lock()
	b.apSeq++
	id := strconv.Itoa(b.apSeq)
	ch := make(chan approveRes, 1)
	b.approve[id] = approveReq{ch: ch, sig: sig, scope: tgScope(chatID)}
	b.mu.Unlock()

	kb := Keyboard{{{Text: "✅ " + b.tr("Permet", "Allow"), Data: "ap:" + id + ":ok"}, {Text: "🧠 " + b.tr("Sempre", "Always"), Data: "ap:" + id + ":always"}, {Text: "⛔️ " + b.tr("Denega", "Deny"), Data: "ap:" + id + ":no"}}}
	msg, err := b.api.SendMessage(ctx, chatID,
		"🔐 "+b.tr("Permís", "Permission")+" · <b>"+esc(name)+"</b>\n"+esc(truncate(args, 900)), kb)
	if err != nil {
		b.logf("sendMessage (permís): %v", err)
		b.mu.Lock()
		delete(b.approve, id)
		b.mu.Unlock()
		return false
	}
	select {
	case res := <-ch:
		resultat := "⛔️ " + b.tr("denegat", "denied")
		if res.ok {
			resultat = "✅ " + b.tr("permès", "allowed")
			if res.always {
				resultat = "🧠 " + b.tr("permès sempre (sessió)", "always allowed (session)")
			}
		}
		// Rebut compacte (una línia): el missatge amb botons no ha d'enterrar
		// la resposta final del xat.
		_ = b.api.EditMessageText(ctx, chatID, msg.MessageID,
			"🔐 <b>"+esc(name)+"</b> · "+resultat+" · <code>"+esc(truncate(args, 80))+"</code>", nil)
		return res.ok
	case <-time.After(180 * time.Second):
		b.mu.Lock()
		delete(b.approve, id)
		b.mu.Unlock()
		_ = b.api.EditMessageText(ctx, chatID, msg.MessageID,
			"🔐 <b>"+esc(name)+"</b> · ⌛️ "+b.tr("caducat (denegat)", "expired (denied)")+" · <code>"+esc(truncate(args, 80))+"</code>", nil)
		return false
	case <-ctx.Done():
		return false
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// goalDirSafe retorna el directori d'objectius.
func (b *Bot) goalDirSafe() string {
	if b.goalsDir != "" {
		return b.goalsDir
	}
	return "."
}

// projectName és el nom del projecte actual.
func (b *Bot) projectName() string {
	parts := strings.Split(strings.TrimRight(b.cwd, "/"), "/")
	if len(parts) == 0 {
		return b.cwd
	}
	return parts[len(parts)-1]
}
