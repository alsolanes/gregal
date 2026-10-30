package nfl

import (
	"strings"
	"testing"

	"gregal/internal/football"
)

// Les ajudes del format (contra què guanya i perd cada crida) es poden
// apagar: les competicions oficials les apaguen per defecte.
func TestHintsSetting(t *testing.T) {
	m := Game{}.NewMatch(7, "en", [2]string{"A", "B"}).(*Match)
	p, _ := m.Next()
	off := m.g.Poss
	if !strings.Contains(p[off].Schema, "vs defenses:") || !strings.Contains(p[1-off].Schema, "beaten by") {
		t.Fatalf("per defecte, amb ajudes: %s", p[off].Schema)
	}
	m.SetHints(false)
	p, _ = m.Next()
	if p[off].Schema != football.OffenseSchemaBare || p[1-off].Schema != football.DefenseSchemaBare {
		t.Fatalf("sense ajudes: %s / %s", p[off].Schema, p[1-off].Schema)
	}
	for _, s := range []string{p[off].Schema, p[1-off].Schema} {
		if strings.Contains(s, "vs defenses") || strings.Contains(s, "stops") || strings.Contains(s, "beaten by") || strings.Contains(s, "++") {
			t.Fatalf("el format sense ajudes no pot portar la taula: %s", s)
		}
	}
	// El format sense ajudes porta les mateixes crides i descripcions.
	for _, c := range football.OffensePlays {
		if !strings.Contains(football.OffenseSchemaBare, c) {
			t.Fatalf("falta %s", c)
		}
	}
	for _, c := range football.DefenseCalls {
		if !strings.Contains(football.DefenseSchemaBare, "- "+c+": ") {
			t.Fatalf("falta %s", c)
		}
	}
	// La taula continua a les regles: és com funciona el motor.
	if !strings.Contains(Game{}.Rules(), "MATCHUP TABLE") {
		t.Fatal("les regles han de portar la taula")
	}
}
