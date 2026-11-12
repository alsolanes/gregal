package web

import (
	"fmt"
	"time"
)

// windNowBrief diu al model quina data i hora és. Sense això se les inventa: un
// torn del Ventet va respondre «Demà és dijous, 12 de febrer de 2026» quan era
// dijous, 12 de novembre de 2026 (encertava el dia de la setmana del 12 i res
// més). És una línia del sistema, no de l'usuari: el model no ha de fer-ne cas
// com si fos una instrucció de la persona.
func windNowBrief() string {
	return windNowAt(time.Now())
}

// windNowAt és la part pura, per poder provar-la amb una data fixa.
func windNowAt(t time.Time) string {
	if loc, err := time.LoadLocation("Europe/Madrid"); err == nil {
		t = t.In(loc)
	}
	return fmt.Sprintf("\n\nNOW: %s, %s (%s). Any date, weekday, month or relative day (today, tomorrow, yesterday, this weekend) you mention must come from this line — never guess or compute it from memory.",
		t.Format("Monday, 2 January 2006"), t.Format("15:04"), t.Format("MST"))
}
