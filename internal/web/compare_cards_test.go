package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/config"
)

// Les dues guies (vents i mode Codi) ensenyen la mateixa comparació en fitxes:
// 2-8 coses, foto o logo, punts per a resums llargs i neutralitat en política.
func TestComparePromptInWindsAndCodeMode(t *testing.T) {
	s := goalTestServer(t)
	for name, prompt := range map[string]string{
		"CardsGuide": companion.CardsGuide,
		"code":       s.sysPromptAmb(agent.ModeCode, s.cwd),
		"chat":       s.sysPromptAmb(agent.ModeChat, s.cwd),
	} {
		for _, want := range []string{"```gregal-compare", "2-8 things", `"logo_domain"`, `"image_query"`, `"url"`, `"facts"`, `"points"`, `"pick"`, `"verdict"`, "never for code diffs or one-line answers", "neutral", "no pick"} {
			if !strings.Contains(prompt, want) {
				t.Errorf("%s: falta %q", name, want)
			}
		}
	}
}

func fakeWrite(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil {
		panic(err)
	}
}

// fakeLogoSites posa els webs de logo_domain en un servidor de prova: cada
// domini és un camí (/nodejs.org/) amb l'HTML que es vulgui.
func fakeLogoSites(t *testing.T, pages map[string]string) *[]string {
	t.Helper()
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, ".png") && !strings.Contains(r.URL.Path, "missing"):
			w.Header().Set("Content-Type", "image/png")
			fakeWrite(w, "\x89PNG")
		case strings.HasSuffix(r.URL.Path, "/"):
			if html, ok := pages[strings.Trim(r.URL.Path, "/")]; ok {
				fakeWrite(w, html)
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	old, oldClient := logoSiteURL, logoClient
	logoSiteURL = func(domain string) string { return srv.URL + "/" + domain + "/" }
	logoClient = srv.Client()
	logoMu.Lock()
	logoCache = map[string]string{}
	logoMu.Unlock()
	t.Cleanup(func() { srv.Close(); logoSiteURL, logoClient = old, oldClient })
	return &hits
}

// logo_domain porta el logo que el web declara (el més gran), comprovat i
// desat a la memòria cau; un web sense icona que funcioni no en té cap.
func TestCompareLogoDomainGetsTheSiteIcon(t *testing.T) {
	hits := fakeLogoSites(t, map[string]string{
		"nodejs.org": `<html><head><link rel="icon" href="/nodejs.org/small.png" sizes="16x16"><link rel="apple-touch-icon" href="touch.png"><link rel="mask-icon" href="/mask.svg"></head></html>`,
		"deno.com":   `<html><head><link rel="icon" href="/deno.com/missing.png"></head></html>`,
	})
	fakeCardImages(t, nil)
	s := &Server{}
	in := "```gregal-compare\n{\"title\":\"Node o Deno\",\"items\":[{\"name\":\"Node.js\",\"logo_domain\":\"nodejs.org\",\"facts\":{\"Any\":\"2009\"}},{\"name\":\"Deno\",\"logo_domain\":\"deno.com\",\"facts\":{\"Any\":\"2018\"}},{\"name\":\"Bun\",\"logo_domain\":\"localhost\",\"facts\":{\"Any\":\"2022\"}}]}\n```"
	out := s.codeCompareImages(in)
	card, ok := cardJSON(strings.TrimSuffix(strings.TrimPrefix(out, "```gregal-compare\n"), "\n```"))
	if !ok {
		t.Fatalf("no és una targeta:\n%s", out)
	}
	items := card["items"].([]any)
	node, deno, bun := items[0].(map[string]any), items[1].(map[string]any), items[2].(map[string]any)
	if !strings.HasSuffix(cardString(node, "logo"), "/nodejs.org/touch.png") {
		t.Fatalf("node logo = %q, want the apple-touch-icon", node["logo"])
	}
	if deno["logo"] != nil || bun["logo"] != nil || strings.Contains(out, "logo_domain") {
		t.Fatalf("no invented logos and no logo_domain left:\n%s", out)
	}
	// Amb logo, no cal cap foto de Commons per a Node ni Deno.
	n := len(*hits)
	s.codeCompareImages(in)
	if again := (*hits)[n:]; strings.Contains(strings.Join(again, " "), "nodejs.org") {
		t.Fatalf("the found logo is cached, asked again: %v", again)
	}
}

// Una fitxa amb logo_domain i image_query (un partit: logo i candidat) en
// porta totes dues; el text fora del bloc no es toca.
func TestCompareItemCanCarryLogoAndPhoto(t *testing.T) {
	fakeLogoSites(t, map[string]string{"partit.cat": `<link rel="icon" href="/partit.cat/logo.png" sizes="192x192">`})
	fakeCardImages(t, map[string]string{"Jane Doe politician": "https://upload.wikimedia.org/jane.jpg"})
	old := compareEnglishNames
	compareEnglishNames = func(*Server, context.Context, []string) map[string]string { return nil }
	t.Cleanup(func() { compareEnglishNames = old })
	s := &Server{}
	out := s.codeCompareImages("Els programes:\n\n```gregal-compare\n{\"title\":\"Programes\",\"items\":[{\"name\":\"Partit A\",\"logo_domain\":\"partit.cat\",\"image_query\":\"Jane Doe politician\",\"points\":[\"Habitatge\"]},{\"name\":\"Partit B\",\"points\":[\"Escola\"],\"image\":\"/inventada.jpg\",\"images\":[\"https://e.com/1.jpg\",\"/inventada2.jpg\",\"https://e.com/2.jpg\",\"https://e.com/3.jpg\"]}]}\n```\nFonts a sota.")
	if !strings.Contains(out, `"logo":"http`) || !strings.Contains(out, `"image":"https://upload.wikimedia.org/jane.jpg"`) || strings.Contains(out, "inventada") || !strings.Contains(out, `"images":["https://e.com/1.jpg","https://e.com/2.jpg"]`) {
		t.Fatalf("logo and photo:\n%s", out)
	}
	if !strings.HasPrefix(out, "Els programes:") || !strings.HasSuffix(out, "Fonts a sota.") {
		t.Fatalf("text around changed:\n%s", out)
	}
	if plain := "Sense targetes ![x](image: Cat)\nhttps://e.com/a.png"; s.codeCompareImages(plain) != plain {
		t.Fatal("a Code reply without a comparison stays as it was")
	}
}

