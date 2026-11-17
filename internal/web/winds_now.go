package web

import (
	"fmt"
	"time"
)

// windNowBrief és la línia del sistema dels torns de xat: la data i el tram
// del dia, que canvien poc. L'hora exacta NO hi va: un minut diferent al mig
// del prompt feia que el servidor local (llama.cpp i similars) hagués de
// tornar a processar tot el que ve després (eines i instruccions, uns 17k
// tokens) a cada missatge, i un model local trigava minuts. L'hora exacta va
// al final del missatge de l'usuari (windClockTag). Sense data el model se
// l'inventa: un torn del Ventet va respondre «Demà és dijous, 12 de febrer de
// 2026» quan era dijous, 12 de novembre de 2026.
func windNowBrief() string {
	return windNowAt(time.Now())
}

func madridTime(t time.Time) time.Time {
	if loc, err := time.LoadLocation("Europe/Madrid"); err == nil {
		return t.In(loc)
	}
	return t
}

func dayPart(t time.Time) string {
	switch h := t.Hour(); {
	case h < 6:
		return "night"
	case h < 12:
		return "morning"
	case h < 20:
		return "afternoon"
	}
	return "evening"
}

// windNowAt és la part pura, per poder provar-la amb una data fixa.
func windNowAt(t time.Time) string {
	t = madridTime(t)
	return fmt.Sprintf("\n\nNOW: %s, %s (%s). The exact time is in brackets at the end of the user's latest message. Any date, weekday, month or relative day (today, tomorrow, yesterday, this weekend) you mention must come from this line — never guess or compute it from memory.",
		t.Format("Monday, 2 January 2006"), dayPart(t), t.Format("MST"))
}

// windClockTag és l'hora exacta, enganxada al final del missatge de l'usuari
// (la part que ja canvia a cada torn) i no al prompt del sistema.
func windClockTag(t time.Time) string {
	return "\n\n[time now: " + madridTime(t).Format("15:04 MST") + "]"
}

// windNowExactBrief és la línia completa, amb hora, per a les crides soltes
// (sense historial ni memòria cau a reaprofitar).
func windNowExactBrief() string {
	t := madridTime(time.Now())
	return fmt.Sprintf("\n\nNOW: %s, %s (%s). Any date, weekday, month or relative day (today, tomorrow, yesterday, this weekend) you mention must come from this line — never guess or compute it from memory.",
		t.Format("Monday, 2 January 2006"), t.Format("15:04"), t.Format("MST"))
}
