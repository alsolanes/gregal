// Package web és la webapp local del gregal (`gregal serve`): xat amb
// streaming + agent amb aprovacions, mateix motor que el TUI.
package web

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/jobs"
	"gregal/internal/llm"
	"gregal/internal/mcp"
	"gregal/internal/runs"
	"gregal/internal/session"
	"gregal/internal/tools"
	"gregal/internal/verify"
)

// Server guarda l'estat (una sessió; un agent a la vegada).
type liveEvent struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type activeAgent struct {
	ID        int         `json:"id"`
	Task      string      `json:"task"`
	StartedAt time.Time   `json:"started_at"`
	Role      string      `json:"role"`
	Mode      string      `json:"mode"`
	Project   string      `json:"project"`
	Events    []liveEvent `json:"events"`
}

type Server struct {
	cfg     *config.Config
	cfgPath string
	client  *llm.Client
	policy  *agent.Policy
	cwd     string
	goalDir string

	mu    sync.Mutex
	convo []llm.Message
	// compacted és el resum de la conversa ja compactada: viu al system
	// prompt i la convo només duu el tram recent. El TUI ho feia des del
	// principi; la finestra no, i era la finestra la que s'hi passa les
	// hores. Una sessió llarga hi creixia sense límit fins que el model
	// tallava el context pel seu compte, que és perdre'l sense avisar.
	compacted string
	// convoFile és el fitxer d'aquesta conversa al magatzem de sessions.
	// Es fixa al primer desat automàtic i no canvia més: així cada torn
	// reescriu la mateixa conversa en comptes d'anar deixant còpies, i no
	// es perd res encara que tanquis l'app sense clicar «Nova sessió».
	convoFile     string
	pinned        bool
	role          string
	rolePinned    bool    // cert si l'usuari ha fixat el rol (router apagat)
	gated         bool    // strict: CAL REVISAR pendent d'aprovar (C2b)
	streak        int     // CAL REVISAR seguits (escalat C2)
	sessCost      float64 // USD acumulats de la sessió (pressupost C2)
	budgetWarned  bool
	mode          string
	modelOverride map[string]string // rol → "provider/model"
	// unavailableModels registra les seleccions que handleModels ha comprovat
	// que ja no anuncia el provider. La preferència es conserva però el runtime
	// usa el model base del rol fins que torni a estar disponible.
	unavailableModels map[string]string
	modelCheckedAt    map[string]time.Time
	lastGoalID        string // últim objectiu creat o executat

	agentBusy bool
	active    *activeAgent
	activeSeq int
	tokenBuf  string
	tokenAt   time.Time
	approvals map[string]approvalReq
	apSeq     int
	// questions pendents de resposta triable (eina question): clau → canal
	// amb l'opció triada (label) o text lliure amb prefix "text:".
	questions map[string]questionReq
	// remembers guarda "sempre en aquesta sessió" (àmbit "web"); es neteja
	// en canviar de sessió i en reiniciar.
	remembers *agent.Remember
	// permissive: mode permissiu (l'agent tira sense demanar; deny
	// continua bloquejant). Només sessió: es perd en reiniciar.
	// Es llegeix/escriu amb s.mu.
	permissive bool
	// jobs programa butlletins i recerques desateses (nil = desactivats).
	jobs *jobs.Store
	mcp  *mcp.Manager
	// jobRunning marca una execució programada en curs (el worker és únic).
	jobRunning bool
	// planRunning marca una exploració /api/plan en curs (el worker és únic).
	planRunning bool
	// flowRunning marca un graf en execució: comparteix treballador amb
	// l'agent i amb els jobs perquè tots tres escriuen al mateix disc.
	flowRunning bool
	// todo és la checklist d'AQUESTA sessió (todowrite/todoread). Global no
	// pot ser: amb dues pestanyes se la trepitjaven.
	todo []tools.TodoItem

	// hub, id, title i createdAt identifiquen la sessió dins del Hub
	// (G1). Amb un sol client i sense id, tot això és la sessió "default"
	// i el comportament és el de sempre.
	hub       *Hub
	id        string
	title     string
	createdAt time.Time
	// activeRun és l'ID del torn en execució a la cua compartida
	// (POST /api/agent/cancel i POST /api/v2/runs/{id}/cancel). Es llegeix
	// i s'escriu amb s.mu. 0 = cap torn en marxa.
	activeRun int64
	// runCancel és el mecanisme legacy de cancel·lació, encara usat pel
	// bucle de mode goal, que no passa per la cua.
	runCancel context.CancelFunc

	// Token buit = sense auth (ús local). Amb token, /api/* exigeix
	// Authorization: Bearer *** per exposar-ho a la LAN/mòbil).
	Token string

	// users valida contrasenyes i resol tokens a usuaris; user és qui ha
	// fet la petició en curs (nil en mode local, sense autenticació).
	// Conviuen amb Token: el token únic de sempre val com a admin i tot
	// queda com abans d'haver afegit usuaris.
	users *Users
	user  *User
	// instanceID identifica aquesta instància del servei per als clients.
	// No és un secret: només serveix per detectar que el port apunta a un
	// procés diferent després d'un reinici o un canvi de perfil.
	instanceID string
}

// APIProtocol és el contracte de capacitats que comparteixen els clients.
// Les rutes antigues continuen existint; aquest valor permet rebutjar un
// backend massa antic abans de començar una sessió compartida.
const APIProtocol = "gregal.v1"

func New(cfg *config.Config, cfgPath string) *Server {
	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "."
	}
	// Si el disc falla, jobs queda a nil i l'API ho diu (no tomba el servidor).
	js, _ := jobs.New("")
	cl := llm.New()
	cfg.ConfigureClient(cl)
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetupPrices(cfg)
	agent.SetupDelegate(cfg, cl)
	s := &Server{
		cfg:               cfg,
		cfgPath:           cfgPath,
		goalDir:           goalDirFor(cfgPath),
		client:            cl,
		policy:            &agent.Policy{Tools: cfg.Permissions.Tools, BashAllow: cfg.Permissions.BashAllow, BashDeny: cfg.Permissions.BashDeny, ProjectDir: cwd},
		remembers:         agent.NewRemember(),
		jobs:              js,
		mcp:               currentMCP(),
		cwd:               cwd,
		role:              cfg.InitialRole(),
		mode:              cfg.Mode,
		modelOverride:     map[string]string{},
		unavailableModels: map[string]string{},
		modelCheckedAt:    map[string]time.Time{},
		approvals:         map[string]approvalReq{},
		questions:         map[string]questionReq{},
		users:             NewUsers(cfg, cfgPath),
		instanceID:        newInstanceID(),
	}
	s.modelOverride = s.defaultModelOverrides()
	return s
}

func newInstanceID() string {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err == nil {
		return "inst-" + fmt.Sprintf("%x", b)
	}
	return fmt.Sprintf("inst-%x", time.Now().UnixNano())
}

// SetToken fixa el token únic de tota la vida i el fa valdre com a admin
// dels usuaris: qui l'ha fet servir fins ara no ha de canviar res.
func (s *Server) SetToken(tok string) {
	s.Token = tok
	if s.users != nil {
		s.users.SetLegacyToken(tok)
	}
}

// InstanceID és la identitat estable durant la vida d'aquest procés.
func (s *Server) InstanceID() string { return s.instanceID }

func (s *Server) roleRef() (config.Provider, config.Role) {
	return s.roleRefFor(s.role)
}

func (s *Server) roleRefFor(name string) (config.Provider, config.Role) {
	r := s.cfg.Roles[name]
	if ov, ok := s.modelOverride[name]; ok && s.unavailableModels[name] != ov {
		if p, m, ok := splitModel(ov); ok {
			if prov, ok := s.cfg.Providers[p]; ok {
				r.Provider, r.Model = p, m
				return prov, r
			}
		}
		if p, ok := s.cfg.Providers[r.Provider]; ok {
			r.Model = ov
			return p, r
		}
	}
	return s.cfg.Providers[r.Provider], r
}

func splitModel(ov string) (string, string, bool) {
	if i := strings.Index(ov, "/"); i > 0 {
		return ov[:i], ov[i+1:], true
	}
	return "", "", false
}

// --- HTTP ---

// Run arrenca l'HTTP a addr (p. ex. "127.0.0.1:8090") amb aquest servidor
// com a sessió default del Hub.
func (s *Server) Run(addr string) error { return s.Hub().Run(addr) }

// Run arrenca l'HTTP del Hub: les rutes de conversa es reparteixen per
// sessió i les d'aplicació (providers, models, jobs, office) van sempre a
// la default.
func (h *Hub) Run(addr string) error {
	s := h.def
	if err := validateListenAuth(addr, s.Token != "" || (s.users != nil && s.users.Actiu())); err != nil {
		return err
	}
	go s.jobLoop(context.Background())
	return http.ListenAndServe(addr, h.Handler())
}

// Handler is the same authenticated API and UI used by every HTTP client.
func (s *Server) Handler() http.Handler { return s.Hub().Handler() }

func (h *Hub) Handler() http.Handler {
	s := h.def
	mux := h.mux()
	var handler http.Handler = mux
	// Amb token únic o amb usuaris, /api/* queda tancat: la diferència és
	// que amb usuaris cadascú entra amb el seu nom i veu les seves coses.
	if s.Token != "" || (s.users != nil && s.users.Actiu()) {
		handler = s.requireUser(mux)
	}
	return handler
}

// HandlerWithIdentity delegates authentication to an embedding application's
// verified identity provider. The callback must never trust unverified headers.
func (s *Server) HandlerWithIdentity(auth func(*http.Request) (*User, error)) http.Handler {
	mux := s.Hub().mux()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			u, err := auth(r)
			if err != nil || u == nil || u.Name == "" {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, u))
		}
		mux.ServeHTTP(w, r)
	})
}

// StartBackground starts the scheduler and returns a channel closed when it
// has stopped. Cancellation asks an active scheduled job to unwind through
// the shared run queue; callers may bound how long they wait for completion.
func (s *Server) StartBackground(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.jobLoop(ctx)
	}()
	return done
}

// Stop cancels active and queued work before the embedding process shuts down.
func (s *Server) Stop() {
	h := s.Hub()
	h.mu.RLock()
	sessions := make([]*Server, 0, len(h.sess))
	for _, session := range h.sess {
		sessions = append(sessions, session)
	}
	h.mu.RUnlock()
	for _, session := range sessions {
		h.Queue().CancelSession(session.id)
		session.stopRun()
	}
}

func validateListenAuth(addr string, authenticated bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if !authenticated && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("remote access requires a token or configured users")
	}
	return nil
}

// routes recull les rutes /api registrades: el test del contracte les
// compara amb docs/api-contract.md perquè no es tornin a documentar a
// mitges.
var routes []string

