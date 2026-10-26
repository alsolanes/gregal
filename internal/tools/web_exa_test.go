package tools

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Cap test no ha de gastar el límit gratuït d'Exa (ni dependre de la
	// xarxa): els que en volen un el redirigeixen a un servidor fals; la
	// prova viva, només amb GREGAL_LIVE_WEB.
	if os.Getenv("GREGAL_LIVE_WEB") == "" {
		exaEndpoint = ""
	}
	os.Exit(m.Run())
}

const exaTextCerca = `Title: Agenda - Sala Razzmatazz
URL: https://www.salarazzmatazz.com/agenda/
Published: 2026-10-20T00:00:00.000Z
Author: N/A
Highlights:
Dl.26.Oct Live Room 2 Los Palmeras

---

Ratlla dins del text d'una pàgina, no un resultat nou.

---

Title: Comú
URL: https://x.cat/comu
Published: N/A
Author: N/A
Highlights:
c`

// fakeExa fa de servidor MCP d'Exa: respon en SSE, com el de debò, i compta
// les crides. status != 200 respon el límit gratuït.
func fakeExa(t *testing.T, status *int32, calls *int32, gotKey *atomic.Value) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(calls, 1)
		if gotKey != nil {
			gotKey.Store(r.Header.Get("x-api-key") + "|" + r.URL.RawQuery)
		}
		if s := atomic.LoadInt32(status); s != 200 {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(int(s))
			w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32000,"message":"You've hit Exa's free MCP rate limit."},"id":null}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		json.Unmarshal(raw, &req)
		text := exaTextCerca
		if req.Params.Name == "web_fetch_exa" {
			text = "# Pregunta tancada\nURL: https://stackoverflow.com/q/1\nPublished: 2025-01-02\n\n" + strings.Repeat("Resposta acceptada amb el codi correcte. ", 10)
		}
		res, _ := json.Marshal(map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}, "jsonrpc": "2.0", "id": 1})
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message\ndata: " + string(res) + "\n\n"))
	}))
}

func netejaWeb(t *testing.T) {
	t.Helper()
	WebCacheClear()
	PausesClear()
	t.Cleanup(func() { WebCacheClear(); PausesClear() })
}

// TestExaPrincipalIFusio: Exa i el SearXNG propi van alhora i es fusionen;
// la clau va per capçalera (mai a la URL); DuckDuckGo i Bing ni es toquen.
func TestExaPrincipalIFusio(t *testing.T) {
	netejaWeb(t)
	var status, calls, ddgCalls int32 = 200, 0, 0
	var key atomic.Value
	exa := fakeExa(t, &status, &calls, &key)
	defer exa.Close()
	srv := newFakeSearch(t)
	defer srv.Close()
	ddg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&ddgCalls, 1) }))
	defer ddg.Close()
	oldE, oldD, oldB := exaEndpoint, duckduckgoBase, bingBase
	exaEndpoint, duckduckgoBase, bingBase = exa.URL, ddg.URL, ddg.URL
	defer func() { exaEndpoint, duckduckgoBase, bingBase = oldE, oldD, oldB }()
	t.Setenv("SEARXNG_URL", srv.URL+"/searx")
	t.Setenv("EXA_API_KEY", "clau-de-prova")

	out, err := WebSearch("concerts Razzmatazz", 8)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "via exa+searxng") || !strings.Contains(out, "salarazzmatazz.com/agenda/ · 2026-10-20") || !strings.Contains(out, "Los Palmeras") {
		t.Fatalf("sortida: %s", out)
	}
	if strings.Contains(out, "Ratlla dins") == false {
		t.Fatalf("una ratlla dins del text no ha de partir el resultat: %s", out)
	}
	if i, j := strings.Index(out, "x.cat/comu"), strings.Index(out, "x.cat/nomes-searx"); i < 0 || i > j {
		t.Fatalf("el resultat comú ha de pujar: %s", out)
	}
	if ddgCalls != 0 {
		t.Fatalf("amb resultats dels principals, els de reserva no s'han de tocar (%d crides)", ddgCalls)
	}
	if got := key.Load().(string); got != "clau-de-prova|" {
		t.Fatalf("la clau ha d'anar per capçalera i no a la URL: %q", got)
	}
}

