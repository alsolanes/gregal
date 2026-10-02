package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/events"
	"gregal/internal/llm"
	"gregal/internal/runs"
	"gregal/internal/session"
)

// Hub multiplexa sessions vives. Fins a la 1.1 el servidor tenia UNA
// conversa i un sol worker: un segon /api/agent responia 409. Ara cada
// sessió és un *Server propi (conversa, mode, rol, permisos, workspace) i
// el Hub els reparteix segons l'id que demani el client.
//
// Compatibilitat: sense id s'usa la sessió "default", que és exactament el
// *Server que ja existia. Els clients antics (Android, VS Code, Telegram,
// webapp 0.9.x) no s'assabenten del canvi.
type Hub struct {
	mu         sync.RWMutex
	def        *Server
	sess       map[string]*Server
	order      []string
	eventStore *events.Store
	eventErr   error
	queue      *runs.Queue
}

// DefaultSession és l'id implícit quan el client no en demana cap.
const DefaultSession = "default"

var sessIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Hub crea (o retorna) el hub que té aquest servidor com a sessió default.
func (s *Server) Hub() *Hub {
	if s.hub != nil {
		return s.hub
	}
	h := &Hub{def: s, sess: map[string]*Server{DefaultSession: s}, order: []string{DefaultSession}, queue: runs.New()}
	// Un sol fitxer per perfil permet que el cursor sigui global i estable
	// encara que el client canviï de pestanya. Si el disc no és escrivible,
	// la resta del servei continua funcionant i l'API ho tracta com a buida.
	h.eventStore, _ = events.Open(filepath.Join(session.DefaultDir(), "events.jsonl"))
	s.id = DefaultSession
	s.createdAt = time.Now()
	s.hub = h
	return h
}

// Queue és la cua d'execucions compartida: tots els torns de totes les
// sessions hi passen, vinguin del web, del TUI delegat o de la API v2.
func (h *Hub) Queue() *runs.Queue {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.queue == nil {
		h.queue = runs.New()
	}
	return h.queue
}

func (h *Hub) appendEvent(e events.Event) error {
	if h == nil || h.eventStore == nil {
		return errors.New("registre d'events no disponible")
	}
	_, err := h.eventStore.Append(e)
	if err != nil {
		h.mu.Lock()
		h.eventErr = err
		h.mu.Unlock()
	}
	return err
}

func (h *Hub) eventError() error {
	if h == nil || h.eventStore == nil {
		return errors.New("registre d'events no disponible")
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.eventErr
}

// sessionID llegeix l'id de sessió de la petició: capçalera X-Gregal-Session
// primer, ?session= després. Un id invàlid es tracta com a absent.
func sessionID(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("X-Gregal-Session"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("session"))
	}
	if id == "" || !sessIDRe.MatchString(id) {
		return DefaultSession
	}
	return id
}

// resolve retorna la sessió de la petició i la crea si encara no existeix.
func (h *Hub) resolve(r *http.Request) *Server {
	return h.SessionFor(userFrom(r), sessionID(r))
}

// keyFor és la clau interna d'una sessió: `usuari-id`. Sense usuari (mode
// local, sense autenticació) la clau és l'id pelat, com abans.
//
// L'id que viatja pel fil ja ve prefijat, així que si el client el torna a
// enviar no el tornem a prefijar: si no, cada volta afegiríem un tros.
func (h *Hub) keyFor(user *User, id string) string {
	if user == nil {
		if id == "" || !sessIDRe.MatchString(id) {
			return DefaultSession
		}
		return id
	}
	if id == "" || !sessIDRe.MatchString(id) {
		id = DefaultSession
	}
	if strings.HasPrefix(id, user.Name+"-") {
		id = strings.TrimPrefix(id, user.Name+"-")
	}
	return user.Name + "-" + id
}

