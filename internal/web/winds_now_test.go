package web

import (
	"strings"
	"testing"
	"time"
)

// La data real ha d'arribar al model: sense això se la inventa. Un torn del
// Ventet va respondre «Demà és dijous, 12 de febrer de 2026» quan era dijous,
// 12 de novembre de 2026.
func TestWindNowBriefCarriesTheRealDate(t *testing.T) {
	got := windNowAt(time.Date(2026, 11, 12, 8, 30, 0, 0, time.UTC))
	for _, want := range []string{"NOW:", "Thursday", "12 November 2026"} {
		if !strings.Contains(got, want) {
			t.Fatalf("la línia de la data no diu %q:\n%s", want, got)
		}
	}
}

// I ha de dir clarament que no s'inventi dies ni dates.
func TestWindNowBriefForbidsGuessing(t *testing.T) {
	got := windNowBrief()
	if !strings.Contains(got, "never guess") {
		t.Fatalf("la data no prohibeix inventar-s'ho: %s", got)
	}
	if !strings.Contains(got, "tomorrow") {
		t.Fatalf("no cobreix els dies relatius: %s", got)
	}
}