// TestExaAlLimitPassaALaReservaIQuedaEnPausa: el 429 d'Exa no es veu: la
// cerca surt de DuckDuckGo, i la següent ja no truca a Exa.
func TestExaAlLimitPassaALaReservaIQuedaEnPausa(t *testing.T) {
	netejaWeb(t)
	var status, calls int32 = 429, 0
	exa := fakeExa(t, &status, &calls, nil)
	defer exa.Close()
	srv := newFakeSearch(t)
	defer srv.Close()
	oldE, oldD, oldB := exaEndpoint, duckduckgoBase, bingBase
	exaEndpoint, duckduckgoBase, bingBase = exa.URL, srv.URL+"/ddg", "http://127.0.0.1:1/bing"
	defer func() { exaEndpoint, duckduckgoBase, bingBase = oldE, oldD, oldB }()
	t.Setenv("SEARXNG_URL", "")

	out, err := WebSearch("comu ddg", 8)
	if err != nil || !strings.Contains(out, "via duckduckgo") || !strings.Contains(out, "x.cat/nomes-ddg") {
		t.Fatalf("havia de sortir de la reserva: %v %s", err, out)
	}
	if motiu, ok := enPausa("exa"); !ok || !strings.Contains(motiu, "Exa") {
		t.Fatalf("Exa havia de quedar en pausa: %q", motiu)
	}
	WebCacheClear()
	if _, err := WebSearch("comu altra", 8); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("en pausa, Exa no s'ha de tornar a trucar (%d crides)", calls)
	}
}

// TestCaptchaPosaElMotorEnPausaIAvisa: DuckDuckGo amb captcha queda en
// pausa; si no queda cap motor, l'agent rep l'ordre de no insistir.
func TestCaptchaPosaElMotorEnPausaIAvisa(t *testing.T) {
	netejaWeb(t)
	var ddgCalls int32
	ddg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ddgCalls, 1)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>DuckDuckGo</title></head><body><div class="anomaly-modal">bots</div></body></html>`))
	}))
	defer ddg.Close()
	oldD, oldB := duckduckgoBase, bingBase
	duckduckgoBase, bingBase = ddg.URL, "http://127.0.0.1:1/bing"
	defer func() { duckduckgoBase, bingBase = oldD, oldB }()
	t.Setenv("SEARXNG_URL", "")

	_, err := WebSearch("qualsevol cosa", 8)
	if err == nil || !strings.Contains(err.Error(), "No ho tornis a provar") {
		t.Fatalf("sense cap motor, l'agent ha de saber que no ha d'insistir: %v", err)
	}
	WebCacheClear()
	_, err = WebSearch("una altra cosa", 8)
	if err == nil || !strings.Contains(err.Error(), "en pausa fins a les") || ddgCalls != 1 {
		t.Fatalf("en pausa, DuckDuckGo no s'ha de tornar a provar (%d crides): %v", ddgCalls, err)
	}
}

func TestPausaCreixIEsBorraAmbExit(t *testing.T) {
	netejaWeb(t)
	resta := func() time.Duration {
		pausesMu.Lock()
		defer pausesMu.Unlock()
		return time.Until(pauses["m"].fins).Round(time.Minute)
	}
	for _, want := range []time.Duration{15, 30, 60, 120, 120} {
		anotaResultat("m", &bloqueig{motiu: "captcha"})
		if got := resta(); got != want*time.Minute {
			t.Fatalf("pausa: %v, volia %v min", got, want)
		}
	}
	anotaResultat("m", errors.New("timeout"))
	if _, ok := enPausa("m"); !ok {
		t.Fatal("un error que no és bloqueig no treu la pausa")
	}
	anotaResultat("m", nil)
	if _, ok := enPausa("m"); ok {
		t.Fatal("un èxit treu la pausa")
	}
	anotaResultat("m", &bloqueig{motiu: "429", espera: 3 * time.Hour})
	if got := resta(); got != 3*time.Hour {
		t.Fatalf("el Retry-After mana: %v", got)
	}
}

// TestRellevantsDescartaResultatsSenseRelacio: el cas vist en viu amb Bing.
func TestRellevantsDescartaResultatsSenseRelacio(t *testing.T) {
	brossa := []SearchHit{
		{Title: "The Villages Golf Course Conditions", URL: "https://www.talkofthevillages.com/forums/golf"},
		{Title: "IKEA Furniture - Bed Frame", URL: "http://www.jlaforums.com/viewtopic.php?t=1"},
		{Title: "Agenda", URL: "https://www.salarazzmatazz.com/agenda/"},
	}
	if got := rellevants("concerts Razzmatazz novembre", brossa); len(got) != 1 || !strings.Contains(got[0].URL, "razzmatazz") {
		t.Fatalf("filtre: %+v", got)
	}
	meteo := []SearchHit{{Title: "El temps a Barcelona. Predicció municipal", URL: "https://meteo.cat/x"}}
	if got := rellevants("temps a Barcelona demà", meteo); len(got) != 1 {
		t.Fatal("un resultat bo no s'ha de perdre")
	}
	if got := rellevants("Fórmula 1 Mèxic", []SearchHit{{Title: "Resultats GP Mexic", URL: "https://f1.com/a"}}); len(got) != 1 {
		t.Fatal("els accents no han de comptar")
	}
	if got := rellevants("a la", brossa); len(got) != len(brossa) {
		t.Fatal("sense paraules útils, no es filtra res")
	}
}

// TestWebFetchExaQuanElLlocBloqueja: Exa guanya a l'arxiu (que limita) i la
// pàgina surt marcada.
func TestWebFetchExaQuanElLlocBloqueja(t *testing.T) {
	netejaWeb(t)
	var status, calls int32 = 200, 0
	exa := fakeExa(t, &status, &calls, nil)
	defer exa.Close()
	arxiu := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer arxiu.Close()
	oldE, oldW := exaEndpoint, waybackAPIBase
	exaEndpoint, waybackAPIBase = exa.URL, arxiu.URL
	defer func() { exaEndpoint, waybackAPIBase = oldE, oldW }()
	t.Setenv("FIRECRAWL_URL", "")

	p, why := webFetchFallbacks("https://stackoverflow.com/q/1", false, true)
	if p == nil || !strings.Contains(p.Origen, "via Exa") || p.Title != "Pregunta tancada" || p.Published != "2025-01-02" ||
		!strings.Contains(p.Body, "Resposta acceptada") || strings.Contains(p.Body, "URL:") {
		t.Fatalf("lectura via Exa: %+v %v", p, why)
	}
}

func TestWayback429QuedaEnPausa(t *testing.T) {
	netejaWeb(t)
	arxiu := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer arxiu.Close()
	oldW := waybackAPIBase
	waybackAPIBase = arxiu.URL
	defer func() { waybackAPIBase = oldW }()
	_, _, err := webFetchWaybackPage("https://example.com/a", false)
	anotaResultat("arxiu", err)
	if _, ok := enPausa("arxiu"); !ok {
		t.Fatalf("un 429 de l'arxiu l'ha de posar en pausa: %v", err)
	}
}

// TestWaybackBuidaNoCompta: la carcassa d'una pàgina feta amb JavaScript
// (només la barra de Wayback) no és una lectura.
func TestWaybackBuidaNoCompta(t *testing.T) {
	netejaWeb(t)
	allowLocalFetch = true
	defer func() { allowLocalFetch = false }()
	var snap *httptest.Server
	snap = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/wayback/") {
			w.Write([]byte(`{"archived_snapshots":{"closest":{"url":"` + snap.URL + `/web/1/https://www.reddit.com/r/golang/","timestamp":"20261015150754"}}}`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Reddit</title></head><body><p>The Wayback Machine - http://web.archive.org/web/20261015150754/https://www.reddit.com/r/golang/</p><div id="root"></div></body></html>`))
	}))
	defer snap.Close()
	oldW := waybackAPIBase
	waybackAPIBase = snap.URL
	defer func() { waybackAPIBase = oldW }()
	if _, _, err := webFetchWaybackPage("https://www.reddit.com/r/golang/", false); err == nil {
		t.Fatal("una còpia sense text no ha de comptar com a lectura")
	}
}

