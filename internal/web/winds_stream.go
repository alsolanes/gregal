package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Streaming dels vents: el mateix torn que el xat clàssic, però amb
// esdeveniments en directe (fase + tokens + done). El client mostra el
// text a mesura que arriba en comptes de mirar punts suspensius.
//
// Protocol (SSE, un event per línia doble):
//   phase: {"phase":"thinking|searching|scheduling|working"}
//   token: {"token":"fragment"}              (només text de resposta)
//   done:  {"reply":..., "history":..., ...}  (mateix payload que el clàssic)
//   error: {"error":"..."}
// Els tokens especulatius d'un pas que acaba en eines es retiren: el
// client els descarta quan arriba una fase (poden ser JSON d'eina, no
// text per a l'usuari).

// windLoopStream és la via de cerca amb emissió; els tests la substitueixen.
var windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRoleStream(ctx, brief, 6, p, role, emit)
}

// rotllanaVoiceLoopStream és la via de cerca en grup amb emissió.
var rotllanaVoiceLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRoleStream(ctx, brief, 6, p, role, emit)
}

// streamPhase tradueix els noms d'eines d'un pas a fase per al client.
func streamPhase(names string) string {
	n := strings.ToLower(names)
	switch {
	case strings.Contains(n, "web_search") || strings.Contains(n, "web_fetch"):
		return "searching"
	case strings.Contains(n, "remind"):
		return "scheduling"
	default:
		return "working"
	}
}

// windChatBrief munta la consigna del xat directe (compartida pel clàssic i l'stream).
func windChatBrief(s *Server, wind companion.Wind, previous []llm.Message, msg string) string {
	system := companion.WindChatSystem(wind, s.personalMemory(), s.projectName(), s.windGoalTitles(), s.lastSessionDigest())
	return system + companion.WindBrainSystem(s.windBrain(wind.ID)) + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + agent.TranscriptDigest(previous, 16000) + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer — no search needed." + windRemindBrief(wind.ID)
}

// rotllanaChatBrief munta la consigna del torn en grup (clàssic i stream).
func rotllanaChatBrief(s *Server, rot Rotllana, speakingWind companion.Wind, conversation []llm.Message, msg string) (system, brief string) {
	system = companion.RotllanaSystem(speakingWind, s.rotllanaRoster(rot), s.personalMemory()) + companion.WindBrainSystem(s.windBrain(speakingWind.ID))
	transcript := agent.TranscriptDigest(conversation, 16000)
	brief = system + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + transcript + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer — no search needed. Never invent facts." + windRemindBrief(speakingWind.ID)
	return system, brief
}

