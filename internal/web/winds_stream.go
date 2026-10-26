package web

import (
	"context"
	"errors"
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
//   reply: {"wind":"id","history":[...]}     (rotllana: una veu acabada i ja desada)
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

// rotllanaTurnTimeout limita cada veu d'una rotllana, no la ronda sencera:
// una veu lenta no pot fer perdre les que ja han acabat. Els tests el redueixen.
var rotllanaTurnTimeout = 3 * time.Minute

// rotllanaFallbackTimeout dona al xat pla un termini propi quan la cerca d'una
// veu ja ha esgotat el seu.
var rotllanaFallbackTimeout = 90 * time.Second

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
	case strings.Contains(n, "web_search") || strings.Contains(n, "web_fetch") || strings.Contains(n, "browser"):
		return "searching"
	case strings.Contains(n, "remind"):
		return "scheduling"
	default:
		return "working"
	}
}

// windChatBrief munta la consigna del xat directe (compartida pel clàssic i l'stream).
func windChatBrief(s *Server, wind companion.Wind, previous []llm.Message, msg string) string {
	system := companion.WindChatSystem(wind, s.personalMemory(), s.projectName(), s.windGoalTitles(), s.lastSessionDigest(), s.usuari())
	// The previous-turn digest leaves room for the wind's own guidance (images, charts) inside the 20k brief.
	return system + companion.WindBrainSystem(s.windBrain(wind.ID)) + s.windMemorySystem(wind.ID) + s.windSkillsSystem(wind.ID) + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + agent.TranscriptDigest(previous, 15000) + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first; if the page needs JavaScript or fetch fails, open it with the browser tool (open/goto/read/screenshot only — look, never click or type) and read it there. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer. If they want to SEE something, use web_images and embed it as ![title](image-url) on its own line." + windRemindBrief(wind.ID)
}

