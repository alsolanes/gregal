package web

import (
	"context"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// maxRondesDebat és quantes voltes pot fer la colla discutint entre ella. Dues
// voltes ja deixen dir-hi la seva a tothom i, si no s'han entès en dues voltes,
// una tercera no ho arregla: torna el pla de qui hi entén més.
const maxRondesDebat = 2

// reglaDebat és el que se'ls diu a la ronda de discussió. El guió sol és la
// manera que té una veu de callar sense gastar una bombolla buida.
const reglaDebat = "\n\nDEBATE ROUND (the group has already answered once): the others have spoken above. " +
	"Read what they said and continue the conversation between you if something must be settled: " +
	"disagree with a concrete reason, add what is missing, or propose the next step. " +
	"If you agree and have nothing to add, reply with exactly - (a single dash) and nothing else. " +
	"If the group now has a plan, start with \"PLA: \" and state it in one or two lines. " +
	"Never repeat what another member already said, never compliment them, one short paragraph at most."

// rotllanaDebat fa parlar la colla entre ella fins a entendre's: cada veu veu
// les altres i pot callar amb un guió. S'atura quan en una ronda no hi ha cap
// veu nova (acord o res a dir) o al màxim de rondes. Torna la conversa
// actualitzada i els missatges que s'hi han afegit.
func (s *Server) rotllanaDebat(rot Rotllana, members []companion.Wind, conversation []llm.Message, msg string, emit func(string, any)) ([]llm.Message, []llm.Message) {
	if len(members) < 2 {
		return conversation, nil
	}
	var noves []llm.Message
	for ronda := 1; ronda <= maxRondesDebat; ronda++ {
		novesAquesta := 0
		for _, sp := range members {
			emit("phase", map[string]string{"phase": "speaker:" + sp.ID})
			system, _ := rotllanaChatBrief(s, rot, sp, conversation, msg)
			system += reglaDebat
			p, role, ok := s.sideRoleRef()
			if !ok {
				return conversation, noves
			}
			s.mu.Lock()
			cfg, client := s.cfg, s.client
			s.mu.Unlock()
			msgs := make([]llm.Message, 0, len(conversation)+1)
			msgs = append(msgs, llm.Message{Role: "system", Content: system})
			msgs = append(msgs, conversation...)
			ctx, cancel := context.WithTimeout(context.Background(), rotllanaTurnTimeout)
			emit("activity", windActivity{Phase: "thinking"})
			out, err := petChatter(ctx, client, cfg, p, role, msgs)
			cancel()
			if err != nil {
				continue
			}
			text := strings.TrimSpace(stripQuoteTrim(out))
			if text == "" || text == "-" || text == "—" || text == "–" || text == "." {
				continue
			}
			// Un model petit a vegades torna el prompt en lloc d'una resposta.
			if semblaPrompt(text) {
				continue
			}
			m := llm.Message{
				Role:        "assistant",
				Content:     sp.Name + ": " + text,
				SpeakerID:   sp.ID,
				SpeakerName: sp.Name,
				Ts:          time.Now().Unix(),
			}
			conversation = append(conversation, m)
			noves = append(noves, m)
			novesAquesta++
			if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
				return conversation, noves
			}
			emit("reply", map[string]any{"wind": sp.ID, "history": s.rotllanaHistoryJSON(conversation)})
			// Si algú ja ha tancat el pla, no cal donar-hi més voltes.
			if strings.Contains(strings.ToUpper(text), "PLA:") {
				return conversation, noves
			}
		}
		if novesAquesta == 0 {
			break
		}
	}
	return conversation, noves
}

// semblaPrompt detecta respostes que són el prompt tornat (un model petit ho fa
// quan es perd): no són una veu i no han d'embrutar el fil.
func semblaPrompt(text string) bool {
	t := strings.ToLower(text)
	for _, marca := range []string{"<ds_system>", "</ds_system>", "you are a helpful assistant", "\"role\": \"system\""} {
		if strings.Contains(t, marca) {
			return true
		}
	}
	return false
}
