package tools

// Prova viva del motor nou (arbre d'accessibilitat, refs, diffs, lots,
// diàlegs, pestanyes, upload i el bloqueig de credencials) contra una pàgina
// local i el Chrome/Edge de la màquina. Salta sense GREGAL_LIVE_BROWSER=1.
//
// Ús: GREGAL_LIVE_BROWSER=1 go test ./internal/tools/ -run TestLiveBrowserEngine -v

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const fixtureHTML = `<!doctype html><html><head><title>Botiga de prova</title></head><body>
<header><nav><a href="/">Inici</a> <a href="/other" target="_blank">Obre una altra pestanya</a></nav></header>
<main>
<h1>Comanda</h1>
<form onsubmit="event.preventDefault(); document.getElementById('out').textContent = 'Enviat: ' + document.getElementById('nom').value + ' / ' + document.getElementById('mida').value;">
  <label>Nom <input id="nom" name="nom"></label>
  <label>Contrasenya <input id="pw" type="password" name="pw"></label>
  <label>Mida <select id="mida"><option value="s">Petita</option><option value="m">Mitjana</option><option value="l">Gran</option></select></label>
  <label>Adjunt <input id="fitxer" type="file"></label>
  <button type="submit">Envia la comanda</button>
</form>
<p id="out"></p>
<button onclick="setTimeout(() => { const p = document.createElement('p'); p.textContent = 'Carregat tard'; document.querySelector('main').append(p); }, 300)">Carrega més</button>
<button onclick="document.getElementById('del').textContent = confirm('Segur que vols esborrar-ho?') ? 'Esborrat' : 'Cancel·lat'">Esborra el compte</button>
<p id="del"></p>
<button onclick="document.getElementById('cont').textContent = confirm('Vols continuar?') ? 'Continuat' : 'Aturat'">Continua</button>
<p id="cont"></p>
<table><tr><th>Producte</th><th>Preu</th></tr><tr><td>Poma</td><td>1 €</td></tr><tr><td>Pera</td><td>2 €</td></tr></table>
<button onclick="console.error('error de prova')">Fes un error</button>
</main></body></html>`