// Al mode Codi la resposta final (i el que se n'emet) ja porta els logos.
func TestCodeModeReplyGetsCompareLogos(t *testing.T) {
	fakeLogoSites(t, map[string]string{"go.dev": `<link rel="apple-touch-icon" href="/go.dev/go.png">`})
	fakeCardImages(t, nil)
	block := "```gregal-compare\n{\"title\":\"Go o Rust\",\"items\":[{\"name\":\"Go\",\"logo_domain\":\"go.dev\",\"facts\":{\"GC\":\"sí\"}},{\"name\":\"Rust\",\"image\":\"https://e.com/rust.png\",\"facts\":{\"GC\":\"no\"}}]}\n```"
	content, _ := json.Marshal(block)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fakeWrite(w, "data: {\"choices\":[{\"delta\":{\"content\":"+string(content)+"},\"finish_reason\":null}]}\n\n")
		fakeWrite(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: provider.URL}}
	role := config.Role{Provider: "p", Model: "m", MaxTokens: 1024, ContextWindow: 32768}
	shown := ""
	reply, _, err := s.conduirTorn(context.Background(), "go o rust?", agent.ModeCode, "code", s.cfg.Providers["p"], role, 1, t.TempDir(), nil, func(kind string, data any) {
		if kind == "assistant" {
			shown = data.(map[string]string)["text"]
		}
	})
	if err != nil {
		t.Fatalf("conduirTorn: %v", err)
	}
	if !strings.Contains(reply, "/go.dev/go.png") || strings.Contains(reply, "logo_domain") {
		t.Fatalf("reply without the logo:\n%s", reply)
	}
	if shown != "" && shown != reply {
		t.Fatalf("what is shown differs from what is saved:\n%s\n---\n%s", shown, reply)
	}
}

// El servidor de prova (127.0.0.1) només s'hi arriba amb el client injectat:
// el de debò no connecta a adreces que no són públiques.
func TestCompareLogoOnlyFromPublicHosts(t *testing.T) {
	logoMu.Lock()
	logoCache = map[string]string{}
	logoMu.Unlock()
	var looked, dialed []string
	oldLookup, oldDial := logoLookup, logoDial
	logoLookup = func(_ context.Context, host string) ([]net.IPAddr, error) {
		looked = append(looked, host)
		ips := map[string]string{"intranet.example.com": "10.0.0.5", "cgnat.example.com": "100.64.3.4", "meta.example.com": "169.254.169.254", "public.example.com": "93.184.216.34"}
		return []net.IPAddr{{IP: net.ParseIP(ips[host])}}, nil
	}
	logoDial = func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, errors.New("no network in tests")
	}
	t.Cleanup(func() { logoLookup, logoDial = oldLookup, oldDial })
	for _, d := range []string{"localhost", "127.0.0.1", "10.0.0.5", "169.254.169.254", "[::1]", "intranet", "intranet.example.com", "cgnat.example.com", "meta.example.com"} {
		if u := cardLogo(context.Background(), d); u != "" {
			t.Errorf("%s: logo %q", d, u)
		}
	}
	if len(dialed) != 0 {
		t.Fatalf("connected to %v", dialed)
	}
	if strings.Join(looked, " ") != "intranet.example.com cgnat.example.com meta.example.com" {
		t.Fatalf("only real names are looked up: %v", looked)
	}
	// Un nom públic sí que s'intenta, a l'adreça mirada i al port 443.
	cardLogo(context.Background(), "public.example.com")
	if len(dialed) == 0 || dialed[0] != "93.184.216.34:443" {
		t.Fatalf("public host not dialed at its checked address: %v", dialed)
	}
	// La connexió també es mira just abans de fer-la, i les redireccions una per una.
	for addr, ok := range map[string]bool{"127.0.0.1:443": false, "10.1.2.3:80": false, "[fd00::1]:443": false, "100.100.0.1:443": false, "[::ffff:127.0.0.1]:443": false, "93.184.216.34:443": true} {
		if err := logoDialControl("tcp4", addr, nil); (err == nil) != ok {
			t.Errorf("control %s: %v", addr, err)
		}
	}
	for raw, ok := range map[string]bool{"https://cdn.example.com/a.png": true, "http://127.0.0.1/a.png": false, "https://cdn.example.com:8443/a.png": false, "ftp://cdn.example.com/a": false, "http://intranet/a.png": false} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		req := &http.Request{URL: u}
		if err := logoRedirect(req, nil); (err == nil) != ok {
			t.Errorf("redirect %s: %v", raw, err)
		}
	}
	if logoRedirect(&http.Request{URL: &url.URL{Scheme: "https", Host: "a.example.com"}}, make([]*http.Request, 3)) == nil {
		t.Error("a fourth redirect must be refused")
	}
}