// mux construeix el router. Separat de Run perquè els tests el puguin
// inspeccionar sense obrir cap port.
func (h *Hub) mux() *http.ServeMux {
	s := h.def
	routes = nil
	mux := http.NewServeMux()
	handle := func(path string, fn http.HandlerFunc) {
		if strings.HasPrefix(path, "/api/") {
			routes = append(routes, path)
		}
		mux.HandleFunc(path, fn)
	}
	_ = s
	handle("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	// Mòduls de la UI (app/*.js). Sense token: són estàtics i no toquen
	// dades; el que demana Bearer és /api/*.
	handle("/app/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if strings.Contains(name, "..") {
			http.Error(w, "no", 400)
			return
		}
		raw, err := assets.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch filepath.Ext(name) {
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		}
		w.Write(raw)
	})
	handle("/api/state", h.route((*Server).handleState))
	handle("/api/health", h.global((*Server).handleHealth))
	handle("/api/active", h.route((*Server).handleActive))
	handle("/api/v2/events", h.route((*Server).handleEvents))
	// Stream durable d'events v2: reprodueix el que falta i queda escoltant.
	// La ruta JSON de dalt continua sent el polling compatible amb clients antics.
	handle("/api/v2/events/stream", h.route((*Server).handleEventsStream))
	// Cua d'execucions compartida (v2): encuar, consultar i cancel·lar
	// torns identificats. El TUI delegat i el web passen pel mateix lloc.
	handle("/api/v2/runs", h.route((*Server).handleV2Runs))
	handle("/api/v2/runs/{id}", h.route((*Server).handleV2Run))
	handle("/api/v2/runs/{id}/cancel", h.route((*Server).handleV2RunCancel))
	handle("/api/chat", h.route((*Server).handleChat))
	handle("/api/parallel", h.route((*Server).handleParallel))
	handle("/api/v2/team/run", h.route((*Server).handleTeamRun))
	handle("/api/agent", h.route((*Server).handleAgent))
	handle("/api/approve", h.route((*Server).handleApprove))
	handle("/api/question", h.route((*Server).handleQuestion))
	handle("/api/permissive", h.route((*Server).handlePermissive))
	handle("/api/mode", h.route((*Server).handleMode))
	handle("/api/goal", h.route((*Server).handleGoal))
	handle("/api/verify", h.route((*Server).handleVerify))
	handle("/api/lang", h.global((*Server).handleLang))
	handle("/api/role", h.route((*Server).handleRole))
	handle("/api/model", h.route((*Server).handleModel))
	handle("/api/sessions", h.global((*Server).handleSessions))
	handle("/api/sessions/pin", h.global((*Server).handlePinSession))
	handle("/api/save", h.route((*Server).handleSave))
	handle("/api/new", h.route((*Server).handleNew))
	handle("/api/verify-approve", h.route((*Server).handleVerifyApprove))
	handle("/api/resume", h.route((*Server).handleResume))
	handle("/api/providers", h.global((*Server).handleProviders))
	handle("/api/github", h.global(adminOnly((*Server).handleGitHub)))
	handle("/api/office/upload", h.global((*Server).handleOfficeUpload))
	handle("/api/office/read", h.global((*Server).handleOfficeRead))
	handle("/api/office/edit", h.global((*Server).handleOfficeEdit))
	handle("/api/office/download", h.global((*Server).handleOfficeDownload))
	handle("/api/office/open", h.global((*Server).handleOfficeOpen))
	handle("/api/models", h.route((*Server).handleModels))
	handle("/api/rewind", h.route((*Server).handleRewind))
	handle("/api/rewind-to", h.route((*Server).handleRewindTo))
	handle("/api/checkpoints", h.route((*Server).handleCheckpoints))
	handle("/api/plan", h.route((*Server).handlePlan))
	handle("/api/jobs", h.global(adminOnly((*Server).handleJobs)))
	handle("/api/jobs/save", h.global(adminOnly((*Server).handleJobSave)))
	handle("/api/jobs/delete", h.global(adminOnly((*Server).handleJobDelete)))
	handle("/api/jobs/run", h.global(adminOnly((*Server).handleJobRun)))
	handle("/api/jobs/runs", h.global(adminOnly((*Server).handleJobRuns)))
	handle("/api/mcp", h.global((*Server).handleMCP))
	// G1: sessions vives i cancel·lació del torn.
	handle("/api/sessions/live", h.handleLiveSessions)
	handle("/api/sessions/open", h.handleOpenSession)
	handle("/api/sessions/close", h.handleCloseSession)
	// Qui és qui: la porta (/api/login) és l'única /api que no demana
	// token; la resta han d'anar identificades.
	handle("/api/login", h.global((*Server).handleLogin))
	handle("/api/logout", h.route((*Server).handleLogout))
	handle("/api/me", h.route((*Server).handleMe))
	handle("/api/agent/cancel", h.route((*Server).handleCancel))
	handle("/api/md", h.route((*Server).handleMarkdown))
	// G2: workspaces (directori de treball per sessió).
	handle("/api/workspaces", h.route((*Server).handleWorkspaces))
	handle("/api/dirs", h.route((*Server).handleDirs))
	// G3: arbre, fitxers i diffs del workspace.
	handle("/api/tree", h.route((*Server).handleTree))
	handle("/api/file", h.route((*Server).handleFile))
	handle("/api/diff", h.route((*Server).handleDiff))
	handle("/api/diff/discard", h.route((*Server).handleDiffDiscard))
	// Grafs: procediments desats a .gregal/flows del projecte de la sessió.
	handle("/api/flows", h.route((*Server).handleFlows))
	handle("/api/flows/load", h.route((*Server).handleFlowLoad))
	handle("/api/flows/save", h.route((*Server).handleFlowSave))
	handle("/api/flows/delete", h.route((*Server).handleFlowDelete))
	handle("/api/flows/run", h.route((*Server).handleFlowRun))
	// G8: processos en segon pla (terminal i bash_background).
	handle("/api/exec", h.route((*Server).handleExec))
	handle("/api/exec/output", h.route((*Server).handleExecOutput))
	handle("/api/exec/stream", h.route((*Server).handleExecStream))
	handle("/api/exec/kill", h.route((*Server).handleExecKill))
	return mux
}

// handleGitHub ofereix la mateixa superfície read-only que l'agent per a la
// web: issues, PRs, diff i checks. La UI no accepta una ordre gh arbitrària;
// tools.GitHub tradueix l'acció a arguments tancats.
func (s *Server) handleGitHub(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		_, err := exec.LookPath("gh")
		writeJSON(w, map[string]any{"available": err == nil, "cwd": s.cwd})
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Tool   string `json:"tool"`
		Action string `json:"action"`
		Number int    `json:"number"`
		Query  string `json:"query"`
		Repo   string `json:"repo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", http.StatusBadRequest)
		return
	}
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	if tool != "gh_issue" && tool != "gh_pr" {
		http.Error(w, "tool GitHub invàlida", http.StatusBadRequest)
		return
	}
	args, _ := json.Marshal(map[string]any{"action": req.Action, "number": req.Number, "query": req.Query, "repo": req.Repo})
	out, err := tools.GitHub(tool, string(args))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]string{"tool": tool, "action": req.Action, "output": out})
}

// requireToken exigeix el token a /api/* només per capçalera Bearer.
// El ?token= a la query ja no s'accepta (queda als logs i a l'historial).
// L'única excepció és l'arrencada de la pàgina (/): el JS llegeix ?token=
// un sol cop, el desa a localStorage i el neteja de la barra.
// --- qui és qui ---

// ctxKeyUser és la clau del context on viatja l'usuari resolt.
type ctxKeyUser struct{}

// userFrom treu l'usuari de la petició (nil en mode local, sense auth).
func userFrom(r *http.Request) *User {
	u, _ := r.Context().Value(ctxKeyUser{}).(*User)
	return u
}

// requireUser resol qui fa la petició: el token únic de sempre (admin) o un
// token de sessió emès per /api/login. Sense cap dels dos, 401.
//
// /api/login queda obert perquè és la porta. Si no hi ha ni token ni
// usuaris, aquest middleware no s'instal·la i tot queda com abans.
func (s *Server) requireUser(next http.Handler) http.Handler {
	senseAuth := strings.TrimSpace(s.Token) == "" && (s.users == nil || !s.users.Actiu())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if senseAuth || !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/login" {
			next.ServeHTTP(w, r)
			return
		}
		got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		var (
			u  *User
			ok bool
		)
		if s.users != nil {
			u, ok = s.users.Resolve(got)
		}
		// El token fixat a mà (tests, desktops) també val: es tracta com
		// l'admin, amb les arrels que li doni el config si en té.
		if !ok && s.Token != "" && !subtleCompare(s.Token, got) {
			u, ok = s.legacyAdmin(), true
		}
		if !ok {
			http.Error(w, `{"error":"cal token"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, u)))
	})
}

// legacyAdmin és el qui entra amb el token únic quan ningú no l'ha
// registrat a Users (tests, arrencades a mà).
func (s *Server) legacyAdmin() *User {
	if s.users != nil {
		return s.users.legacyUser
	}
	return &User{Name: "local", Admin: true, Tothom: true}
}

// usuari és el nom de qui fa la petició (buit en mode local).
func (s *Server) usuari() string {
	if s == nil || s.user == nil {
		return ""
	}
	return s.user.Name
}

// sessDir és la carpeta de converses de l'usuari d'aquesta sessió.
func (s *Server) sessDir() string {
	name := ""
	if s.user != nil {
		name = s.user.Name
	}
	return session.DirFor(name)
}

// handleLogin bescanvia usuari i contrasenya per un token de sessió.
//
// Els intents fallits es compten per adreça: una contrasenya de debò no es
// pot anar provant. La resposta d'error és sempre la mateixa, tant si el
// nom no existeix com si la contrasenya no hi és.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "cal POST", http.StatusMethodNotAllowed)
		return
	}
	if s.users == nil || !s.users.Actiu() {
		http.Error(w, `{"error":"aquest servidor no té usuaris: fes servir el token"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"cos invàlid"}`, http.StatusBadRequest)
		return
	}
	ip := clientIP(r)
	if !s.users.Permes(ip) {
		http.Error(w, `{"error":"massa intents fallits; espera uns minuts"}`, http.StatusTooManyRequests)
		return
	}
	tok, u, err := s.users.Login(req.User, req.Password)
	if err != nil {
		s.users.Fallit(ip)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	s.users.Encertat(ip)
	writeJSON(w, map[string]any{
		"token": tok, "user": u.Name, "roots": u.Arrels(), "admin": u.Admin,
	})
}

// handleLogout oblida el token de qui el demana.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if s.users != nil {
		s.users.Logout(tok)
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleMe diu qui ets i on pots treballar (l'app ho fa servir per saber si
// ha de mostrar la pantalla d'entrada i quines carpetes oferir).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		writeJSON(w, map[string]any{"local": true, "user": "", "roots": []string{}, "admin": true})
		return
	}
	quants := 0
	if s.users != nil {
		quants = len(s.users.Noms())
	}
	writeJSON(w, map[string]any{
		"user": u.Name, "roots": u.Arrels(), "admin": u.Admin,
		"home": u.Home, "usuaris": quants, "local": false,
	})
}

// clientIP treu l'adreça del client (darrere d'un proxy, la primera de
// X-Forwarded-For).
func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		if i := strings.Index(xf, ","); i > 0 {
			return strings.TrimSpace(xf[:i])
		}
		return strings.TrimSpace(xf)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// subtleCompare compara en temps constant (true = no coincideix).
func subtleCompare(want, got string) bool {
	if len(got) != len(want) {
		return true
	}
	bad := 0
	for i := range want {
		bad |= int(got[i]) ^ int(want[i])
	}
	return bad != 0
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// sse obre un stream SSE i retorna l'emissor (segur per a concurrència).
func sse(w http.ResponseWriter) func(ev string, v any) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl, _ := w.(http.Flusher)
	var mu sync.Mutex
	return func(ev string, v any) {
		b, _ := json.Marshal(v)
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev, b)
		if fl != nil {
			fl.Flush()
		}
	}
}

// sseWithHeartbeat manté la connexió oberta enviant un ping periòdic
// si el canal està en silenci (crucial durant la inferència de models locals i eines llargues).
func sseWithHeartbeat(w http.ResponseWriter, ctx context.Context, interval time.Duration) (func(ev string, v any), func()) {
	emit := sse(w)
	if interval <= 0 {
		interval = 15 * time.Second
	}
	hbCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				emit("ping", map[string]any{})
			}
		}
	}()
	return emit, cancel
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	queued := s.queueHasActiveTurn()
	s.mu.Lock()
	p, role := s.roleRef()
	cwd := s.cwd
	roles := []string{}
	for _, n := range []string{"chat", "think", "code", "reviewer"} {
		if _, ok := s.cfg.Roles[n]; ok {
			roles = append(roles, n)
		}
	}
	out := map[string]any{
		"role": roles, "current_role": s.role, "role_pinned": s.rolePinned, "mode": s.mode, "lang": s.cfg.Lang(),
		"model": role.Model, "provider": pName(p, role),
		"verify":     s.cfg.Verify.Mode,
		"agent_busy": queued || s.agentBusy || s.planRunning, "cwd": cwd, "project": filepath.Base(cwd),
		"permissive": s.permissive, // handleState ja té s.mu; no usar permissiveNow() aquí (deadlock)
		// Estimació del context ocupat, per al mesurador de la UI (J4). El TUI
		// ho compta al client; la web no tenia res i pintava un percentatge
		// que no existia. context_window 0 = el rol no el declara.
		// El resum de compactació viu al system prompt i ocupa context: si
		// no el comptéssim, el mesurador baixaria a gairebé zero després de
		// compactar i tornaria a mentir en sentit contrari.
		"used_tokens":    llm.EstimateTokens(s.convo) + tokensResum(s.compacted),
		"context_window": agent.Window(s.cfg, role),
		// Transcript curt perquè canviar de pestanya no perdi l'historial
		// visible (abans switchTo buidava el fil i només deia "Sessió · id").
		"transcript": transcriptView(s.convo, 50),
	}
	s.mu.Unlock()
	// Git pot haver d'esperar un procés o un disc ocupat. No mantenim el
	// mutex de la sessió mentre el consultem: l'agent ha de poder emetre
	// events i rebre cancel·lacions fins i tot durant aquesta lectura.
	out["branch"] = tools.GitBranch(cwd)
	writeJSON(w, out)
}

// handleHealth és una resposta petita i estable per descobrir el servei.
// Els clients la poden consultar abans de carregar l'estat de conversa i
// detectar un procés diferent encara que el port s'hagi reutilitzat.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "només GET", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	id := s.instanceID
	s.mu.Unlock()
	writeJSON(w, map[string]any{
		"status":      "ok",
		"protocol":    APIProtocol,
		"instance_id": id,
		"capabilities": map[string]bool{
			"sessions":           true,
			"runs":               true,
			"events":             s.hub == nil || s.hub.eventError() == nil,
			"durable_events":     s.hub != nil && s.hub.eventError() == nil,
			"event_stream":       s.hub != nil && s.hub.eventError() == nil,
			"interactive_events": s.hub != nil && s.hub.eventError() == nil,
			"jobs":               s.jobs != nil,
			"mcp":                s.mcp != nil,
		},
	})
}

func pName(p config.Provider, r config.Role) string { return r.Provider }

// handleActive exposa la sessió que ocupa l'únic worker web. És només de
// lectura: reprendre-la mentre executa canviaria la conversa compartida.
func (s *Server) handleActive(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		writeJSON(w, map[string]any{"active": nil})
		return
	}
	out := *s.active
	out.Events = append([]liveEvent(nil), s.active.Events...)
	writeJSON(w, map[string]any{"active": out})
}

// handleEvents retorna events persistents amb un cursor monotònic. El client
// pot fer polling amb after i continuar des d'on ho havia deixat després d'un
// reinici o una desconnexió de xarxa.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	after := uint64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("after")); raw != "" {
		if n, err := strconv.ParseUint(raw, 10, 64); err == nil {
			after = n
		} else {
			http.Error(w, "cursor invàlid", http.StatusBadRequest)
			return
		}
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		} else {
			http.Error(w, "límit invàlid", http.StatusBadRequest)
			return
		}
	}
	if limit < 1 || limit > 500 {
		http.Error(w, "límit fora de rang", http.StatusBadRequest)
		return
	}
	if s.hub == nil || s.hub.eventError() != nil {
		http.Error(w, "registre d'events no disponible", http.StatusServiceUnavailable)
		return
	}
	scope := strings.TrimSpace(r.URL.Query().Get("session"))
	if scope == "" {
		scope = s.id
	}
	if scope != "" && !sessIDRe.MatchString(scope) {
		http.Error(w, "sessió invàlida", http.StatusBadRequest)
		return
	}
	// Els usuaris tenen una clau interna prefijada (`nom-id`), però el fil
	// HTTP només coneix l'id curt que envia al client.
	if s.hub != nil {
		scope = s.hub.keyFor(s.user, scope)
	}
	got, err := s.hub.eventStore.AfterScope(after, limit, s.usuari(), scope)
	if err != nil {
		http.Error(w, "no es poden llegir els events", http.StatusInternalServerError)
		return
	}
	next := after
	if len(got) > 0 {
		next = got[len(got)-1].ID
	}
	writeJSON(w, map[string]any{"events": got, "cursor": after, "next": next})
}

