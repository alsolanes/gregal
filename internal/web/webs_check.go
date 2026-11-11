package web

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Comprovació de l'URL real després de publicar (B11): l'amfitrió demana la
// web on la veurà la gent (<nom>.<domini>, <nom>.lan.<domini> o /w/<nom>/)
// amb un testimoni d'un sol ús que fa servir la versió candidata. Passa si:
// HTTP 200, és l'index.html d'aquesta versió (capçalera de versió) i els
// enllaços interns de la pàgina responen 200. Si no passa, la versió activa
// no canvia.

const (
	webCheckTimeout  = 30 * time.Second
	webCheckMaxLinks = 40
)

// webCheckDial: els tests hi posen un marcador que porta qualsevol host al
// servidor de prova (així es prova l'enrutament per Host sense DNS).
var webCheckDial func(ctx context.Context, network, addr string) (net.Conn, error)

var reWebLink = regexp.MustCompile(`(?i)\s(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

func webCheckClient() *http.Client {
	tr := &http.Transport{Proxy: nil, ResponseHeaderTimeout: 10 * time.Second}
	if webCheckDial != nil {
		tr.DialContext = webCheckDial
	} else {
		// RFC 6761: *.localhost és la mateixa màquina (Go a Windows no ho
		// resol); serveix per provar el domini en local.
		d := &net.Dialer{Timeout: 10 * time.Second}
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if h, p, err := net.SplitHostPort(addr); err == nil && (h == "localhost" || strings.HasSuffix(h, ".localhost")) {
				addr = net.JoinHostPort("127.0.0.1", p)
			}
			return d.DialContext(ctx, network, addr)
		}
	}
	return &http.Client{Transport: tr, Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			req.Header.Set(webCheckHeader, via[0].Header.Get(webCheckHeader))
			return nil
		}}
}

// webCheckURL és l'URL real d'una web segons la visibilitat. Sense domini,
// /w/<nom>/ al mateix Gregal: a la seva adreça pública si en té, si no a
// la que fa servir el client o al port local.
func (s *Server) webCheckURL(name, vis, base string) string {
	if u := s.webPublicURL(name, vis); u != "" {
		return u
	}
	if vis == webLAN || base == "" {
		if gregalListenPort > 0 {
			base = "http://127.0.0.1:" + strconv.Itoa(gregalListenPort)
		}
	}
	if base == "" {
		base = publicBase()
	}
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/w/" + name + "/"
}

// webCheckVersion comprova la versió n de la web name com si fos l'activa.
func (s *Server) webCheckVersion(name string, n int, vis, base string) *webCheck {
	chk := &webCheck{At: time.Now().UTC(), Version: n, URL: s.webCheckURL(name, vis, base)}
	if chk.URL == "" {
		chk.Problems = []string{"the server does not know its own address: set publish.domain or GREGAL_PUBLIC_URL"}
		return chk
	}
	tok := webRandom()
	webChecks.Store(tok, webCheckGrant{name: name, n: n})
	defer webChecks.Delete(tok)
	ctx, cancel := context.WithTimeout(context.Background(), webCheckTimeout)
	defer cancel()
	cl := webCheckClient()
	get := func(u string) (int, string, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return 0, "", nil, err
		}
		req.Header.Set(webCheckHeader, tok)
		req.Header.Set("User-Agent", "Gregal-publish-check")
		res, err := cl.Do(req)
		if err != nil {
			return 0, "", nil, err
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(res.Body, siteMaxFileBytes))
		return res.StatusCode, res.Header.Get(webVersionHeader) + "|" + res.Header.Get("Content-Type"), body, nil
	}
	status, meta, body, err := get(chk.URL)
	chk.Status = status
	if err != nil {
		chk.Problems = append(chk.Problems, "unreachable: "+webErrText(err))
		return chk
	}
	ver, ct, _ := strings.Cut(meta, "|")
	if status != http.StatusOK {
		chk.Problems = append(chk.Problems, fmt.Sprintf("HTTP %d at %s", status, chk.URL))
		return chk
	}
	if ver != strconv.Itoa(n) {
		chk.Problems = append(chk.Problems, "the address does not serve this version (another server, an old route or a cache answers)")
		return chk
	}
	if !strings.HasPrefix(ct, "text/html") {
		chk.Problems = append(chk.Problems, "index.html is not served as HTML")
		return chk
	}
	pageURL, _ := url.Parse(chk.URL)
	seen := map[string]bool{}
	for _, m := range reWebLink.FindAllStringSubmatch(string(body), -1) {
		raw := strings.TrimSpace(m[1] + m[2])
		u := webInternalLink(pageURL, raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		if len(seen) > webCheckMaxLinks {
			break
		}
		st, _, _, err := get(u)
		chk.Links++
		if err != nil {
			chk.Problems = append(chk.Problems, "broken link "+raw+": "+webErrText(err))
		} else if st != http.StatusOK {
			chk.Problems = append(chk.Problems, fmt.Sprintf("broken link %s (HTTP %d)", raw, st))
		}
	}
	chk.OK = len(chk.Problems) == 0
	return chk
}

// webInternalLink: l'URL absoluta d'un enllaç intern de la web, o "".
func webInternalLink(base *url.URL, raw string) string {
	if raw == "" || strings.HasPrefix(raw, "#") || strings.Contains(raw, "{{") || strings.Contains(raw, "${") {
		return ""
	}
	low := strings.ToLower(raw)
	for _, p := range []string{"mailto:", "tel:", "javascript:", "data:", "blob:", "sms:"} {
		if strings.HasPrefix(low, p) {
			return ""
		}
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u := base.ResolveReference(ref)
	if u.Host != base.Host || u.Scheme != base.Scheme {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

func webErrText(err error) string {
	s := err.Error()
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}