// SessionFor retorna la sessió (usuari, id), creant-la si cal.
func (h *Hub) SessionFor(user *User, id string) *Server {
	key := h.keyFor(user, id)
	h.mu.RLock()
	s := h.sess[key]
	h.mu.RUnlock()
	if s != nil {
		return s
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if s = h.sess[key]; s != nil {
		return s
	}
	s = h.clone(key)
	s.user = user
	// Each newly created conversation starts with this user's most recent
	// model choices. Resuming a saved conversation replaces these defaults
	// with the model choices stored alongside that conversation.
	s.modelOverride = s.defaultModelOverrides()
	// Si el workspace que s'ha copiat no és seu (la còpia ve del servidor),
	// la sessió comença a la seua carpeta: si no, tindria l'arbre i els
	// fitxers tancats fins que en triés una.
	//
	// L'ordre és: la carpeta de treball (`home`), la primera arrel i, si
	// no hi ha res, el que ja hi havia.
	if user != nil {
		inici := user.Home
		if inici == "" {
			inici = user.Arrel()
		}
		if inici != "" && user.Allow(s.cwd) != nil {
			s.cwd = inici
			if s.policy != nil {
				s.policy.ProjectDir = s.cwd
			}
		}
	}
	h.sess[key] = s
	h.order = append(h.order, key)
	return s
}

// Session retorna la sessió id de qui no ha identificat ningú (el TUI i els
// clients locals de sempre).
func (h *Hub) Session(id string) *Server { return h.SessionFor(nil, id) }

// clone fabrica una sessió nova amb la configuració compartida però amb
// estat de conversa propi. cfg, client i jobs són compartits a posta: els
// providers i els models són de l'aplicació, no de la pestanya.
func (h *Hub) clone(id string) *Server {
	d := h.def
	cfg := d.cfg
	return &Server{
		cfg: cfg, cfgPath: d.cfgPath, client: d.client, goalDir: d.goalDir,
		jobs: d.jobs, mcp: d.mcp, Token: d.Token, cwd: d.cwd, hub: h, id: id,
		instanceID: d.instanceID,
		// users també es comparteix: una sessió clonada ha de poder
		// resoldre qui pregunta (/api/me, /api/login des d'una pestanya).
		users:             d.users,
		createdAt:         time.Now(),
		role:              cfg.InitialRole(),
		mode:              cfg.Mode,
		modelOverride:     map[string]string{},
		unavailableModels: map[string]string{},
		modelCheckedAt:    map[string]time.Time{},
		approvals:         map[string]approvalReq{},
		questions:         map[string]questionReq{},
		remembers:         agent.NewRemember(),
		policy: &agent.Policy{
			Tools: cfg.Permissions.Tools, BashAllow: cfg.Permissions.BashAllow,
			BashDeny: cfg.Permissions.BashDeny, ProjectDir: d.cwd,
		},
	}
}

// Close tanca una sessió viva. La default no es pot tancar (és la que
// serveix els clients que no envien id).
func (h *Hub) Close(id string) bool { return h.CloseFor(nil, id) }

// CloseFor tanca una sessió de l'usuari. Ningú no pot tancar la d'un altre:
// la clau porta el nom a dins.
func (h *Hub) CloseFor(user *User, id string) bool {
	key := h.keyFor(user, id)
	if key == h.keyFor(user, DefaultSession) {
		return false
	}
	h.mu.Lock()
	s := h.sess[key]
	if s != nil {
		delete(h.sess, key)
		for i, n := range h.order {
			if n == key {
				h.order = append(h.order[:i], h.order[i+1:]...)
				break
			}
		}
	}
	h.mu.Unlock()
	if s == nil {
		return false
	}
	s.stopRun()
	// Tancar la pestanya s'endú els seus processos de segon pla: si no,
	// un `npm run dev` quedaria orfe fins a reiniciar el servidor.
	agent.Procs().KillSession(key)
	return true
}

// list retorna el resum de cada sessió viva, en ordre de creació.
func (h *Hub) list() []map[string]any { return h.listFor(nil) }

// listFor retorna només les sessions de l'usuari: a la barra de pestanyes
// no hi ha de sortir la conversa de ningú altre.
func (h *Hub) listFor(user *User) []map[string]any {
	h.mu.RLock()
	ids := append([]string(nil), h.order...)
	byID := make(map[string]*Server, len(h.sess))
	for k, v := range h.sess {
		byID[k] = v
	}
	h.mu.RUnlock()
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		s := byID[id]
		if s == nil {
			continue
		}
		if user != nil {
			if !strings.HasPrefix(id, user.Name+"-") {
				continue
			}
		} else if s.user != nil {
			continue
		}
		queued := s.queueHasActiveTurn()
		s.mu.Lock()
		item := map[string]any{
			"id": id, "title": s.title, "msgs": len(s.convo),
			"busy": queued || s.agentBusy || s.planRunning, "mode": s.mode,
			"role": s.role, "cwd": s.cwd, "project": projectName(s.cwd),
			"created_at": s.createdAt.UTC().Format(time.RFC3339),
		}
		if s.title == "" {
			item["title"] = firstUserLine(s.convo)
		}
		s.mu.Unlock()
		out = append(out, item)
	}
	return out
}

// firstUserLine dona un títol provisional a partir del primer missatge
// de l'usuari, retallat: així la llista de pestanyes diu alguna cosa abans
// que l'usuari posi nom a la sessió.
func firstUserLine(convo []llm.Message) string {
	for _, m := range convo {
		if m.Role != "user" {
			continue
		}
		t := strings.TrimSpace(strings.SplitN(m.Content, "\n", 2)[0])
		if t == "" {
			continue
		}
		if r := []rune(t); len(r) > 48 {
			t = string(r[:48]) + "…"
		}
		return t
	}
	return ""
}

// newSessionID dona un id curt i únic per a una pestanya nova.
func newSessionID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "s" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "s" + hex.EncodeToString(b)
}

