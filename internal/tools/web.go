package tools

// Eines web: cerca i lectura de pàgines pensades per a un model, no per a
// una persona amb navegador.
//
// web_search consulta diversos motors ALHORA (DuckDuckGo i Bing sense cap
// servei local ni clau; SearXNG si n'hi ha un de configurat) i fusiona els
// resultats: el que surt a més d'un motor puja. Abans depenia d'un SearXNG
// a 127.0.0.1:4000 i, sense ell, la cerca simplement no existia.
//
// web_fetch baixa la pàgina i en torna el contingut principal en markdown
// (vegeu webread.go): sense menús, peus, imatges ni formularis. Quan la
// via directa falla (JS, WAF, paywall), les alternatives (render local,
// Wayback) corren en paral·lel i guanya la primera que respon; archive.today
// és l'últim recurs. Les respostes es guarden uns minuts en memòria perquè
// repetir una cerca o una pàgina dins del mateix torn sigui immediat.

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Límit de descàrrega web (anti-bomba) i timeouts.
const (
	WebMaxBodyBytes = 1_500_000
	// WebTimeout és el temps màxim de la via directa d'una pàgina i de
	// cada motor de cerca. 20 s eren massa per a un motor que no respon:
	// bloquejava la resposta de tots els altres.
	WebTimeout = 12 * time.Second
	// SearchEngineTimeout és el que s'espera cada motor. Els bons responen
	// en menys d'un segon; passat això, es fusiona el que hi hagi.
	SearchEngineTimeout = 8 * time.Second
	webCacheTTL         = 10 * time.Minute
)

// webUA és un user-agent de navegador: amb "Gregal/1.0" DuckDuckGo torna
// captcha i força llocs tornen 403.
const webUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

// allowLocalFetch desbloqueja hosts locals (només per tests amb httptest;
// en producció el guard SSRF sempre rebutja loopback i metadades cloud).
var allowLocalFetch = false

