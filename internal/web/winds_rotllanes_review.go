package web

import (
	"strings"
	"unicode/utf8"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Revisió entre companys. Quan un vent respon, un altre membre que no ha
// parlat el llegeix amb el que només ell sap (ofici, memòria, habilitats) i,
// si hi veu un error, un risc o una millor opció, ho diu; l'autor contesta
// breument. És proactiu però barat: sense res a afegir, el revisor calla («-»)
// i no es veu res.

// rotllanaReviewMinLen: una resposta més curta (salutacions, «fet») no val
// una revisió.
const rotllanaReviewMinLen = 160

// rotllanaWantsReview diu si aquest torn mereix que un company el revisi.
// Es respecta qui ha adreçat un vent concret, les continuacions i els torns
// que ja són revisió.
func rotllanaWantsReview(directed, isContinue bool, intent, reply string) bool {
	reply = strings.TrimSpace(reply)
	if directed || isContinue || intent == "review" {
		return false
	}
	if utf8.RuneCountInString(reply) < rotllanaReviewMinLen {
		return false
	}
	return true
}

// rotllanaReviewer tria qui revisa: el membre que no ha parlat i que més
// encaixa amb el tema; en cas d'empat, el que menys ha participat.
func (s *Server) rotllanaReviewer(members, spoke []companion.Wind, msg string, tail []llm.Message) (companion.Wind, bool) {
	var cands []companion.Wind
	for _, w := range members {
		used := false
		for _, sp := range spoke {
			if sp.ID == w.ID {
				used = true
				break
			}
		}
		if !used {
			cands = append(cands, w)
		}
	}
	if len(cands) == 0 {
		return companion.Wind{}, false
	}
	return rotllanaSpecialist(cands, s.rankWindSpecialists(cands, msg), tail), true
}

// rotllanaReviewBrief és la consigna del revisor.
func rotllanaReviewBrief(reviewer, author companion.Wind) string {
	return "\n\nPEER REVIEW. You, " + reviewer.Name + ", did not answer this message; " + author.Name + " did (see the conversation above). " +
		"Read that answer with what only you know: your specialty, your memory and skills, the whole conversation. " +
		"Be a colleague, not a gatekeeper: if you spot a factual error, a wrong assumption, a missing risk, a better option, or something the human would clearly want that was left out, say it in 1-4 sentences, addressing " + author.Name + " by name, with the concrete fix or a source (verify with your tools when unsure). " +
		"Do not repeat what was said, do not praise, do not ask the human questions, do not speak for anybody else. " +
		"If the answer is right and complete from where you stand, reply with exactly - and nothing else."
}

// rotllanaAuthorBrief és la rèplica de l'autor al revisor.
func rotllanaAuthorBrief(author, reviewer companion.Wind) string {
	return "\n\nREPLY TO YOUR PEER. " + reviewer.Name + " just reviewed your answer (see the end of the conversation). " +
		"In 1-3 sentences, addressing " + reviewer.Name + " by name: if they are right, accept it and give the corrected version of what changes, for the human; if they are wrong or it does not apply, say why in one line. " +
		"No thanks, no repetition. If there is nothing to add, reply with exactly - and nothing else."
}