// adminOnly embolcalla un handler de sessió i el deixa només a l'amo del
// servidor. Amb diversos usuaris, els jobs del servidor i el token de GitHub
// no són de tothom: qui no és admin rep 403 i el mateix missatge que si la
// ruta no existís.
//
// Sense autenticació (ús local, el TUI i l'escriptori de sempre) tot passa.
func adminOnly(fn func(*Server, http.ResponseWriter, *http.Request)) func(*Server, http.ResponseWriter, *http.Request) {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		if u := userFrom(r); u != nil && !u.Admin {
			http.Error(w, `{"error":"només per a l'administrador"}`, http.StatusForbidden)
			return
		}
		fn(s, w, r)
	}
}

// route lliga un handler de sessió a la sessió que demana la petició.
func (h *Hub) route(fn func(*Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { fn(h.resolve(r), w, r) }
}

// global lliga un handler que no depèn de la pestanya (providers, models,
// jobs, office) però sí de qui pregunta: s'executa a la sessió default de
// l'usuari, no a la d'un altre.
func (h *Hub) global(fn func(*Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(h.SessionFor(userFrom(r), DefaultSession), w, r)
	}
}

// --- endpoints de sessions vives ---

func (h *Hub) handleLiveSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"sessions": h.listFor(userFrom(r)), "current": sessionID(r)})
}

func (h *Hub) handleOpenSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Cwd   string `json:"cwd"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = newSessionID()
	}
	if !sessIDRe.MatchString(id) {
		http.Error(w, "id de sessió invàlid", 400)
		return
	}
	s := h.SessionFor(userFrom(r), id)
	if strings.TrimSpace(req.Title) != "" {
		s.mu.Lock()
		s.title = strings.TrimSpace(req.Title)
		s.mu.Unlock()
	}
	if cwd := strings.TrimSpace(req.Cwd); cwd != "" {
		// Ningú no obre una sessió a la carpeta d'un altre.
		if err := s.user.Allow(cwd); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if err := s.setWorkspace(cwd); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	s.mu.Lock()
	out := map[string]any{"id": s.id, "title": s.title, "cwd": s.cwd, "project": projectName(s.cwd)}
	s.mu.Unlock()
	writeJSON(w, out)
}

func (h *Hub) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = sessionID(r)
	}
	if !h.CloseFor(userFrom(r), id) {
		http.Error(w, "no es pot tancar aquesta sessió", 400)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "closed": id})
}

// handleCancel atura el torn en curs de la sessió (xat, agent o pla).
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	stopped := s.stopRun()
	writeJSON(w, map[string]any{"ok": true, "cancelled": stopped})
}

// stopRun talla el torn en curs de la sessió a la cua compartida. Cert si
// n'hi havia cap de viu. Els torns només en cua no es toquen: pertanyen a
// qui els ha enviat i es poden cancel·lar per ID des de la API v2.
func (s *Server) stopRun() bool {
	stopped := false
	s.mu.Lock()
	id := s.activeRun
	s.mu.Unlock()
	if id == 0 {
		// Cursa: la cua marca el torn com a «running» abans que
		// l'executor n'hagi registrat l'id, i entremig hi cap una
		// premuda d'Atura. Sense això, qui atura just quan el torn
		// arrenca no atura res: el botó respon que sí i el torn segueix
		// (i amb ell la mar de la capçalera, que és el que es veia).
		for _, r := range s.Hub().Queue().ListSession(s.id) {
			if r.State.Active() {
				id = r.ID
				break
			}
		}
	}
	if id != 0 && s.Hub().Queue().Cancel(id) {
		stopped = true
	}
	// El bucle de goal no passa per la cua: conserva el tall legacy.
	s.mu.Lock()
	c := s.runCancel
	s.runCancel = nil
	s.mu.Unlock()
	if c != nil {
		c()
		stopped = true
	}
	return stopped
}

// runContext obre el context legacy (mode goal) i el desa perquè la
// cancel·lació el pugui tallar. El defer que retorna l'ha de tancar sempre.
func (s *Server) runContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.runCancel != nil {
		s.runCancel()
	}
	s.runCancel = cancel
	s.mu.Unlock()
	return ctx, func() {
		cancel()
		s.mu.Lock()
		if s.runCancel != nil {
			s.runCancel = nil
		}
		s.mu.Unlock()
	}
}

// setActiveRun registra el torn que s'està executant en aquesta sessió
// (0 = cap) perquè /api/agent/cancel el pugui trobar sense conèixer la v2.
func (s *Server) setActiveRun(id int64) {
	s.mu.Lock()
	s.activeRun = id
	s.mu.Unlock()
}

// activeRunID llegeix el torn en execució.
func (s *Server) activeRunID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeRun
}

// projectName és el nom curt del workspace per a la capçalera.
func projectName(dir string) string { return filepath.Base(dir) }
