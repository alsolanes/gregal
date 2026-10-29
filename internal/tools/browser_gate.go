package tools

// Permís del navegador per lloc, com Claude in Chrome i l'agent de ChatGPT:
//
//   - la primera acció sobre un lloc nou (clicar, escriure, triar…) demana
//     permís; un cop aprovada, la resta d'accions d'aquell lloc passen soles
//     durant la sessió. Llegir amb el perfil propi del gregal no en demana;
//     amb el Chrome de l'usuari (les seves sessions), fins i tot llegir un
//     lloc nou en demana.
//   - una acció irreversible (enviar, pagar, comprar, esborrar, publicar,
//     confirmar…) pregunta SEMPRE, també amb aprovació automàtica.
//   - les contrasenyes, targetes i codis d'un sol ús no s'escriuen mai
//     (browser_ax.go, jsCredential): ho fa la persona.
//
// localhost i 127.0.0.1 (les apps que l'usuari desenvolupa) no demanen
// permís de lloc.
//
// Amb el perfil propi del gregal, el permís de lloc és una aprovació normal
// (l'aprovació automàtica i el mode autònom la passen: les tasques de fons no
// es queden penjades). Amb el Chrome de l'usuari i en les irreversibles és
// forçada.

import (
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

var (
	sitesMu sync.Mutex
	sites   = map[string]map[string]bool{} // sessió → host → permès
)

// allowSite deixa actuar a host durant la sessió.
func allowSite(scope, host string) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return
	}
	sitesMu.Lock()
	defer sitesMu.Unlock()
	if sites[scope] == nil {
		sites[scope] = map[string]bool{}
	}
	sites[scope][host] = true
}

func siteAllowed(scope, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || loopbackHost(host) {
		return true
	}
	sitesMu.Lock()
	defer sitesMu.Unlock()
	return sites[scope][host]
}

// ForgetBrowserSites oblida els llocs permesos (sessió nova o tancada).
func ForgetBrowserSites(scope string) {
	sitesMu.Lock()
	delete(sites, scope)
	sitesMu.Unlock()
}

func loopbackHost(h string) bool {
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	return ip != nil && ip.IsLoopback()
}

func hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// host és el lloc on és ara la pàgina (de la memòria; sense CDP si pot).
func (b *Browser) host() string {
	b.mu.Lock()
	u := ""
	if b.ax != nil {
		u = b.ax.url
	}
	if u == "" {
		u = b.lastURL
	}
	b.mu.Unlock()
	if u == "" && b.t != nil {
		u, _ = b.evalString("location.href")
	}
	return hostOf(u)
}

// callHosts són els llocs que toca una crida: on és ara i on va (open).
func callHosts(current string, args map[string]any) []string {
	steps := []map[string]any{args}
	if strings.EqualFold(browserArgs(args).str("action"), "batch") {
		steps = batchSteps(args)
	}
	var out []string
	add := func(h string) {
		if h == "" {
			return
		}
		for _, x := range out {
			if x == h {
				return
			}
		}
		out = append(out, h)
	}
	cur := current
	for _, s := range steps {
		a := browserArgs(s)
		switch strings.ToLower(a.str("action")) {
		case "open", "goto", "navigate", "tab_new":
			if h := hostOf(a.str("url")); h != "" {
				cur = h
			}
		}
		add(cur)
	}
	return out
}

// irreversibleRe: noms de botons i enllaços que fan alguna cosa que no es
// desfà. Millor preguntar de més que pagar sense voler.
var irreversibleRe = regexp.MustCompile(`(?i)(^|[^\pL])(send|submit|pay|pay now|buy|purchase|order now|place order|complete order|complete purchase|confirm|delete|remove|erase|publish|post|tweet|reply|transfer|withdraw|unsubscribe|book now|reserve|sign and|accept and|enviar|envia|envieu|pagar|paga|comprar|compra|confirmar|confirma|eliminar|elimina|esborrar|esborra|suprimir|suprimeix|publicar|publica|transferir|transfereix|reservar|reserva|tramitar|tramita|finalitzar|finalizar|borrar|darse de baja|donar-se de baixa)($|[^\pL])`)

// IrreversibleName diu si el nom d'un botó o enllaç sembla irreversible.
func IrreversibleName(name string) bool {
	return irreversibleRe.MatchString(strings.TrimSpace(name))
}

// sessionBrowser és el navegador obert d'una sessió (nil si no n'hi ha).
func sessionBrowser(scope string) *Browser {
	browserMu.Lock()
	defer browserMu.Unlock()
	if scope == "" {
		return browserInst
	}
	return browserSessions[scope]
}

// wantFromArgs tria on va la crida: chrome:true/false mana; si no, el
// Chrome de l'usuari es manté mentre la sessió hi treballi; si no, el config.
func wantFromArgs(scope string, a browserArgs) string {
	eff := EffectiveBrowserMode()
	if a.has("chrome") {
		if a.boolean("chrome") {
			return BrowserModeChrome
		}
		if eff == BrowserModeChrome {
			return chooseBrowserMode(BrowserModeAuto, false, browserAppBridge() != "")
		}
		return eff
	}
	// També si ha mort (l'usuari ha aturat): no es canvia en silenci al
	// navegador propi; la crida falla i l'agent ho ha de preguntar.
	if b := sessionBrowser(scope); b != nil && b.want == BrowserModeChrome {
		return BrowserModeChrome
	}
	return eff
}

// BrowserGate decideix el permís d'una crida browser abans d'executar-la.
// decision: "allow", "ask" o "deny"; force: preguntar encara que hi hagi
// aprovació automàtica o un «recorda-ho» (accions irreversibles).
// autonomous relaxa el permís de lloc al perfil propi del gregal (mai al
// Chrome de l'usuari ni les accions irreversibles).
func BrowserGate(scope string, args map[string]any, autonomous bool) (decision, reason string, force bool) {
	a := browserArgs(args)
	chrome := wantFromArgs(scope, a) == BrowserModeChrome
	b := sessionBrowser(scope)
	steps := []map[string]any{args}
	if strings.EqualFold(a.str("action"), "batch") {
		steps = batchSteps(args)
	}
	for _, s := range steps {
		sa := browserArgs(s)
		switch strings.ToLower(sa.str("action")) {
		case "click":
			if b == nil {
				continue
			}
			if _, name, ok := b.RefInfo(sa.target()); ok && IrreversibleName(name) {
				return "ask", "browser: irreversible action «" + axClip(name, 60) + "» — confirm it", true
			}
			if t := sa.target(); t != "" && IrreversibleName(t) {
				return "ask", "browser: irreversible action «" + axClip(t, 60) + "» — confirm it", true
			}
		case "dialog":
			if sa.boolean("accept") || sa.str("text") != "" {
				return "ask", "browser: accept a page dialog (usually «are you sure?») — confirm it", true
			}
		}
	}
	needSite := chrome || !BrowserCallSegura(args)
	if !needSite || (autonomous && !chrome) {
		return "allow", "", false
	}
	cur := ""
	if b != nil && b.alive() && (b.want == BrowserModeChrome) == chrome {
		cur = b.host()
	}
	for _, h := range callHosts(cur, args) {
		if !siteAllowed(scope, h) {
			if chrome {
				// Les sessions de l'usuari: es pregunta encara que hi hagi
				// aprovació automàtica.
				return "ask", "browser: use your Chrome on " + h + " (once allowed, Gregal can read and act there for this session)", true
			}
			return "ask", "browser: act on " + h + " (once allowed, Gregal can click and type there for this session)", false
		}
	}
	return "allow", "", false
}
