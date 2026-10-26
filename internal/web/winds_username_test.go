package web

import "testing"

// TestWindUserNameIgnoresTechnicalNames: sense comptes l'usuari es diu «local»;
// això no és un nom i els vents no l'han de fer servir.
func TestWindUserNameIgnoresTechnicalNames(t *testing.T) {
	for _, u := range []*User{nil, {Name: "local", Admin: true, Tothom: true}, {Name: "local", Admin: true}, {Name: "Admin"}} {
		if got := (&Server{user: u}).windUserName(); got != "" {
			t.Errorf("%+v → %q, volia buit", u, got)
		}
	}
	if got := (&Server{user: &User{Name: "Aleix"}}).windUserName(); got != "Aleix" {
		t.Errorf("nom real perdut: %q", got)
	}
}