const (
	// El registre és append-only i no té un canal de notificació propi: el
	// stream consulta periòdicament el cursor. És prou curt per a una UI en
	// viu sense mantenir un goroutine per cada append.
	durableEventsPollInterval      = 100 * time.Millisecond
	durableEventsHeartbeatInterval = 15 * time.Second
)

// handleEventsStream és la variant SSE durable d'handleEvents. Primer
// reprodueix tots els events posteriors al cursor i després espera els nous;
// si la connexió cau, el client pot reconnectar amb l'últim id SSE o amb
// ?after= i no perd ni duplica cap event del registre.
//
// El middleware requireUser embolcalla el mux sencer, de manera que aquesta
// ruta té exactament la mateixa autenticació que /api/v2/events.
func (s *Server) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "només GET", http.StatusMethodNotAllowed)
		return
	}

	after, err := eventCursor(r)
	if err != nil {
		http.Error(w, "cursor invàlid", http.StatusBadRequest)
		return
	}
	limit, err := eventLimit(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.hub == nil || s.hub.eventError() != nil {
		http.Error(w, "registre d'events no disponible", http.StatusServiceUnavailable)
		return
	}
	scope, err := eventScope(s, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE no disponible", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	// Evita que proxies que entenen aquesta capçalera bufferitzin el stream.
	w.Header().Set("X-Accel-Buffering", "no")
	// Indica al client quan pot provar la reconnexió automàtica si el procés
	// tanca la connexió. El cursor real sempre és l'id de cada event.
	if _, err := fmt.Fprint(w, "retry: 1000\n\n"); err != nil {
		return
	}
	fl.Flush()

	poll := time.NewTicker(durableEventsPollInterval)
	defer poll.Stop()
	heartbeat := time.NewTicker(durableEventsHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		got, err := s.hub.eventStore.AfterScope(after, limit, s.usuari(), scope)
		if err != nil {
			// Un error de lectura posterior a les capçaleres no es pot convertir
			// en un status HTTP; un event SSE explícit permet al client tancar i
			// reprendre o mostrar el problema.
			_ = writeEventSSE(w, fl, events.Event{Kind: "error", Text: "no es poden llegir els events"})
			return
		}
		for _, e := range got {
			if err := writeEventSSE(w, fl, e); err != nil {
				return
			}
			after = e.ID
		}
		if len(got) > 0 {
			continue
		}

		select {
		case <-r.Context().Done():
			return
		case <-poll.C:
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

// eventCursor accepta ?after= com a opció explícita i Last-Event-ID com a
// fallback estàndard d'SSE. Un ?after= buit conserva el comportament de
// començar des de zero; només un valor no buit invàlid és un error.
func eventCursor(r *http.Request) (uint64, error) {
	raw, present := r.URL.Query()["after"]
	value := ""
	if present && len(raw) > 0 {
		value = strings.TrimSpace(raw[0])
	} else {
		value = strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	}
	if value == "" {
		return 0, nil
	}
	return strconv.ParseUint(value, 10, 64)
}

func eventLimit(r *http.Request) (int, error) {
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, errors.New("límit invàlid")
		}
		limit = n
	}
	if limit < 1 || limit > 500 {
		return 0, errors.New("límit fora de rang")
	}
	return limit, nil
}

func eventScope(s *Server, r *http.Request) (string, error) {
	scope := strings.TrimSpace(r.URL.Query().Get("session"))
	if scope == "" {
		scope = s.id
	}
	if scope != "" && !sessIDRe.MatchString(scope) {
		return "", errors.New("sessió invàlida")
	}
	if s.hub != nil {
		scope = s.hub.keyFor(s.user, scope)
	}
	return scope, nil
}

func writeEventSSE(w http.ResponseWriter, fl http.Flusher, e events.Event) error {
	kind := strings.TrimSpace(e.Kind)
	if kind == "" {
		kind = "message"
	}
	// Els kinds actuals són constants del backend; no permetem igualment que
	// una dada futura injecti capçalera SSE amb un salt de línia.
	kind = strings.NewReplacer("\r", "", "\n", "").Replace(kind)
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, kind, b); err != nil {
		return err
	}
	fl.Flush()
	return nil
}

// appendRunEvent deixa al registre durable el cicle de vida del torn
// (encuat, iniciat, fi): qualsevol client amb el cursor el llegeix, i si el
// procés moren els events ja escrits no es perden.
func (s *Server) appendRunEvent(run runs.Run, kind, text string) {
	_ = s.Hub().appendEvent(events.Event{
		SessionID: s.id, User: s.usuari(), RunID: int(run.ID), Kind: kind, Text: text,
	})
}

// watchRun tanca el cicle de vida al registre: quan la cua acaba el torn hi
// deixa el resultat. Un torn que mai no arriba a executar-se (cancel·lat en
// cua) també queda tancat: el watcher no depèn de l'executor.
func (s *Server) watchRun(run runs.Run) {
	q := s.Hub().Queue()
	go func() {
		// Done sempre es tanca: la cua tanca el canal a la fi de cada
		// execució, i un ID desconegut retorna un canal ja tancat.
		<-q.Done(run.ID)
		fin, ok := q.Get(run.ID)
		if !ok {
			return
		}
		kind, text := "run_completed", "torn completat"
		switch fin.State {
		case runs.Failed:
			kind, text = "run_failed", "torn fallit: "+fin.Err
		case runs.Cancelled:
			kind, text = "run_cancelled", "torn cancel·lat"
		}
		s.appendRunEvent(fin, kind, text)
	}()
}

// runOwned comprova que el torn demanat pertany a aquesta sessió. La clau
// de sessió ja incorpora l'usuari: ningú no pot veure ni cancel·lar els
// torns d'un altre.
func (s *Server) runOwned(id int64) (runs.Run, bool) {
	run, ok := s.Hub().Queue().Get(id)
	if !ok || run.Session != s.id {
		return runs.Run{}, false
	}
	return run, true
}

// handleV2Runs reparteix el recurs de col·lecció: POST encua, GET llista.
func (s *Server) handleV2Runs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleV2RunCreate(w, r)
	case http.MethodGet:
		s.handleV2RunList(w, r)
	default:
		http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
	}
}

// handleV2Run reparteix el recurs individual (GET consulta l'estat).
func (s *Server) handleV2Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
		return
	}
	s.handleV2RunGet(w, r)
}

