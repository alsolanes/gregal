package web

// Visor de l'ordinador i control humà (paquet B3, pla v2 §5 i §6 «Camí B»).
//
//	POST /api/ordinador/viewer  {id?, control?, handoff?}
//	     → {url, expiresAt, health, controlOwner, leaseExpiry}
//	     Testimoni d'un sol ús (60 s) lligat al propietari. Si l'ordinador
//	     és desat o aturat, el desperta (motiu «viewer») i diu starting.
//	GET|POST /api/ordinador/control  {id?, action: take|release}
//	     La lease de control: mentre la persona la té, l'agent no actua al
//	     navegador (tools.SetBrowserControlGuard) ni l'ordinador es desa.
//
//	/ordinador/visor/entra?t=…  bescanvia el testimoni per la galeta de
//	     sessió (HttpOnly, només aquest camí) i porta a la pàgina del visor.
//	/ordinador/visor/            pàgina del visor: estat, prendre/retornar
//	     el control i el noVNC en un iframe (només lectura si no el tens).
//	/ordinador/visor/estat|batec|control  JSON, galeta + X-Gregal-Visor.
//	/ordinador/visor/vnc/…       pont cap al noVNC de la VM (agent
//	     /v1/viewer/) o, en mode local, cap a GREGAL_ORDINADOR_VISOR.
//
// Fora de /api: no hi ha Bearer (un WebView no el pot posar a cada
// subrecurs ni al WebSocket). Tot el que no és «entra» exigeix una sessió
// del visor vigent; sense, 401. Ni el testimoni ni la galeta surten mai als
// logs ni als events, i la galeta no passa a la VM.

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/ordinador"
	"gregal/internal/tools"
)

const (
	visorPrefix = "/ordinador/visor/"
	visorCookie = "gregal_visor"
)

//go:embed ordinador_visor.html
var visorPage []byte

var (
	ordinadorCtlMu     sync.Mutex
	ordinadorCtlLeases *ordinador.Leases
	ordinadorCtlPath   string
	ordinadorViewers   = ordinador.NewViewers()
	// visorWaking: quan s'ha demanat despertar l'ordinador per obrir el
	// visor (el hipervisor pot trigar a tornar): mentrestant, starting.
	visorWaking sync.Map // id → visorWake
)

type visorWake struct {
	at  time.Time
	err string
}

// ordinadorLeases és el registre de leases de control, desat al costat del
// config. La primera crida connecta la guarda del navegador i el gestor.
func (s *Server) ordinadorLeases() *ordinador.Leases {
	path := filepath.Join(goalDirFor(s.cfgPath), "ordinador-control.json")
	ordinadorCtlMu.Lock()
	defer ordinadorCtlMu.Unlock()
	if ordinadorCtlLeases == nil || ordinadorCtlPath != path {
		ordinadorCtlLeases, ordinadorCtlPath = ordinador.NewLeases(path), path
		l := ordinadorCtlLeases
		tools.SetBrowserControlGuard(l.HeldByUser)
		if m, _ := ordinadorHost(); m != nil {
			m.SetLeases(l)
		}
	}
	return ordinadorCtlLeases
}

// visorTarget és l'ordinador de qui pregunta.
type visorTarget struct {
	key, owner string
	m          *ordinador.Manager
	c          *ordinador.Computer // nil en mode local
}

var errVisorForbidden = errors.New("this computer belongs to another account")

// ordinadorTargetFor resol l'ordinador d'un propietari (id opcional). Un id
// d'un altre compte → errVisorForbidden.
func ordinadorTargetFor(owner, id string) (visorTarget, int, error) {
	m, err := ordinadorHost()
	if err != nil {
		return visorTarget{}, http.StatusServiceUnavailable, errOrdinadorConfig
	}
	if m != nil {
		var c *ordinador.Computer
		if id != "" {
			if c = m.ByID(id); c == nil {
				return visorTarget{}, http.StatusNotFound, errors.New("unknown computer")
			}
			if c.Owner != owner {
				return visorTarget{}, http.StatusForbidden, errVisorForbidden
			}
		} else if c, err = m.For(owner); err != nil {
			return visorTarget{}, http.StatusNotFound, errors.New("no computer for this account")
		}
		return visorTarget{key: c.LeaseKey(), owner: owner, m: m, c: c}, 0, nil
	}
	if declared := tools.OrdinadorOwnerName(); declared != "" && owner != "" && owner != declared {
		return visorTarget{}, http.StatusForbidden, errVisorForbidden
	}
	key := tools.OwnerKeyFor(owner)
	if id != "" && id != ordinadorID(key) {
		return visorTarget{}, http.StatusNotFound, errors.New("unknown computer")
	}
	return visorTarget{key: key, owner: owner}, 0, nil
}

