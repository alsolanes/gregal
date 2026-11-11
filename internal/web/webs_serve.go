package web

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/events"
)

// Servir les webs amb nom: per capçalera Host sota el domini base, o per
// camí /w/<nom>/ (i /wp/<nom>/ per a la previsualització) a qualsevol host.
//
//   <nom>.<domini>          la web (public; lan des de la xarxa local; private amb sessió)
//   <nom>.lan.<domini>      el mateix, pensat per a la xarxa local
//   <nom>.preview.<domini>  l'última versió, encara que no sigui l'activa (propietari)
//
// Un host del domini que no és cap web (o és reservat, o és el del mateix
// Gregal) passa a l'app de sempre. Les webs en un host propi no porten la
// CSP sandbox: tenen el seu origen i no veuen el localStorage de Gregal. Per
// camí (/w/) sí, perquè comparteixen origen amb Gregal.

const (
	webCheckHeader   = "X-Gregal-Web-Check"
	webVersionHeader = "X-Gregal-Web-Version"
	webTicketParam   = "gregal_ticket"
	webTicketTTL     = 2 * time.Minute
	webSessionTTL    = 12 * time.Hour
)

// --- bitllets i sessions del propietari ---

type webTicket struct {
	owner, name string
	exp         time.Time
}

var (
	webTicketsMu sync.Mutex
	webTickets   = map[string]webTicket{} // bitllet d'un sol ús → qui i quina web
	webSessions  = map[string]webTicket{} // galeta → qui i quina web
)

func webRandom() string {
	b := make([]byte, 20)
	_, _ = cryptorand.Read(b)
	return hex.EncodeToString(b)
}

// newWebTicket: el propietari, ja autenticat a l'API, rep un bitllet curt
// d'un sol ús que el host de la web bescanvia per una galeta pròpia.
func newWebTicket(owner, name string) string {
	t := webRandom()
	webTicketsMu.Lock()
	defer webTicketsMu.Unlock()
	now := time.Now()
	for k, v := range webTickets {
		if now.After(v.exp) {
			delete(webTickets, k)
		}
	}
	webTickets[t] = webTicket{owner: owner, name: name, exp: now.Add(webTicketTTL)}
	return t
}

func redeemWebTicket(t, name string) (string, bool) {
	webTicketsMu.Lock()
	defer webTicketsMu.Unlock()
	v, ok := webTickets[t]
	delete(webTickets, t)
	if !ok || v.name != name || time.Now().After(v.exp) {
		return "", false
	}
	sess := webRandom()
	now := time.Now()
	for k, x := range webSessions {
		if now.After(x.exp) {
			delete(webSessions, k)
		}
	}
	webSessions[sess] = webTicket{owner: v.owner, name: name, exp: now.Add(webSessionTTL)}
	return sess, true
}

func webCookieName(secure bool) string {
	if secure {
		return "__Host-gregal_web"
	}
	return "gregal_web"
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || r.Header.Get("Cf-Visitor") != ""
}

// webOwnerSession diu si la petició porta la sessió del propietari d'aquesta web.
func webOwnerSession(r *http.Request, w *webSite) bool {
	for _, name := range []string{"__Host-gregal_web", "gregal_web"} {
		c, err := r.Cookie(name)
		if err != nil || c.Value == "" {
			continue
		}
		webTicketsMu.Lock()
		v, ok := webSessions[c.Value]
		webTicketsMu.Unlock()
		if ok && v.name == w.Name && v.owner == w.Owner && time.Now().Before(v.exp) {
			return true
		}
	}
	return false
}

// --- comprovacions en curs ---

type webCheckGrant struct {
	name string
	n    int
}

var webChecks sync.Map // testimoni → webCheckGrant

// --- xarxa local ---

// viaTunnel: Cloudflare (també el túnel, cloudflared) posa aquestes
// capçaleres a l'extrem i el client no les pot treure.
func viaTunnel(r *http.Request) bool {
	h := r.Header
	return h.Get("Cf-Connecting-Ip") != "" || h.Get("Cf-Ray") != "" || h.Get("Cf-Visitor") != "" ||
		strings.Contains(strings.ToLower(h.Get("Cdn-Loop")), "cloudflare")
}