// handleV2RunCreate encua un torn per a la sessió i torna l'execució
// (202). Amb clau d'idempotència, reenviar el mateix missatge retorna
// l'execució ja existent (200) en comptes de crear-ne una altra.
func (s *Server) handleV2RunCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Task           string   `json:"task"`
		Images         []string `json:"images"`
		Mode           string   `json:"mode"`
		IdempotencyKey string   `json:"idempotency_key"`
		// Workspace és el directori on ha de córrer el torn (el del client
		// que delega). Buit = el de la sessió, com abans.
		Workspace string `json:"workspace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Task) == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "tasca buida"})
		return
	}
	if len(req.Images) > 3 {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "màxim 3 imatges per torn"})
		return
	}
	s.mu.Lock()
	gated := s.gated
	s.mu.Unlock()
	if gated {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusLocked)
		w.Write([]byte(`{"gated":true,"error":"torn bloquejat: el revisor ha dit CAL REVISAR. Aprova'l o comença de nou."}`))
		return
	}
	imgs := make([]string, 0, len(req.Images))
	for i, u := range req.Images {
		n, err := tools.NormalizeDataURL(u, fmt.Sprintf("imatge %d", i+1))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		imgs = append(imgs, n)
	}
	// El client delegat (TUI) treballa en un altre directori que la sessió
	// del servei: sense això el torn actuaria al projecte equivocat.
	ws, code, msg := s.resolveWorkspace(req.Workspace)
	if code != 0 {
		w.WriteHeader(code)
		writeJSON(w, map[string]string{"error": msg})
		return
	}
	exec := s.turnExecutor(turnOpts{
		task: strings.TrimSpace(req.Task), imgs: imgs, mode: req.Mode,
		workspace: ws,
		emit:      func(event string, value any) { s.recordActive(event, value) },
	})
	cursor := uint64(0)
	if s.hub != nil && s.hub.eventStore != nil {
		cursor = s.hub.eventStore.Cursor()
	}
	run, existing, err := s.Hub().Queue().Submit(exec, runs.Run{
		Session: s.id, Workspace: runs.WorkspaceKey(ws),
		Priority: runs.PriorityInteractive, Key: req.IdempotencyKey,
	})
	if err != nil {
		w.WriteHeader(http.StatusTooManyRequests)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	if existing {
		writeJSON(w, map[string]any{"run": run, "cursor": 0})
		return
	}
	s.appendRunEvent(run, "run_queued", "torn encuat")
	s.watchRun(run)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{"run": run, "cursor": cursor})
}

// resolveWorkspace valida el workspace demanat per a un torn: buit = el de
// la sessió; si no, ha de ser absolut, existir com a directori i passar el
// guard de l'usuari. Torna el codi HTTP i el missatge quan no val (0 = bé).
func (s *Server) resolveWorkspace(req string) (string, int, string) {
	ws := strings.TrimSpace(req)
	if ws == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.cwd, 0, ""
	}
	if !filepath.IsAbs(ws) {
		return "", http.StatusBadRequest, "workspace relatiu: cal ruta absoluta"
	}
	ws = filepath.Clean(ws)
	fi, err := os.Stat(ws)
	if err != nil || !fi.IsDir() {
		return "", http.StatusBadRequest, "workspace inexistent"
	}
	// El guard no filtra què hi ha a fora: el missatge és genèric a posta.
	if err := s.user.Allow(ws); err != nil {
		return "", http.StatusForbidden, err.Error()
	}
	return ws, 0, ""
}

// handleV2RunList torna les execucions de la sessió, de més nova a més vella.
func (s *Server) handleV2RunList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"runs": s.Hub().Queue().ListSession(s.id)})
}

// handleV2RunGet torna l'estat d'una execució de la sessió.
func (s *Server) handleV2RunGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id invàlid", http.StatusBadRequest)
		return
	}
	run, ok := s.runOwned(id)
	if !ok {
		http.Error(w, "execució desconeguda", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"run": run})
}

// handleV2RunCancel cancel·la una execució identificada: en cua no s'executa
// mai; en marxa talla el context de l'executor.
func (s *Server) handleV2RunCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id invàlid", http.StatusBadRequest)
		return
	}
	if _, ok := s.runOwned(id); !ok {
		http.Error(w, "execució desconeguda", http.StatusNotFound)
		return
	}
	cancelled := s.Hub().Queue().Cancel(id)
	writeJSON(w, map[string]any{"ok": true, "cancelled": cancelled})
}

func (s *Server) recordActive(event string, value any) {
	if event == "token" {
		chunk, _ := value.(map[string]string)
		s.mu.Lock()
		if s.active != nil {
			s.tokenBuf += chunk["text"]
			// Una escriptura durable per cada token saturava el registre i
			// l'event stream en tirades de milers de passos.
			if len(s.tokenBuf) >= 512 || time.Since(s.tokenAt) >= 180*time.Millisecond {
				s.flushTokensLocked()
			}
		}
		s.mu.Unlock()
		return
	}
	b, _ := json.Marshal(value)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	text := ""
	kind := "activity"
	switch event {
	case "thinking":
		text = "💭 " + data["text"]
	case "tool_call":
		text = "🔧 " + data["name"]
	case "tool_result":
		text = "↳ " + data["name"] + ": " + data["output"]
	case "blocked":
		text = "⛔️ " + data["reason"]
	case "verify":
		text = "✅ " + data["verdict"] + " " + data["detail"]
	case "assistant":
		kind, text = "text", data["text"]
	case "error":
		kind, text = "error", "⚠️ "+data["message"]
	case "done":
		kind, text = "done", data["reply"]
		if text == "" {
			text = "torn completat"
		}
	default:
		kind = event
		text = data["text"]
		if text == "" {
			text = data["message"]
		}
		if text == "" {
			text = data["name"]
		}
		if text == "" {
			text = strings.TrimSpace(string(b))
		}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	// Les entrades d'activitat són resums. El text de l'assistent i la
	// resposta final han d'arribar sencers al transcript durable del desktop.
	if kind != "text" && kind != "done" && len([]rune(text)) > 900 {
		text = string([]rune(text)[:900]) + "…"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return
	}
	s.flushTokensLocked()
	s.active.Events = append(s.active.Events, liveEvent{Kind: kind, Text: text})
	if len(s.active.Events) > 80 {
		s.active.Events = s.active.Events[len(s.active.Events)-80:]
	}
	// Escrivim dins el mateix bloqueig de sessió perquè l'event queda lligat
	// al run que encara és visible a /api/active.
	runID := s.active.ID
	// Al registre durable, les eines i el text de treball conserven el seu
	// nom: la web (app/runs.js) hi pinta les targetes d'eina i hi tanca la
	// bombolla de cada pas. /api/active continua veient "activity".
	durable := kind
	switch event {
	case "thinking", "tool_call", "tool_result", "blocked":
		durable = event
	}
	s.hub.appendEvent(events.Event{
		SessionID: s.id, User: s.usuari(), RunID: runID, Kind: durable, Text: text,
		Payload: interactiveEventPayload(event, value),
	})
}

// flushTokensLocked envia un fragment de text en viu sense convertir cada
// token en una transacció del registre durable. Cal tenir s.mu.
func (s *Server) flushTokensLocked() {
	if s.active == nil || s.tokenBuf == "" {
		return
	}
	s.hub.appendEvent(events.Event{
		SessionID: s.id, User: s.usuari(), RunID: s.active.ID,
		Kind: "token", Text: s.tokenBuf,
	})
	s.tokenBuf = ""
	s.tokenAt = time.Now()
}

// interactiveEventPayload conserva només el contracte necessari per reprendre
// una aprovació, una pregunta o pintar un checkpoint des d'un client que s'ha
// reconnectat. No desa camps arbitraris del valor SSE, que podrien contenir
// metadades o secrets no necessaris per pintar la interacció.
func interactiveEventPayload(event string, value any) json.RawMessage {
	var allowed []string
	switch event {
	case "approve_request":
		allowed = []string{"key", "call_id", "name", "args", "timeout_s", "auto_approve_allowed"}
	case "question_request":
		allowed = []string{"key", "call_id", "query", "options", "timeout_s"}
	case "tool_call":
		allowed = []string{"id", "name", "args"}
	case "tool_result":
		allowed = []string{"id", "name", "output"}
	case "thinking":
		allowed = []string{"text"}
	case "blocked":
		allowed = []string{"id", "reason"}
	case "autonomous_checkpoint":
		allowed = []string{"number", "checks", "review"}
	default:
		return nil
	}
	// La sortida d'una eina pot ser llarga: la UI en pinta 3000 caràcters.
	if m, ok := value.(map[string]string); ok && event == "tool_result" && len([]rune(m["output"])) > 3000 {
		c := make(map[string]string, len(m))
		for k, v := range m {
			c[k] = v
		}
		c["output"] = string([]rune(m["output"])[:3000]) + "…"
		value = c
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil
	}
	out := make(map[string]json.RawMessage, len(allowed))
	for _, key := range allowed {
		if field, ok := fields[key]; ok && json.Valid(field) {
			out[key] = field
		}
	}
	if len(out) == 0 {
		return nil
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return payload
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		http.Error(w, "missatge buit", 400)
		return
	}
	// Via ràpida també en mode objectiu: un "hola" no necessita el model.
	if reply, ok := agent.SmallTalk(req.Message); ok {
		task := strings.TrimSpace(req.Message)
		s.mu.Lock()
		s.convo = append(s.convo, llm.Message{Role: "user", Content: task})
		s.convo = append(s.convo, llm.Message{Role: "assistant", Content: reply})
		tools.Active.MarkConvoFor(s.id, len(s.convo))
		s.mu.Unlock()
		emit := sse(w)
		emit("assistant", map[string]string{"text": reply})
		emit("done", map[string]string{"reply": reply})
		s.autosave()
		return
	}
	// Tot mode que promet eines passa per l'agent (mateix motiu que al TUI):
	// chat i consulta anaven per ChatStream, sense `tools`, amb un prompt que
	// deia "POTS llegir": DeepSeek escrivia la crida en DSML com a text. La
	// política de l'agent ja hi denega escriure. Només goal es queda parlant.
	s.mu.Lock()
	mode := s.mode
	s.mu.Unlock()
	if mode != agent.ModeGoal {
		body, _ := json.Marshal(map[string]string{"task": req.Message})
		r.Body = io.NopCloser(bytes.NewReader(body))
		s.handleAgent(w, r)
		return
	}
	runCtx, endRun := s.runContext()
	defer endRun()
	emit, stopHeartbeat := sseWithHeartbeat(w, runCtx, 15*time.Second)
	defer stopHeartbeat()
	s.mu.Lock()
	roleName := s.role
	s.mu.Unlock()
	if selection, unavailable := s.refreshModelAvailability(roleName); unavailable {
		fallback := s.cfg.Roles[roleName]
		emit("status", map[string]string{"message": fmt.Sprintf("Model %s no està disponible; faré servir %s/%s.", selection, fallback.Provider, fallback.Model)})
	}
	s.mu.Lock()
	pComp, roleComp := s.roleRefFor(roleName)
	s.mu.Unlock()
	s.compactaSiCal(r.Context(), pComp, roleComp, emit)
	s.mu.Lock()
	content := s.expandMentions(req.Message, s.cwd)
	s.convo = append(s.convo, llm.Message{Role: "user", Content: content})
	p, role := s.roleRefFor(roleName)
	hist := append([]llm.Message{{Role: "system", Content: s.sysPrompt()}}, s.convo...)
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(runCtx, 180*time.Second)
	defer cancel()
	ctx = llm.WithRetryHook(ctx, func(attempt, total int, wait time.Duration, err error) {
		emit("status", map[string]string{"message": llm.RetryNote(attempt, total, wait, err)})
	})
	reply, err := s.client.ChatStream(ctx, p.BaseURL, p.APIKey, role.Model, hist, role.Temperature, role.MaxTokens,
		func(tok string) { emit("token", map[string]string{"text": tok}) }, nil)
	if err != nil {
		if exceeded, actualCtx := agent.ParseContextExceeded(err); exceeded {
			s.mu.Lock()
			if actualCtx > 0 {
				agent.LearnWindow(p.BaseURL, role.Model, actualCtx)
				role.ContextWindow = actualCtx
				if rcfg, ok := s.cfg.Roles[s.role]; ok {
					rcfg.ContextWindow = actualCtx
					s.cfg.Roles[s.role] = rcfg
				}
			}
			if len(s.convo) > 4 {
				abans := len(s.convo)
				s.convo = agent.TrimKeep(s.convo, 4)
				hist = append([]llm.Message{{Role: "system", Content: s.sysPrompt()}}, s.convo...)
				s.mu.Unlock()
				emit("compact", map[string]int{"abans": abans, "despres": len(s.convo)})
				ctx2, cancel2 := context.WithTimeout(runCtx, 180*time.Second)
				defer cancel2()
				reply, err = s.client.ChatStream(ctx2, p.BaseURL, p.APIKey, role.Model, hist, role.Temperature, role.MaxTokens,
					func(tok string) { emit("token", map[string]string{"text": tok}) }, nil)
			} else {
				s.mu.Unlock()
			}
		}
	}
	if err != nil {
		emit("error", map[string]string{"message": err.Error()})
		return
	}
	s.mu.Lock()
	if strings.TrimSpace(reply) != "" {
		s.convo = append(s.convo, llm.Message{Role: "assistant", Content: reply})
	}
	tools.Active.MarkConvoFor(s.id, len(s.convo))
	mode = s.mode
	s.mu.Unlock()
	if mode == agent.ModeGoal {
		s.emitGoal(emit, reply)
	}
	emit("done", map[string]string{"reply": reply})
	s.autosave()
	if s.autoVerify() {
		s.emitVerify(emit, s.cwd)
	}
}

type parallelWebResult struct {
	Role     string `json:"role"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Reply    string `json:"reply,omitempty"`
	Error    string `json:"error,omitempty"`
}

func parallelRoleNamesWeb(cfg *config.Config, active string) []string {
	order := []string{active, "chat", "think", "reviewer", "code"}
	seenTarget, seenRole := map[string]bool{}, map[string]bool{}
	var out []string
	for _, name := range order {
		if name == "" || seenRole[name] {
			continue
		}
		r, ok := cfg.Roles[name]
		if !ok || strings.TrimSpace(r.Model) == "" {
			continue
		}
		seenRole[name] = true
		key := r.Provider + "\x00" + r.Model
		if seenTarget[key] {
			continue
		}
		seenTarget[key] = true
		out = append(out, name)
		if len(out) == 4 {
			break
		}
	}
	return out
}

// handleParallel compara explícitament els rols configurats. No s'activa en
// un torn normal: així el cloud només es factura quan l'usuari ho demana.
func (s *Server) handleParallel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if s.agentBusy {
		s.mu.Unlock()
		http.Error(w, "ja hi ha una feina en marxa", http.StatusConflict)
		return
	}
	s.agentBusy = true
	active := s.role
	hist := append([]llm.Message{{Role: "system", Content: s.sysPrompt()}}, s.convo...)
	hist = append(hist, llm.Message{Role: "user", Content: strings.TrimSpace(req.Message)})
	names := parallelRoleNamesWeb(s.cfg, active)
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.agentBusy = false
		s.mu.Unlock()
	}()
	if len(names) < 2 {
		http.Error(w, "calen almenys dos rols amb models diferents", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	results := make([]parallelWebResult, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			r := cfg.Roles[name]
			result := parallelWebResult{Role: name, Provider: r.Provider, Model: r.Model}
			p, ok := cfg.Providers[r.Provider]
			if !ok {
				result.Error = "provider desconegut: " + r.Provider
				results[i] = result
				return
			}
			answer, _, err := client.ChatFO(ctx, cfg.PrimTarget(p, r), cfg.FallbackTarget(r), hist, r.Temperature, r.MaxTokens, nil)
			if err != nil {
				result.Error = err.Error()
			} else {
				result.Reply = strings.TrimSpace(answer)
			}
			results[i] = result
		}(i, name)
	}
	wg.Wait()
	var combined strings.Builder
	for _, result := range results {
		if result.Error != "" {
			fmt.Fprintf(&combined, "%s (%s/%s): ERROR: %s\n\n", result.Role, result.Provider, result.Model, result.Error)
		} else {
			fmt.Fprintf(&combined, "%s (%s/%s):\n%s\n\n", result.Role, result.Provider, result.Model, result.Reply)
		}
	}
	s.mu.Lock()
	s.convo = append(s.convo, llm.Message{Role: "user", Content: strings.TrimSpace(req.Message)}, llm.Message{Role: "assistant", Content: strings.TrimSpace(combined.String())})
	tools.Active.MarkConvoFor(s.id, len(s.convo))
	s.mu.Unlock()
	writeJSON(w, map[string]any{"results": results})
}

// mentionRe accepta rutes absolutes de Windows (C:\…): el ":" i la "\" hi
// són permesos; els signes de puntuació finals es treuen després.
//
// La primera alternativa és @"amb cometes": sense això, una ruta amb
// espais —que a Windows són la norma, no l'excepció— es tallava al primer
// espai i l'agent deia que el fitxer no existia.
var mentionRe = regexp.MustCompile(`@"([^"]+)"|@([A-Za-z0-9_./~\\:-][^\s,;'"]*)`)

// refDeMencio treu la ruta de qualsevol de les dues formes.
func refDeMencio(m []string) string {
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// expandMentions adjunta fins a 200 línies per @fitxer (com el TUI).
func (s *Server) expandMentions(text, base string) string {
	var extra []string
	for _, m := range mentionRe.FindAllStringSubmatch(text, -1) {
		ref := strings.TrimRight(refDeMencio(m), ".,;:")
		path := ref
		if strings.HasPrefix(path, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				cleanHome := strings.TrimPrefix(path[1:], "/")
				cleanHome = strings.TrimPrefix(cleanHome, "\\")
				path = filepath.Join(home, filepath.FromSlash(cleanHome))
			}
		} else if !tools.IsRooted(path) && !filepath.IsAbs(path) {
			path = filepath.Join(base, ref)
		}
		var raw string
		var err error
		if tools.OfficeKind(path) != "" {
			// Un .docx/.xlsx/.pptx adjunt és el seu text, no el zip (com al TUI).
			raw, err = tools.OfficeRead(path)
		} else {
			raw, err = tools.Read(path, 1, 200)
		}
		if err != nil {
			extra = append(extra, fmt.Sprintf("[@%s: %s]", ref, err.Error()))
			continue
		}
		extra = append(extra, fmt.Sprintf("[@%s]\n%s", ref, raw))
	}
	if len(extra) == 0 {
		return text
	}
	return text + "\n\n" + strings.Join(extra, "\n\n")
}

