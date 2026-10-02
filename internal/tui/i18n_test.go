package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Una traducció a mitges no fa cap error: només et deixa mitja pantalla en
// l'idioma que no toca. Els dos diccionaris han de tenir les mateixes claus.
func TestDiccionarisAmbLesMateixesClaus(t *testing.T) {
	for k := range catala {
		if _, ok := angles[k]; !ok {
			t.Errorf("%q només és al català", k)
		}
	}
	for k := range angles {
		if _, ok := catala[k]; !ok {
			t.Errorf("%q només és a l'anglès", k)
		}
	}
	for nom, d := range diccionaris {
		for k, v := range d {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: %q és buida", nom, k)
			}
		}
	}
}

func TestTraduccio(t *testing.T) {
	t.Cleanup(func() { SetIdioma("ca") })
	SetIdioma("ca")
	if got := T("barra.llest"); got != "llest" {
		t.Fatalf("català: %q", got)
	}
	SetIdioma("en")
	if got := T("barra.llest"); got != "ready" {
		t.Fatalf("anglès: %q", got)
	}
	// Un idioma que no existeix no canvia res.
	SetIdioma("klingon")
	if got := T("barra.llest"); got != "ready" {
		t.Fatalf("un idioma desconegut no ha de canviar l'actual: %q", got)
	}
	// Una clau sense traducció cau al català, no a la clau crua.
	SetIdioma("en")
	if got := T("no.existeix.enlloc"); got != "no.existeix.enlloc" {
		t.Fatalf("una clau inexistent es torna ella mateixa: %q", got)
	}
}

// Una clau que ja no fa servir ningú és pes mort: s'ha de traduir, es
// revisa, i no es veu enlloc. Van sortir soles convertint els missatges
// d'estat (n'hi havia dues que al codi no existien).
func TestCapClauMorta(t *testing.T) {
	_, this, _, _ := runtime.Caller(0)
	dir := filepath.Dir(this)
	fitxers, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var codi strings.Builder
	for _, f := range fitxers {
		if strings.HasSuffix(f, "i18n.go") {
			continue // el diccionari no compta com a ús
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		codi.Write(raw)
	}
	text := codi.String()
	var mortes []string
	for k := range catala {
		if !strings.Contains(text, `"`+k+`"`) {
			mortes = append(mortes, k)
		}
	}
	sort.Strings(mortes)
	if len(mortes) > 0 {
		t.Fatalf("claus que no fa servir ningú: %s", strings.Join(mortes, ", "))
	}
}
