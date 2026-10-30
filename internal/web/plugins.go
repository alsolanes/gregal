package web

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Plugins: les funcions que no són el nucli de Gregal (l'Arena, els partits
// dels vents, el Racó) s'hi registren soles des del seu fitxer
// (plugin_*.go). El nucli (serve.go) només recorre el registre: munta les
// rutes, arrenca els bucles de fons i diu a la UI què ha de mostrar.
//
// Cada plugin es pot encendre o apagar per servidor (només l'admin) i
// l'estat es desa al config (`plugins:`). Per defecte tots van encesos:
// qui actualitza no perd res. Un plugin apagat:
//   - respon 404 {"error":"plugin disabled"} a totes les seves rutes;
//   - no té cap bucle de fons en marxa (s'aturen en apagar-lo i tornen a
//     arrencar en encendre'l);
//   - desapareix de la barra lateral de la web i de la portada de l'Android.

// Plugin descriu una funció opcional.
type Plugin struct {
	ID string
	// Name i Description en anglès; NameCA i DescriptionCA en català (la
	// UI web també els té a i18n.js com a plugin.<id>.name/.desc).
	Name, NameCA               string
	Description, DescriptionCA string
	// Icon és un caràcter (o emoji) per a la barra lateral i Preferències.
	Icon string
	// View és la vista de la web que obre (setView) i la fila de l'Android.
	View string
	// Order posa els plugins en ordre a la barra lateral.
	Order     int
	DefaultOn bool
	// Routes registra els handlers HTTP. handle ja hi posa la porta: si el
	// plugin és apagat, la ruta respon 404 sense arribar al handler.
	Routes func(h *Hub, handle func(path string, fn http.HandlerFunc))
	// Start arrenca la feina de fons del plugin; ha de tornar quan ctx
	// s'acaba (o abans, si no té res de llarg). Opcional.
	Start func(ctx context.Context, s *Server)
	// Stop s'executa en apagar el plugin en calent, després de cancel·lar
	// el context de Start: per a la feina que no penja d'aquell context.
	// Opcional.
	Stop func(s *Server)
}

var (
	pluginRegMu sync.Mutex
	pluginReg   []*Plugin
)

// registerPlugin afegeix un plugin al registre (des d'un init()).
func registerPlugin(p *Plugin) {
	pluginRegMu.Lock()
	defer pluginRegMu.Unlock()
	for i, q := range pluginReg {
		if q.ID == p.ID {
			pluginReg[i] = p
			return
		}
	}
	pluginReg = append(pluginReg, p)
	sort.SliceStable(pluginReg, func(i, j int) bool { return pluginReg[i].Order < pluginReg[j].Order })
}

// registeredPlugins retorna una còpia del registre global, en ordre.
func registeredPlugins() []*Plugin {
	pluginRegMu.Lock()
	defer pluginRegMu.Unlock()
	return append([]*Plugin(nil), pluginReg...)
}

// pluginRuntime és l'estat viu dels plugins d'un Hub.
type pluginRuntime struct {
	mu      sync.Mutex
	list    []*Plugin // nil = el registre global (els tests en posen un de propi)
	loaded  bool
	enabled map[string]bool
	base    context.Context // nil fins que arrenca la feina de fons
	cancel  map[string]context.CancelFunc
}

func (h *Hub) plugins() []*Plugin {
	h.plug.mu.Lock()
	l := h.plug.list
	h.plug.mu.Unlock()
	if l != nil {
		return l
	}
	return registeredPlugins()
}

func (h *Hub) pluginByID(id string) *Plugin {
	for _, p := range h.plugins() {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// pluginStates llegeix (un sol cop) què diu el config i hi aplica els
// valors per defecte. El config es copia abans d'agafar plug.mu perquè
// l'ordre dels candaus sigui sempre el mateix.
func (h *Hub) pluginStates() map[string]bool {
	h.plug.mu.Lock()
	loaded := h.plug.loaded
	h.plug.mu.Unlock()
	if !loaded {
		saved := map[string]bool{}
		s := h.def
		s.mu.Lock()
		if s.cfg != nil {
			for k, v := range s.cfg.Plugins {
				saved[k] = v
			}
		}
		s.mu.Unlock()
		st := map[string]bool{}
		for _, p := range h.plugins() {
			on, ok := saved[p.ID]
			if !ok {
				on = p.DefaultOn
			}
			st[p.ID] = on
		}
		h.plug.mu.Lock()
		if !h.plug.loaded {
			h.plug.enabled, h.plug.loaded = st, true
		}
		h.plug.mu.Unlock()
	}
	h.plug.mu.Lock()
	defer h.plug.mu.Unlock()
	out := make(map[string]bool, len(h.plug.enabled))
	for k, v := range h.plug.enabled {
		out[k] = v
	}
	return out
}

// PluginEnabled diu si el plugin id és encès en aquest servidor.
func (h *Hub) PluginEnabled(id string) bool { return h.pluginStates()[id] }

// pluginGate deixa passar la petició només si el plugin és encès.
func (h *Hub) pluginGate(id string, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.PluginEnabled(id) {
			writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "plugin disabled"})
			return
		}
		fn(w, r)
	}
}