// isLANRequest: adreça privada (o de la mateixa màquina) i mai pel túnel.
// Darrere un proxy local (Caddy) es mira l'adreça que hi ha afegit el proxy.
func isLANRequest(r *http.Request) bool {
	if viaTunnel(r) {
		return false
	}
	ip := net.ParseIP(clientIP(r))
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

// --- enrutament ---

func hostOnly(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	return strings.TrimSuffix(h, ".")
}

// webHostName parteix un host sota el domini: (nom, mode) amb mode
// "site", "lan" o "preview". ok=false si no és d'una web.
func webHostName(host, domain string) (name, mode string, ok bool) {
	if domain == "" || !strings.HasSuffix(host, "."+domain) {
		return "", "", false
	}
	labels := strings.Split(strings.TrimSuffix(host, "."+domain), ".")
	switch {
	case len(labels) == 1:
		mode = "site"
	case len(labels) == 2 && labels[1] == "lan":
		mode = "lan"
	case len(labels) == 2 && labels[1] == "preview":
		mode = "preview"
	default:
		return "", "", false
	}
	name = labels[0]
	if validWebName(name) != nil {
		return "", "", false
	}
	return name, mode, true
}

// webHosts posa les webs amb nom davant de l'app (i de l'autenticació de l'API).
func (h *Hub) webHosts(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := h.def
		host := hostOnly(r.Host)
		if d := s.publishDomain(); d != "" && host != hostOnly(hostOfURL(publicBase())) {
			if name, mode, ok := webHostName(host, d); ok {
				serveWeb(w, r, name, mode, strings.TrimPrefix(r.URL.Path, "/"), "")
				return
			}
		}
		for _, pre := range []string{"/w/", "/wp/"} {
			if !strings.HasPrefix(r.URL.Path, pre) {
				continue
			}
			rest := strings.TrimPrefix(r.URL.Path, pre)
			name, rel, slash := strings.Cut(rest, "/")
			if validWebName(name) != nil {
				break
			}
			if !slash {
				q := ""
				if r.URL.RawQuery != "" {
					q = "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, pre+name+"/"+q, http.StatusFound)
				return
			}
			mode := "site"
			if pre == "/wp/" {
				mode = "preview"
			}
			serveWeb(w, r, name, mode, rel, pre+name+"/")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hostOfURL(u string) string {
	if p, err := url.Parse(u); err == nil {
		return p.Host
	}
	return ""
}

// serveWeb serveix una web. prefix és "" en un host propi o /w/<nom>/.
func serveWeb(w http.ResponseWriter, r *http.Request, name, mode, rel, prefix string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	site := getWeb(name)
	if site == nil {
		webNotFound(w)
		return
	}
	// El bitllet del propietari: el bescanvia per una galeta d'aquest host
	// (o d'aquest camí) i torna a la mateixa adreça sense el bitllet.
	if t := r.URL.Query().Get(webTicketParam); t != "" {
		sess, ok := redeemWebTicket(t, name)
		if ok {
			secure := requestIsHTTPS(r)
			c := &http.Cookie{Name: webCookieName(secure && prefix == ""), Value: sess, Path: "/", HttpOnly: true,
				Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int(webSessionTTL / time.Second)}
			if prefix != "" {
				c.Name, c.Path = "gregal_web", prefix
			}
			http.SetCookie(w, c)
		}
		q := r.URL.Query()
		q.Del(webTicketParam)
		u := *r.URL
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.RequestURI(), http.StatusFound)
		return
	}
	n := 0
	// La comprovació posterior a publicar veu la versió candidata a l'URL
	// real, sense que la versió activa hagi canviat.
	if tok := r.Header.Get(webCheckHeader); tok != "" {
		if g, ok := webChecks.Load(tok); ok && g.(webCheckGrant).name == name {
			n = g.(webCheckGrant).n
		}
	}
	if n == 0 {
		switch mode {
		case "preview":
			if !webOwnerSession(r, site) {
				webNotFound(w)
				return
			}
			n = site.latest()
		default:
			if !webVisibleTo(r, site) {
				webNotFound(w)
				return
			}
			n = site.Current
		}
	}
	if n == 0 {
		webNotFound(w)
		return
	}
	serveWebFile(w, r, webVersionDir(name, n), rel, prefix, n, mode == "preview")
}

// webVisibleTo aplica la visibilitat de la web a qui la demana.
func webVisibleTo(r *http.Request, site *webSite) bool {
	switch site.Visibility {
	case webPublic:
		return true
	case webLAN:
		return isLANRequest(r)
	}
	return webOwnerSession(r, site)
}

func webNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>404</title><body style="font:16px system-ui;margin:3em;color:#333">Not found.</body>`))
}

// serveWebFile serveix un fitxer d'una versió (mateixes regles que share_site.go).
func serveWebFile(w http.ResponseWriter, r *http.Request, dir, rel, prefix string, n int, preview bool) {
	if rel == "" || strings.HasSuffix(rel, "/") {
		rel += "index.html"
	}
	clean := path.Clean("/" + rel)[1:]
	if clean == "" || strings.Contains(clean, "\\") || strings.HasPrefix(clean, "..") {
		webNotFound(w)
		return
	}
	full := filepath.Join(dir, filepath.FromSlash(clean))
	info, err := os.Lstat(full)
	if err == nil && info.IsDir() {
		base := prefix
		if base == "" {
			base = "/"
		}
		http.Redirect(w, r, base+clean+"/", http.StatusFound)
		return
	}
	if err != nil || !info.Mode().IsRegular() {
		webNotFound(w)
		return
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		webNotFound(w)
		return
	}
	ct := mime.TypeByExtension(path.Ext(clean))
	if ct == "" {
		ct = http.DetectContentType(raw)
	}
	apk := strings.EqualFold(path.Ext(clean), ".apk")
	if apk {
		ct = "application/vnd.android.package-archive"
	}
	h := w.Header()
	if prefix != "" {
		// Mateix origen que Gregal: origen opac, com /share/.
		if strings.HasPrefix(ct, "text/html") {
			raw = injectStorageShim(raw)
		}
		h.Set("Content-Security-Policy", siteCSP)
	}
	h.Set("Content-Type", ct)
	if apk {
		h.Set("Content-Disposition", `attachment; filename="`+path.Base(clean)+`"`)
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-cache")
	h.Set(webVersionHeader, strconv.Itoa(n))
	if preview {
		h.Set("X-Robots-Tag", "noindex, nofollow")
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(raw)
}

// emitWeb avisa els clients que una web ha canviat (llista «Webs»).
func (s *Server) emitWeb(w *webSite) {
	if w == nil {
		return
	}
	b, _ := json.Marshal(map[string]any{"name": w.Name, "visibility": w.Visibility, "version": w.Current, "state": webState(w)})
	_ = s.Hub().appendEvent(events.Event{User: s.usuari(), Kind: "web", Text: w.Name, Payload: b})
}
