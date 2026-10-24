package companion

import (
	"testing"
)

// TestByRefTrobaPerNomOSlug: el model passa el nom del vent, l'id llarg o el
// slug; tots tres han de resoldre al mateix vent.
func TestByRefTrobaPerNomOSlug(t *testing.T) {
	ws := NewWindStore(t.TempDir())
	w, err := ws.Save(Wind{Name: "Garbí", Role: "Esports"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, ref := range []string{w.ID, "Garbí", "garbí", "GARBI", "garbi"} {
		got, ok := ws.ByRef(ref)
		if !ok {
			t.Errorf("ByRef(%q) no ha trobat el vent", ref)
			continue
		}
		if got.ID != w.ID {
			t.Errorf("ByRef(%q) ha tornat %q, calia %q", ref, got.ID, w.ID)
		}
	}
	if _, ok := ws.ByRef("Migjorn"); ok {
		t.Errorf("ByRef d'un vent que no existeix no pot trobar res")
	}
	if _, ok := ws.ByRef(""); ok {
		t.Errorf("ByRef buit no pot trobar res")
	}
}
