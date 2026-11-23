package web

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func terminalPost(s *Server, h func(*Server, http.ResponseWriter, *http.Request), path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h(s, w, r)
	return w
}

var terminalTags = regexp.MustCompile(`<[^>]+>`)

// terminalWaitFor llegeix l'stream del terminal (en trossos curts, perquè
// l'SSE no s'acaba sol) fins que la pantalla o l'historial mostren want.
func terminalWaitFor(t *testing.T, s *Server, want string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var text string
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		w := httptest.NewRecorder()
		s.handleTerminalStream(w, httptest.NewRequest("GET", "/api/terminal/stream", nil).WithContext(ctx))
		cancel()
		// Cada «data:» és JSON amb files en HTML: «frame» les porta en un
		// objecte per número de fila, «history» en una llista.
		var rows []string
		for _, l := range strings.Split(w.Body.String(), "\n") {
			var ev struct {
				Lines json.RawMessage `json:"lines"`
			}
			if !strings.HasPrefix(l, "data: ") || json.Unmarshal([]byte(l[6:]), &ev) != nil {
				continue
			}
			var byRow map[string]string
			var list []string
			if json.Unmarshal(ev.Lines, &byRow) == nil {
				for _, v := range byRow {
					rows = append(rows, v)
				}
			} else if json.Unmarshal(ev.Lines, &list) == nil {
				rows = append(rows, list...)
			}
		}
		text = html.UnescapeString(terminalTags.ReplaceAllString(strings.Join(rows, "\n"), ""))
		if strings.Contains(text, want) {
			return
		}
	}
	t.Fatalf("el terminal no mostra %q:\n%s", want, text)
}

// Ordres que mostren una cadena que no apareix tal qual en el que s'escriu:
// així el test no es dona per bo amb l'eco de les tecles.
func terminalEcho() (cmd, want string) {
	if runtime.GOOS == "windows" {
		return "Write-Output ('ter'+'m-ok')\r", "term-ok"
	}
	return "echo ter''m-ok\r", "term-ok"
}

func openTestTerminal(t *testing.T, s *Server, body string) {
	t.Helper()
	if w := terminalPost(s, (*Server).handleTerminal, "/api/terminal", body); w.Code != 200 {
		t.Fatalf("obrir: %d %s", w.Code, w.Body.String())
	}
	t.Cleanup(func() { closeShellTerm(s.id) })
}

// Un shell de debò a la carpeta de la sessió: el que s'hi escriu s'executa
// i la sortida torna per l'stream.
func TestTerminalExecutaElQueSHiEscriu(t *testing.T) {
	s, dir := fileTestServer(t)
	// Ample perquè la ruta de la carpeta no es parteixi en dues files.
	openTestTerminal(t, s, `{"cols":300,"rows":24}`)
	cmd, want := terminalEcho()
	if w := terminalPost(s, (*Server).handleTerminalInput, "/api/terminal/input", `{"data":`+strconv.Quote(cmd)+`}`); w.Code != 200 {
		t.Fatalf("entrada: %d %s", w.Code, w.Body.String())
	}
	terminalWaitFor(t, s, want)
	pwd := "echo cw''d=$(pwd)\r"
	if runtime.GOOS == "windows" {
		pwd = "Write-Output ('cw'+'d='+(Get-Location).Path)\r"
	}
	terminalPost(s, (*Server).handleTerminalInput, "/api/terminal/input", `{"data":`+strconv.Quote(pwd)+`}`)
	terminalWaitFor(t, s, "cwd="+dir)
}

// Redimensionar el panell redimensiona el pseudoterminal: el shell veu la
// mida nova.
func TestTerminalRedimensiona(t *testing.T) {
	s, _ := fileTestServer(t)
	openTestTerminal(t, s, `{"cols":80,"rows":24}`)
	openTestTerminal(t, s, `{"cols":103,"rows":31}`)
	cmd, want := "stty size\r", "31 103"
	if runtime.GOOS == "windows" {
		cmd, want = "Write-Output (\"w=\"+$Host.UI.RawUI.WindowSize.Width)\r", "w=103"
	}
	terminalPost(s, (*Server).handleTerminalInput, "/api/terminal/input", `{"data":`+strconv.Quote(cmd)+`}`)
	terminalWaitFor(t, s, want)
}

// Obrir-lo dues vegades torna el mateix shell (amagar el panell no el
// mata), i tancar la sessió sí que se l'endú.
func TestTerminalUnPerSessioIMorAmbElla(t *testing.T) {
	h := hubTestServer(t).Hub()
	s := h.Session("consola")
	openTestTerminal(t, s, `{"cols":80,"rows":24}`)
	first := s.shellTerm()
	openTestTerminal(t, s, `{"cols":80,"rows":24}`)
	if first == nil || s.shellTerm() != first {
		t.Fatal("reobrir el panell ha d'ensenyar el mateix shell")
	}
	if !h.Close("consola") {
		t.Fatal("no s'ha tancat la sessió")
	}
	select {
	case <-first.done:
	case <-time.After(5 * time.Second):
		t.Fatal("el shell hauria d'haver mort amb la sessió")
	}
	if s.shellTerm() != nil {
		t.Fatal("la sessió tancada encara té terminal")
	}
}

// Amb comptes, el shell no queda tancat a les arrels de l'usuari: només
// l'administrador el pot obrir o escriure-hi.
func TestTerminalNomesAdmin(t *testing.T) {
	s, _ := fileTestServer(t)
	for _, c := range []struct {
		path string
		h    func(*Server, http.ResponseWriter, *http.Request)
	}{
		{"/api/terminal", (*Server).handleTerminal},
		{"/api/terminal/input", (*Server).handleTerminalInput},
		{"/api/terminal/stream", (*Server).handleTerminalStream},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", c.path, strings.NewReader(`{"cols":80,"rows":24,"data":"x"}`))
		r.Header.Set("Content-Type", "application/json")
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, &User{Name: "anna"}))
		c.h(s, w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: %d, volia 403", c.path, w.Code)
		}
	}
	if s.shellTerm() != nil {
		t.Fatal("un no-admin ha engegat un shell")
	}
}

// Sense comptes no hi ha token que aturi una altra web: un POST «simple»
// (text/plain, sense preflight) no ha de poder escriure al shell.
func TestTerminalVolJSON(t *testing.T) {
	s, _ := fileTestServer(t)
	w := httptest.NewRecorder()
	s.handleTerminal(w, httptest.NewRequest("POST", "/api/terminal", strings.NewReader(`{"cols":80,"rows":24}`)))
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("%d, volia 415", w.Code)
	}
	if s.shellTerm() != nil {
		t.Fatal("s'ha engegat sense JSON")
	}
}