// --- providers (mateixa lògica que /provider del TUI, via UI) ---

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == "GET" {
		type prov struct {
			Name   string   `json:"name"`
			URL    string   `json:"url"`
			Key    string   `json:"key"` // emmascarada: (buida) | ${VAR} | sí (oculta)
			UsedBy []string `json:"used_by"`
		}
		usedBy := map[string][]string{}
		for rn, r := range s.cfg.Roles {
			usedBy[r.Provider] = append(usedBy[r.Provider], rn+"/"+r.Model)
		}
		var out []prov
		for n, p := range s.cfg.Providers {
			out = append(out, prov{Name: n, URL: p.BaseURL, Key: p.MaskedKey(), UsedBy: usedBy[n]})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		roles := map[string]map[string]any{}
		for rn, r := range s.cfg.Roles {
			roles[rn] = map[string]any{"provider": r.Provider, "model": r.Model, "long_run": r.LongRunsAllowed()}
		}
		writeJSON(w, map[string]any{"providers": out, "roles": roles, "autonomous": s.cfg.AutonomousConfig(), "catalog": config.Cataleg()})
		return
	}
	var req struct {
		Action       string `json:"action"` // add|rm|rol|url|key|test
		Name         string `json:"name"`
		URL          string `json:"url"`
		Key          string `json:"key"`
		Role         string `json:"role"`
		Provider     string `json:"provider"`
		Model        string `json:"model"`
		LongRun      bool   `json:"long_run"`
		MaxMinutes   int    `json:"max_minutes"`
		MaxToolSteps int    `json:"max_tool_steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", 400)
		return
	}
	msg, err := s.providerAction(req.Action, req.Name, req.URL, req.Key, req.Role, req.Provider, req.Model, req.LongRun, req.MaxMinutes, req.MaxToolSteps)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]string{"msg": msg})
}

func (s *Server) providerAction(action, name, url, key, role, provider, model string, longRun bool, maxMinutes, maxToolSteps int) (string, error) {
	name = strings.TrimSpace(name)
	save := func() error { return s.cfg.Save(s.cfgPath) }
	switch action {
	case "preset":
		preset, ok := config.ProveidorPerNom(name)
		if !ok || preset.Local || preset.DefaultModel == "" {
			return "", errors.New("select a hosted provider preset")
		}
		if existing, found := s.cfg.Providers[preset.Nom]; found && existing.BaseURL != preset.BaseURL {
			return "", errors.New("custom endpoint retained; assign its roles manually")
		}
		oldProviders, oldRoles := s.cfg.Providers, s.cfg.Roles
		providers := make(map[string]config.Provider, len(oldProviders)+1)
		for n, p := range oldProviders {
			providers[n] = p
		}
		if _, found := providers[preset.Nom]; !found {
			providers[preset.Nom] = config.Provider{BaseURL: preset.BaseURL, APIKey: "${" + preset.EnvVar + "}"}
		}
		roles := make(map[string]config.Role, len(oldRoles))
		for n, r := range oldRoles {
			r.Provider, r.Model = preset.Nom, preset.DefaultModel
			r.ContextWindow, r.FallbackProvider, r.FallbackModel, r.Think = 0, "", "", ""
			roles[n] = r
		}
		s.cfg.Providers, s.cfg.Roles = providers, roles
		if _, found := oldProviders[preset.Nom]; !found {
			s.cfg.SetRawKey(preset.Nom, "${"+preset.EnvVar+"}")
		}
		if err := save(); err != nil {
			s.cfg.Providers, s.cfg.Roles = oldProviders, oldRoles
			return "", err
		}
		if _, found := oldProviders[preset.Nom]; !found {
			p := providers[preset.Nom]
			p.APIKey = os.Getenv(preset.EnvVar)
			providers[preset.Nom] = p
			s.cfg.SetRawKey(preset.Nom, "${"+preset.EnvVar+"}")
		}
		return "Provider preset selected: " + preset.Etiqueta, nil
	case "add":
		if !validProvName(name) {
			return "", fmt.Errorf("nom invàlid (lletres, digits, - i _)")
		}
		url = strings.TrimSuffix(strings.TrimSpace(url), "/")
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return "", fmt.Errorf("url ha de començar per http:// o https://")
		}
		if _, ok := s.cfg.Providers[name]; ok {
			return "", errors.New("ja existeix: " + name)
		}
		if s.cfg.Providers == nil {
			s.cfg.Providers = map[string]config.Provider{}
		}
		s.cfg.Providers[name] = config.Provider{BaseURL: url, APIKey: os.ExpandEnv(strings.TrimSpace(key))}
		s.cfg.SetRawKey(name, strings.TrimSpace(key))
		if err := save(); err != nil {
			return "", err
		}
		msg := "provider " + name + " desat"
		if ids, err := fetchModelIDs(url, os.ExpandEnv(strings.TrimSpace(key))); err == nil && len(ids) > 0 {
			msg += " — models: " + shortIDs(ids)
		}
		return msg, nil
	case "url":
		p, ok := s.cfg.Providers[name]
		if !ok {
			return "", errors.New("no existeix: " + name)
		}
		url = strings.TrimSuffix(strings.TrimSpace(url), "/")
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return "", errors.New("url ha de començar per http:// o https://")
		}
		p.BaseURL = url
		s.cfg.Providers[name] = p
		if err := save(); err != nil {
			return "", err
		}
		return name + " → " + url, nil
	case "rm":
		if _, ok := s.cfg.Providers[name]; !ok {
			return "", errors.New("no existeix: " + name)
		}
		for rn, r := range s.cfg.Roles {
			if r.Provider == name {
				return "", errors.New("en ús pel rol " + rn)
			}
		}
		delete(s.cfg.Providers, name)
		if err := save(); err != nil {
			return "", err
		}
		return "provider " + name + " esborrat", nil
	case "rol":
		r, ok := s.cfg.Roles[role]
		if !ok {
			return "", errors.New("rol desconegut: " + role)
		}
		if _, ok := s.cfg.Providers[provider]; !ok {
			return "", errors.New("provider desconegut: " + provider)
		}
		r.Provider = provider
		if strings.TrimSpace(model) != "" {
			r.Model = strings.TrimSpace(model)
		} else if ids, err := fetchModelIDs(s.cfg.Providers[provider].BaseURL, s.cfg.Providers[provider].APIKey); err == nil && len(ids) > 0 {
			found := false
			for _, id := range ids {
				if id == r.Model {
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("%s no té el model %s — disponibles: %s", provider, r.Model, shortIDs(ids))
			}
		}
		s.cfg.Roles[role] = r
		if err := save(); err != nil {
			return "", err
		}
		return fmt.Sprintf("rol %s → %s/%s", role, provider, r.Model), nil
	case "long_run":
		r, ok := s.cfg.Roles[role]
		if !ok {
			return "", errors.New("rol desconegut: " + role)
		}
		r.LongRun = &longRun
		s.cfg.Roles[role] = r
		if err := save(); err != nil {
			return "", err
		}
		if longRun {
			return "tirades llargues activades per al rol " + role, nil
		}
		return "tirades llargues desactivades per al rol " + role, nil
	case "autonomous_limits":
		if maxMinutes < 1 || maxMinutes > 10080 {
			return "", errors.New("durada invàlida (1–10080 minuts)")
		}
		if maxToolSteps < 1 || maxToolSteps > 10000 {
			return "", errors.New("nombre d'eines invàlid (1–10000)")
		}
		s.cfg.Agent.Autonomous.MaxMinutes = maxMinutes
		s.cfg.Agent.Autonomous.MaxToolSteps = maxToolSteps
		if err := save(); err != nil {
			return "", err
		}
		return fmt.Sprintf("mode autònom: %d minuts / %d eines", maxMinutes, maxToolSteps), nil
	case "key":
		p, ok := s.cfg.Providers[name]
		if !ok {
			return "", errors.New("no existeix: " + name)
		}
		raw := strings.TrimSpace(key)
		if raw == "-" {
			raw = ""
		}
		p.APIKey = os.ExpandEnv(raw)
		s.cfg.Providers[name] = p
		if strings.HasPrefix(raw, "${") || raw == "" {
			s.cfg.SetRawKey(name, raw)
			if err := save(); err != nil {
				return "", err
			}
			return "clau de " + name + " desada", nil
		}
		if err := save(); err != nil {
			return "", err
		}
		return "clau activa només en memòria (no desada al disc)", nil
	case "test":
		p, ok := s.cfg.Providers[name]
		if !ok {
			return "", errors.New("no existeix: " + name)
		}
		return testProviderURL(p.BaseURL, p.APIKey)
	default:
		return "", fmt.Errorf("acció desconeguda")
	}
}

// fetchModelIDs retorna els IDs de GET <base>/models.
func fetchModelIDs(base, key string) ([]string, error) {
	return llm.ProbeModels(base, key)
}

func shortIDs(ids []string) string { return llm.ShortIDs(ids) }

// testProviderURL fa GET <base>/models i resumeix (sense exposar la clau).
func testProviderURL(base, key string) (string, error) {
	ids, err := fetchModelIDs(base, key)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "respon (sense llista de models)", nil
	}
	return fmt.Sprintf("respon (%d models): %s", len(ids), shortIDs(ids)), nil
}

func validProvName(n string) bool {
	if n == "" {
		return false
	}
	for _, r := range n {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (s *Server) sysPrompt() string { return s.sysPromptAmb(s.mode, s.cwd) }

// sysPromptAmb és el system prompt per a un mode concret (un torn pot
// anar més restringit que la sessió: vegeu handleAgent). Rep el workspace
// efectiu del torn perquè el model vegi el projecte on realment treballa,
// no el de la sessió quan un client delega des d'un altre directori.
func (s *Server) sysPromptAmb(mode, workspace string) string {
	base := s.cfg.SystemPrompt()
	prompt := agent.PromptFor(base, mode)
	project := filepath.Base(filepath.Clean(workspace))
	prompt = fmt.Sprintf(`%s

FETS DE L'ENTORN (autoritatius, no els dedueixis):
- Directori de treball: %q
- Projecte actual: %q
- No tens cap llista implícita de fitxers, carpetes ni altres projectes.
- Treballa DINS del Directori de treball (glob/read/grep amb rutes
  relatives o d'aquest directori): no vagis a Downloads, Documents,
  Escriptori ni altres carpetes tret que l'usuari ho demani explícitament.

No inventis noms de projectes, fitxers, resultats ni contingut. Si t'han de
preguntar què hi ha al projecte, primer comprova-ho amb les eines disponibles
(glob/read/grep) i cita què has comprovat. En mode xat, si no pots inspeccionar
el disc, digues explícitament que no ho pots saber en lloc d'inventar-t'ho.`, prompt, workspace, project)
	if info := agent.ContextProjecte(workspace); info != "" {
		prompt += "\n\n" + info
	}
	if r := strings.TrimSpace(s.compacted); r != "" {
		prompt += "\n\n[Resum de la conversa anterior — continua amb aquest context]\n" + r
	}
	return prompt
}

// compactaSiCal resumeix la conversa quan l'estimació passa del llindar de
// la finestra i deixa només els darrers missatges. Es crida abans de cada
// torn: així el torn que ve ja hi cap, en comptes d'assabentar-nos que no
// hi cabia quan el model retalla o peta.
//
// Cal cridar-la SENSE el mutex: fa una crida al model. Si el resum falla no
// passa res —continuem amb la conversa llarga—, perquè quedar-se sense
// resum és molest i perdre el torn ho és molt més.
func (s *Server) compactaSiCal(ctx context.Context, p config.Provider, role config.Role, emet func(string, any)) {
	s.mu.Lock()
	convo := append([]llm.Message(nil), s.convo...)
	sys := s.sysPrompt()
	previ := s.compacted
	s.mu.Unlock()
	hist := append([]llm.Message{{Role: "system", Content: sys}}, convo...)
	if !agent.NeedsCompact(llm.EstimateTokens(hist), agent.Budget(s.cfg, role)) {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	font := append([]llm.Message(nil), convo...)
	if strings.TrimSpace(previ) != "" {
		font = append([]llm.Message{{Role: "user", Content: "Resum del context de les tasques anteriors:\n" + previ}}, font...)
	}
	resum, err := agent.Summarize(cctx, s.client, s.cfg.PrimTarget(p, role), s.cfg.FallbackTarget(role), font)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// El torn està serialitzat, però si mentrestant la conversa s'ha
	// escurçat (un rewind) no la retallem més: el resum ja no la descriu.
	if len(s.convo) < len(convo) {
		return
	}
	abans := len(s.convo)
	s.convo = agent.TrimKeep(s.convo, 4)
	if emet != nil {
		emet("compact", map[string]int{"abans": abans, "despres": len(s.convo)})
	}
	// El resum nou ja incorpora l'anterior. Substituir-lo manté el prompt
	// acotat al llarg de centenars de tasques i en reprendre sessions.
	if r := []rune(resum); len(r) > 12000 {
		resum = "[part més antiga del resum retallada]\n" + string(r[len(r)-12000:])
	}
	s.compacted = resum
}

func (s *Server) autoVerify() bool {
	return s.cfg.Verify.Mode == "auto" || s.cfg.Verify.Mode == "both" || s.cfg.Verify.Mode == "strict"
}

func (s *Server) transcriptTail(n int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.convo
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Role + ": " + m.Content + "\n")
	}
	return b.String()
}

// transcriptView retorna els últims n missatges user/assistant no buits per
// pintar l'historial en canviar de pestanya. Sense tool calls: només text.
func transcriptView(convo []llm.Message, n int) []map[string]string {
	if len(convo) > n {
		convo = convo[len(convo)-n:]
	}
	out := make([]map[string]string, 0, len(convo))
	for _, m := range convo {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		// El mapa del projecte és per al model: a la bombolla, el que va
		// escriure la persona.
		out = append(out, map[string]string{"role": m.Role, "content": agent.SenseMapa(m.Content)})
	}
	return out
}

func (s *Server) emitVerify(emit func(string, any), workspace string) string {
	emit("verify_start", map[string]string{})
	r := s.cfg.Roles["reviewer"]
	p := s.cfg.Providers[r.Provider]
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	v, raw, err := verify.Run(ctx, s.client, p, r, s.transcriptTail(6), tools.GitDiff(workspace))
	if err != nil {
		emit("verify", map[string]string{"verdict": "error", "detail": err.Error()})
		return "error"
	}
	vs := verdictStr(v)
	emit("verify", map[string]string{"verdict": vs, "detail": raw})
	return vs
}

// verdictStr resumeix el veredicte per la UI.
func verdictStr(v verify.Verdict) string {
	if v.Approved {
		return "APROVAT: " + v.Summary
	}
	return "CAL REVISAR: " + v.Summary
}

// --- agent ---

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Task   string   `json:"task"`
		Images []string `json:"images"`
		// Mode, opcional, pot restringir el torn (chat/inspect) o activar el
		// contracte autònom. Office continua enviant inspect explícitament.
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Task) == "" {
		http.Error(w, "tasca buida", 400)
		return
	}
	if len(req.Images) > 3 {
		http.Error(w, "màxim 3 imatges per torn", 400)
		return
	}
	s.mu.Lock()
	gated := s.gated
	s.mu.Unlock()
	if gated {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusLocked)
		w.Write([]byte(`{"gated":true,"error":"torn bloquejat: el revisor ha dit CAL REVISAR. Aprova'l o comença de nou (/nova)."}`))
		return
	}
	imgs := make([]string, 0, len(req.Images))
	for i, u := range req.Images {
		n, err := tools.NormalizeDataURL(u, fmt.Sprintf("imatge %d", i+1))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		imgs = append(imgs, n)
	}
	// Cortesia pura ("hola") es respon sense worker ni model: no ocupa el
	// torn, no fa 409 i no crema passos. L'add-in també hi passa.
	if reply, ok := agent.SmallTalk(req.Task); ok && len(imgs) == 0 {
		task := strings.TrimSpace(req.Task)
		s.mu.Lock()
		s.convo = append(s.convo, llm.Message{Role: "user", Content: task})
		s.convo = append(s.convo, llm.Message{Role: "assistant", Content: reply})
		tools.Active.MarkConvoFor(s.id, len(s.convo))
		s.mu.Unlock()
		emit := sse(w)
		emit("assistant", map[string]string{"text": reply})
		emit("done", map[string]string{"reply": reply})
		s.autosave()
		return
	}
	// El torn passa per la cua compartida del servei: si la sessió ja
	// treballa, la petició espera el seu torn amb l'SSE en pausa (el
	// heartbeat manté la connexió viva) en comptes de rebre un 409. Tancar
	// el navegador mentre s'encua cancel·la només l'espera del missatge,
	// mai el torn que un altre client està seguint.
	rawEmit, stopHeartbeat := sseWithHeartbeat(w, r.Context(), 15*time.Second)
	defer stopHeartbeat()
	emit := func(event string, value any) {
		s.recordActive(event, value)
		rawEmit(event, value)
	}
	started := make(chan struct{})
	exec := s.turnExecutor(turnOpts{
		task: strings.TrimSpace(req.Task), imgs: imgs, mode: req.Mode,
		emit: emit, rawEmit: rawEmit,
		onStart: func() { close(started) },
	})
	run, _, err := s.Hub().Queue().Submit(exec, runs.Run{
		Session: s.id, Workspace: runs.WorkspaceKey(s.cwd),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}
	done := s.Hub().Queue().Done(run.ID)
	s.appendRunEvent(run, "run_queued", "torn encuat")
	s.watchRun(run)
	select {
	case <-started:
	case <-r.Context().Done():
		s.Hub().Queue().Cancel(run.ID)
		return
	}
	<-done
}

// turnOpts és el torn que la cua ha d'executar: les dades de la petició i
// per on han de sortir els events.
type turnOpts struct {
	task string
	imgs []string
	mode string
	// workspace és el directori on corre el torn (eines, mapa, verificació).
	// Buit = el de la sessió en engegar el torn.
	workspace string
	// emit grava a la sessió (i al registre durable via recordActive).
	emit func(string, any)
	// rawEmit és la sortida SSE nua per als events de negociació (compacta)
	// que no han de quedar a l'activitat.
	rawEmit func(string, any)
	onStart func()
}

// turnExecutor embolcalla el bucle d'agent perquè el worker de la cua l'executi:
// marca la sessió com a ocupada, registra el torn perquè la cancel·lació el
// trobi, i tradueix pànic i cancel·lació a l'estat de la cua.
func (s *Server) turnExecutor(o turnOpts) func(ctx context.Context, run runs.Run) error {
	return func(ctx context.Context, run runs.Run) error {
		if o.onStart != nil {
			o.onStart()
		}
		s.appendRunEvent(run, "run_started", "torn iniciat")
		// Cortesia pura ("hola"): resposta local, sense model ni eines. A la
		// via SSE antiga es resol abans d'encuar; aquí el torn ja és a la cua
		// i simplement no gasta passos ni crides.
		if reply, ok := agent.SmallTalk(o.task); ok && len(o.imgs) == 0 {
			s.mu.Lock()
			s.convo = append(s.convo, llm.Message{Role: "user", Content: o.task})
			s.convo = append(s.convo, llm.Message{Role: "assistant", Content: reply})
			tools.Active.MarkConvoFor(s.id, len(s.convo))
			s.mu.Unlock()
			s.Hub().appendEvent(events.Event{SessionID: s.id, User: s.usuari(), RunID: int(run.ID), Kind: "text", Text: reply})
			s.Hub().appendEvent(events.Event{SessionID: s.id, User: s.usuari(), RunID: int(run.ID), Kind: "done", Text: reply})
			return nil
		}
		// El workspace es resol en engegar el torn, no en encuar-lo: la
		// sessió pot haver canviat de projecte mentre esperava a la cua.
		// Buit = el de la sessió (el cas de la web i del handler antic).
		s.mu.Lock()
		ws := o.workspace
		if ws == "" {
			ws = s.cwd
		}
		s.mu.Unlock()
		// Vista /api/active i busy de sessió: mateix contracte que abans de
		// la cua, però ara el fixa el worker i no el handler HTTP.
		s.mu.Lock()
		s.agentBusy = true
		s.todo = nil
		s.active = &activeAgent{
			ID: int(run.ID), Task: o.task, StartedAt: time.Now(),
			Role: s.role, Mode: s.mode, Project: filepath.Base(ws),
		}
		s.tokenBuf = ""
		s.tokenAt = time.Now()
		s.mu.Unlock()
		s.setActiveRun(run.ID)
		var execErr error
		func() {
			defer func() {
				// Un pànic al torn (eina, provider, JSON) no tomba el servidor
				// ni deixa el worker encallat: s'emet error i queda failed.
				if rec := recover(); rec != nil {
					o.emit("error", map[string]string{"message": fmt.Sprintf("torn interromput: %v", rec)})
					o.emit("done", map[string]string{"reply": "El torn s'ha interromput per un error intern. Pots tornar-ho a provar."})
					execErr = fmt.Errorf("torn interromput: %v", rec)
				}
			}()
			// La resposta ja ha sortit per emit dins el bucle: aquí només
			// interessa l'error, que marca l'execució com a fallida.
			_, execErr = s.runTurn(ctx, o.task, o.imgs, o.mode, ws, o.emit, o.rawEmit)
		}()
		s.mu.Lock()
		if s.active != nil && s.active.ID == int(run.ID) {
			s.flushTokensLocked()
			s.active = nil
		}
		s.agentBusy = false
		s.mu.Unlock()
		s.setActiveRun(0)
		// La cancel·lació mana sobre qualsevol error del bucle: el client que
		// ha premut Atura no ha de veure «fallit».
		if ctx.Err() != nil {
			return runs.ErrCancelled
		}
		return execErr
	}
}

// runTurn és el bucle model→eines del torn. Viu al worker de la cua: emet
// per emit (activitat de sessió, i SSE si el client és del web) i amb
// rawEmit per als events que no han de quedar gravats. Torna l'última
// resposta i l'error que marca l'execució com a fallida a la cua (error del
// proveïdor a mig torn, cancel·lació).
func (s *Server) runTurn(runCtx context.Context, task string, imgs []string, reqMode, workspace string, emit, rawEmit func(string, any)) (string, error) {
	s.mu.Lock()
	roleName := s.role
	pinned := s.rolePinned
	mode := s.mode
	// El workspace del torn mana sobre el de la sessió: és el directori
	// que el client delegat ha demanat (validat al submit) o el de la
	// sessió. La política es copia amb el ProjectDir del torn perquè les
	// escriptures dins del projecte passin igual que en un torn local.
	ws := workspace
	if ws == "" {
		ws = s.cwd
	}
	var pol *agent.Policy
	if s.policy != nil {
		cp := *s.policy
		cp.ProjectDir = ws
		pol = &cp
	}
	s.mu.Unlock()
	if reqMode == agent.ModeChat || reqMode == agent.ModeInspect || reqMode == agent.ModeAutonomous {
		mode = reqMode
	}
	roleWhy := ""
	if !pinned {
		if routed, why := s.cfg.Route(task, len(imgs) > 0, roleName); routed != roleName {
			roleName, roleWhy = routed, why
		}
	}
	if roleWhy != "" {
		emit("route", map[string]string{"message": roleWhy, "role": roleName})
	}
	if selection, unavailable := s.refreshModelAvailability(roleName); unavailable {
		fallback := s.cfg.Roles[roleName]
		emit("status", map[string]string{"message": fmt.Sprintf("Model %s no està disponible; faré servir %s/%s.", selection, fallback.Provider, fallback.Model)})
	}
	s.mu.Lock()
	pComp, roleComp := s.roleRefFor(roleName)
	s.mu.Unlock()
	if mode == agent.ModeAutonomous && !roleComp.LongRunsAllowed() {
		msg := fmt.Sprintf("el model %s/%s no està habilitat per a tirades llargues; activa «long_run» al perfil del rol", roleComp.Provider, roleComp.Model)
		emit("error", map[string]string{"message": msg})
		return "", errors.New(msg)
	}
	s.compactaSiCal(runCtx, pComp, roleComp, rawEmit)
	// Mapa del projecte (fitxers, git, fitxers que la tasca anomena): el
	// model no ha de gastar les primeres voltes a orientar-se. Fora del
	// mutex: executa git, i la UI consulta la sessió cada cinc segons.
	s.mu.Lock()
	cwdMapa, convoMapa := ws, append([]llm.Message(nil), s.convo...)
	s.mu.Unlock()
	mapa := agent.MapaProjecte(cwdMapa, task, convoMapa)
	if mode == agent.ModeChat {
		mapa = ""
	}
	s.mu.Lock()
	p, role := s.roleRefFor(roleName)
	maxSteps := s.cfg.Agent.MaxSteps
	content := s.expandMentions(task, ws)
	if mapa != "" {
		content += "\n\n" + mapa
	}
	s.convo = append(s.convo, llm.Message{Role: "user", Content: content, Images: imgs})
	// La tasca apareix a Converses de seguida, fins i tot si dura hores o
	// l'usuari canvia de pestanya abans de la resposta final.
	s.autosaveLocked()
	s.mu.Unlock()

	// El bucle viu al motor compartit (agent.Torn), com al TUI, Telegram
	// i el headless: conduirTorn (motor_web.go) només en fa l'E/S.
	// Finestra del model segons el proveïdor (cache de deu minuts).
	dctx, dcancel := context.WithTimeout(runCtx, 5*time.Second)
	agent.DetectRoleWindows(dctx, s.cfg, role)
	dcancel()
	lastReply, hist, err := s.conduirTorn(runCtx, task, mode, roleName, p, role, maxSteps, ws, pol, emit)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	if strings.TrimSpace(lastReply) != "" {
		s.convo = append(s.convo, llm.Message{Role: "assistant", Content: lastReply})
	}
	s.mu.Unlock()
	if mode == agent.ModeGoal {
		s.emitGoal(emit, lastReply)
	}
	emit("done", map[string]string{"reply": lastReply})
	s.autosave()
	verdict := ""
	if s.autoVerify() {
		verdict = s.emitVerify(emit, ws)
	}
	s.mu.Lock()
	tools.Active.MarkConvoFor(s.id, len(s.convo))
	s.mu.Unlock()
	s.trackTurnCost(emit, hist, roleName, role)
	s.trackVerdict(emit, verdict, roleName)
	return lastReply, nil
}

// trackTurnCost suma el cost del torn al pressupost de sessió i avisa una
// sola vegada en superar budget.session_usd. Sense preu al config no hi ha
// res a sumar (mai s'inventa).
func (s *Server) trackTurnCost(emit func(string, any), hist []llm.Message, roleName string, role config.Role) {
	_, _, usd, ok := agent.HistCost(hist, role.Provider, role.Model)
	if !ok || usd <= 0 {
		return
	}
	limit := s.cfg.Budget.SessionUSD
	s.mu.Lock()
	s.sessCost += usd
	total, warned := s.sessCost, s.budgetWarned
	if limit > 0 && total >= limit {
		s.budgetWarned = true
	}
	s.mu.Unlock()
	if limit > 0 && total >= limit && !warned {
		emit("budget", map[string]string{
			"total": agent.FmtCost(total), "limit": agent.FmtCost(limit),
			"message": "Pressupost de sessió superat (" + agent.FmtCost(total) + " ≥ " + agent.FmtCost(limit) + "). La feina continua: és un avís, no un bloqueig.",
		})
	}
}

// trackVerdict compta CAL REVISAR seguits i escala al rol fort quan toca.
// APROVAT trenca la ratxa; errors del revisor no compten.
func (s *Server) trackVerdict(emit func(string, any), verdict, roleName string) {
	if verdict == "" || verdict == "error" {
		return
	}
	s.mu.Lock()
	if strings.HasPrefix(verdict, "APROVAT") {
		s.streak = 0
		s.mu.Unlock()
		return
	}
	s.streak++
	if s.cfg.Verify.Mode == "strict" {
		s.gated = true
	}
	target, ok := s.cfg.EscalateTarget(s.streak, s.rolePinned, roleName)
	if ok {
		s.role = target
		s.streak = 0
	}
	s.mu.Unlock()
	if s.cfg.Verify.Mode == "strict" && strings.HasPrefix(verdict, "CAL REVISAR") {
		emit("gated", map[string]string{
			"message": "Revisor insatisfet: el pròxim torn d'agent queda bloquejat fins que l'aprovis."})
	}
	if ok {
		emit("escalate", map[string]string{"from": roleName, "to": target,
			"message": "Revisor insatisfet repetidament: el pròxim torn usa el rol " + target + "."})
	}
}

// handleVerifyApprove obre la porta tancada per strict (C2b).
// approve=true: continua; false: també desbloqueja (l'usuari assumeix la
// revisió manualment). En tots dos casos queda registrat a l'activitat via
// l'event corresponent del frontend.
func (s *Server) handleVerifyApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Approve bool `json:"approve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	s.mu.Lock()
	was := s.gated
	s.gated = false
	s.mu.Unlock()
	if !was {
		writeJSON(w, map[string]string{"unlocked": "false", "note": "no hi havia cap bloqueig"})
		return
	}
	if req.Approve {
		writeJSON(w, map[string]string{"unlocked": "true", "note": "revisió aprovada: pots continuar"})
	} else {
		writeJSON(w, map[string]string{"unlocked": "true", "note": "bloqueig aixecat sense aprovar: revisa-ho tu"})
	}
}

// truncConvoWeb retalla l'historial a n (E2b; -1 = sense info).
func truncConvoWeb(convo []llm.Message, n int) []llm.Message {
	if n < 0 || n > len(convo) {
		return convo
	}
	return convo[:n]
}

func toolMsg(c llm.ToolCall, out string, images ...string) llm.Message {
	return agent.ToolMsg(c, out, images...)
}

// webRememberScope és l'àmbit dels "sempre en aquesta sessió" del servidor
// (un sol worker: la sessió activa).
const webRememberScope = "web"

// approvalReq és una aprovació pendent: canal de resposta + signatura per
// poder-la recordar ("sempre en aquesta sessió").
type approvalReq struct {
	ch  chan bool
	sig string
}

// fetsSessio llegeix els fets del checklist propi de la sessió (amb
// mutex): el global no serveix amb diverses pestanyes.
func (s *Server) fetsSessio() (done, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return tools.StatsTodos(s.todo)
}

// execTool és execToolCtx sense torn al darrere (proves i crides soltes).
func (s *Server) execTool(c llm.ToolCall) (string, []string) {
	s.mu.Lock()
	ws := s.cwd
	s.mu.Unlock()
	return s.execToolCtx(context.Background(), c, ws)
}

// execToolCtx executa una eina amb el context del torn: aturar el torn
// atura també l'eina a mitges (un go test de dos minuts), no només la
// crida següent al model.
func (s *Server) execToolCtx(ctx context.Context, c llm.ToolCall, workspace string) (string, []string) {
	// La checklist és de la SESSIÓ, no del procés. El magatzem de tools és
	// un de sol i amb dues pestanyes es trepitjaven: la que començava un
	// torn esborrava la llista de la que estava treballant, i un todoread
	// podia tornar la checklist de l'altra conversa.
	switch c.Function.Name {
	case "todowrite":
		var tw struct {
			Items []tools.TodoItem `json:"items"`
		}
		if err := json.Unmarshal([]byte(c.Function.Arguments), &tw); err != nil {
			return "ERROR: " + err.Error(), nil
		}
		nets := tools.NetejaTodos(tw.Items)
		s.mu.Lock()
		s.todo = nets
		s.mu.Unlock()
		return tools.RenderTodos(nets), nil
	case "todoread":
		s.mu.Lock()
		llista := append([]tools.TodoItem(nil), s.todo...)
		s.mu.Unlock()
		return tools.RenderTodos(llista), nil
	}
	// Amb l'id de la sessió: així els processos de segon pla surten al
	// Terminal d'aquesta pestanya i es maten quan es tanca.
	out, imgs, err := agent.ExecCtx(ctx, s.id, workspace, c.Function.Name, c.Function.Arguments)
	if err != nil {
		return "ERROR: " + err.Error(), nil
	}
	return out, imgs
}

// esperaResposta és el temporitzador d'una aprovació o pregunta: el canal
// no dispara mai si no hi ha límit (config agent.approval_timeout_s).
func (s *Server) esperaResposta() (<-chan time.Time, func(), int) {
	d := s.cfg.ApprovalTimeout()
	if d <= 0 {
		return nil, func() {}, 0
	}
	t := time.NewTimer(d)
	return t.C, func() { t.Stop() }, int(d / time.Second)
}

// waitApproval demana permís a la UI. Torna (permès, timeout): si l'usuari
// no respon a temps es denega PERÒ s'emet approve_timeout perquè la UI ho
// mostri ("no has respost", no "ho has denegat"). Aturar el torn també la
// talla: abans el torn cancel·lat quedava penjat fins al límit.
func (s *Server) waitApproval(ctx context.Context, emit func(string, any), id, name, args, sig string, autoApproveAllowed bool) (bool, bool) {
	s.mu.Lock()
	s.apSeq++
	key := fmt.Sprintf("%d", s.apSeq)
	ch := make(chan bool, 1)
	s.approvals[key] = approvalReq{ch: ch, sig: sig}
	s.mu.Unlock()
	limit, para, segons := s.esperaResposta()
	defer para()
	// timeout_s és text pel contracte existent; auto_approve_allowed és booleà perquè
	// el client només aprovi automàticament quan el servidor ho autoritza.
	emit("approve_request", map[string]any{"key": key, "call_id": id, "name": name, "args": args, "timeout_s": strconv.Itoa(segons), "auto_approve_allowed": autoApproveAllowed})
	oblida := func() {
		s.mu.Lock()
		delete(s.approvals, key)
		s.mu.Unlock()
	}
	select {
	case ok := <-ch:
		return ok, false
	case <-ctx.Done():
		oblida()
		return false, false
	case <-limit:
		oblida()
		emit("approve_timeout", map[string]string{"call_id": id, "name": name})
		return false, true
	}
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key      string `json:"key"`
		Approve  bool   `json:"approve"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	s.mu.Lock()
	ar, ok := s.approvals[req.Key]
	if ok {
		delete(s.approvals, req.Key)
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "aprovació caducada o inexistent", 404)
		return
	}
	if req.Remember && req.Approve {
		s.remembers.Allow(webRememberScope, ar.sig)
	}
	ar.ch <- req.Approve
	writeJSON(w, map[string]bool{"ok": true})
}

// questionReq és una pregunta seleccionable pendent (eina question).
type questionReq struct {
	ch chan string
}

// waitQuestion emet la pregunta a la UI i espera la tria (el mateix límit
// que les aprovacions). Torna (resposta, timeout): resposta és el label
// triat o "text:<lliure>".
func (s *Server) waitQuestion(ctx context.Context, emit func(string, any), id, query string, opts []tools.QuestionOption) (string, bool) {
	s.mu.Lock()
	s.apSeq++
	key := fmt.Sprintf("q%d", s.apSeq)
	ch := make(chan string, 1)
	s.questions[key] = questionReq{ch: ch}
	s.mu.Unlock()
	labels := make([]map[string]string, 0, len(opts))
	for _, o := range opts {
		labels = append(labels, map[string]string{"label": o.Label, "description": o.Description})
	}
	limit, para, segons := s.esperaResposta()
	defer para()
	emit("question_request", map[string]any{"key": key, "call_id": id, "query": query, "options": labels, "timeout_s": strconv.Itoa(segons)})
	oblida := func() {
		s.mu.Lock()
		delete(s.questions, key)
		s.mu.Unlock()
	}
	select {
	case ans := <-ch:
		return ans, false
	case <-ctx.Done():
		oblida()
		return "", false
	case <-limit:
		oblida()
		emit("question_timeout", map[string]string{"call_id": id})
		return "", true
	}
}

func (s *Server) handleQuestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key    string `json:"key"`
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Key) == "" {
		http.Error(w, "petició il·legible", 400)
		return
	}
	s.mu.Lock()
	qr, ok := s.questions[req.Key]
	if ok {
		delete(s.questions, req.Key)
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "pregunta caducada o inexistent", 404)
		return
	}
	qr.ch <- req.Answer
	writeJSON(w, map[string]bool{"ok": true})
}

// permissiveNow llegeix el mode permissiu (amb mutex).
func (s *Server) permissiveNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.permissive
}

// handlePermissive: GET → {on}; POST {on} → activa/desactiva el mode
// permissiu (l'agent tira sense demanar; deny continua bloquejant).
// Només sessió: es perd en reiniciar el servidor.
func (s *Server) handlePermissive(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		writeJSON(w, map[string]bool{"on": s.permissiveNow()})
		return
	}
	var req struct {
		On bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	s.mu.Lock()
	s.permissive = req.On
	s.mu.Unlock()
	writeJSON(w, map[string]bool{"on": req.On})
}

// --- controls ---

func (s *Server) handleMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	if !agent.ValidMode(req.Mode) {
		http.Error(w, "mode invàlid (code|inspect|chat|goal|autonomous)", 400)
		return
	}
	s.mu.Lock()
	s.mode = req.Mode
	s.cfg.Mode = req.Mode
	err := s.cfg.Save(s.cfgPath)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, "mode aplicat però no desat: "+err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"mode": req.Mode})
}

// handleLang canvia l'idioma. És d'aplicació, no de sessió: decideix en
// quina llengua respon l'agent, i tenir dues pestanyes responent diferent
// seria més confusió que comoditat.
func (s *Server) handleLang(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lang string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	ok := false
	for _, l := range config.LangsSuportats {
		if req.Lang == l {
			ok = true
		}
	}
	if !ok {
		http.Error(w, "idioma invàlid ("+strings.Join(config.LangsSuportats, "|")+")", 400)
		return
	}
	s.mu.Lock()
	old := s.cfg.Language
	s.cfg.Language = req.Lang
	err := s.cfg.Save(s.cfgPath)
	if err != nil {
		s.cfg.Language = old
	}
	s.mu.Unlock()
	if err != nil {
		http.Error(w, "idioma no desat: "+err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "lang": req.Lang})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	switch req.Mode {
	case "off", "manual", "auto", "both", "strict":
	default:
		http.Error(w, "verify invàlid (off|manual|auto|both|strict)", 400)
		return
	}
	s.mu.Lock()
	old := s.cfg.Verify.Mode
	s.cfg.Verify.Mode = req.Mode
	err := s.cfg.Save(s.cfgPath)
	if err != nil {
		s.cfg.Verify.Mode = old
	}
	s.mu.Unlock()
	if err != nil {
		http.Error(w, "verify no desat: "+err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"mode": req.Mode})
}

func (s *Server) handleRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// "auto" torna la tria al router. Sense això, triar un rol una vegada
	// l'apagava per a la resta de la sessió i no hi havia manera de
	// desfer-ho: un camí d'anada sense tornada amagat dins d'un desplegable.
	if req.Role == "auto" {
		s.rolePinned = false
		s.role = s.cfg.InitialRole()
		writeJSON(w, map[string]any{"role": s.role, "pinned": false})
		return
	}
	if _, ok := s.cfg.Roles[req.Role]; !ok {
		http.Error(w, "rol desconegut", 400)
		return
	}
	s.role = req.Role
	s.rolePinned = true
	writeJSON(w, map[string]any{"role": req.Role, "pinned": true})
}

func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Model) == "" {
		http.Error(w, "model buit", http.StatusBadRequest)
		return
	}

	// L'app només pot seleccionar un model que el provider anunciï ara. Això
	// impedeix guardar un override inventat que fallaria més tard al primer torn.
	raw := strings.TrimSpace(req.Model)
	s.mu.Lock()
	roleName := s.role
	role, ok := s.cfg.Roles[roleName]
	providerName, modelName := role.Provider, raw
	if p, m, hasProvider := splitModel(raw); hasProvider {
		providerName, modelName = p, m
	}
	provider, found := s.cfg.Providers[providerName]
	s.mu.Unlock()
	if !ok || !found || strings.TrimSpace(modelName) == "" {
		http.Error(w, "provider o model desconegut", http.StatusBadRequest)
		return
	}
	ids, err := fetchModelIDs(provider.BaseURL, provider.APIKey)
	if err != nil {
		http.Error(w, "no puc validar els models del provider: "+err.Error(), http.StatusBadGateway)
		return
	}
	valid := false
	for _, id := range ids {
		if id == modelName {
			valid = true
			break
		}
	}
	if !valid {
		http.Error(w, "model no disponible al provider", http.StatusBadRequest)
		return
	}
	selection := providerName + "/" + modelName
	s.mu.Lock()
	if err := session.SaveModelDefault(s.usuari(), roleName, selection); err != nil {
		s.mu.Unlock()
		http.Error(w, "no puc desar la preferència de model: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.modelOverride[roleName] = selection
	delete(s.unavailableModels, roleName)
	delete(s.modelCheckedAt, roleName)
	s.autosaveLocked()
	s.mu.Unlock()
	writeJSON(w, map[string]string{"provider": providerName, "model": modelName, "selection": selection})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	type prov struct {
		name, base, key string
	}
	var provs []prov
	for n, p := range s.cfg.Providers {
		provs = append(provs, prov{n, p.BaseURL, p.APIKey})
	}
	sort.Slice(provs, func(i, j int) bool { return provs[i].name < provs[j].name })
	ov, hasOv := s.modelOverride[s.role]
	role := s.role
	s.mu.Unlock()

	// Llistat en paral·lel (cada provider té el seu timeout de 10 s).
	type res struct {
		name string
		ids  []string
		err  string
	}
	out := make(chan res, len(provs))
	for _, pr := range provs {
		go func(pr prov) {
			if strings.TrimSpace(pr.key) == "" {
				for _, preset := range config.Cataleg() {
					if !preset.Local && strings.TrimSuffix(pr.base, "/") == preset.BaseURL {
						out <- res{name: pr.name, err: "API key required"}
						return
					}
				}
			}
			ids, err := fetchModelIDs(pr.base, pr.key)
			r := res{name: pr.name, ids: ids}
			if err != nil {
				r.err = err.Error()
			}
			out <- r
		}(pr)
	}
	models := map[string][]string{}
	errs := map[string]string{}
	for range provs {
		r := <-out
		models[r.name] = r.ids
		if r.err != "" {
			errs[r.name] = r.err
		}
	}
	cur := ""
	if r, ok := s.cfg.Roles[role]; ok {
		cur = r.Provider + "/" + r.Model
	}
	fallback := cur
	selected := ""
	var selectedAvailable *bool
	if hasOv {
		selected = ov
		cur = ov
		if selectedProvider, _, ok := splitModel(ov); ok && errs[selectedProvider] == "" {
			available := modelSelectionAvailable(ov, models)
			selectedAvailable = &available
			s.mu.Lock()
			if s.modelOverride[role] == ov {
				if s.unavailableModels == nil {
					s.unavailableModels = map[string]string{}
				}
				if s.modelCheckedAt == nil {
					s.modelCheckedAt = map[string]time.Time{}
				}
				if available {
					delete(s.unavailableModels, role)
				} else {
					s.unavailableModels[role] = ov
				}
				s.modelCheckedAt[role] = time.Now()
			}
			s.mu.Unlock()
			if available {
				cur = ov
			} else {
				cur = fallback
			}
		} else if _, _, ok := splitModel(ov); ok {
			s.mu.Lock()
			knownUnavailable := s.unavailableModels[role] == ov
			s.mu.Unlock()
			if knownUnavailable {
				unavailable := false
				selectedAvailable = &unavailable
				cur = fallback
			}
		}
	}
	writeJSON(w, map[string]any{
		"models": models, "errors": errs, "current": cur, "role": role,
		"selected_override": selected, "selected_available": selectedAvailable,
		"fallback": fallback,
	})
}

func modelSelectionAvailable(selection string, models map[string][]string) bool {
	provider, model, ok := splitModel(selection)
	if !ok || strings.TrimSpace(model) == "" {
		return false
	}
	for _, id := range models[provider] {
		if id == model {
			return true
		}
	}
	return false
}

// handleSessions llista les converses desades (GET) i n'esborra una
// (DELETE ?name=). El títol i la marca de temps en RFC3339 són el que la
// UI necessita per fer la llista de l'esquerra estil ChatGPT: agrupada per
// data i amb un nom que digui de què anava.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "cal name", 400)
			return
		}
		if err := session.Delete(s.sessDir(), name); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		s.mu.Lock()
		if s.convoFile == name {
			s.convoFile = "" // ja no escrivim en una conversa esborrada
		}
		s.mu.Unlock()
		writeJSON(w, map[string]string{"deleted": name})
		return
	}
	infos, err := session.List(s.sessDir())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.mu.Lock()
	current := s.convoFile
	s.mu.Unlock()
	type row struct {
		Name      string `json:"name"`
		Title     string `json:"title"`
		Msgs      int    `json:"msgs"`
		When      string `json:"when"`
		At        string `json:"at"`
		Workspace string `json:"workspace,omitempty"`
		Current   bool   `json:"current,omitempty"`
		Pinned    bool   `json:"pinned,omitempty"`
	}
	out := []row{}
	for _, in := range infos {
		title := in.Title
		if title == "" {
			title = in.Name
		}
		out = append(out, row{
			Name:      in.Name,
			Title:     title,
			Msgs:      in.Msgs,
			When:      in.SavedAt.Format("02-01 15:04"),
			At:        in.SavedAt.Format(time.RFC3339),
			Workspace: in.Workspace,
			Current:   in.Name == current,
			Pinned:    in.Pinned,
		})
	}
	writeJSON(w, out)
}

// handlePinSession changes navigation metadata without creating a second
// conversation file. It is deliberately separate from DELETE and resume.
func (s *Server) handlePinSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name   string `json:"name"`
		Pinned bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "cal name", http.StatusBadRequest)
		return
	}
	// Coordina lectura, actualització i escriptura amb els autosaves de totes
	// les pestanyes vives que poden apuntar al mateix fitxer. Si escrivíssim
	// primer i propaguéssim el bool després, un autosave concurrent podria
	// tornar a posar el valor antic.
	var locked []*Server
	if h := s.hub; h != nil {
		h.mu.RLock()
		keys := make([]string, 0, len(h.sess))
		for key, candidate := range h.sess {
			if sameUserScope(candidate.user, s.user) {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			candidate := h.sess[key]
			candidate.mu.Lock()
			locked = append(locked, candidate)
		}
		h.mu.RUnlock()
	} else {
		s.mu.Lock()
		locked = append(locked, s)
	}
	unlock := func() {
		for _, candidate := range locked {
			candidate.mu.Unlock()
		}
	}
	sess, err := session.Load(s.sessDir(), req.Name)
	if err != nil {
		unlock()
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	sess.Pinned = req.Pinned
	if _, err := session.SaveSession(s.sessDir(), sess); err != nil {
		unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, candidate := range locked {
		if candidate.convoFile == sess.Name {
			candidate.pinned = sess.Pinned
		}
	}
	unlock()
	writeJSON(w, map[string]any{"name": sess.Name, "pinned": sess.Pinned})
}

func sameUserScope(a, b *User) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Name == b.Name
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.convo) == 0 {
		http.Error(w, "res a desar", 400)
		return
	}
	path, err := session.SaveSession(s.sessDir(), session.Session{
		Name:           req.Name,
		Role:           s.role,
		Pinned:         s.pinned,
		Workspace:      s.cwd,
		Compacted:      s.compacted,
		ModelOverrides: cloneModelOverrides(s.modelOverride),
		Convo:          s.convo,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]string{"path": path})
}

// handleRewind desfés els canvis de fitxers de la sessió (journal global).
func (s *Server) handleRewind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	// G2: amb diverses sessions vives el journal és compartit; desfem
	// només els fitxers del workspace d'aquesta sessió.
	out, err := tools.Active.RewindFiltered(0, s.rewindFilter())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.mu.Lock()
	s.convo = truncConvoWeb(s.convo, tools.Active.ConvoLenAtFor(s.id, 0))
	s.mu.Unlock()
	if strings.TrimSpace(out) == "" {
		out = "res a desfer"
	}
	writeJSON(w, map[string]string{"summary": out})
}

// handleCheckpoints llista els checkpoints per pas (un per escriptura).
func (s *Server) handleCheckpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "només GET", 405)
		return
	}
	type view struct {
		Seq  int    `json:"seq"`
		Op   string `json:"op"`
		Path string `json:"path"`
		At   string `json:"at"`
	}
	cps := tools.Active.Checkpoints()
	out := make([]view, 0, len(cps))
	for _, e := range cps {
		if f := s.rewindFilter(); f != nil && !f(e.Path) {
			continue
		}
		out = append(out, view{Seq: e.Seq, Op: e.Op, Path: e.Path, At: e.At.Format("15:04:05")})
	}
	writeJSON(w, out)
}

// handleRewindTo torna al checkpoint seq ({"seq": N}).
func (s *Server) handleRewindTo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Seq *int `json:"seq"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Seq == nil {
		http.Error(w, "cal {\"seq\": N} (vegeu /api/checkpoints)", 400)
		return
	}
	out, err := tools.Active.RewindFiltered(*req.Seq, s.rewindFilter())
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.mu.Lock()
	s.convo = truncConvoWeb(s.convo, tools.Active.ConvoLenAtFor(s.id, *req.Seq))
	s.mu.Unlock()
	writeJSON(w, map[string]string{"summary": out})
}

// resetSessionCounters neteja ratxa, cost i avís (sessió nova o reprsa).
func (s *Server) resetSessionCounters() {
	s.gated = false
	s.streak = 0
	s.sessCost = 0
	s.budgetWarned = false
}

// autosave desa la conversa sense que ningú ho demani, en acabar cada torn.
// Abans només es desava clicant «Nova sessió»: si tancaves l'app, o
// senzillament seguies treballant, la conversa no existia enlloc i la llista
// de l'esquerra era una sola entrada anònima. Ara cada conversa és un fitxer
// amb títol, com a ChatGPT o Claude.
//
// Cal tenir s.mu.
func (s *Server) autosaveLocked() {
	if len(s.convo) == 0 {
		return
	}
	name := s.convoFile
	if name == "" {
		name = session.NewName()
	}
	path, err := session.SaveSession(s.sessDir(), session.Session{
		Name:           name,
		Role:           s.role,
		Pinned:         s.pinned,
		Workspace:      s.cwd,
		Compacted:      s.compacted,
		ModelOverrides: cloneModelOverrides(s.modelOverride),
		Convo:          s.convo,
	})
	if err != nil {
		return // desar és millor-esforç: mai no ha de tombar un torn
	}
	s.convoFile = strings.TrimSuffix(filepath.Base(path), ".json")
}

// autosave agafa el mutex i desa (per cridar-lo des de fora d'una secció
// crítica, en acabar el torn).
func (s *Server) autosave() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autosaveLocked()
}

// handleNew desa la conversa actual (si n'hi ha) i neteja per començar-ne una.
func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	if s.hasActiveTurn() {
		http.Error(w, "aquesta sessió encara té una tasca en marxa; obre una sessió nova", http.StatusConflict)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// La conversa ja es desa sola a cada torn: aquí només cal tancar-la
	// (deixar d'escriure al seu fitxer) i començar-ne una de nova. Abans
	// aquest desat en creava una còpia amb un nom nou cada vegada.
	saved := ""
	if len(s.convo) > 0 {
		s.autosaveLocked()
		saved = s.convoFile
	}
	s.convo = nil
	s.compacted = ""
	s.convoFile = ""
	s.pinned = false
	s.modelOverride = s.defaultModelOverrides()
	s.unavailableModels = map[string]string{}
	s.modelCheckedAt = map[string]time.Time{}
	s.remembers.Clear(webRememberScope)
	s.resetSessionCounters()
	writeJSON(w, map[string]string{"saved": saved})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	if s.hasActiveTurn() {
		http.Error(w, "aquesta sessió encara té una tasca en marxa; obre una sessió nova", http.StatusConflict)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "nom buit", 400)
		return
	}
	sess, err := session.Load(s.sessDir(), req.Name)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	// Validate the recorded workspace before replacing the live conversation.
	// A missing or forbidden folder must never silently fall back to the
	// workspace of the currently selected session.
	var restoredWorkspace string
	if strings.TrimSpace(sess.Workspace) != "" {
		restoredWorkspace, err = resolveWorkspace(sess.Workspace)
		if err == nil {
			err = s.user.Allow(restoredWorkspace)
		}
		if err != nil {
			http.Error(w, "workspace de la sessió no disponible: "+err.Error(), http.StatusConflict)
			return
		}
	}
	s.mu.Lock()
	s.convo = sess.Convo
	// Restaurem el resum compactat d'aquesta sessió si en tenia
	s.compacted = sess.Compacted
	// Reprendre és continuar-la, no clonar-la: els torns següents
	// reescriuen aquesta mateixa conversa.
	s.convoFile = sess.Name
	s.pinned = sess.Pinned
	s.role = sess.Role
	if sess.ModelOverrides != nil {
		s.modelOverride = cloneModelOverrides(sess.ModelOverrides)
	} else {
		// Sessions saved before model overrides were serialized inherit the
		// user's latest defaults when first resumed.
		s.modelOverride = s.defaultModelOverrides()
	}
	s.unavailableModels = map[string]string{}
	s.modelCheckedAt = map[string]time.Time{}
	s.resetSessionCounters()
	if restoredWorkspace != "" {
		s.cwd = restoredWorkspace
		if s.policy != nil {
			s.policy.ProjectDir = restoredWorkspace
		}
	}
	s.mu.Unlock()
	if restoredWorkspace != "" {
		rememberWorkspace(restoredWorkspace)
	}

	s.mu.Lock()
	activeWs := s.cwd
	s.mu.Unlock()
	s.remembers.Clear(webRememberScope)
	writeJSON(w, map[string]any{
		"msgs":       len(sess.Convo),
		"role":       sess.Role,
		"title":      sess.Title,
		"name":       sess.Name,
		"workspace":  activeWs,
		"compacted":  sess.Compacted,
		"transcript": sess.Convo,
	})
}

