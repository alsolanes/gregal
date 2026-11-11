package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
)

// Un amfitrió de prova amb tres comptes genèrics: anna i bert poden
// publicar (publish.publishers), cesc no. Cap ordinador configurat: les webs
// s'han de servir igualment (l'ordinador aturat no hi intervé).
type websRig struct {
	t     *testing.T
	s     *Server
	h     http.Handler
	srv   *httptest.Server
	toks  map[string]string
	check int // comprovacions fetes (pel marcador)
}

func newWebsRig(t *testing.T, domain string) *websRig {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_PUBLIC_URL", "")
	t.Setenv("GREGAL_PUBLISH_DOMAIN", "")
	t.Setenv("GREGAL_ORDINADORS", "")
	s := hubTestServer(t)
	s.cfg.Publish = config.PublishCfg{Domain: domain, Scheme: "http", Publishers: []string{"anna", "bert"}}
	s.cfg.Users = map[string]config.UserCfg{
		"anna": {Password: HashPassword("clau-anna-01")},
		"bert": {Password: HashPassword("clau-bert-01")},
		"cesc": {Password: HashPassword("clau-cesc-01")},
	}
	s.users = NewUsers(s.cfg, "")
	s.users.path = filepath.Join(t.TempDir(), "tokens.json")
	rig := &websRig{t: t, s: s, h: s.Hub().Handler(), toks: map[string]string{}}
	rig.srv = httptest.NewServer(rig.h)
	t.Cleanup(rig.srv.Close)
	// La comprovació demana l'URL real; aquí qualsevol host va al servidor
	// de prova (com si el DNS comodí apuntés a l'amfitrió).
	addr := rig.srv.Listener.Addr().String()
	old := webCheckDial
	webCheckDial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		rig.check++
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	t.Cleanup(func() { webCheckDial = old })
	for _, u := range []string{"anna", "bert", "cesc"} {
		rec := rig.do("", "POST", "http://gregal.test/api/login", `{"user":"`+u+`","password":"clau-`+u+`-01"}`, nil)
		var out struct{ Token string }
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if out.Token == "" {
			t.Fatalf("login %s: %d %s", u, rec.Code, rec.Body)
		}
		rig.toks[u] = out.Token
	}
	return rig
}

// do fa una petició al handler complet. target pot portar host propi.
func (r *websRig) do(user, method, target, body string, mod func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:40000"
	if user != "" {
		req.Header.Set("Authorization", "Bearer "+r.toks[user])
	}
	if mod != nil {
		mod(req)
	}
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	return rec
}