func (t visorTarget) computerID() string {
	if t.c != nil {
		return t.c.ID
	}
	return ordinadorID(t.key)
}

// localVisorAddr és el noVNC de loopback del mode local (buit = sense visor).
func localVisorAddr() string { return strings.TrimSpace(os.Getenv("GREGAL_ORDINADOR_VISOR")) }

// health és la salut per al visor: la del gestor, amb starting mentre es
// desperta per obrir el visor.
func (s *Server) visorHealth(ctx context.Context, t visorTarget) (health, power string) {
	if t.c == nil {
		p := tools.ProbeOrdinador(ctx, t.key, s.ordinadorFilesDir(nil))
		return p.Health, ""
	}
	r := t.m.Status(ctx, t.c)
	health, power = r.Health, r.Power
	if v, ok := visorWaking.Load(t.c.ID); ok {
		w := v.(visorWake)
		switch {
		case health == "running" || time.Since(w.at) > 2*time.Minute:
			visorWaking.Delete(t.c.ID)
		case w.err != "":
			health = "unreachable"
		case health == "stopped":
			health = "starting"
		}
	}
	return health, power
}

// wakeForViewer desperta l'ordinador desat per obrir el visor, sense
// esperar-lo: la pàgina del visor diu starting fins que respon.
func wakeForViewer(t visorTarget) {
	if t.c == nil {
		return
	}
	if _, loaded := visorWaking.LoadOrStore(t.c.ID, visorWake{at: time.Now()}); loaded {
		return
	}
	goBackground(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := t.m.Wake(ctx, t.c, "viewer"); err != nil {
			visorWaking.Store(t.c.ID, visorWake{at: time.Now(), err: "wake failed"})
		}
	})
}

func (s *Server) visorState(ctx context.Context, t visorTarget) map[string]any {
	health, power := s.visorHealth(ctx, t)
	out := map[string]any{"id": t.computerID(), "health": health, "controlOwner": "none", "leaseExpiry": nil}
	if power != "" {
		out["power"] = power
	}
	if t.c == nil && localVisorAddr() == "" {
		out["viewer"] = false
	} else {
		out["viewer"] = true
	}
	if x := s.ordinadorLeases().Get(t.key); x != nil {
		out["controlOwner"] = "user"
		out["leaseExpiry"] = x.Expires.UTC().Format(time.RFC3339)
		out["mine"] = true
		if x.Handoff != "" {
			out["handoff"] = x.Handoff
		}
	}
	return out
}

func decodeVisorReq(r *http.Request, v any) {
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(v)
	}
}

// handleOrdinadorViewer és POST /api/ordinador/viewer.
func (s *Server) handleOrdinadorViewer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Control bool   `json:"control"`
		Handoff string `json:"handoff"`
	}
	decodeVisorReq(r, &req)
	_, owner := ordinadorCaller(r)
	t, code, err := ordinadorTargetFor(owner, strings.TrimSpace(req.ID))
	if err != nil {
		writeJSONStatus(w, code, map[string]any{"error": err.Error()})
		return
	}
	if t.c == nil && localVisorAddr() == "" {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "this server has no remote viewer", "reason": "no_viewer"})
		return
	}
	if req.Handoff != "" && !s.ownsPendingHandoff(req.Handoff) {
		req.Handoff = ""
	}
	if t.c != nil {
		if h, _ := s.visorHealth(r.Context(), t); h == "stopped" {
			wakeForViewer(t)
		}
		t.m.Touch(t.c)
	}
	if req.Control || req.Handoff != "" {
		reason := "viewer"
		if req.Handoff != "" {
			reason = "handoff"
		}
		s.ordinadorLeases().Take(t.key, reason, req.Handoff)
	}
	ticket, exp := ordinadorViewers.Issue(t.key, t.owner, t.computerID())
	out := s.visorState(r.Context(), t)
	out["url"] = visorPrefix + "entra?t=" + ticket
	out["expiresAt"] = exp.UTC().Format(time.RFC3339)
	writeJSON(w, out)
}

// handleOrdinadorControl és /api/ordinador/control.
func (s *Server) handleOrdinadorControl(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	switch r.Method {
	case http.MethodGet:
		req.ID = r.URL.Query().Get("id")
	case http.MethodPost:
		decodeVisorReq(r, &req)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, owner := ordinadorCaller(r)
	t, code, err := ordinadorTargetFor(owner, strings.TrimSpace(req.ID))
	if err != nil {
		writeJSONStatus(w, code, map[string]any{"error": err.Error()})
		return
	}
	if r.Method == http.MethodPost && !s.applyControl(t, req.Action) {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "action must be take or release"})
		return
	}
	writeJSON(w, s.visorState(r.Context(), t))
}