// hasActiveTurn impedeix substituir la conversa mentre un worker encara hi
// escriu. Inclou els torns encuats, que agentBusy encara no reflecteix.
func (s *Server) hasActiveTurn() bool {
	if s.queueHasActiveTurn() {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentBusy || s.planRunning
}

func (s *Server) queueHasActiveTurn() bool {
	for _, run := range s.Hub().Queue().ListSession(s.id) {
		if run.State.Active() {
			return true
		}
	}
	return false
}

// SetupMCP arrenca els servidors MCP del config (com el TUI/headless).
func SetupMCP(cfg *config.Config) {
	out := map[string]mcp.Server{}
	for name, sv := range cfg.MCP {
		out[name] = mcp.Server{Command: sv.Command, Args: sv.Args, Env: sv.Env}
	}
	manager := mcp.Setup(out)
	mcpMu.Lock()
	activeMCP = manager
	mcpMu.Unlock()
}

var (
	mcpMu     sync.RWMutex
	activeMCP *mcp.Manager
)

func currentMCP() *mcp.Manager {
	mcpMu.RLock()
	defer mcpMu.RUnlock()
	return activeMCP
}

// handleMCP exposes safe connector diagnostics for the desktop. Commands,
// arguments and environment variables stay server-side.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	m := s.mcp
	if m == nil {
		m = currentMCP()
	}
	if m == nil {
		writeJSON(w, map[string]any{"summary": "inactiu", "servers": []any{}, "tools": []any{}})
		return
	}
	tools := make([]string, 0, len(m.Tools))
	for _, tool := range m.Tools {
		tools = append(tools, tool.Spec.Name)
	}
	sort.Strings(tools)
	writeJSON(w, map[string]any{"summary": m.Summary(), "servers": m.Status(), "tools": tools})
}

// Open obre l'URL al navegador (Linux: xdg-open).
func Open(url string) {
	if runtime.GOOS != "linux" {
		return
	}
	_ = exec.Command("xdg-open", url).Start()
}

// tokensResum compta el que ocupa el resum de compactació al system prompt.
// Sense el cas buit, una conversa acabada de començar ja deia 9 tokens: el
// sobrecost del sobre d'un missatge que no existeix.
func tokensResum(resum string) int {
	if strings.TrimSpace(resum) == "" {
		return 0
	}
	return llm.EstimateTokens([]llm.Message{{Role: "system", Content: resum}})
}