func (r *websRig) api(user, method, path, body string, want int) map[string]any {
	r.t.Helper()
	rec := r.do(user, method, "http://gregal.test"+path, body, nil)
	if rec.Code != want {
		r.t.Fatalf("%s %s %s → %d (vull %d) %s", user, method, path, rec.Code, want, rec.Body)
	}
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func (r *websRig) publish(user, name, html, vis string, want int) map[string]any {
	r.t.Helper()
	b, _ := json.Marshal(map[string]any{"name": name, "kind": "html", "content": html, "title": name, "visibility": vis})
	return r.api(user, "POST", "/api/webs", string(b), want)
}

func from(ip string, hdr ...string) func(*http.Request) {
	return func(q *http.Request) {
		q.RemoteAddr = net.JoinHostPort(ip, "51000")
		for i := 0; i+1 < len(hdr); i += 2 {
			q.Header.Set(hdr[i], hdr[i+1])
		}
	}
}

func TestWebsPublishByHostAndSecondAccount(t *testing.T) {
	r := newWebsRig(t, "example.org")
	out := r.publish("anna", "aliments", "<h1>Aliments v1</h1>", "public", 200)
	if out["ok"] != true || !strings.Contains(out["receipt"].(string), "aliments.example.org") || !strings.Contains(out["receipt"].(string), "1") {
		t.Fatalf("publicar: %v", out)
	}
	// Per Host, sense token, des d'internet: és pública.
	page := r.do("", "GET", "http://aliments.example.org/", "", from("203.0.113.9"))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Aliments v1") || page.Header().Get(webVersionHeader) != "1" {
		t.Fatalf("pàgina: %d %s", page.Code, page.Body)
	}
	if csp := page.Header().Get("Content-Security-Policy"); strings.Contains(csp, "sandbox") {
		t.Fatalf("un host propi no necessita sandbox: %q", csp)
	}
	// Per camí, també (sense domini o a la LAN), amb origen opac.
	if p := r.do("", "GET", "http://gregal.test/w/aliments/", "", nil); p.Code != 200 || !strings.Contains(p.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("/w/: %d %q", p.Code, p.Header().Get("Content-Security-Policy"))
	}
	// Un segon compte no pot prendre ni sobreescriure el nom.
	r.publish("bert", "aliments", "<h1>Bert</h1>", "public", http.StatusConflict)
	r.api("bert", "POST", "/api/webs/aliments/visibility", `{"visibility":"private"}`, 404)
	r.api("bert", "POST", "/api/webs/aliments/undo", ``, 404)
	r.api("bert", "DELETE", "/api/webs/aliments", ``, 404)
	r.api("bert", "POST", "/api/webs/aliments/open", `{"preview":true}`, 404)
	if lst := r.api("bert", "GET", "/api/webs", "", 200); len(lst["sites"].([]any)) != 0 {
		t.Fatalf("bert veu webs de l'anna: %v", lst)
	}
	if p := r.do("", "GET", "http://aliments.example.org/", "", from("203.0.113.9")); !strings.Contains(p.Body.String(), "Aliments v1") {
		t.Fatalf("la web de l'anna ha canviat: %s", p.Body)
	}
	// Sense la capacitat «pot publicar», res.
	r.publish("cesc", "cesc", "<h1>c</h1>", "public", http.StatusForbidden)
	if lst := r.api("cesc", "GET", "/api/webs", "", 200); lst["canPublish"] != false {
		t.Fatalf("cesc: %v", lst)
	}
	// L'administrador la hi pot donar.
	if err := setWebPublisher("cesc", true); err != nil {
		t.Fatal(err)
	}
	r.publish("cesc", "cesc", "<h1>c</h1>", "lan", 200)
}

func TestWebsNamesReservedAndHosts(t *testing.T) {
	r := newWebsRig(t, "example.org")
	for _, n := range []string{"www", "gregal", "api", "lan", "preview", "-x", "x-", "a.b", "xn--abc", strings.Repeat("a", 64)} {
		b, _ := json.Marshal(map[string]any{"name": n, "kind": "html", "content": "x"})
		if rec := r.do("anna", "POST", "http://gregal.test/api/webs", string(b), nil); rec.Code != 400 {
			t.Errorf("nom %q: %d", n, rec.Code)
		}
	}
	// Els hosts reservats (i el domini nu) són de l'app, no d'una web.
	for _, h := range []string{"gregal.example.org", "www.example.org", "example.org", "a.b.c.example.org"} {
		if rec := r.do("", "GET", "http://"+h+"/api/health", "", nil); rec.Code == 404 && strings.Contains(rec.Body.String(), "Not found.") {
			t.Errorf("%s no ha d'anar a les webs", h)
		}
	}
	// Un nom que no existeix: 404 de les webs, no l'app.
	if rec := r.do("", "GET", "http://ningu.example.org/", "", nil); rec.Code != 404 {
		t.Fatalf("nom inexistent: %d", rec.Code)
	}
}

func TestWebsLANNeverThroughTunnel(t *testing.T) {
	r := newWebsRig(t, "example.org")
	r.publish("anna", "casa", "<p>només a casa</p>", "lan", 200)
	cases := []struct {
		name string
		mod  func(*http.Request)
		want int
	}{
		{"xarxa local", from("192.168.1.20"), 200},
		{"mateixa màquina", from("127.0.0.1"), 200},
		{"ipv6 local", from("fd00::5"), 200},
		{"internet", from("203.0.113.9"), 404},
		{"túnel (cloudflared a la mateixa màquina)", from("127.0.0.1", "Cf-Connecting-Ip", "192.168.1.20", "Cf-Ray", "abc-MAD"), 404},
		{"túnel amb Cdn-Loop", from("127.0.0.1", "Cdn-Loop", "cloudflare"), 404},
		{"proxy local amb client d'internet", from("127.0.0.1", "X-Forwarded-For", "203.0.113.9"), 404},
		{"proxy local amb client local", from("127.0.0.1", "X-Forwarded-For", "192.168.1.30"), 200},
	}
	for _, host := range []string{"casa.lan.example.org", "casa.example.org"} {
		for _, c := range cases {
			if rec := r.do("", "GET", "http://"+host+"/", "", c.mod); rec.Code != c.want {
				t.Errorf("%s %s: %d (vull %d)", host, c.name, rec.Code, c.want)
			}
		}
	}
	if rec := r.do("", "GET", "http://gregal.test/w/casa/", "", from("127.0.0.1", "Cf-Ray", "x")); rec.Code != 404 {
		t.Errorf("/w/ pel túnel: %d", rec.Code)
	}
}