func TestLiveBrowserEngine(t *testing.T) {
	if os.Getenv("GREGAL_LIVE_BROWSER") == "" {
		t.Skip("sense GREGAL_LIVE_BROWSER=1")
	}
	if BrowserExecutable() == "" {
		t.Skip("cap Chrome/Edge a la màquina")
	}
	t.Setenv("GREGAL_BROWSER_HEADLESS", "1")
	t.Setenv("GREGAL_BROWSER_PROFILE", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/other" {
			fmt.Fprint(w, `<!doctype html><title>Una altra</title><h1>Pestanya nova</h1>`)
			return
		}
		fmt.Fprint(w, fixtureHTML)
	}))
	defer srv.Close()
	defer CloseBrowser()
	const scope = "live-engine"
	do := func(action string, args map[string]any) string {
		t.Helper()
		if args == nil {
			args = map[string]any{}
		}
		args["action"] = action
		t0 := time.Now()
		out, _, err := BrowserActionFor(nil, scope, action, args)
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		t.Logf("── %s (%v)\n%s", action, time.Since(t0).Round(time.Millisecond), out)
		return out
	}

	page := do("open", map[string]any{"url": srv.URL + "/"})
	for _, want := range []string{`heading "Comanda"`, `textbox "Nom"`, `combobox "Mida"`, `button "Envia la comanda"`, "row: Poma | 1 €"} {
		if !strings.Contains(page, want) {
			t.Errorf("snapshot sense %q", want)
		}
	}
	nom, mida := refOf(t, page, `textbox "Nom"`), refOf(t, page, `combobox "Mida"`)
	envia, pw := refOf(t, page, `button "Envia la comanda"`), refOf(t, page, `textbox "Contrasenya"`)

	// Lot: escriure, triar i enviar en una sola crida, amb un sol diff.
	out := do("batch", map[string]any{"steps": []any{
		map[string]any{"action": "type", "target": nom, "text": "Laia"},
		map[string]any{"action": "select", "target": mida, "option": "Gran"},
		map[string]any{"action": "click", "target": envia},
	}})
	if !strings.Contains(out, "Enviat: Laia / l") || !strings.Contains(out, "Changes on") {
		t.Errorf("el lot havia d'enviar el formulari i tornar el diff:\n%s", out)
	}

	// Credencials: mai.
	if _, _, err := BrowserActionFor(nil, scope, "type", map[string]any{"target": pw, "text": "secret"}); err == nil || !strings.Contains(err.Error(), "never types credentials") {
		t.Errorf("la contrasenya s'havia de bloquejar: %v", err)
	}

	// Contingut que arriba tard: wait.
	do("click", map[string]any{"target": refOf(t, page, `button "Carrega més"`)})
	if w := do("wait", map[string]any{"text": "Carregat tard"}); !strings.Contains(w, "condition met") {
		t.Errorf("wait: %s", w)
	}

	// Diàleg confirm: es cancel·la sol i es diu; amb dialog accept es repeteix.
	esborra := refOf(t, page, `button "Esborra el compte"`)
	if d, _, _ := BrowserGate(scope, map[string]any{"action": "click", "target": esborra}, true); d != "ask" {
		t.Errorf("«Esborra el compte» és irreversible: ha de preguntar (%s)", d)
	}
	// L'usuari ja ha confirmat el clic irreversible: el «segur?» de la pàgina
	// és la mateixa decisió i s'accepta sol.
	out = do("click", map[string]any{"target": esborra})
	if !strings.Contains(out, "confirm dialog") || !strings.Contains(out, "Esborrat") {
		t.Errorf("el confirm d'una acció confirmada s'havia d'acceptar:\n%s", out)
	}
	// Un botó qualsevol: el confirm es cancel·la i es diu; amb dialog accept es repeteix.
	continua := refOf(t, page, `button "Continua"`)
	if out = do("click", map[string]any{"target": continua}); !strings.Contains(out, "CANCELLED") || !strings.Contains(out, "Aturat") {
		t.Errorf("el confirm s'havia de cancel·lar i dir-ho:\n%s", out)
	}
	do("dialog", map[string]any{"accept": true})
	if out = do("click", map[string]any{"target": continua}); !strings.Contains(out, "Continuat") {
		t.Errorf("amb dialog accept havia de continuar:\n%s", out)
	}

	// Errors de consola: surten sols després de l'acció.
	if out = do("click", map[string]any{"target": refOf(t, page, `button "Fes un error"`)}); !strings.Contains(out, "error de prova") {
		t.Errorf("l'error de consola havia de sortir:\n%s", out)
	}

	// Upload.
	f := filepath.Join(t.TempDir(), "adjunt.txt")
	os.WriteFile(f, []byte("hola"), 0o644)
	full := do("read", nil)
	fileRef := refOf(t, full, `button "Adjunt"`)
	if out = do("upload", map[string]any{"target": fileRef, "path": f}); !strings.Contains(out, "attached 1 file") {
		t.Errorf("upload: %s", out)
	}

	// Pestanya nova: el clic hi porta l'agent.
	if out = do("click", map[string]any{"target": refOf(t, full, `link "Obre una altra pestanya"`)}); !strings.Contains(out, "new tab") || !strings.Contains(out, "Pestanya nova") {
		t.Errorf("havia de passar a la pestanya nova:\n%s", out)
	}
	if out = do("tabs", nil); strings.Count(out, "\n[") < 1 {
		t.Errorf("tabs: %s", out)
	}
	do("tab_select", map[string]any{"tab": "1"})

	// Captura i clic per coordenades.
	if _, imgs, err := BrowserActionFor(nil, scope, "screenshot", map[string]any{}); err != nil || len(imgs) != 1 {
		t.Fatalf("screenshot: %v", err)
	}
}

// refOf treu la ref de la línia que coincideix amb pattern.
func refOf(t *testing.T, page, pattern string) string {
	t.Helper()
	m := regexp.MustCompile(pattern + `[^\n]*\[ref=(e\d+)\]`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no ref for %q in:\n%s", pattern, page)
	}
	return m[1]
}
