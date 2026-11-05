package web

import "testing"

func TestRepairChoicesBlock(t *testing.T) {
	cases := map[string]string{
		// La primera opció a la línia de la tanca.
		"Fet.\n\n```choices 45 minuts de peses\nCursa d'una hora\n```":
			"Fet.\n\n```choices\n45 minuts de peses\nCursa d'una hora\n```",
		// Tot en una línia (casos reals de les proves).
		"Bon profit 🍳\n\n```choices Con huevo escalfado encima Lo hago revuelto y más rápido Qué hago de postre con lo que tenga ```\n🍳":
			"Bon profit 🍳\n\n```choices\nCon huevo escalfado encima\nLo hago revuelto y más rápido\nQué hago de postre con lo que tenga\n```\n🍳",
		"Bé.\n\n```choices Per què «Seda» i no un altre Una novel·la curta més recent Què llegeixo després si m'agrada ```":
			"Bé.\n\n```choices\nPer què «Seda» i no un altre\nUna novel·la curta més recent\nQuè llegeixo després si m'agrada\n```",
		// Ja bé: no es toca.
		"Hola\n\n```choices\nSí\nNo\n```": "Hola\n\n```choices\nSí\nNo\n```",
		// Sense bloc: no es toca.
		"Res de botons.": "Res de botons.",
	}
	for in, want := range cases {
		if got := repairChoicesBlock(in); got != want {
			t.Errorf("\nin:   %q\ngot:  %q\nwant: %q", in, got, want)
		}
	}
	// Massa ambigu per partir-ho (una sola opció llarga): es deixa estar.
	in := "Vale.\n\n```choices Llegeix Pedro Páramo de Juan Rulfo ```"
	if got := repairChoicesBlock(in); got != in {
		t.Errorf("ambigu: %q", got)
	}
}