func TestWebsPrivateNeedsOwnerSessionAndVisibilityWithoutRepublish(t *testing.T) {
	r := newWebsRig(t, "example.org")
	r.publish("anna", "diari", "<p>privat</p>", "private", 200)
	r.publish("bert", "bert", "<p>bert</p>", "private", 200)
	if rec := r.do("", "GET", "http://diari.example.org/", "", from("192.168.1.20")); rec.Code != 404 {
		t.Fatalf("privada sense sessió: %d", rec.Code)
	}
	// El bitllet d'un altre compte (o d'una altra web) no obre aquesta.
	bu := r.api("bert", "POST", "/api/webs/bert/open", `{}`, 200)["url"].(string)
	bt := mustQuery(t, bu, webTicketParam)
	if rec := r.do("", "GET", "http://diari.example.org/?"+webTicketParam+"="+bt, "", nil); len(rec.Result().Cookies()) != 0 {
		t.Fatalf("un bitllet d'una altra web ha donat galeta")
	}
	open := r.api("anna", "POST", "/api/webs/diari/open", `{}`, 200)["url"].(string)
	if !strings.HasPrefix(open, "http://diari.example.org/?"+webTicketParam+"=") {
		t.Fatalf("open: %s", open)
	}
	rec := r.do("", "GET", open, "", nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("bitllet: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatalf("galeta: %+v", cookies)
	}
	withCookie := func(q *http.Request) { q.AddCookie(cookies[0]) }
	if rec := r.do("", "GET", "http://diari.example.org/", "", withCookie); rec.Code != 200 {
		t.Fatalf("amb sessió: %d", rec.Code)
	}
	// Un sol ús.
	if rec := r.do("", "GET", open, "", nil); len(rec.Result().Cookies()) != 0 {
		t.Fatal("el bitllet s'ha pogut fer servir dos cops")
	}
	// La galeta d'aquesta web no obre la d'un altre.
	if rec := r.do("", "GET", "http://bert.example.org/", "", withCookie); rec.Code != 404 {
		t.Fatalf("galeta creuada: %d", rec.Code)
	}
	// Canviar a pública sense tornar a publicar: mateixa versió, ara oberta.
	v := r.api("anna", "POST", "/api/webs/diari/visibility", `{"visibility":"public"}`, 200)
	if v["version"].(float64) != 1 || v["visibility"] != "public" {
		t.Fatalf("visibilitat: %v", v)
	}
	if rec := r.do("", "GET", "http://diari.example.org/", "", from("203.0.113.9")); rec.Code != 200 {
		t.Fatalf("pública: %d", rec.Code)
	}
}

func TestWebsUndoAndFailedCheckKeepsVersion(t *testing.T) {
	r := newWebsRig(t, "example.org")
	r.publish("anna", "forn", `<h1>v1</h1>`, "public", 200)
	r.publish("anna", "forn", `<h1>v2</h1>`, "", 200)
	get := func() string {
		return r.do("", "GET", "http://forn.example.org/", "", from("203.0.113.9")).Body.String()
	}
	if !strings.Contains(get(), "v2") {
		t.Fatal("v2 no és l'activa")
	}
	// Comprovació fallida (enllaç intern trencat): la versió activa no canvia.
	out := r.publish("anna", "forn", `<h1>v3</h1><a href="falta.html">x</a><img src="img/no.png">`, "", 200)
	if out["ok"] != false || !strings.Contains(out["receipt"].(string), "2") {
		t.Fatalf("comprovació: %v", out)
	}
	chk := out["check"].(map[string]any)
	if len(chk["problems"].([]any)) != 2 {
		t.Fatalf("problemes: %v", chk)
	}
	if !strings.Contains(get(), "v2") {
		t.Fatalf("la comprovació fallida ha canviat la web: %s", get())
	}
	site := out["site"].(map[string]any)
	if site["version"].(float64) != 2 || site["state"] != "check_failed" {
		t.Fatalf("estat: %v", site)
	}
	// Desfés: a l'instant, sense comprovar de nou.
	before := r.check
	u := r.api("anna", "POST", "/api/webs/forn/undo", ``, 200)
	if u["version"].(float64) != 1 || !strings.Contains(get(), "v1") || r.check != before {
		t.Fatalf("desfés: %v", u)
	}
	r.api("anna", "POST", "/api/webs/forn/undo", ``, 400)
	// Una que sí passa, amb enllaços interns bons.
	ok := r.publish("anna", "forn", `<link href="estil.css" rel="stylesheet"><a href="#a">a</a><a href="https://example.com/">ext</a><h1>v4</h1>`, "", 200)
	if ok["ok"] != false { // estil.css no hi és (una sola pàgina)
		t.Fatalf("estil.css falta i ha passat: %v", ok)
	}
}

func TestWebsPreviewAndVersionsKept(t *testing.T) {
	r := newWebsRig(t, "example.org")
	r.publish("anna", "joc", "<h1>v1</h1>", "public", 200)
	b, _ := json.Marshal(map[string]any{"name": "joc", "kind": "html", "content": "<h1>esborrany</h1>", "preview": true})
	st := r.api("anna", "POST", "/api/webs", string(b), 200)
	if st["version"].(float64) != 2 || st["check"] != nil {
		t.Fatalf("previsualització: %v", st)
	}
	if rec := r.do("", "GET", "http://joc.example.org/", "", from("203.0.113.9")); !strings.Contains(rec.Body.String(), "v1") {
		t.Fatal("la previsualització ha canviat la web pública")
	}
	if rec := r.do("", "GET", "http://joc.preview.example.org/", "", from("192.168.1.2")); rec.Code != 404 {
		t.Fatalf("previsualització sense sessió: %d", rec.Code)
	}
	open := r.api("anna", "POST", "/api/webs/joc/open", `{"preview":true}`, 200)["url"].(string)
	if !strings.HasPrefix(open, "http://joc.preview.example.org/?") {
		t.Fatalf("open preview: %s", open)
	}
	c := r.do("", "GET", open, "", nil).Result().Cookies()
	if len(c) != 1 {
		t.Fatal("sense galeta de previsualització")
	}
	if rec := r.do("", "GET", "http://joc.preview.example.org/", "", func(q *http.Request) { q.AddCookie(c[0]) }); !strings.Contains(rec.Body.String(), "esborrany") {
		t.Fatalf("previsualització: %s", rec.Body)
	}
	// Activar la versió previsualitzada (amb comprovació).
	if a := r.api("anna", "POST", "/api/webs/joc/activate", `{"version":2}`, 200); a["ok"] != true {
		t.Fatalf("activar: %v", a)
	}
	// Es conserven les 10 últimes versions (i l'activa).
	for i := 3; i <= 14; i++ {
		r.publish("anna", "joc", fmt.Sprintf("<h1>v%d</h1>", i), "", 200)
	}
	ents, _ := os.ReadDir(filepath.Join(websDir(), "sites", "joc"))
	w := getWeb("joc")
	if len(ents) != 10 || len(w.Versions) != 10 || w.Current != 14 || w.version(4) != nil {
		t.Fatalf("versions: %d dirs, %d al registre, activa %d", len(ents), len(w.Versions), w.Current)
	}
}

func TestWebsWithoutDomainUsePath(t *testing.T) {
	r := newWebsRig(t, "")
	out := r.publish("anna", "notes", "<h1>sense domini</h1>", "public", 200)
	site := out["site"].(map[string]any)
	if out["ok"] != true || site["url"] != "http://gregal.test/w/notes/" || site["host"] != "/w/notes/" {
		t.Fatalf("sense domini: %v", out)
	}
	if rec := r.do("", "GET", "http://notes.example.org/", "", nil); strings.Contains(rec.Body.String(), "sense domini") {
		t.Fatal("sense domini no s'enruta per Host")
	}
	open := r.api("anna", "POST", "/api/webs/notes/open", `{"preview":true}`, 200)["url"].(string)
	if !strings.HasPrefix(open, "http://gregal.test/wp/notes/?") {
		t.Fatalf("preview sense domini: %s", open)
	}
}

// L'eina publish: el primer «public» d'un nom nou és una decisió; sense
// confirmació la web queda privada. Les actualitzacions no pregunten.
func TestWebsToolFirstPublicNeedsDecision(t *testing.T) {
	r := newWebsRig(t, "example.org")
	anna := r.s.Hub().SessionFor(r.s.users.Usuari("anna"), "t")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<h1>receptes</h1><a href="b.html">b</a>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.html"), []byte(`b`), 0o600); err != nil {
		t.Fatal(err)
	}
	anna.user.Roots = []string{dir}
	args := `{"dir":` + jsonStr(dir) + `,"name":"receptes","visibility":"public"}`
	if need, where := anna.webNeedsPublicDecision(args); !need || where != "http://receptes.example.org/" {
		t.Fatalf("primer public: %v %q", need, where)
	}
	ctx := withLinkBase(context.Background(), httptest.NewRequest("GET", "http://gregal.test/", nil))
	out, err := anna.runPublishTool(ctx, dir, args)
	if err != nil || !strings.Contains(out, "PRIVATE") || getWeb("receptes").Visibility != webPrivate {
		t.Fatalf("sense confirmar: %v %s", err, out)
	}
	// La persona diu que sí (decisió del fil): ara és pública.
	grantWebPublic("anna", "receptes")
	out, err = anna.runPublishTool(ctx, dir, args)
	if err != nil || !strings.Contains(out, "comprovat") && !strings.Contains(out, "checked") || getWeb("receptes").Visibility != webPublic {
		t.Fatalf("confirmada: %v %s", err, out)
	}
	if need, _ := anna.webNeedsPublicDecision(args); need {
		t.Fatal("una actualització no ha de tornar a preguntar")
	}
	// Un altre compte no pot fer servir el nom amb l'eina.
	bert := r.s.Hub().SessionFor(r.s.users.Usuari("bert"), "t")
	bert.user.Roots = []string{dir}
	if _, err := bert.runPublishTool(ctx, dir, args); err == nil || !strings.Contains(err.Error(), "another account") {
		t.Fatalf("bert: %v", err)
	}
	if need, _ := bert.webNeedsPublicDecision(args); need {
		t.Fatal("el nom d'un altre no és una decisió de bert")
	}
}

