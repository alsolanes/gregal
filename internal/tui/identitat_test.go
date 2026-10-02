package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// docs/identitat.md és la font de veritat de la paleta i theme.go n'és un
// derivat. Aquest test és el que impedeix que tornin a divergir: cada front
// havia acabat amb la seva paleta perquè no hi havia res que ho comprovés.

// paletaDoc llegeix la taula "## Paleta" del document: nom → hex.
func paletaDoc(t *testing.T) map[string]string {
	t.Helper()
	_, this, _, _ := runtime.Caller(0)
	doc := filepath.Join(filepath.Dir(this), "..", "..", "docs", "identitat.md")
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("no trobo la font de veritat: %v", err)
	}
	fila := regexp.MustCompile("^\\| `([A-Za-z]+)` \\| `(#[0-9A-Fa-f]{6})` \\|")
	out := map[string]string{}
	dins := false
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(l, "## ") {
			dins = strings.HasPrefix(l, "## Paleta")
			continue
		}
		if !dins {
			continue
		}
		if m := fila.FindStringSubmatch(l); m != nil {
			out[m[1]] = strings.ToUpper(m[2])
		}
	}
	if len(out) < 10 {
		t.Fatalf("la taula de la paleta té %d files; el format ha canviat?", len(out))
	}
	return out
}

// Les variables de theme.go amb nom a la taula han de valer el que diu la
// taula, ni un dígit diferent.
func TestIdentitatThemeSegueixElDocument(t *testing.T) {
	doc := paletaDoc(t)
	theme := map[string]lipgloss.Color{
		"escuma": escuma, "arena": arena, "onada": onada, "algua": algua,
		"fons": fons, "night": night, "slate": slate, "green": green, "red": red,
	}
	for nom, c := range theme {
		want, ok := doc[nom]
		if !ok {
			t.Errorf("%s és a theme.go però no a docs/identitat.md", nom)
			continue
		}
		if got := strings.ToUpper(string(c)); got != want {
			t.Errorf("%s: theme.go diu %s, el document diu %s", nom, got, want)
		}
	}
}

// I al revés: cap hex escrit a theme.go pot quedar fora del document. Si cal
// un color nou, primer s'apunta a la taula (amb nom i rol) i després s'usa.
func TestIdentitatCapColorFantasma(t *testing.T) {
	doc := paletaDoc(t)
	coneguts := map[string]bool{}
	for _, hex := range doc {
		coneguts[hex] = true
	}
	// boxIdle és algua aclarida i el document el tolera explícitament.
	coneguts["#3C5750"] = true

	_, this, _, _ := runtime.Caller(0)
	// TOT el paquet, no només theme.go: els colors fantasma que hi havia
	// (un blau genèric a la caixa de preguntes i al selector, un verd que no
	// era el verd) vivien justament als altres fitxers, i per això aquest
	// test no els havia vist mai.
	fitxers, err := filepath.Glob(filepath.Join(filepath.Dir(this), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fitxers {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, hex := range regexp.MustCompile("#[0-9A-Fa-f]{6}").FindAllString(string(raw), -1) {
			if !coneguts[strings.ToUpper(hex)] {
				t.Errorf("%s fa servir %s i docs/identitat.md no el coneix", filepath.Base(f), hex)
			}
		}
	}
}
