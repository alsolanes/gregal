package web

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Logos de les comparacions (logo_domain a ```gregal-compare): la icona que
// el mateix web declara (apple-touch-icon, icon o /favicon.ico), llegida pel
// servidor i comprovada que és una imatge. Sense claus ni serveis de tercers,
// i mai una adreça que no s'ha comprovat: sense logo, la fitxa porta la inicial.

// logoSiteURL és la portada d'un domini; els tests la redirigeixen.
var logoSiteURL = func(domain string) string { return "https://" + domain + "/" }

var (
	logoDomainRe = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,24}$`)
	logoLinkRe   = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	logoAttrRe   = regexp.MustCompile(`(?is)\b(rel|href|sizes)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	// logoCache: domini → logo ("" si no en té). Es buida en passar de 500.
	logoCache = map[string]string{}
	logoMu    sync.Mutex
)

// L'adreça del web la tria el model (logo_domain), així que el servidor no
// pot anar enlloc que no sigui un web públic: només noms (no IP ni
// «localhost»), http(s) als ports de sempre, com a molt tres redireccions
// mirades una per una, i cada connexió, també les de les redireccions, passa
// per logoDialContext, que només connecta a adreces públiques (ni loopback,
// ni xarxes privades, ni link-local, ni CGNAT): un nom públic que apunta a
// dins de casa, o que canvia d'adreça entre la consulta i la connexió, no hi arriba.
var (
	// logoLookup i logoDial: els tests els substitueixen per veure que no es connecta.
	logoLookup = net.DefaultResolver.LookupIPAddr
	logoDial   = (&net.Dialer{Timeout: 4 * time.Second, Control: logoDialControl}).DialContext
	cgnat      = net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}
)

// logoClient: els tests hi posen el client del servidor de prova.
var logoClient = &http.Client{
	Timeout:       6 * time.Second,
	Transport:     &http.Transport{DialContext: logoDialContext, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 4 * time.Second},
	CheckRedirect: logoRedirect,
}

func logoRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 || !logoURLOK(req.URL) {
		return errors.New("redirect refused")
	}
	return nil
}

// logoURLOK: http(s), port per defecte i un nom amb punt (no una IP).
func logoURLOK(u *url.URL) bool {
	h := u.Hostname()
	return (u.Scheme == "https" || u.Scheme == "http") && (u.Port() == "" || u.Port() == "80" || u.Port() == "443") &&
		strings.Contains(h, ".") && net.ParseIP(h) == nil && !strings.HasSuffix(h, ".localhost")
}

// internalIP: una adreça on el servidor no ha d'anar mai a buscar un logo.
func internalIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip)
}

// logoDialContext resol el nom, rebutja el que no és públic i connecta a
// l'adreça que ha mirat (no torna a resoldre).
func logoDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if port != "80" && port != "443" {
		return nil, errors.New("logo: port not allowed")
	}
	ips, err := logoLookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("logo: no address")
	}
	for _, ip := range ips {
		if internalIP(ip.IP) {
			return nil, errors.New("logo: address not public")
		}
	}
	return logoDial(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// logoDialControl torna a mirar l'adreça just abans de connectar.
func logoDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || internalIP(ip) {
		return errors.New("logo: address not public")
	}
	return nil
}

// cardLogo torna l'adreça del logo d'un domini, o "" si no en té cap que
// funcioni.
func cardLogo(ctx context.Context, domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://"), "/")
	if !logoDomainRe.MatchString(d) || strings.HasSuffix(d, ".local") || strings.HasSuffix(d, ".lan") || strings.HasSuffix(d, ".localhost") {
		return ""
	}
	logoMu.Lock()
	u, ok := logoCache[d]
	logoMu.Unlock()
	if ok {
		return u
	}
	u = findLogo(ctx, d)
	// Un temps esgotat no vol dir que no en tingui: la propera vegada es torna a mirar.
	if ctx.Err() == nil {
		logoMu.Lock()
		if len(logoCache) >= 500 {
			logoCache = map[string]string{}
		}
		logoCache[d] = u
		logoMu.Unlock()
	}
	return u
}

func findLogo(ctx context.Context, domain string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logoSiteURL(domain), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Gregal)")
	resp, err := logoClient.Do(req)
	if err != nil {
		return ""
	}
	html, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if cerr := resp.Body.Close(); err != nil || cerr != nil {
		return ""
	}
	base := resp.Request.URL
	cands := append(logoCandidates(string(html), base), base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String())
	for i, u := range cands {
		if i < 3 && isImageURL(ctx, u) {
			return u
		}
	}
	return ""
}

// logoCandidates: les icones que declara la pàgina, de millor a pitjor
// (apple-touch-icon, i després la icona més gran).
func logoCandidates(html string, base *url.URL) []string {
	type cand struct {
		u    string
		rank int
	}
	var cands []cand
	for _, tag := range logoLinkRe.FindAllString(html, -1) {
		attrs := map[string]string{}
		for _, m := range logoAttrRe.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(m[1])] = strings.TrimSpace(m[2] + m[3] + m[4])
		}
		rank := 0
		for _, r := range strings.Fields(strings.ToLower(attrs["rel"])) {
			switch r {
			case "apple-touch-icon", "apple-touch-icon-precomposed":
				rank = 1000
			case "icon":
				// «192x192» val més que «16x16»; sense mida, petita.
				rank = max(rank, 16)
				if n, err := strconv.Atoi(strings.SplitN(strings.Fields(attrs["sizes"] + " 0")[0], "x", 2)[0]); err == nil {
					rank = max(rank, n)
				}
			}
		}
		if u, err := base.Parse(attrs["href"]); rank > 0 && attrs["href"] != "" && err == nil && (u.Scheme == "https" || u.Scheme == "http") {
			cands = append(cands, cand{u.String(), rank})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].rank > cands[j].rank })
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.u
	}
	return out
}

// isImageURL: l'adreça respon 200 amb una imatge.
func isImageURL(ctx context.Context, u string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Gregal)")
	resp, err := logoClient.Do(req)
	if err != nil {
		return false
	}
	ok := resp.StatusCode == http.StatusOK && strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/")
	return resp.Body.Close() == nil && ok
}
