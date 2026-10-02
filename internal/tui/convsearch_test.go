package tui

import "testing"

func TestFindNextNavegaIContinua(t *testing.T) {
	m := modelDeBarra(t, 90)
	m.lines = nil
	for _, line := range []string{"inici", "primer ERROR", "mig", "segon error", "final"} {
		m.push(line)
	}
	m.vp.GotoTop()
	if !m.findNext("error") || m.findLine != 1 {
		t.Fatalf("primera coincidència=%d", m.findLine)
	}
	if !m.findNext("error") || m.findLine != 3 {
		t.Fatalf("segona coincidència=%d", m.findLine)
	}
	if !m.findNext("error") || m.findLine != 1 {
		t.Fatalf("la cerca no ha tornat al principi: %d", m.findLine)
	}
}

func TestFindIgnoraANSI(t *testing.T) {
	m := modelDeBarra(t, 90)
	m.lines = []string{okStyle.Render("resultat aprovat")}
	if !m.findNext("aprovat") {
		t.Fatal("la cerca no troba text pintat")
	}
}
