package agent

import (
	"strings"
)

// Missatges de l'usuari escrits mentre el torn treballa (com fan els altres
// agents de codi): no esperen que acabi el torn ni el tallen. Entren a
// l'historial al següent punt segur (quan totes les crides del lot ja tenen
// resultat, via les guies) i el model els té en compte en el pas següent.
// Demanat el 2026-10-14: «si mentre treballa escrivim alguna cosa, que ho
// consideri i després segueixi amb les consideracions pertinents».

// GuiaInterjeccio emmarca el missatge perquè el model sàpiga d'on surt i què
// se n'espera: ajustar el que fa, no començar de zero.
func GuiaInterjeccio(text string) string {
	return "The user sent this message while you were working:\n\n" + text +
		"\n\nTake it into account from now on. If it changes what they want, adjust or stop the current approach; " +
		"if it adds a requirement or a correction, apply it; otherwise carry on. Do not redo work that is already done " +
		"and do not ask for confirmation unless the message really leaves you unable to continue."
}

// Interjeccio afegeix un missatge de l'usuari al torn en curs. Si el torn
// ja havia escrit la resposta final (el missatge ha arribat mentre la
// redactava), el torn es reobre: aquella resposta queda com a pas de
// treball i el model continua amb el missatge nou. Un torn fallit no es
// reobre. Torna si el missatge ha entrat.
func (t *Torn) Interjeccio(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || t.fallit {
		return false
	}
	if t.acabat {
		t.acabat = false
		t.resposta = ""
		if t.esgotat {
			// Sense passos lliures el missatge no es podria atendre.
			t.esgotat = false
			t.extres += 8
		}
	}
	t.guies = append(t.guies, GuiaInterjeccio(text))
	return true
}

// guiesPendents diu si hi ha missatges esperant el punt segur (tests).
func (t *Torn) guiesPendents() int { return len(t.guies) }