// SearXNGBase retorna el SearXNG configurat (SEARXNG_URL) o buit si no
// n'hi ha: ja no s'assumeix cap servei local.
func SearXNGBase() string {
	if v := strings.TrimSpace(os.Getenv("SEARXNG_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return ""
}

// ---- cache ----

type webCacheEntry struct {
	at  time.Time
	out string
}

var (
	webCacheMu sync.Mutex
	webCache   = map[string]webCacheEntry{}
)

func webCacheGet(key string) (string, bool) {
	webCacheMu.Lock()
	defer webCacheMu.Unlock()
	e, ok := webCache[key]
	if !ok || time.Since(e.at) > webCacheTTL {
		delete(webCache, key)
		return "", false
	}
	return e.out, true
}

func webCachePut(key, out string) {
	webCacheMu.Lock()
	defer webCacheMu.Unlock()
	if len(webCache) > 128 {
		for k, e := range webCache {
			if time.Since(e.at) > webCacheTTL {
				delete(webCache, k)
			}
		}
		if len(webCache) > 128 {
			webCache = map[string]webCacheEntry{}
		}
	}
	webCache[key] = webCacheEntry{at: time.Now(), out: out}
}

// WebCacheClear buida la cache (tests).
func WebCacheClear() {
	webCacheMu.Lock()
	webCache = map[string]webCacheEntry{}
	webCacheMu.Unlock()
}

// ---- cerca ----

// SearchHit és un resultat d'un motor.
type SearchHit struct {
	Title   string
	URL     string
	Snippet string
	Engine  string
}

// searchEngine és un motor: nom + funció. La llista és una var perquè els
// tests puguin substituir-la.
type searchEngine struct {
	name string
	run  func(ctx context.Context, query string) ([]SearchHit, error)
}

func webEngines() []searchEngine {
	var e []searchEngine
	if SearXNGBase() != "" {
		e = append(e, searchEngine{"searxng", searchSearXNG})
	}
	if !nomesSearXNG {
		e = append(e, searchEngine{"duckduckgo", searchDuckDuckGo}, searchEngine{"bing", searchBing})
	}
	return e
}

// nomesSearXNG limita els motors al SearXNG configurat (tests: cap
// petició a internet).
var nomesSearXNG = false

// WebSearch cerca en paral·lel a tots els motors disponibles i fusiona.
// Torna títol+url+resum per resultat.
func WebSearch(query string, count int) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("web_search: query buida")
	}
	if count <= 0 || count > 20 {
		count = 8
	}
	key := fmt.Sprintf("s|%d|%s", count, strings.ToLower(query))
	if out, ok := webCacheGet(key); ok {
		return out, nil
	}
	engines := webEngines()
	if len(engines) == 0 {
		return "", fmt.Errorf("web_search: cap motor de cerca disponible")
	}
	ctx, cancel := context.WithTimeout(context.Background(), SearchEngineTimeout+2*time.Second)
	defer cancel()
	type res struct {
		name string
		hits []SearchHit
		err  error
	}
	results := make([]res, len(engines))
	var wg sync.WaitGroup
	for i, e := range engines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ectx, ecancel := context.WithTimeout(ctx, SearchEngineTimeout)
			defer ecancel()
			hits, err := e.run(ectx, query)
			for k := range hits {
				hits[k].Engine = e.name
			}
			results[i] = res{e.name, hits, err}
		}()
	}
	wg.Wait()
	var llistes [][]SearchHit
	errs := 0
	for _, r := range results {
		if r.err != nil {
			errs++
			continue
		}
		llistes = append(llistes, r.hits)
	}
	merged := fusiona(llistes, count)
	if len(merged) == 0 {
		var why []string
		for _, r := range results {
			if r.err != nil {
				why = append(why, r.name+": "+shortErr(r.err))
			} else {
				why = append(why, r.name+": 0 resultats")
			}
		}
		if errs == len(engines) {
			return "", fmt.Errorf("web_search: cap motor ha respost (%s)", strings.Join(why, "; "))
		}
		return "sense resultats (" + strings.Join(why, "; ") + ")", nil
	}
	var b strings.Builder
	var vies []string
	for _, r := range results {
		if r.err == nil && len(r.hits) > 0 {
			vies = append(vies, r.name)
		}
	}
	fmt.Fprintf(&b, "%d resultats per «%s» (via %s). Obre'n els útils amb web_fetch; diverses crides en un mateix pas van en paral·lel.\n", len(merged), truncateInline(query, 80), strings.Join(vies, "+"))
	for i, h := range merged {
		title := strings.TrimSpace(h.Title)
		if title == "" {
			title = h.URL
		}
		fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, truncateInline(title, 160), h.URL)
		if s := strings.TrimSpace(collapseWS(html.UnescapeString(h.Snippet))); s != "" {
			fmt.Fprintf(&b, "   %s\n", truncateInline(s, 400))
		}
	}
	out := truncate(b.String(), MaxOutputChars)
	webCachePut(key, out)
	return out, nil
}

func fusiona(llistes [][]SearchHit, count int) []SearchHit {
	type acc struct {
		hit   SearchHit
		score float64
		ordre int
	}
	byKey := map[string]*acc{}
	var ordre []string
	for _, hits := range llistes {
		for rank, h := range hits {
			u := strings.TrimSpace(h.URL)
			if u == "" || !strings.HasPrefix(u, "http") {
				continue
			}
			k := normURL(u)
			a, ok := byKey[k]
			if !ok {
				a = &acc{hit: h, ordre: len(ordre)}
				byKey[k] = a
				ordre = append(ordre, k)
			} else {
				// Queda el títol i el resum més llargs (més informatius).
				if len(h.Title) > len(a.hit.Title) {
					a.hit.Title = h.Title
				}
				if len(h.Snippet) > len(a.hit.Snippet) {
					a.hit.Snippet = h.Snippet
				}
			}
			a.score += 1 / float64(rank+1)
		}
	}
	accs := make([]*acc, 0, len(ordre))
	for _, k := range ordre {
		accs = append(accs, byKey[k])
	}
	sort.SliceStable(accs, func(i, j int) bool {
		if accs[i].score != accs[j].score {
			return accs[i].score > accs[j].score
		}
		return accs[i].ordre < accs[j].ordre
	})
	out := make([]SearchHit, 0, len(accs))
	for _, a := range accs {
		out = append(out, a.hit)
		if count > 0 && len(out) >= count {
			break
		}
	}
	return out
}

