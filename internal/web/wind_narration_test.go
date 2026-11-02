package web

import "testing"

func TestTornTextSeparaLaNarracioDeLaResposta(t *testing.T) {
	var tt tornText
	tt.token("Ara miro els fitxers")
	tt.fita() // una eina pel mig
	tt.token("Hi ha 11 fitxers")
	n := tt.narracions("Hi ha 11 fitxers")
	if len(n) != 1 || n[0] != "Ara miro els fitxers" {
		t.Fatalf("narracions = %v", n)
	}
}

func TestTornTextNoRepeteixElQueJaEsALaResposta(t *testing.T) {
	var tt tornText
	tt.token("Primer")
	tt.fita()
	tt.token("Primer i segon")
	n := tt.narracions("Primer i segon")
	if len(n) != 0 {
		t.Fatalf("no hauria de repetir res: %v", n)
	}
	if got := ambNarracions(nil, "res"); got != "res" {
		t.Fatalf("ambNarracions = %q", got)
	}
	if got := ambNarracions([]string{"a"}, "b"); got != "a\n\nb" {
		t.Fatalf("ambNarracions = %q", got)
	}
}

func TestTornTextSenseFitesNoInventaNarracio(t *testing.T) {
	var tt tornText
	tt.token("Només resposta")
	if n := tt.narracions("Només resposta"); len(n) != 0 {
		t.Fatalf("narracions = %v", n)
	}
}
