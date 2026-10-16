package llm

import (
	"strings"
	"unicode/utf8"
)

// Bucles de repetició.
//
// Els models locals (Qwen3 i R1 sobretot, a temperatura baixa) a vegades
// entren en un bucle i repeteixen el mateix paràgraf fins a esgotar
// max_tokens: en local són minuts perduts i, al final, una resposta tallada.
// El guard mira el text que va arribant: si el bloc dels darrers 200
// caràcters ja hi és 4 vegades als darrers 8000, el corrent es talla, es
// conserva el text fins a la primera aparició i s'hi afegeix una nota.

const (
	repBloc    = 200
	repFinestra = 8000
	repVegades = 4
)

// NotaRepeticio s'afegeix al text tallat.
const NotaRepeticio = "\n\n[resposta tallada: el model repetia el mateix text]"

type repGuard struct {
	revisat int
	tall    int
}

// veu revisa el text acumulat; true si hi ha un bucle (i desa on tallar).
func (g *repGuard) veu(s string) bool {
	if len(s)-g.revisat < repBloc {
		return false
	}
	g.revisat = len(s)
	bloc := s[len(s)-repBloc:]
	if strings.TrimSpace(bloc) == "" {
		return false
	}
	ini := 0
	if len(s) > repFinestra {
		ini = len(s) - repFinestra
	}
	finestra := s[ini:]
	if strings.Count(finestra, bloc) < repVegades {
		return false
	}
	primer := ini + strings.Index(finestra, bloc) + len(bloc)
	for primer < len(s) && !utf8.RuneStart(s[primer]) {
		primer++
	}
	g.tall = primer
	return true
}

// talla torna el text fins al primer bloc repetit, amb la nota.
func (g *repGuard) talla(s string) string {
	if g.tall <= 0 || g.tall > len(s) {
		return s + NotaRepeticio
	}
	return strings.TrimRight(s[:g.tall], " \n") + NotaRepeticio
}
