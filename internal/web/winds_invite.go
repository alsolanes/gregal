package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// «La Lluerna en sap més · Pregunta-li»: abans obria un fil nou amb ella i hi
// tornava a enviar la pregunta. La resposta de l'altre vent desapareixia de la
// vista (semblava esborrada) i la Lluerna començava de zero, sense saber què
// s'havia dit. Ara el vent convidat respon al mateix fil, a sota, amb el
// context de la conversa (com fa Gregal quan consulta un especialista).
//
// El seu missatge porta Kind «guest» i el seu nom: el vent amfitrió el llegeix
// com el que és, una aportació d'un altre vent, no com una cosa que va dir ell.

const guestKind = "guest"

// inviteContextMsgs: quants missatges recents del fil veu el convidat.
const inviteContextMsgs = 12

// handleWindInvite: POST {wind, session?, guest, question?} → el convidat
// respon al fil i torna {history}.
func (s *Server) handleWindInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind     string `json:"wind"`
		Session  string `json:"session"`
		Guest    string `json:"guest"`
		Question string `json:"question"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	host, ok := s.winds.ByID(strings.TrimSpace(req.Wind))
	if !ok {
		http.Error(w, "unknown wind", http.StatusBadRequest)
		return
	}
	guest, ok := s.winds.ByID(strings.TrimSpace(req.Guest))
	if !ok || guest.ID == host.ID {
		http.Error(w, "unknown guest", http.StatusBadRequest)
		return
	}
	hist, ok := s.requestedWindHistory(w, host.ID, strings.TrimSpace(req.Session))
	if !ok {
		return
	}
	hist.mu.RLock()
	recent := append([]llm.Message(nil), hist.messages...)
	hist.mu.RUnlock()
	if len(recent) > inviteContextMsgs {
		recent = recent[len(recent)-inviteContextMsgs:]
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		question = lastUserText(recent)
	}
	if question == "" {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), windTurnTimeout)
	defer cancel()
	reply, _, err := s.windAskOnceAs(ctx, guest, inviteBrief(host, recent, question))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	msg := llm.Message{Role: "assistant", Kind: guestKind, Content: reply, SpeakerID: guest.ID, SpeakerName: guest.Name,
		Ts: time.Now().Unix(), ID: newMessageID()}
	turn := windDirectTurn(s, host.ID)
	turn.Lock()
	err = s.appendWindThread(host, hist, msg)
	hist.mu.RLock()
	history := append([]llm.Message(nil), hist.messages...)
	hist.mu.RUnlock()
	turn.Unlock()
	if err != nil {
		http.Error(w, "no s'ha pogut desar la conversa", http.StatusInternalServerError)
		return
	}
	// Has tancat l'app mentre pensava: que soni igualment (winds_absent.go).
	s.noteReplyIfAway(r, host, replySource, msg.ID, reply)
	writeJSON(w, map[string]any{"reply": reply, "guest": guest.ID, "history": petHistoryJSON(history)})
}

// inviteBrief és el que rep el convidat: la conversa recent i la pregunta.
func inviteBrief(host companion.Wind, recent []llm.Message, question string) string {
	var b strings.Builder
	b.WriteString("[The user invited you into their conversation with " + host.Name + " because you know more about this. ")
	b.WriteString("Answer the user directly, in their language, with what you add from your trade and memory. ")
	b.WriteString("Do not repeat what " + host.Name + " already said; if you agree, say so in a few words and add what is missing.]\n\nCONVERSATION SO FAR:\n")
	for _, m := range recent {
		text := strings.TrimSpace(m.Content)
		if text == "" || (m.Role != "user" && m.Role != "assistant") || m.Kind == memoryKind {
			continue
		}
		if r := []rune(text); len(r) > 1500 {
			text = string(r[:1500]) + " […]"
		}
		who := "User"
		if m.Role == "assistant" {
			who = firstNonEmpty(m.SpeakerName, host.Name)
		}
		b.WriteString(who + ": " + text + "\n\n")
	}
	b.WriteString("THE USER ASKS YOU: " + question)
	return b.String()
}

func lastUserText(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && strings.TrimSpace(msgs[i].Content) != "" {
			return strings.TrimSpace(msgs[i].Content)
		}
	}
	return ""
}
