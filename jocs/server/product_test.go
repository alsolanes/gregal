package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gregal/internal/web"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// Amb els jocs registrats, /jocs/ és Gregal Jocs: la mateixa web amb el seu
// data-product i els seus fitxers, servits a les URL de sempre.
func TestJocsIsItsOwnProduct(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	h := s.host.(*web.Server).Handler()
	page := get(t, h, "/jocs/").Body.String()
	if !strings.Contains(page, `<html lang="ca" data-product="jocs">`) || !strings.Contains(page, `<title data-i18n="ui.titleJocs">Gregal Jocs</title>`) {
		t.Fatal("/jocs/ must be Gregal Jocs")
	}
	for _, src := range []string{`href="/app/jocs-site.css?fgv=`, `src="/app/jocs-site.js?fgv=`, `src="/app/arena.js?fgv=`, `src="/app/football.js?fgv=`} {
		if !strings.Contains(page, src) {
			t.Errorf("/jocs/ has no %s", src)
		}
	}
	if w := get(t, h, "/api/health"); !strings.Contains(w.Body.String(), `"products":["jocs"]`) {
		t.Errorf("/api/health must list the jocs product: %s", w.Body.String())
	}
	if root := get(t, h, "/").Body.String(); strings.Contains(root, "jocs-site") || strings.Contains(root, "/app/arena.js") {
		t.Error("the games' files only go on /jocs/")
	}
	for _, f := range []string{"/app/jocs-site.js", "/app/jocs-site.css", "/app/arena.js", "/app/football.js", "/app/arena-stream.html", "/app/vendor/three.module.min.js", "/app/i18n.js"} {
		if w := get(t, h, f); w.Code != 200 || w.Body.Len() == 0 {
			t.Errorf("%s: %d", f, w.Code)
		}
	}
}

// Els plugins dels jocs: hi són, i apagar l'Arena treu les seves rutes.
func TestPluginArenaApagadaRutes404(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	ws := s.host.(*web.Server)
	hub := ws.Hub()
	h := ws.Handler()
	if !hub.PluginEnabled("arena") || !hub.PluginEnabled("games") {
		t.Fatal("els plugins dels jocs han d'estar encesos per defecte")
	}
	if w := get(t, h, "/api/arena"); w.Code != 200 {
		t.Fatalf("/api/arena encesa: %d %s", w.Code, w.Body.String())
	}
	if err := hub.SetPluginEnabled("arena", false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/api/arena", "/api/arena/stream", "/api/arena/match?id=x"} {
		if w := get(t, h, p); w.Code != 404 || !strings.Contains(w.Body.String(), "plugin disabled") {
			t.Errorf("%s: %d %s", p, w.Code, w.Body.String())
		}
	}
	if !hub.PluginEnabled("games") || !hub.PluginEnabled("corner") {
		t.Fatal("apagar l'Arena no ha de tocar els altres")
	}
}

// El contracte HTTP: amb els jocs registrats, totes les rutes surten a
// docs/api-contract.md, les dels jocs a la secció «## Gregal Jocs», i
// aquella secció no promet cap ruta que no hi sigui.
func TestContracteJocs(t *testing.T) {
	s, _ := companionTestServer(t)
	routes := s.host.(*web.Server).Hub().APIRoutes()
	raw, err := os.ReadFile("../../docs/api-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(string(raw), "\r\n", "\n")
	i := strings.Index(doc, "\n## Gregal Jocs")
	if i < 0 {
		t.Fatal("falta la secció «## Gregal Jocs» a docs/api-contract.md")
	}
	section := doc[i:]
	if j := strings.Index(section[1:], "\n## "); j >= 0 {
		section = section[:j+1]
	}
	viva := map[string]bool{}
	for _, r := range routes {
		viva[r] = true
		if !strings.Contains(doc, r) {
			t.Errorf("ruta sense documentar a docs/api-contract.md: %s", r)
		}
		if (strings.HasPrefix(r, "/api/arena") || strings.HasPrefix(r, "/api/football")) && !strings.Contains(section, "`"+r+"`") {
			t.Errorf("%s és dels jocs: ha d'anar a la secció de Gregal Jocs", r)
		}
	}
	for _, r := range []string{"/api/arena", "/api/football"} {
		if !viva[r] {
			t.Fatalf("%s no està registrada amb els jocs", r)
		}
	}
	for _, tok := range strings.Fields(strings.ReplaceAll(section, "`", " ")) {
		tok = strings.Trim(tok, ",.;:()|")
		if !strings.HasPrefix(tok, "/api/") || strings.ContainsAny(tok, "{?*") {
			continue
		}
		if !viva[tok] {
			t.Errorf("la secció de Gregal Jocs documenta %s, que no existeix", tok)
		}
	}
}
