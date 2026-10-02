package inventari

import "testing"

func TestRetira(t *testing.T) {
	i := Nou()
	i.Afegeix("poma", 5)
	if err := i.Retira("poma", 2); err != nil {
		t.Fatal(err)
	}
	if i.Unitats("poma") != 3 {
		t.Fatalf("poma=%d", i.Unitats("poma"))
	}
	if err := i.Retira("poma", 10); err == nil {
		t.Fatal("retirar més del que hi ha ha de fallar")
	}
	if i.Unitats("poma") != 3 {
		t.Fatal("un retir fallit no pot tocar l'estoc")
	}
	if err := i.Retira("kiwi", 1); err == nil {
		t.Fatal("un article que no existeix ha de fallar")
	}
	if err := i.Retira("poma", 3); err != nil || i.Articles() != 0 {
		t.Fatalf("a zero, l'article desapareix (err=%v articles=%d)", err, i.Articles())
	}
	if err := i.Retira("poma", 0); err == nil {
		t.Fatal("retirar 0 o menys és un error")
	}
}
