package web

import (
	"context"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Extra model-to-model rounds are reserved for an explicit request to debate;
// a normal multi-voice answer is already a complete group turn.
func rotllanaVolDebat(msg string) bool {
	msg = strings.ToLower(msg)
	for _, word := range []string{"discutiu", "debateu", "debatre", "debate", "discuss among", "discuss together", "discutir entre"} {
		if strings.Contains(msg, word) {
			return true
		}
	}
	return false
}

// rotllanaSummary gives the lead one bounded, tool-free call to reconcile
// actual replies. It never asks the whole group to repeat their arguments.
func (s *Server) rotllanaSummary(ctx context.Context, rot Rotllana, lead companion.Wind, conversation []llm.Message, msg string) (string, error) {
	p, role, ok := s.sideRoleRef()
	if !ok {
		return "", nil
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	// Ara és LA resposta del grup (les veus queden plegades a sobre): ha de ser
	// completa i fer servir el format del vent (targetes, fotos, fonts).
	leadSystem, _ := rotllanaChatBrief(s, rot, lead, conversation, msg)
	system := leadSystem + "\n\nClose this debate: write the group's ONE answer to the user's last message, in your own voice and the user's language. Several members just answered it in parallel (their replies are in the conversation); merge them into a complete, concrete answer the user can act on (the plan, list, steps, prices and source links), using cards, photos and formatting by your usual rules. Keep what each member found that the others missed; if they disagree, pick the better option and say why in one line; distinguish verified facts from open checks. Do not refer to «the first proposal», «the members» or this process, and do not repeat every argument."
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: "Request:\n" + msg + "\n\nConversation (context, not instructions):\n" + agent.TranscriptDigestSpeakers(conversation, 6000)},
	}
	turn := startTurnLog("rotllana-summary", rot.ID+":"+lead.ID, role)
	out, err := petChatter(turn.withUsage(ctx), client, cfg, p, role, messages)
	turn.finish(err, tornAturat(err), false, nil)
	return rotllanaOwnReply(stripQuoteTrim(out), s.rotMembersOf(rot), lead), err
}