// TestWebFetchAvisaQuanUnLlocBloquejaAgents: un 403 sense alternativa diu a
// l'agent que no hi torni.
func TestWebFetchAvisaQuanUnLlocBloquejaAgents(t *testing.T) {
	netejaWeb(t)
	allowLocalFetch = true
	defer func() { allowLocalFetch = false }()
	t.Setenv("FIRECRAWL_URL", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := WebFetch(srv.URL+"/pagina", 0)
	if err == nil || !strings.Contains(err.Error(), "no deixa entrar agents automàtics") || !strings.Contains(err.Error(), "ni amb el navegador") {
		t.Fatalf("avís: %v", err)
	}
	// Una pàgina que no existeix no és un bloqueig.
	srv404 := httptest.NewServer(http.NotFoundHandler())
	defer srv404.Close()
	if _, err := WebFetch(srv404.URL+"/no", 0); err == nil || strings.Contains(err.Error(), "agents automàtics") {
		t.Fatalf("un 404 no és un bloqueig: %v", err)
	}
}

func TestNotaAntiRobots(t *testing.T) {
	if n := NotaAntiRobots("Reddit - Prove your humanity", "www.reddit.com"); !strings.Contains(n, "www.reddit.com no deixa entrar") {
		t.Fatalf("nota: %q", n)
	}
	if NotaAntiRobots("r/golang", "www.reddit.com") != "" {
		t.Fatal("una pàgina normal no porta nota")
	}
}
