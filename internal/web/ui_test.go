package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIconaDeMarcaEsServeix(t *testing.T) {
	s, _ := fileTestServer(t)
	for _, tc := range []struct{ path, mime string }{
		{"/app/icon.png", "image/png"},
		{"/app/ventet.svg", "image/svg+xml"},
	} {
		w := httptest.NewRecorder()
		s.Hub().mux().ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != 200 || w.Header().Get("Content-Type") != tc.mime || w.Body.Len() == 0 {
			t.Errorf("%s: status=%d mime=%q mida=%d", tc.path, w.Code, w.Header().Get("Content-Type"), w.Body.Len())
		}
	}
}

// La UI 1.1 ha de portar les pestanyes de sessió, les vistes noves i el
// pont amb els mòduls: si això marxa, l'escriptori torna a ser un mirall.
func TestIndexPorta11UI(t *testing.T) {
	html := string(indexHTML)
	for _, want := range []string{
		`id="sessionTabs"`,
		`data-view="canvis"`,
		`data-view="fitxers"`,
		`data-view="terminal"`,
		`id="canvisPage"`,
		`id="fitxersPage"`,
		`id="terminalPage"`,
		`id="changesBody"`,
		`id="treeBody"`,
		`id="fileView"`,
		`id="procOut"`,
		`id="hudCost"`,
		`X-Gregal-Session`,
		`window.gregal = {`,
		`import { boot } from '/app/panels.js'`,
		`window.gregalPanels`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("falta a la UI: %s", want)
		}
	}
}

// El mòdul de panells s'ha d'embeure i servir.
func TestModulPanells(t *testing.T) {
	raw, err := assets.ReadFile("app/panels.js")
	if err != nil {
		t.Fatalf("app/panels.js no està embegut: %v", err)
	}
	js := string(raw)
	for _, want := range []string{
		"export function boot",
		"/api/sessions/live",
		"/api/diff/discard",
		"/api/tree",
		"/api/exec",
		"export const composer",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("falta al mòdul: %s", want)
		}
	}
}

// Amb «lang: en» al config, el TUI sortia en anglès i la finestra en
// català: idioma() només mirava el navegador i, si no hi havia res desat,
// es quedava amb 'ca'. Ara el config del servidor és el valor per defecte
// i la tria del navegador, quan n'hi ha, continua manant.
func TestLIdiomaDelServidorArribaALaFinestra(t *testing.T) {
	if !strings.Contains(string(indexHTML), "window.gregalLang(r.lang)") {
		t.Error("refresh() no passa el lang del servidor a la finestra")
	}
	pn, _ := assets.ReadFile("app/panels.js")
	panells := string(pn)
	if !strings.Contains(panells, "window.gregalLang = defineixPerDefecte") {
		t.Error("boot() no exposa defineixPerDefecte com a window.gregalLang")
	}
	in, _ := assets.ReadFile("app/i18n.js")
	i18n := string(in)
	for _, want := range []string{
		"export function defineixPerDefecte",
		"let perDefecte",
	} {
		if !strings.Contains(i18n, want) {
			t.Errorf("i18n.js no té %q", want)
		}
	}
	// El valor desat al navegador ha de guanyar: si defineixPerDefecte
	// hi escrivís, canviar l'idioma des del config esborraria la tria.
	if strings.Contains(i18n, "setItem(CLAU_DESAT") {
		pos := strings.Index(i18n, "export function defineixPerDefecte")
		fi := strings.Index(i18n[pos:], "\n}")
		if strings.Contains(i18n[pos:pos+fi], "setItem") {
			t.Error("defineixPerDefecte desa al navegador: el config trepitjaria la tria de l'usuari")
		}
	}
}

// El TUI té historial d'entrada des de sempre; la finestra no en tenia
// gens, i per repetir un missatge o corregir-ne un de llarg l'havies de
// tornar a escriure sencer.
func TestLaFinestraTeHistorialDEntrada(t *testing.T) {
	html := string(indexHTML)
	for _, want := range []string{
		"gregal_historial",
		"function histNavega",
		"histApunta(txt)",
		// El desplegable d'ordres i el de mencions es queden les fletxes:
		// allà serveixen per triar de la llista.
		"#suggest:not([hidden])",
		// Un cop navegues, continues fins que escrius: si no, en recuperar
		// un missatge de dues línies el cursor queda a la segona i la
		// fletxa següent ja no és de l'historial.
		"if (histIdx >= 0) return true;",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("falta a l'historial del composer: %s", want)
		}
	}
}
