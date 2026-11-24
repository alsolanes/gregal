package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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

var logoClient = &http.Client{
	Timeout: 4 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
			return errors.New("redirect refused")
		}
		return nil
	},
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
