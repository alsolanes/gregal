package web

import (
	"testing"

	"gregal/internal/companion"
)

// TestWindNamedInSkipsRouter: si el missatge nomena un sol vent, respon ell
// sense passar pel model que tria; un nom dins d'una altra paraula no compta.
func TestWindNamedInSkipsRouter(t *testing.T) {
	members := []companion.Wind{{ID: "ponent", Name: "Ponent"}, {ID: "garbi", Name: "Garbí"}, {ID: "gregal", Name: "Gregal"}}
	for msg, want := range map[string]string{
		"Ponent, i un de poesia?":       "ponent",
		"@garbí quan juga el Barça?":    "garbi",
		"Ponent i Garbí, què en dieu?":  "",
		"Ponentí no és ningú":           "",
		"un llibre curt per al viatge?": "",
	} {
		w, ok := windNamedIn(members, msg)
		if got := map[bool]string{true: w.ID, false: ""}[ok]; got != want {
			t.Errorf("%q → %q, volia %q", msg, got, want)
		}
	}
}
