package web

import (
	"strings"
	"time"
)

// Màxim de text que viatja a l'avís del torn acabat. El fil sencer ja és a
// l'app; l'avís només ha de dir de què anava sense omplir la pantalla.
const tornAvísMax = 400

// recordTornAcabat deixa una novetat quan el torn de Gregal s'ha acabat, perquè
// el mòbil ho sàpiga amb l'app tancada o en segon pla.
//
// Reaprofita el camí de les novetats —que ja porta el push, el sondeig de
// quinze minuts, el registre de duplicats de l'app i el toc per obrir el fil—
// en lloc de muntar un segon sistema d'avisos. No crea cap targeta de la
// safata «Avui»: un torn no és una cosa per decidir, només una cosa que has
// demanat i s'ha acabat.
func (s *Server) recordTornAcabat(reply string) {
	if s == nil {
		return
	}
	text := strings.TrimSpace(reply)
	if text == "" {
		return
	}
	if r := []rune(text); len(r) > tornAvísMax {
		text = strings.TrimSpace(string(r[:tornAvísMax])) + "…"
	}
	n := windNovetat{
		Ts:     time.Now().Unix(),
		Wind:   gregalWindID,
		Name:   "Gregal",
		Source: "torn",
		Text:   text,
		ID:     newNovetatID(),
		Owner:  s.usuari(),
	}
	novetatsMu.Lock()
	saveNovetats(s, append(loadNovetats(s), n))
	novetatsMu.Unlock()
	s.appendNovetatEvent(eventNovetat, novetatEventPayload{
		ID: n.ID, Wind: n.Wind, Source: n.Source, Ts: n.Ts,
	})
	s.pushNovetat(n)
}