// pluginRoutes registra les rutes de tots els plugins (encesos o no: la
// porta decideix a cada petició, i així encendre'ls no demana reiniciar).
func (h *Hub) pluginRoutes(handle func(string, http.HandlerFunc)) {
	for _, p := range h.plugins() {
		if p.Routes == nil {
			continue
		}
		id := p.ID
		p.Routes(h, func(path string, fn http.HandlerFunc) { handle(path, h.pluginGate(id, fn)) })
	}
}

// startPlugins arrenca la feina de fons dels plugins encesos i en recorda
// el context perquè els que s'encenguin més tard també hi pengin.
func (h *Hub) startPlugins(ctx context.Context) {
	st := h.pluginStates()
	h.plug.mu.Lock()
	defer h.plug.mu.Unlock()
	h.plug.base = ctx
	for _, p := range h.pluginsLocked() {
		if st[p.ID] {
			h.startPluginLocked(p)
		}
	}
}

func (h *Hub) pluginsLocked() []*Plugin {
	if h.plug.list != nil {
		return h.plug.list
	}
	return registeredPlugins()
}

func (h *Hub) startPluginLocked(p *Plugin) {
	if p.Start == nil || h.plug.base == nil || h.plug.base.Err() != nil {
		return
	}
	if h.plug.cancel == nil {
		h.plug.cancel = map[string]context.CancelFunc{}
	}
	if h.plug.cancel[p.ID] != nil {
		return
	}
	ctx, cancel := context.WithCancel(h.plug.base)
	h.plug.cancel[p.ID] = cancel
	s := h.def
	go p.Start(ctx, s)
}

func (h *Hub) stopPluginLocked(p *Plugin) {
	if cancel := h.plug.cancel[p.ID]; cancel != nil {
		cancel()
		delete(h.plug.cancel, p.ID)
	}
}

// pluginRunning diu si la feina de fons del plugin és en marxa (tests).
func (h *Hub) pluginRunning(id string) bool {
	h.plug.mu.Lock()
	defer h.plug.mu.Unlock()
	return h.plug.cancel[id] != nil
}

// SetPluginEnabled encén o apaga un plugin: ho desa al config i, si va bé,
// arrenca o atura la seva feina de fons.
func (h *Hub) SetPluginEnabled(id string, on bool) error {
	p := h.pluginByID(id)
	if p == nil {
		return errUnknownPlugin
	}
	h.pluginStates() // carrega l'estat abans de tocar-lo
	s := h.def
	s.mu.Lock()
	var err error
	if s.cfg != nil {
		old, had := s.cfg.Plugins[id]
		if s.cfg.Plugins == nil {
			s.cfg.Plugins = map[string]bool{}
		}
		s.cfg.Plugins[id] = on
		if s.cfgPath != "" {
			if err = s.cfg.Save(s.cfgPath); err != nil {
				if had {
					s.cfg.Plugins[id] = old
				} else {
					delete(s.cfg.Plugins, id)
				}
			}
		}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	h.plug.mu.Lock()
	h.plug.enabled[id] = on
	if on {
		h.startPluginLocked(p)
	} else {
		h.stopPluginLocked(p)
	}
	h.plug.mu.Unlock()
	if !on && p.Stop != nil {
		p.Stop(s)
	}
	return nil
}

type pluginError string

func (e pluginError) Error() string { return string(e) }

const errUnknownPlugin = pluginError("unknown plugin")

// handlePlugins: GET la llista; POST (admin) {id, enabled}.
func (h *Hub) handlePlugins(w http.ResponseWriter, r *http.Request) {
	s := h.SessionFor(userFrom(r), DefaultSession)
	u := userFrom(r)
	admin := u == nil || u.Admin
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		if !admin {
			http.Error(w, `{"error":"només per a l'administrador"}`, http.StatusForbidden)
			return
		}
		var req struct {
			ID      string `json:"id"`
			Enabled *bool  `json:"enabled"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		if req.Enabled == nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "enabled is required"})
			return
		}
		if err := h.SetPluginEnabled(strings.TrimSpace(req.ID), *req.Enabled); err != nil {
			code := http.StatusInternalServerError
			if err == errUnknownPlugin {
				code = http.StatusNotFound
			}
			writeJSONStatus(w, code, map[string]string{"error": err.Error()})
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	st := h.pluginStates()
	list := []map[string]any{}
	for _, p := range h.plugins() {
		list = append(list, map[string]any{
			"id": p.ID, "name": s.tr(p.NameCA, p.Name), "description": s.tr(p.DescriptionCA, p.Description),
			"icon": p.Icon, "view": p.View, "default_on": p.DefaultOn, "enabled": st[p.ID],
		})
	}
	writeJSON(w, map[string]any{"plugins": list, "admin": admin})
}