// handleWindChatStream és el xat directe amb emissió en directe.
func (s *Server) handleWindChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind     string `json:"wind"`
		Message  string `json:"message"`
		Continue bool   `json:"continue"` // reprèn l'última resposta parcial
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	wind, ok := s.winds.ByID(strings.TrimSpace(req.Wind))
	if !ok {
		http.Error(w, "unknown wind", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(req.Message)
	hist := windHistory(s, wind.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock()
	previous := append([]llm.Message(nil), hist.messages...)
	if len(previous) > 40 {
		previous = previous[len(previous)-40:]
	}
	if msg == "" && !(req.Continue && len(previous) > 0) {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	emit := sse(w)
	emit("phase", map[string]string{"phase": "thinking"})
	turnStart := time.Now()
	isContinue := req.Continue && len(previous) > 0
	var res exploracio
	var err error
	if isContinue {
		s.mu.Lock()
		p, role := s.windRoleRef()
		s.mu.Unlock()
		res, err = s.runReadOnlyLoopForRoleStream(ctx, "", 6, p, role, func(kind, text string) {
			switch kind {
			case "token":
				emit("token", map[string]string{"token": text})
			case "phase":
				emit("phase", map[string]string{"phase": streamPhase(text)})
			}
		}, previous)
	} else {
		brief := windChatBrief(s, wind, previous, msg)
		res, err = windLoopStream(s, ctx, brief, func(kind, text string) {
			switch kind {
			case "token":
				emit("token", map[string]string{"token": text})
			case "phase":
				emit("phase", map[string]string{"phase": streamPhase(text)})
			}
		})
	}
	if err != nil {
		emit("error", map[string]string{"error": redactError(err)})
		return
	}
	reply := stripQuoteTrim(res.text)
	history := append([]llm.Message(nil), hist.messages...)
	if !isContinue {
		history = append(history, llm.Message{Role: "user", Content: msg})
	}
	history = append(history, llm.Message{Role: "assistant", Content: reply})
	if err := saveWindHistory(s, wind, history); err != nil {
		emit("error", map[string]string{"error": "no s'ha pogut desar la conversa"})
		return
	}
	hist.messages = history
	s.startWindLearn(wind, msg, reply)
	emit("done", map[string]any{"reply": reply, "wind": wind.ID, "parcial": res.parcial, "history": petHistoryJSON(history), "actions": remindActionsSince(wind.ID, turnStart)})
}

// handleRotllanaChatStream és el torn en grup amb emissió en directe.
func (s *Server) handleRotllanaChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Rotllana string `json:"rotllana"`
		Message  string `json:"message"`
		Wind     string `json:"wind"`
		Continue bool   `json:"continue"` // reprèn l'última resposta parcial
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	rot, ok := s.rotllanaByID(strings.TrimSpace(req.Rotllana))
	if !ok {
		http.Error(w, "unknown rotllana", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(req.Message)
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock()
	isContinue := req.Continue && len(hist.messages) > 0
	if msg == "" && !isContinue {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	speakingWind, err := s.nextWindInHistory(rot, strings.TrimSpace(req.Wind), hist.messages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	conversation := append([]llm.Message(nil), hist.messages...)
	if !isContinue {
		conversation = append(conversation, llm.Message{Role: "user", Content: msg})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	emit := sse(w)
	emit("phase", map[string]string{"phase": "thinking"})
	system, brief := rotllanaChatBrief(s, rot, speakingWind, conversation, msg)
	turnStart := time.Now()
	emitTokens := func(kind, text string) {
		switch kind {
		case "token":
			emit("token", map[string]string{"token": text})
		case "phase":
			emit("phase", map[string]string{"phase": streamPhase(text)})
		}
	}
	parcial := false
	var res exploracio
	if isContinue {
		s.mu.Lock()
		p, role := s.windRoleRef()
		s.mu.Unlock()
		res, err = s.runReadOnlyLoopForRoleStream(ctx, "", 6, p, role, emitTokens, conversation)
		if err == nil {
			parcial = res.parcial
		}
	} else {
		res, err = rotllanaVoiceLoopStream(s, ctx, brief, emitTokens)
		if err == nil {
			parcial = res.parcial
		}
	}
	out := res.text
	if err != nil {
		// Com al clàssic: la cerca no pot tombar el torn (però sense tokens).
		p, role, ok2 := s.sideRoleRef()
		if !ok2 {
			emit("error", map[string]string{"error": "no side role available"})
			return
		}
		s.mu.Lock()
		cfg, client := s.cfg, s.client
		s.mu.Unlock()
		msgs := []llm.Message{{Role: "system", Content: system}}
		msgs = append(msgs, conversation...)
		if len(msgs) > 41 {
			msgs = append(msgs[:1], msgs[len(msgs)-40:]...)
		}
		plain, perr := petChatter(ctx, client, cfg, p, role, msgs)
		if perr != nil {
			emit("error", map[string]string{"error": redactError(perr)})
			return
		}
		out = plain
	}
	reply := stripQuoteTrim(out)
	voices := splitRotllanaVoices(reply, s.rotMembersOf(rot), speakingWind)
	history := conversation
	for _, v := range voices {
		history = append(history, llm.Message{Role: "assistant", Content: v.wind.Name + ": " + v.text, SpeakerID: v.wind.ID, SpeakerName: v.wind.Name})
	}
	if err := saveRotllanaHistory(s, rot.ID, history); err != nil {
		emit("error", map[string]string{"error": "no s'ha pogut desar la conversa"})
		return
	}
	hist.messages = history
	replyWind, replyText := speakingWind, reply
	if len(voices) > 0 {
		replyWind, replyText = voices[0].wind, voices[0].text
	}
	emit("done", map[string]any{
		"reply":   map[string]string{"wind": replyWind.ID, "text": replyText},
		"parcial": parcial,
		"history": s.rotllanaHistoryJSON(history),
		"actions": remindActionsSince(speakingWind.ID, turnStart),
	})
}
