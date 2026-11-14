package web

import (
	"net/http"
	"strings"

	"gregal/internal/companion"
)

// Una resposta que acaba quan ja no hi ha ningú mirant.
//
// El torn d'un xat corre amb un context propi (context.Background): si tanques
// l'app o la pestanya a mitja feina, la resposta s'acaba i es desa igualment.
// Però ningú no la rebia: el client que l'havia de mostrar (i fer sonar el
// mòbil amb workFinished) ja no hi era, i com que no era una «veu proactiva»
// tampoc no sortia a /api/winds/novetats ni enviava cap push.
//
// Ara, si qui l'ha demanada ja s'ha desconnectat quan es desa, la resposta
// queda com a novetat (font «resposta» o «resposta:<rotllana>»): fa sonar el
// mòbil amb el push o amb la comprovació periòdica, i l'avís obre el missatge
// exacte. Si el client encara hi és, no es fa res: ja avisa ell.

const replySource = "resposta"

// isReplySource: la novetat és una resposta que vas demanar tu.
func isReplySource(source string) bool {
	return source == replySource || strings.HasPrefix(source, replySource+":")
}

// clientGone: qui ha fet la petició ja no l'escolta.
func clientGone(r *http.Request) bool { return r != nil && r.Context().Err() != nil }

// noteReplyIfAway apunta la resposta com a novetat si el client ja no hi és.
func (s *Server) noteReplyIfAway(r *http.Request, w companion.Wind, source, messageID, text string) {
	if !clientGone(r) || strings.TrimSpace(text) == "" {
		return
	}
	recordNovetatMsg(s, w, source, "", messageID, text)
}
