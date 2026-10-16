package ooxml

import "testing"

func TestDeckSpecAcceptsModelShapes(t *testing.T) {
	spec, err := ParseDeckSpec(`{"slides":[{"layout":"content","title":"T","table":[{"Trimestre":"T1","Vendes":10},{"Trimestre":"T2","Vendes":"14"}],"chart":{"type":"column","categories":["T1","T2"],"series":[{"name":"V","values":["10","14,5"]}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	tb := spec.Slides[0].Table
	if len(tb) != 3 || tb[0][0] != "Trimestre" || tb[0][1] != "Vendes" || tb[2][1] != "14" {
		t.Fatalf("taula: %v", tb)
	}
	if v := spec.Slides[0].Chart.Series[0].Values; len(v) != 2 || v[1] != 14.5 {
		t.Fatalf("valors: %v", v)
	}
	// La forma clàssica continua funcionant.
	spec, err = ParseDeckSpec(`[{"table":[["a","b"],[1,true]]}]`)
	if err != nil || spec.Slides[0].Table[1][1] != "true" {
		t.Fatalf("taula clàssica: %v %v", spec, err)
	}
}