// rotllanaChatBrief munta la consigna del torn en grup (clàssic i stream).
func rotllanaChatBrief(s *Server, rot Rotllana, speakingWind companion.Wind, conversation []llm.Message, msg string) (system, brief string) {
	system = companion.RotllanaSystem(speakingWind, s.rotllanaRoster(rot), s.personalMemory(), s.usuari()) + companion.WindBrainSystem(s.windBrain(speakingWind.ID)) + s.windMemorySystem(speakingWind.ID) + s.windSkillsSystem(speakingWind.ID)
	transcript := agent.TranscriptDigest(conversation, 16000)
	brief = system + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + transcript + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first; if the page needs JavaScript or fetch fails, open it with the browser tool (open/goto/read/screenshot only — look, never click or type) and read it there. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer — no search needed. Never invent facts. When a picture genuinely helps, find it with web_images and embed it as ![title](image-url) on its own line; never invent an image URL." + windRemindBrief(speakingWind.ID)
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
		res, err = windLoopStream(s, agent.WithSkillsDirs(ctx, s.windSkillsDir(wind.ID)), brief, func(kind, text string) {
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
	// Torn mort a mitges (proveïdor caigut després d'un pas bo): el
	// resum mecànic no és una resposta del vent. Una frase curta i
	// accionable, en la seva llengua, sense adreces internes.
	if err == nil && res.parcial && esRespostaMecanica(reply) {
		reply = fraseTornAturat(res.motiu, s.cfg.Lang())
	}
	history := append([]llm.Message(nil), hist.messages...)
	if !isContinue {
		history = append(history, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}
	history = append(history, llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix()})
	if err := saveWindHistory(s, wind, history); err != nil {
		emit("error", map[string]string{"error": "no s'ha pogut desar la conversa"})
		return
	}
	hist.messages = history
	s.startWindLearn(wind, msg, reply)
	s.startWindMemoryLearn(wind, msg, reply)
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
	// Qui parla: el router tria 1-3 vents (o cap); amb vent explícit o
	// «continua», el d'abans.
	rctx, rcancel := context.WithTimeout(context.Background(), 90*time.Second)
	speakers := s.routeRotllanaSpeakers(rctx, rot, strings.TrimSpace(req.Wind), msg, hist.messages)
	rcancel()
	if len(speakers) == 0 && (strings.TrimSpace(req.Wind) != "" || isContinue) {
		speakingWind, err := s.nextWindInHistory(rot, strings.TrimSpace(req.Wind), hist.messages)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		speakers = []companion.Wind{speakingWind}
	}
	conversation := append([]llm.Message(nil), hist.messages...)
	if !isContinue {
		conversation = append(conversation, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}

	emit := sse(w)
	emit("phase", map[string]string{"phase": "thinking"})
	// Silenci: ningú hi té res a dir. Es desa el missatge i prou.
	if len(speakers) == 0 {
		if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
			emit("error", map[string]string{"error": "no s'ha pogut desar la conversa"})
			return
		}
		hist.messages = conversation
		emit("done", map[string]any{
			"reply":   map[string]string{"wind": "", "text": ""},
			"silenci": true,
			"history": s.rotllanaHistoryJSON(conversation),
		})
		return
	}
	turnStart := time.Now()
	speakingWind := speakers[0]
	emitTokens := func(kind, text string) {
		switch kind {
		case "token":
			emit("token", map[string]string{"token": text})
		case "phase":
			emit("phase", map[string]string{"phase": streamPhase(text)})
		}
	}
	type veu struct {
		wind companion.Wind
		text string
	}
	var veus []veu
	var failed []string
	var lastErr error
	parcial := false
	for si, sp := range speakers {
		// Nova veu, bombolla nova: l'app tanca l'anterior en veure-ho.
		emit("phase", map[string]string{"phase": "speaker:" + sp.ID})
		system, brief := rotllanaChatBrief(s, rot, sp, conversation, msg)
		var res exploracio
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), rotllanaTurnTimeout)
		if isContinue && si == 0 {
			s.mu.Lock()
			p, role := s.windRoleRef()
			s.mu.Unlock()
			res, err = s.runReadOnlyLoopForRoleStream(ctx, "", 6, p, role, emitTokens, conversation)
			if err == nil {
				parcial = res.parcial
			}
		} else {
			res, err = rotllanaVoiceLoopStream(s, agent.WithSkillsDirs(ctx, s.windSkillsDir(sp.ID)), brief, emitTokens)
			if err == nil {
				parcial = res.parcial || parcial
			}
		}
		cancel()
		out := res.text
		// Un torn que mor a mitges no és una resposta del vent: cau al
		// xat pla com al clàssic (vegeu winds_rotllanes.go).
		if err == nil && res.parcial && esRespostaMecanica(out) {
			err = errors.New(res.motiu)
			out = ""
		}
		if err != nil {
			// Com al clàssic: la cerca no pot tombar el torn (però sense tokens).
			p, role, ok2 := s.sideRoleRef()
			if !ok2 {
				lastErr = errors.New("no side role available")
				failed = append(failed, sp.Name)
				continue
			}
			s.mu.Lock()
			cfg, client := s.cfg, s.client
			s.mu.Unlock()
			msgs := []llm.Message{{Role: "system", Content: system}}
			msgs = append(msgs, conversation...)
			if len(msgs) > 41 {
				msgs = append(msgs[:1], msgs[len(msgs)-40:]...)
			}
			fctx, fcancel := context.WithTimeout(context.Background(), rotllanaFallbackTimeout)
			plain, perr := petChatter(fctx, client, cfg, p, role, msgs)
			fcancel()
			if perr != nil {
				// Aquesta veu calla; les altres continuen i el que ja s'ha dit es manté.
				lastErr = perr
				failed = append(failed, sp.Name)
				continue
			}
			out = plain
		}
		reply := stripQuoteTrim(out)
		voices := splitRotllanaVoices(reply, s.rotMembersOf(rot), sp)
		if len(voices) == 0 {
			conversation = append(conversation, llm.Message{Role: "assistant", Content: sp.Name + ": " + reply, SpeakerID: sp.ID, SpeakerName: sp.Name, Ts: time.Now().Unix()})
			veus = append(veus, veu{sp, reply})
		}
		for _, v := range voices {
			conversation = append(conversation, llm.Message{Role: "assistant", Content: v.wind.Name + ": " + v.text, SpeakerID: v.wind.ID, SpeakerName: v.wind.Name, Ts: time.Now().Unix()})
			veus = append(veus, veu{v.wind, v.text})
		}
		// Cada veu acabada es desa de seguida (amb el missatge de l'usuari, la
		// primera vegada): si una veu posterior falla, aquesta no es perd.
		if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
			emit("error", map[string]string{"error": "no s'ha pogut desar la conversa"})
			return
		}
		hist.messages = conversation
		emit("reply", map[string]any{"wind": sp.ID, "history": s.rotllanaHistoryJSON(conversation)})
	}
	if len(veus) == 0 {
		// Ningú no ha pogut respondre: no es desa res i el client conserva l'esborrany.
		emit("error", map[string]string{"error": redactError(lastErr)})
		return
	}
	if strings.TrimSpace(msg) != "" {
		for _, v := range veus {
			s.startWindMemoryLearn(v.wind, msg, v.text)
		}
	}
	emit("done", map[string]any{
		"reply":         map[string]string{"wind": veus[0].wind.ID, "text": veus[0].text},
		"parcial":       parcial,
		"history":       s.rotllanaHistoryJSON(conversation),
		"actions":       remindActionsSince(speakingWind.ID, turnStart),
		"failed":        failed,
		"failed_reason": redactError(lastErr),
	})
}