func (s *Server) applyControl(t visorTarget, action string) bool {
	l := s.ordinadorLeases()
	switch action {
	case "take":
		l.Take(t.key, "viewer", "")
		if t.c != nil {
			t.m.Touch(t.c)
		}
	case "release":
		l.Release(t.key)
		if t.c != nil {
			t.m.Touch(t.c) // el compte enrere de desar comença ara
		}
	default:
		return false
	}
	return true
}

// handleVisor serveix /ordinador/visor/… (fora de /api: autenticat amb la
// sessió del visor).
func (s *Server) handleVisor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	rest := strings.TrimPrefix(r.URL.Path, visorPrefix)
	if rest == "entra" {
		s.visorEnter(w, r)
		return
	}
	ck, err := r.Cookie(visorCookie)
	if err != nil {
		visorDenied(w, r)
		return
	}
	sess, ok := ordinadorViewers.Session(ck.Value, rest == "batec")
	if !ok {
		visorDenied(w, r)
		return
	}
	t, _, terr := ordinadorTargetFor(sess.Owner, sess.Computer)
	if terr != nil || t.key != sess.Key {
		ordinadorViewers.End(sess.ID)
		visorDenied(w, r)
		return
	}
	switch {
	case rest == "":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'self'")
		_, _ = w.Write(visorPage)
	case rest == "estat":
		writeJSON(w, s.visorState(r.Context(), t))
	case rest == "batec" || rest == "control" || rest == "surt":
		if r.Method != http.MethodPost || r.Header.Get("X-Gregal-Visor") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch rest {
		case "batec":
			// El visor és a la vista: compta com a ús i allarga la lease.
			if t.c != nil {
				t.m.Touch(t.c)
			}
			s.ordinadorLeases().Renew(t.key)
		case "control":
			var req struct {
				Action string `json:"action"`
			}
			decodeVisorReq(r, &req)
			if !s.applyControl(t, req.Action) {
				writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "action must be take or release"})
				return
			}
		case "surt":
			ordinadorViewers.End(sess.ID)
		}
		writeJSON(w, s.visorState(r.Context(), t))
	case rest == "accions":
		// Les accions de l'agent en directe (B13): només les del propietari.
		s.Hub().serveComputerStream(w, r, computerOwnerOf(sess.Owner))
	case rest == "accions/recents":
		writeJSON(w, s.Hub().computerRecentJSON(computerOwnerOf(sess.Owner), r, "captura/"))
	case strings.HasPrefix(rest, "captura/"):
		b, ok := s.Hub().computer.shot(computerOwnerOf(sess.Owner), strings.TrimPrefix(rest, "captura/"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		serveComputerShot(w, b)
	case strings.HasPrefix(rest, "vnc/"):
		s.visorProxy(t).ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) visorProxy(t visorTarget) http.Handler {
	if t.c != nil {
		return t.m.ViewerProxy(t.c, visorPrefix+"vnc/")
	}
	h, err := ordinador.NewLocalViewerProxy(localVisorAddr(), visorPrefix+"vnc/")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "this server has no remote viewer", http.StatusServiceUnavailable)
		})
	}
	return h
}

func (s *Server) visorEnter(w http.ResponseWriter, r *http.Request) {
	sess, ok := ordinadorViewers.Redeem(r.URL.Query().Get("t"))
	if !ok {
		visorDenied(w, r)
		return
	}
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name: visorCookie, Value: sess.ID, Path: visorPrefix, HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(ordinador.ViewerSessionMax / time.Second),
	})
	dest := visorPrefix
	if lang := r.URL.Query().Get("lang"); lang == "en" || lang == "ca" {
		dest += "?lang=" + lang
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func visorDenied(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(strings.TrimPrefix(r.URL.Path, visorPrefix), "vnc/") || r.Header.Get("X-Gregal-Visor") == "1" {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]any{"error": "viewer session expired", "reason": "expired"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><title>Gregal</title>` +
		`<body style="font:15px system-ui,sans-serif;padding:24px;background:#0e1015;color:#f3f5fa">` +
		`<p>La sessió del visor ha caducat. Torna a obrir l'ordinador des de Gregal.</p>` +
		`<p style="color:#a3adc2">The viewer session has expired. Open the computer again from Gregal.</p>`))
}
