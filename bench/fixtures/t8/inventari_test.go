package inventari

import "testing"

func TestAfegeix(t *testing.T) {
	i := Nou()
	i.Afegeix("poma", 3)
	i.Afegeix("poma", 2)
	i.Afegeix("pera", 0)
	if i.Unitats("poma") != 5 || i.Articles() != 1 {
		t.Fatalf("poma=%d articles=%d", i.Unitats("poma"), i.Articles())
	}
}