// normURL iguala variants de la mateixa pàgina: esquema, www., barra final,
// paràmetres de seguiment i majúscules del host.
func normURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || lk == "fbclid" || lk == "gclid" || lk == "ref" || lk == "source" {
			q.Del(k)
		}
	}
	path := strings.TrimRight(u.Path, "/")
	s := host + path
	if enc := q.Encode(); enc != "" {
		s += "?" + enc
	}
	return s
}

// searchSearXNG consulta un SearXNG (format=json).
func searchSearXNG(ctx context.Context, query string) ([]SearchHit, error) {
	base := SearXNGBase()
	if base == "" {
		return nil, fmt.Errorf("SearXNG no configurat")
	}
	reqURL := base + "/search?q=" + url.QueryEscape(query) + "&format=json"
	body, err := webGet(ctx, reqURL, "application/json", 2_000_000)
	if err != nil {
		return nil, err
	}
	var data struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("resposta no JSON: %v", err)
	}
	out := make([]SearchHit, 0, len(data.Results))
	for _, r := range data.Results {
		if strings.TrimSpace(r.URL) == "" {
			continue
		}
		out = append(out, SearchHit{Title: r.Title, URL: strings.TrimSpace(r.URL), Snippet: r.Content})
	}
	return out, nil
}

// duckduckgoBase i bingBase són vars per redirigir-los a servidors de prova.
var (
	duckduckgoBase = "https://html.duckduckgo.com/html/"
	bingBase       = "https://www.bing.com/search"
)

// searchDuckDuckGo llegeix la versió HTML (sense JS) de DuckDuckGo.
func searchDuckDuckGo(ctx context.Context, query string) ([]SearchHit, error) {
	body, err := webGet(ctx, duckduckgoBase+"?q="+url.QueryEscape(query)+"&kl=wt-wt", "text/html", 1_000_000)
	if err != nil {
		return nil, err
	}
	return parseDuckDuckGo(string(body))
}

