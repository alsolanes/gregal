package web

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gregal/internal/tema"
)

// actualitzaCSS reescriu el bloc generat: go test ./internal/web -run TestCSSAlDia -update
var actualitzaCSS = os.Getenv("GREGAL_CSS_UPDATE") != ""

// El :root d'index.html el genera internal/tema. Aquest test és el que
// impedeix que tornin a ser dues còpies: si canvies la paleta i no
// regeneres el full, salta.
//
// Per regenerar-lo:
//
//	GREGAL_CSS_UPDATE=1 go test ./internal/web -run TestCSSAlDia
func TestCSSAlDia(t *testing.T) {
	_, this, _, _ := runtime.Caller(0)
	ruta := filepath.Join(filepath.Dir(this), "index.html")
	raw, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	// El checkout a Windows pot convertir a CRLF segons el core.autocrlf
	// de cada màquina; el generador emet LF. Es treballa normalitzat perquè
	// el test vigili la paleta, no els finals de línia del checkout (i en
	// regenerar, el bloc queda en LF com al repo).
	norm := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
	html := norm(string(raw))
	vol := tema.CSSRoot()

	ini, fi, ok := bloc(html)
	if !ok {
		t.Fatal("no trobo els blocs :root a index.html")
	}
	te := strings.TrimSpace(html[ini:fi])
	if te == strings.TrimSpace(norm(vol)) {
		return
	}
	if !actualitzaCSS {
		t.Fatalf("el :root d'index.html no és el que genera internal/tema.\n"+
			"Regenera'l amb: GREGAL_CSS_UPDATE=1 go test ./internal/web -run TestCSSAlDia\n\n"+
			"--- té ---\n%s\n\n--- vol ---\n%s", te, vol)
	}
	nou := html[:ini] + vol + "\n" + html[fi:]
	if err := os.WriteFile(ruta, []byte(nou), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Log("index.html actualitzat des d'internal/tema")
}

// bloc localitza el tros que va del comentari de la paleta fosca fins al
// tancament de l'últim bloc amb data-theme (light, gpt, claude, opencode…).
func bloc(html string) (int, int, bool) {
	ini := strings.Index(html, ":root {")
	if ini < 0 {
		return 0, 0, false
	}
	// El comentari que el precedeix també és generat.
	if c := strings.LastIndex(html[:ini], "/*"); c >= 0 && ini-c < 900 {
		ini = c
	}
	marca := `:root[data-theme="`
	j := strings.LastIndex(html, marca)
	if j < 0 {
		return 0, 0, false
	}
	fi := strings.Index(html[j:], "\n}")
	if fi < 0 {
		return 0, 0, false
	}
	return ini, j + fi + 2, true
}

// Cap color escrit a mà fora del bloc generat: el mateix criteri que ja
// vigilava identitat.test.js, però també per als hex que hi pogués
// afegir el codi de Go.
func TestCapHexFosaDelBlocGenerat(t *testing.T) {
	_, this, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(this), "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	sense := regexp.MustCompile(`(?s):root(\[[^\]]*\])?\s*\{.*?\}`).ReplaceAllString(string(raw), "")
	if solts := regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`).FindAllString(sense, -1); len(solts) > 0 {
		t.Fatalf("hex solts a index.html: %s — passa'ls a var(--…)", strings.Join(solts, ", "))
	}
}

// calma.css repinta el fosc a mà (no és el bloc generat); les superfícies
// han de ser les de tema.Fosc() perquè el fosc sigui negre de debò.
func TestCalmaFoscSegueixElTema(t *testing.T) {
	_, this, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(this), "app", "calma.css"))
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	bloc := func(sel string) string {
		i := strings.Index(css, sel)
		if i < 0 {
			t.Fatalf("calma.css no té %s", sel)
		}
		return css[i : i+strings.Index(css[i:], "}")]
	}
	f := tema.Fosc()
	base := bloc(`:root, :root[data-theme="dark"] {`)
	for nom, vol := range map[string]string{
		"--bg": f.Fons, "--sidebar": f.Abisme, "--panel": f.Abisme,
		"--surface": f.Night, "--surface-hi": f.SuperficieAlta, "--codi-fons": f.Abisme,
	} {
		if !strings.Contains(base, nom+":"+vol+";") {
			t.Errorf("calma.css: %s hauria de ser %s", nom, vol)
		}
	}
	if !strings.Contains(bloc(`:root[data-theme="dark"] { --composer`), "--composer:"+f.Night+";") {
		t.Errorf("calma.css: --composer del fosc hauria de ser %s", f.Night)
	}
}