func TestWebsDomainWithPort(t *testing.T) {
	r := newWebsRig(t, "example.org:8080")
	out := r.publish("anna", "port", "<h1>p</h1>", "lan", 200)
	if out["ok"] != true || out["site"].(map[string]any)["url"] != "http://port.lan.example.org:8080/" {
		t.Fatalf("amb port: %v", out)
	}
	if rec := r.do("", "GET", "http://port.lan.example.org:8080/", "", from("192.168.1.4")); rec.Code != 200 {
		t.Fatalf("host amb port: %d", rec.Code)
	}
	for _, bad := range []string{"example.org:0", "example.org:x", "a/b", "[::1]:80"} {
		r.s.cfg.Publish.Domain = bad
		if d := r.s.publishDomain(); d != "" {
			t.Errorf("%q → %q", bad, d)
		}
	}
}

func TestWebsWildcardIngress(t *testing.T) {
	setWebIngress("example.org", "casa")
	old := gregalListenPort
	gregalListenPort = 8080
	t.Cleanup(func() { setWebIngress("", ""); gregalListenPort = old })
	wc, ok := webIngressRule("")
	if !ok {
		t.Fatal("sense regla")
	}
	cfg := namedTunnelConfig("casa", []tunnelRecord{{Hostname: "blog.example.org", Target: "http://127.0.0.1:3000"}, wc})
	if !strings.Contains(cfg, "hostname: \"*.example.org\"\n    service: http://127.0.0.1:8080") ||
		strings.Index(cfg, "blog.example.org") > strings.Index(cfg, "*.example.org") {
		t.Fatalf("config:\n%s", cfg)
	}
	if _, ok := webIngressRule("altre"); ok {
		t.Fatal("un altre túnel no rep la regla")
	}
}

func mustQuery(t *testing.T, raw, key string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get(key)
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