func parseDuckDuckGo(src string) ([]SearchHit, error) {
	root, err := xhtml.Parse(strings.NewReader(src))
	if err != nil {
		return nil, err
	}
	var hits []SearchHit
	var cur *SearchHit
	var walk func(n *xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			cls := attr(n, "class")
			switch {
			case n.DataAtom == atom.A && strings.Contains(cls, "result__a"):
				hits = append(hits, SearchHit{Title: collapseWS(textDe(n)), URL: ddgURL(attr(n, "href"))})
				cur = &hits[len(hits)-1]
				return
			case strings.Contains(cls, "result__snippet"):
				if cur != nil && cur.Snippet == "" {
					cur.Snippet = collapseWS(textDe(n))
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if len(hits) == 0 {
		if t := strings.ToLower(htmlTitle(src)); strings.Contains(t, "anomaly") || strings.Contains(src, "anomaly-modal") {
			return nil, fmt.Errorf("DuckDuckGo demana captcha")
		}
	}
	out := hits[:0]
	for _, h := range hits {
		if h.URL != "" && !strings.Contains(h.URL, "duckduckgo.com/y.js") {
			out = append(out, h)
		}
	}
	return out, nil
}

// ddgURL desfà el redirector //duckduckgo.com/l/?uddg=<url>.
func ddgURL(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if strings.Contains(u.Host, "duckduckgo.com") {
		if d := u.Query().Get("uddg"); d != "" {
			return d
		}
		return ""
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return u.String()
	}
	return ""
}

// searchBing llegeix el canal RSS de Bing (XML net, sense scraping).
func searchBing(ctx context.Context, query string) ([]SearchHit, error) {
	// setlang/cc fixos: amb l'Accept-Language en català el canal RSS de
	// Bing tornava resultats d'una altra cerca (vist en viu: fòrums de
	// correu per a una consulta sobre una biblioteca Go).
	body, err := webGet(ctx, bingBase+"?format=rss&setlang=en&cc=US&q="+url.QueryEscape(query), "application/rss+xml, application/xml, text/xml", 1_000_000)
	if err != nil {
		return nil, err
	}
	return parseBingRSS(body)
}

func parseBingRSS(body []byte) ([]SearchHit, error) {
	var rss struct {
		Items []struct {
			Title string `xml:"title"`
			Link  string `xml:"link"`
			Desc  string `xml:"description"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("RSS il·legible: %v", err)
	}
	out := make([]SearchHit, 0, len(rss.Items))
	for _, it := range rss.Items {
		link := strings.TrimSpace(it.Link)
		if link == "" {
			continue
		}
		out = append(out, SearchHit{
			Title:   collapseWS(html.UnescapeString(reTag.ReplaceAllString(it.Title, ""))),
			URL:     link,
			Snippet: collapseWS(html.UnescapeString(reTag.ReplaceAllString(it.Desc, ""))),
		})
	}
	return out, nil
}

// webGet fa un GET amb capçaleres de navegador i cos limitat.
func webGet(ctx context.Context, target, accept string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", webUA)
	req.Header.Set("Accept", accept+";q=0.9,*/*;q=0.5")
	req.Header.Set("Accept-Language", "ca,es;q=0.8,en;q=0.7")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// WebEnginesDisponibles prova cada motor amb una cerca curta i diu quins
// responen (per al --doctor).
func WebEnginesDisponibles(ctx context.Context) (ok []string, ko []string) {
	for _, e := range webEngines() {
		ectx, cancel := context.WithTimeout(ctx, 6*time.Second)
		hits, err := e.run(ectx, "gregal agent")
		cancel()
		if err != nil || len(hits) == 0 {
			ko = append(ko, e.name)
			continue
		}
		ok = append(ok, e.name)
	}
	return ok, ko
}

// ---- lectura de pàgines ----

// webBlockedHost rebutja loopback, 0.0.0.0, metadades cloud i noms locals.
// El DNS rebinding queda fora d'abast (agent monousuari a xarxa domèstica).
func webBlockedHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return true
	}
	if hn, _, err := net.SplitHostPort(h); err == nil {
		h = hn
	}
	h = strings.Trim(h, "[]")
	if h == "localhost" || h == "localhost.localdomain" || h == "0.0.0.0" ||
		h == "169.254.169.254" || h == "::" || h == "::1" {
		return true
	}
	if strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".lan") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
			ip.IsMulticast() || ip.IsUnspecified()
	}
	return false
}

// Firecrawl local (render amb navegador, sense claus ni tercers).
const FirecrawlTimeout = 60 * time.Second

// waybackAPIBase i archiveTodayDomains són vars per poder-les redirigir a
// servidors de prova en els tests.
var waybackAPIBase = "https://archive.org"

var archiveTodayDomains = []string{"archive.ph", "archive.md", "archive.li", "archive.is"}

// FirecrawlBase retorna el Firecrawl local (FIRECRAWL_URL o 127.0.0.1:3002).
func FirecrawlBase() string {
	if v := strings.TrimSpace(os.Getenv("FIRECRAWL_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://127.0.0.1:3002"
}

// WebFetchOpts són les opcions de lectura d'una pàgina.
type WebFetchOpts struct {
	MaxChars int
	// Find, si no és buit, torna només els paràgrafs que contenen el text
	// (o les seves paraules), amb el seu número: per a pàgines llargues.
	Find string
	// Raw torna la pàgina sencera en text pla (sense extreure el bloc
	// principal): per quan el lector deixa fora el que buscaves.
	Raw bool
}

// WebFetch baixa una URL i en torna el contingut principal en markdown.
// Manté la signatura antiga; les opcions noves són a WebFetchOpts.
func WebFetch(rawURL string, maxChars int) (string, error) {
	return WebFetchOpts_(rawURL, WebFetchOpts{MaxChars: maxChars})
}

// WebFetchOpts_ és WebFetch amb opcions: cadena de recuperació directe →
// (render local ‖ còpia arxivada, en paral·lel) → archive.today. Cada via
// que no és en viu porta etiqueta de procedència.
func WebFetchOpts_(rawURL string, o WebFetchOpts) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("web_fetch: cal una URL http(s) completa")
	}
	if !allowLocalFetch && webBlockedHost(u.Host) {
		return "", fmt.Errorf("web_fetch: host local o reservat bloquejat")
	}
	if o.MaxChars <= 0 || o.MaxChars > 30000 {
		o.MaxChars = MaxOutputChars
	}
	key := "f|" + u.String()
	if o.Raw {
		key += "|raw"
	}
	var pg *pagina
	if cached, ok := webCacheGet(key); ok {
		pg = &pagina{}
		_ = json.Unmarshal([]byte(cached), pg)
	}
	if pg == nil {
		var attempts []string
		p, err := webFetchDirectPage(u.String(), WebTimeout, o.Raw)
		if err == nil {
			pg = p
		} else {
			attempts = append(attempts, "directe: "+shortErr(err))
			p, why := webFetchFallbacks(u.String(), o.Raw, !webBlockedHost(u.Host))
			attempts = append(attempts, why...)
			if p == nil {
				return "", fmt.Errorf("web_fetch: pàgina il·legible (%s)", strings.Join(attempts, "; "))
			}
			pg = p
		}
		if raw, err := json.Marshal(pg); err == nil {
			webCachePut(key, string(raw))
		}
	}
	return pg.render(o), nil
}

// pagina és el resultat llegit (es desa a la cache com a JSON).
type pagina struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Byline    string `json:"byline"`
	Published string `json:"published"`
	Words     int    `json:"words"`
	Whole     bool   `json:"whole"`
	Body      string `json:"body"`
	Origen    string `json:"origen"` // "" en viu; si no, etiqueta de procedència
}

// render munta la sortida per al model: capçalera curta + cos, amb find i
// retall.
func (p *pagina) render(o WebFetchOpts) string {
	var h strings.Builder
	if p.Origen != "" {
		h.WriteString(p.Origen + "\n")
	}
	if t := strings.TrimSpace(p.Title); t != "" {
		h.WriteString("# " + t + "\n")
	}
	meta := []string{p.URL}
	if p.Published != "" {
		meta = append(meta, "data "+p.Published)
	}
	if p.Byline != "" {
		meta = append(meta, "autor "+p.Byline)
	}
	meta = append(meta, fmt.Sprintf("%d paraules", p.Words))
	if p.Whole {
		meta = append(meta, "pàgina sencera")
	} else {
		meta = append(meta, "contingut principal")
	}
	h.WriteString(strings.Join(meta, " · ") + "\n\n")
	body := p.Body
	if f := strings.TrimSpace(o.Find); f != "" {
		body = cercaParagrafs(body, f)
	}
	out := h.String() + body
	if len(out) > o.MaxChars {
		tall := o.MaxChars
		for tall > 0 && tall < len(out) && (out[tall]&0xC0) == 0x80 {
			tall--
		}
		nota := fmt.Sprintf("\n…(retallat: %d de %d caràcters; demana max_chars més gran, o find:\"text\" per veure només els paràgrafs que el contenen)", tall, len(out))
		out = out[:tall] + nota
	}
	return out
}

// cercaParagrafs torna els paràgrafs que contenen el text (o, si cap no el
// conté sencer, els que contenen alguna de les seves paraules de 4+ lletres).
func cercaParagrafs(body, find string) string {
	pars := strings.Split(body, "\n\n")
	lf := strings.ToLower(find)
	match := func(p string, terms []string) bool {
		lp := strings.ToLower(p)
		for _, t := range terms {
			if strings.Contains(lp, t) {
				return true
			}
		}
		return false
	}
	var idx []int
	for i, p := range pars {
		if match(p, []string{lf}) {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		var terms []string
		for _, w := range strings.Fields(lf) {
			if len([]rune(w)) >= 4 {
				terms = append(terms, w)
			}
		}
		if len(terms) > 0 {
			for i, p := range pars {
				if match(p, terms) {
					idx = append(idx, i)
				}
			}
		}
	}
	if len(idx) == 0 {
		return fmt.Sprintf("(cap paràgraf conté «%s»; la pàgina té %d paràgrafs)", find, len(pars))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d de %d paràgrafs contenen «%s»:\n\n", len(idx), len(pars), find)
	for _, i := range idx {
		fmt.Fprintf(&b, "[¶%d] %s\n\n", i+1, strings.TrimSpace(pars[i]))
	}
	return strings.TrimSpace(b.String())
}

// webFetchFallbacks corre el render local i el Wayback alhora i es queda
// amb el primer que respon; si cap, archive.today. Torna la pàgina i les
// raons de cada via que ha fallat.
// ambArxius=false (hosts locals: només tests) no consulta Wayback ni
// archive.today: cap arxiu públic té pàgines d'una xarxa privada.
func webFetchFallbacks(target string, raw bool, ambArxius bool) (*pagina, []string) {
	type r struct {
		p   *pagina
		via string
		err error
	}
	ch := make(chan r, 2)
	go func() {
		md, err := webFetchFirecrawl(target)
		if err != nil {
			ch <- r{nil, "render", err}
			return
		}
		ch <- r{&pagina{URL: target, Title: firstLineTitle(md), Body: md, Words: comptaParaules(md),
			Origen: "〔render en viu via navegador local〕"}, "render", nil}
	}()
	go func() {
		if !ambArxius {
			ch <- r{nil, "arxiu", fmt.Errorf("web_fetch: host privat, sense arxiu")}
			return
		}
		p, when, err := webFetchWaybackPage(target, raw)
		if err != nil {
			ch <- r{nil, "arxiu", err}
			return
		}
		p.Origen = fmt.Sprintf("〔còpia arxivada del %s — pot estar desactualitzada〕", when)
		ch <- r{p, "arxiu", nil}
	}()
	var why []string
	var arxiu *pagina
	for i := 0; i < 2; i++ {
		got := <-ch
		if got.err != nil {
			why = append(why, got.via+": "+shortErr(got.err))
			continue
		}
		if got.via == "render" {
			return got.p, why
		}
		arxiu = got.p
	}
	if arxiu != nil {
		return arxiu, why
	}
	if !ambArxius {
		return nil, why
	}
	if text, err := webFetchArchiveToday(target); err == nil {
		return &pagina{URL: target, Body: text, Words: comptaParaules(text), Whole: true,
			Origen: "〔còpia d'arxiu (archive.today) — verifica la data a la capçalera〕"}, why
	} else {
		why = append(why, "archive.today: "+shortErr(err))
	}
	return nil, why
}

func firstLineTitle(md string) string {
	l := strings.TrimSpace(firstLine(md))
	return strings.TrimSpace(strings.TrimLeft(l, "# "))
}

func shortErr(err error) string {
	s := strings.TrimSpace(err.Error())
	s = strings.TrimPrefix(s, "web_fetch: ")
	return truncateInline(s, 90)
}

// webFetchDirect baixa una URL i la torna en text (per a compatibilitat i
// per a les vies d'arxiu, que ja porten l'HTML de la pàgina original).
func webFetchDirect(target string, timeout time.Duration) (string, error) {
	p, err := webFetchDirectPage(target, timeout, false)
	if err != nil {
		return "", err
	}
	return p.Body, nil
}

// webFetchDirectPage baixa una URL http/https i en fa la lectura. Rebutja
// també els 200-trampa (intersticials WAF/captcha amb cos fals).
func webFetchDirectPage(target string, timeout time.Duration, raw bool) (*pagina, error) {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("massa redireccions")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirecció a esquema no suportat")
			}
			if !allowLocalFetch && webBlockedHost(req.URL.Host) {
				return fmt.Errorf("redirecció a host bloquejat")
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", webUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,application/json;q=0.8,*/*;q=0.5")
	req.Header.Set("Accept-Language", "ca,es;q=0.8,en;q=0.7")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web_fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("web_fetch: HTTP %d", resp.StatusCode)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "text") && !strings.Contains(ct, "json") &&
		!strings.Contains(ct, "xml") {
		return nil, fmt.Errorf("web_fetch: contingut no textual (%s)", ct)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, WebMaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > WebMaxBodyBytes {
		return nil, fmt.Errorf("web_fetch: pàgina massa gran (>1.5MB)")
	}
	final := target
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return llegeixCos(final, ct, string(body), raw)
}

// llegeixCos converteix el cos segons el tipus: HTML → lector; JSON/text →
// tal qual (compactat).
func llegeixCos(final, ct, body string, raw bool) (*pagina, error) {
	s := strings.TrimSpace(body)
	esHTML := strings.Contains(ct, "html") || (ct == "" && (strings.HasPrefix(strings.ToLower(s), "<!doctype") || strings.HasPrefix(strings.ToLower(s), "<html"))) ||
		(strings.Contains(ct, "xml") && strings.Contains(strings.ToLower(s[:min(len(s), 300)]), "<html"))
	if !esHTML {
		if strings.Contains(ct, "json") {
			var v any
			if json.Unmarshal([]byte(s), &v) == nil {
				if pretty, err := json.MarshalIndent(v, "", " "); err == nil {
					s = string(pretty)
				}
			}
		}
		if s == "" {
			return nil, fmt.Errorf("web_fetch: la pàgina no té text llegible")
		}
		return &pagina{URL: final, Body: s, Words: comptaParaules(s), Whole: true}, nil
	}
	if title := htmlTitle(body); isInterstitialTitle(title) {
		return nil, fmt.Errorf("web_fetch: intersticial de bot (%s)", truncateInline(title, 60))
	}
	if raw {
		text := htmlToText(body)
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("web_fetch: la pàgina no té text llegible")
		}
		return &pagina{URL: final, Title: netejaTitol(htmlTitle(body)), Body: text, Words: comptaParaules(text), Whole: true}, nil
	}
	base, _ := url.Parse(final)
	doc := ReadableHTML(body, base)
	if strings.TrimSpace(doc.Markdown) == "" {
		return nil, fmt.Errorf("web_fetch: la pàgina no té text llegible")
	}
	if doc.Canonical != "" && strings.HasPrefix(doc.Canonical, "http") {
		final = doc.Canonical
	}
	return &pagina{URL: final, Title: doc.Title, Byline: doc.Byline, Published: doc.Published,
		Words: doc.Words, Whole: doc.Whole, Body: doc.Markdown}, nil
}

var (
	reScript  = regexp.MustCompile(`(?is)<(script|style|noscript|template|svg)[^>]*>.*?</(script|style|noscript|template|svg)>`)
	reComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reTag     = regexp.MustCompile(`<[^>]+>`)
	reWS      = regexp.MustCompile(`[ \t\f\v]+`)
)

// htmlToText converteix HTML en text pla (pàgina sencera, sense lector).
func htmlToText(src string) string {
	s := reScript.ReplaceAllString(src, "\n")
	s = reComment.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "</p>", "\n")
	s = strings.ReplaceAll(s, "<br", "\n<br")
	s = strings.ReplaceAll(s, "</li>", "\n")
	s = strings.ReplaceAll(s, "</h1>", "\n")
	s = strings.ReplaceAll(s, "</h2>", "\n")
	s = strings.ReplaceAll(s, "</h3>", "\n")
	s = strings.ReplaceAll(s, "</h4>", "\n")
	s = strings.ReplaceAll(s, "</tr>", "\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var lines []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(reWS.ReplaceAllString(ln, " "))
		if ln != "" {
			lines = append(lines, ln)
		}
	}
	return strings.Join(lines, "\n")
}

func collapseWS(s string) string {
	return strings.TrimSpace(reWS.ReplaceAllString(strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), " ", " "), " "))
}

func truncateInline(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	tall := n
	for tall > 0 && (s[tall]&0xC0) == 0x80 {
		tall--
	}
	return s[:tall] + "…"
}

var reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// htmlTitle extreu el <title> (per detectar intersticials abans de netejar).
func htmlTitle(body string) string {
	m := reTitle.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(collapseWS(html.UnescapeString(reTag.ReplaceAllString(m[1], ""))))
}

// isInterstitialTitle detecta 200-trampa: Cloudflare, captchas i WAF.
func isInterstitialTitle(title string) bool {
	lower := strings.ToLower(title)
	markers := []string{
		"just a moment", "attention required", "verify you are human",
		"verifying you are", "are you a robot", "captcha", "access denied",
		"403 forbidden", "406 waf", "waf forbidden", "redirecting",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// firecrawlDisponible diu si hi ha un render local escoltant (comprovació
// de port, 300 ms, recordada un minut): sense ell no s'espera cap timeout.
func firecrawlDisponible() bool {
	base := FirecrawlBase()
	if v, ok := webCacheGet("fc|" + base); ok {
		return v == "1"
	}
	u, err := url.Parse(base)
	ok := false
	if err == nil {
		host := u.Host
		if !strings.Contains(host, ":") {
			host += ":80"
		}
		if c, err := net.DialTimeout("tcp", host, 300*time.Millisecond); err == nil {
			c.Close()
			ok = true
		}
	}
	if ok {
		webCachePut("fc|"+base, "1")
	} else {
		webCachePut("fc|"+base, "0")
	}
	return ok
}

// webFetchFirecrawl renderitza amb el navegador local (JS, SPA i WAF lleugers).
func webFetchFirecrawl(target string) (string, error) {
	if !firecrawlDisponible() {
		return "", fmt.Errorf("web_fetch: render local no disponible (%s)", FirecrawlBase())
	}
	payload, _ := json.Marshal(map[string]any{
		"url": target, "formats": []string{"markdown"}, "onlyMainContent": true,
	})
	client := &http.Client{Timeout: FirecrawlTimeout}
	req, err := http.NewRequest("POST", FirecrawlBase()+"/v1/scrape", strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_fetch: render no respon: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2_000_000))
	if err != nil {
		return "", err
	}
	var data struct {
		Success bool `json:"success"`
		Data    struct {
			Markdown string `json:"markdown"`
			Title    string `json:"title"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &data); err != nil || !data.Success {
		msg := data.Error
		if msg == "" {
			msg = "resposta no vàlida"
		}
		return "", fmt.Errorf("web_fetch: render fallit (%s)", truncateInline(msg, 80))
	}
	md := strings.TrimSpace(data.Data.Markdown)
	if md == "" || isInterstitialTitle(data.Data.Title) || isInterstitialTitle(firstLine(md)) {
		return "", fmt.Errorf("web_fetch: el render ha tornat un intersticial")
	}
	return md, nil
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// webFetchWayback recupera la snapshot més propera (text, data).
func webFetchWayback(target string) (string, string, error) {
	p, when, err := webFetchWaybackPage(target, false)
	if err != nil {
		return "", "", err
	}
	return p.Body, when, nil
}

func webFetchWaybackPage(target string, raw bool) (*pagina, string, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	api := waybackAPIBase + "/wayback/available?url=" + url.QueryEscape(target)
	resp, err := client.Get(api)
	if err != nil {
		return nil, "", fmt.Errorf("web_fetch: arxiu no respon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("web_fetch: arxiu HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 200_000))
	if err != nil {
		return nil, "", err
	}
	var data struct {
		Snapshots struct {
			Closest struct {
				URL       string `json:"url"`
				Timestamp string `json:"timestamp"`
			} `json:"closest"`
		} `json:"archived_snapshots"`
	}
	if err := json.Unmarshal(body, &data); err != nil || data.Snapshots.Closest.URL == "" {
		return nil, "", fmt.Errorf("web_fetch: sense còpia arxivada")
	}
	p, err := webFetchDirectPage(data.Snapshots.Closest.URL, 20*time.Second, raw)
	if err != nil {
		return nil, "", err
	}
	p.URL = target
	ts := data.Snapshots.Closest.Timestamp
	when := ts
	if len(ts) >= 8 {
		when = ts[0:4] + "-" + ts[4:6] + "-" + ts[6:8]
	}
	return p, when, nil
}

// webFetchArchiveToday prova els dominis rotatius (paywalls i esborrats).
func webFetchArchiveToday(target string) (string, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	for _, d := range archiveTodayDomains {
		req, err := http.NewRequest("GET", "https://"+d+"/newest/"+target, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", webUA)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, WebMaxBodyBytes+1))
		resp.Body.Close()
		if err != nil || resp.StatusCode == http.StatusTooManyRequests {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		s := string(body)
		if !archiveBodyOK(s) {
			continue
		}
		base, _ := url.Parse(target)
		doc := ReadableHTML(s, base)
		text := doc.Markdown
		if strings.TrimSpace(text) == "" {
			continue
		}
		return text, nil
	}
	return "", fmt.Errorf("web_fetch: archive.today sense còpia útil")
}

// archiveBodyOK rebutja els 429/intersticials amb cos gros (èxits falsos).
func archiveBodyOK(body string) bool {
	if len(body) < 3000 {
		return false
	}
	lower := strings.ToLower(body)
	for _, m := range []string{
		"too many requests", "retry later", "rate limit", "rate-limit",
		"just a moment", "attention required", "are you a robot",
	} {
		if strings.Contains(lower, m) {
			return false
		}
	}
	return !isInterstitialTitle(htmlTitle(body))
}
