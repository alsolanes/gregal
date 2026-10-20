package web

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Xat amb mascota: conversa llança amb la veu de cada mascota.
//
//	POST /api/companion/chat {pet,message} → {reply,pet}
//
// Historial propi per mascota i usuari (en memòria: és xat de company, no
// sessions de feina). La persona de la mascota + la memòria d'usuari fan el
// system; l'ancoratge de «mai inventar» ve del PetCheckinSystem? No: aquí es
// pot xerrar lliurement — és conversa, no check-in. Però sense dades reals:
// la persona ho diu clar («no tinc accés a dades, xerra amb mi»).

// petHistoryEntry conserva l'historial en memòria i el desa al magatzem
// durador de converses de l'usuari després de cada resposta.
type petHistoryEntry struct {
	mu       sync.Mutex
	loadOnce sync.Once
	messages []llm.Message
}

var petHistories sync.Map // conversationHistoryKey -> *petHistoryEntry

func petHistory(s *Server, petID string) *petHistoryEntry {
	h, _ := petHistories.LoadOrStore(historyKey(s, "pet", petID), &petHistoryEntry{})
	entry := h.(*petHistoryEntry)
	entry.loadOnce.Do(func() {
		if saved, err := session.Load(s.sessDir(), "company-"+petID); err == nil {
			entry.messages = append([]llm.Message(nil), saved.Convo...)
		}
	})
	return entry
}

func savePetHistory(s *Server, pet companion.Pet, history []llm.Message) error {
	_, err := session.SaveSession(s.sessDir(), session.Session{
		Name:  "company-" + pet.ID,
		Title: "Company — " + pet.Name,
		Role:  "chat",
		Convo: append([]llm.Message(nil), history...),
	})
	return err
}

func petHistoryJSON(history []llm.Message) []map[string]string {
	out := make([]map[string]string, 0, len(history))
	for _, m := range history {
		from := "pet"
		if m.Role == "user" {
			from = "user"
		}
		out = append(out, map[string]string{"from": from, "text": m.Content})
	}
	return out
}

// petChatter és la via de xat amb mascota; els tests la substitueixen.
var petChatter = defaultPetChat

func defaultPetChat(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
	return agent.SideCallLateral(ctx, client, cfg, p, role, msgs)
}

// petChatSystem compon el system del xat amb mascota: persona + com es
// comporta en conversa (curt, càlid, sense inventar dades externes).
func petChatSystem(pet companion.Pet, memory string) string {
	sys := pet.Persona + `
CONVERSATION MODE. You are chatting with your human. Rules:
- Short replies (1-4 sentences), warm, in their language (Catalan if unsure).
- Stay in character as ` + pet.Name + ` at all times.
- You have NO access to external data (no news, no system status, no metrics). If you would need it, say so playfully and ask them instead.
- Never invent facts about the user or the world. What you know about the user comes from the memory below; anything else, ask.
- Keep it fun: personality over information.`
	if strings.TrimSpace(memory) == "" {
		memory = "(still empty: get to know them by asking)"
	}
	sys += "\n\nWHAT YOU KNOW ABOUT THEM:\n" + memory
	return sys
}

// stripQuoteTrim neteja la resposta del model: cometes d'eco i espais.
func stripQuoteTrim(s string) string {
	s = strings.TrimSpace(s)
	pairs := map[rune]rune{'"': '"', '«': '»', '\'': '\'', '`': '`'}
	for {
		first := []rune(s)[0:1]
		o, isOpen := pairs[first[0]]
		if !isOpen || len(s) < 2 {
			return strings.TrimSpace(s)
		}
		last, _ := utf8.DecodeLastRuneInString(s)
		if last != o {
			return strings.TrimSpace(s)
		}
		s = strings.TrimSpace(s[len(string(first[0])) : len(s)-len(string(last))])
	}
}

// handleCompanionPetChat carrega o l'historial durador o xateja amb una mascota.
func (s *Server) handleCompanionPetChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pet, ok := companion.PetByID(strings.TrimSpace(r.URL.Query().Get("pet")))
		if !ok {
			http.Error(w, "unknown pet", http.StatusBadRequest)
			return
		}
		hist := petHistory(s, pet.ID)
		hist.mu.Lock()
		messages := append([]llm.Message(nil), hist.messages...)
		hist.mu.Unlock()
		writeJSON(w, map[string]any{"pet": pet.ID, "history": petHistoryJSON(messages)})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Pet     string `json:"pet"`
		Message string `json:"message"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	pet, ok := companion.PetByID(strings.TrimSpace(req.Pet))
	if !ok {
		http.Error(w, "unknown pet", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	p, role, ok2 := s.sideRoleRef()
	if !ok2 {
		http.Error(w, "no side role available", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()

	hist := petHistory(s, pet.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock() // the entire turn is a conversation transaction
	conversation := append(append([]llm.Message(nil), hist.messages...), llm.Message{Role: "user", Content: msg})

	memory := s.personalMemory()
	msgs := []llm.Message{{Role: "system", Content: petChatSystem(pet, memory)}}
	msgs = append(msgs, conversation...)
	// Compte: si l'historial creix molt, talla els últims 16 torns.
	if len(msgs) > 33 {
		msgs = append(msgs[:1], msgs[len(msgs)-32:]...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := petChatter(ctx, client, cfg, p, role, msgs)
	if err != nil {
		http.Error(w, redactError(err), http.StatusBadGateway)
		return
	}
	reply := stripQuoteTrim(out)
	history := append(conversation, llm.Message{Role: "assistant", Content: reply})
	if err := savePetHistory(s, pet, history); err != nil {
		http.Error(w, "no s'ha pogut desar la conversa", http.StatusInternalServerError)
		return
	}
	hist.messages = history
	writeJSON(w, map[string]any{"reply": reply, "pet": pet.ID, "history": petHistoryJSON(history)})
}
