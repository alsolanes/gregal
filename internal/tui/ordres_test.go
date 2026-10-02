package tui

import (
	"strings"
	"testing"
)

// Les descripcions de les ordres han de seguir l'idioma actiu. Com a
// variable de paquet no ho feien: els T() s'avaluaven a la inicialització,
// abans que New() fixés l'idioma, i el desplegable de "/" es quedava en
// català per sempre. És un error mut —surt text, només que en l'idioma que
// no toca— i per això té test.
func TestOrdresSegueixenLIdioma(t *testing.T) {
	t.Cleanup(func() { SetIdioma("ca") })
	troba := func(nom string) string {
		for _, c := range ordres() {
			if c.name == nom {
				return c.desc
			}
		}
		t.Fatalf("no hi ha cap ordre %q", nom)
		return ""
	}
	SetIdioma("ca")
	if got := troba("quit"); got != "surt" {
		t.Fatalf("català: %q", got)
	}
	SetIdioma("en")
	if got := troba("quit"); !strings.Contains(got, "quit") {
		t.Fatalf("anglès: %q", got)
	}
}
