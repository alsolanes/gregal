package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
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
//   activity: {"phase":"waiting|choosing|preparing|model|thinking|responding|tool|tool_done|retry|fallback", "tool":"name"?, "step":N?}
//   ping: {} (transport keepalive, NOT progress)
// Els tokens especulatius d'un pas que acaba en eines es retiren: el
// client els descarta quan arriba una fase (poden ser JSON d'eina, no
// text per a l'usuari).

// windActivity is additive to the legacy phase/token protocol. Never put model
// reasoning, tool arguments/results, paths or conversation text in this DTO.
type windActivity struct {
	Phase string `json:"phase"`
	Tool  string `json:"tool,omitempty"`
	Step  int    `json:"step,omitempty"`
}

func emitWindActivity(emit func(string, any), text string) {
	var activity windActivity
	if json.Unmarshal([]byte(text), &activity) == nil {
		emit("activity", activity)
	}
}

// windSSE keeps a silent model/lock wait connected without inventing progress.
// Join the heartbeat goroutine before the handler returns: no ResponseWriter
// writes may outlive the request. Disconnect stops pings, not the durable turn.
func windSSE(w http.ResponseWriter, ctx context.Context, interval time.Duration) (func(string, any), func()) {
	emit := sse(w)
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				emit("ping", map[string]any{})
			}
		}
	}()
	var once sync.Once
	return emit, func() { once.Do(func() { cancel(); <-done }) }
}

// windLoopStream és la via de cerca amb emissió; els tests la substitueixen.
var windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRoleStream(ctx, brief, windTurnSteps, p, role, emit)
}

// windTurnSteps és el pressupost de passos de cada ronda d'un torn de vent:
// fer i provar un joc o una app no hi cap en 6 (crear, escriure, provar,
// corregir, publicar). Una pregunta curta s'acaba igualment en 1-2 passos.
var windTurnSteps = 12

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

// windCodeBrief orienta els vents quan se'ls demana feina de construir: una
// web, un joc, un programa. Són especialistes que acaben la feina: projecte
// propi, provar, publicar o servir, i dir on és i com obrir-ho.
const windCodeBrief = "\n\nBUILDING (a website, a game, a program, a script): you are a specialist who finishes the job, without asking for permission for normal work inside the project." +
	"\n1. PROJECT: create <projects folder>/<slug>/ and keep every file there, never in the Gregal repo root." +
	"\n2. BUILD with read/grep/glob/write/edit/patch, and run commands with bash inside the project (python, node, npm install, pip install, go build: all fine there). curl only against 127.0.0.1/localhost to test your own app; to read a web page use web_fetch; browser libraries from a CDN <script src> or installed inside the project." +
	"\n3. TEST before saying it works: a syntax check (node --check, python -m py_compile), run it, curl it." +
	"\n4. DELIVER: a static site or browser game → call publish {dir, title} and give the link it returns. Something that needs a server (an API, data saved on the server, a multiplayer game) → call app {action:start, name, dir, command}; it must listen on 127.0.0.1 at $PORT and use RELATIVE URLs (fetch('api/x'), not '/api/x'); test it and give the link. Never start servers with bash, nohup or a trailing &." +
	"\n5. Browser code: wrap localStorage in try/catch (published pages run sandboxed)." +
	"\n6. REPORT at the end (here you may use up to 8 lines): what you built, the folder, the link, and how you checked it. If something could not be done, say exactly what and why."

// windChatBrief munta la consigna del xat directe (compartida pel clàssic i l'stream).
// windEstilBrief és com s'escriu la resposta: el que la persona llegeix al
// mòbil. Sense això els vents tornaven taules de SHA, branques i comptes de
// línies, que és informació de feina i no una resposta.
const windEstilBrief = "\n\nHOW TO WRITE THE REPLY: 2-4 short lines of plain sentences, in the user's language: what you did, where it is, and what is left. No diff counters, no branch names or commit SHAs unless the user asked for them, no tables for a small thing, no jargon. If something could not be done, say it in one plain line — never claim it as done."

func windChatBrief(s *Server, wind companion.Wind, previous []llm.Message, msg string) string {
	ws, _ := s.ventWorkspace()
	brief := windProjectesBrief(ws)
	system := companion.WindChatSystem(wind, s.personalMemory(), s.projectName(), s.windGoalTitles(), s.lastSessionDigest(), s.windUserName())
	// The previous-turn digest leaves room for the wind's own guidance (images, charts, building, lessons) inside the 20k brief.
	return system + companion.WindBrainSystem(s.windBrain(wind.ID)) + s.windMemorySystem(wind.ID) + s.windSkillsSystem(wind.ID) + s.windCardsBrief(wind.ID) + s.windServerBrief() + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + agent.TranscriptDigest(previous, 12500) + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first (to read any URL use web_fetch, never curl: shell downloads are blocked); if the page needs JavaScript or fetch fails, open it with the browser tool (open/goto/read/screenshot only — look, never click or type) and read it there. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer. If they want to SEE something, use web_images and embed it as ![title](image-url) on its own line." + windRemindBrief(wind.ID) + windDocsBrief + windCodeBrief + s.windLessonsBrief(wind.ID) + brief + windEstilBrief
}

// rotllanaChatBrief munta la consigna del torn en grup (clàssic i stream).
func rotllanaChatBrief(s *Server, rot Rotllana, speakingWind companion.Wind, conversation []llm.Message, msg string) (system, brief string) {
	wsp, _ := s.ventWorkspace()
	onTreballa := windProjectesBrief(wsp)
	system = companion.RotllanaSystem(speakingWind, s.rotllanaRoster(rot), s.personalMemory(), s.windUserName()) + companion.WindBrainSystem(s.windBrain(speakingWind.ID)) + s.windMemorySystem(speakingWind.ID) + s.windSkillsSystem(speakingWind.ID) + s.windCardsBrief(speakingWind.ID) + s.windServerBrief()
	// Cada pas de cada veu torna a enviar la conversa: 10k caràcters recents basten per seguir el fil.
	transcript := agent.TranscriptDigest(conversation, 10000)
	brief = system + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + transcript + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first (to read any URL use web_fetch, never curl: shell downloads are blocked); if the page needs JavaScript or fetch fails, open it with the browser tool (open/goto/read/screenshot only — look, never click or type) and read it there. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer — no search needed. Never invent facts. When a picture genuinely helps, find it with web_images and embed it as ![title](image-url) on its own line; never invent an image URL." +
		// La colla respon sencera quan la pregunta és per a tots: si només
		// parla una veu, sembla que els altres no hi siguin.
		"\n\nWHO ANSWERS: this is a group chat with humans and winds. If the human talks to the whole group (nobody named), the GROUP answers: write one short block per member who has something real to add, each block starting with exactly \"<member name>: \" (their name as listed above). 1-3 sentences per member, no filler, never repeat what another member just said, and leave out whoever has nothing to add — a member that adds nothing stays silent instead of agreeing. If the human names one member (@Name or the name in the text), only that member answers and you do not write the name prefix." + windRemindBrief(speakingWind.ID) + windCodeBrief + s.windLessonsBrief(speakingWind.ID) + onTreballa
	return system, brief
}

// windTurnTimeout és el temps d'un torn del xat directe d'un vent. Amb
// l'auto-continuació (plan.go) un torn pot encadenar tres rondes de passos:
// fer, provar i publicar un joc amb un model lent no hi cabia en 5 minuts
// («context deadline exceeded» a mitja feina).
var windTurnTimeout = 15 * time.Minute

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
	if msg == "" && !req.Continue {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	emit, stopStream := windSSE(w, r.Context(), 15*time.Second)
	defer stopStream()
	emit("activity", windActivity{Phase: "waiting"})
	hist := windHistory(s, wind.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock()
	previous := append([]llm.Message(nil), hist.messages...)
	if len(previous) > 40 {
		previous = previous[len(previous)-40:]
	}
	if msg == "" && !(req.Continue && len(previous) > 0) {
		emit("error", map[string]string{"error": "missatge buit"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), windTurnTimeout)
	defer cancel()
	ctx, doneTurn := s.beginWindTurn("wind", wind.ID, ctx)
	defer doneTurn()
	ctx = s.windDocsCtx(s.interactiveWindCtx(withLinkBase(ctx, r)))

	// «Aprèn a fer jocs web»: un estudi amb pràctica (winds_expert.go), no
	// un torn de xat.
	if tema, ok := learnRequest(msg); ok && !req.Continue {
		reply := s.startLearnFromChat(wind, tema)
		history := append(append([]llm.Message(nil), hist.messages...),
			llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()},
			llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix()})
		if err := saveWindHistory(s, wind, history); err == nil {
			hist.messages = history
		}
		emit("done", map[string]any{"reply": reply, "wind": wind.ID, "parcial": false, "history": petHistoryJSON(history), "learning": tema})
		return
	}

	emit("phase", map[string]string{"phase": "thinking"})
	emit("activity", windActivity{Phase: "preparing"})
	tornTexte := &tornText{}
	turnStart := time.Now()
	isContinue := req.Continue && len(previous) > 0
	s.mu.Lock()
	_, logRole := s.windRoleRef()
	s.mu.Unlock()
	tl := startTurnLog("wind", wind.ID, logRole)
	var res exploracio
	var err error
	if isContinue {
		s.mu.Lock()
		p, role := s.windRoleRef()
		s.mu.Unlock()
		res, err = s.runReadOnlyLoopForRoleStream(ctx, "", windTurnSteps, p, role, func(kind, text string) {
			switch kind {
			case "activity":
				emitWindActivity(emit, text)
			case "token":
				tl.token()
				tornTexte.token(text)
				emit("token", map[string]string{"token": text})
			case "phase":
				tornTexte.fita()
				emit("phase", map[string]string{"phase": streamPhase(text)})
			}
		}, previous)
	} else {
		brief := windChatBrief(s, wind, previous, msg)
		res, err = windLoopStream(s, agent.WithSkillsDirs(ctx, s.windSkillsDir(wind.ID)), brief, func(kind, text string) {
			switch kind {
			case "activity":
				emitWindActivity(emit, text)
			case "token":
				tl.token()
				tornTexte.token(text)
				emit("token", map[string]string{"token": text})
			case "phase":
				tornTexte.fita()
				emit("phase", map[string]string{"phase": streamPhase(text)})
			}
		})
	}
	tl.finish(err, tornAturat(err), res.parcial, &res.temps)
	if err != nil {
		if tornAturat(err) {
			emit("error", map[string]any{"error": "aturat", "cancelat": true})
			return
		}
		emit("error", map[string]string{"error": redactError(err)})
		return
	}
	log.Printf("vents xat %s: %.1fs (%s)", wind.ID, time.Since(turnStart).Seconds(), res.temps)
	reply := stripQuoteTrim(res.text)
	// Torn mort a mitges (proveïdor caigut després d'un pas bo): el
	// resum mecànic no és una resposta del vent. Una frase curta i
	// accionable, en la seva llengua, sense adreces internes.
	if err == nil && res.parcial && esRespostaMecanica(reply) {
		reply = fraseTornAturat(res.motiu, s.cfg.Lang())
	}
	reply += s.windDocLinks(turnStart, reply)
	history := append([]llm.Message(nil), hist.messages...)
	if !isContinue {
		history = append(history, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}
	// El que el vent ha escrit abans de les eines també és conversa: si només
	// es desa la resposta final, en recarregar el fil desapareix.
	for _, n := range tornTexte.narracions(reply) {
		history = append(history, llm.Message{Role: "assistant", Content: n, Ts: time.Now().Unix()})
	}
	history = append(history, llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix()})
	if err := saveWindHistory(s, wind, history); err != nil {
		emit("error", map[string]string{"error": "no s'ha pogut desar la conversa"})
		return
	}
	hist.messages = history
	s.startWindLearn(wind, msg, reply)
	s.startWindMemoryLearn(wind, msg, reply)
	s.startWindLessons(wind, msg, reply, res.hist)
	emit("done", map[string]any{"reply": reply, "wind": wind.ID, "parcial": res.parcial, "history": petHistoryJSON(history), "actions": remindActionsSince(s.remindOwner(), wind.ID, turnStart)})
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
	// La colla d'aquest fil espera resposta: el missatge la reprèn.
	if !req.Continue && msg != "" && s.crewTakeAnswer(rot, msg) {
		sse(w)("done", map[string]any{"reply": map[string]string{"wind": "", "text": ""}, "silenci": true, "history": s.rotllanaHistoryJSON(s.crewHistory(rot))})
		return
	}
	if msg == "" && !req.Continue {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	if wanted := strings.TrimSpace(req.Wind); wanted != "" {
		if _, err := s.nextWindInHistory(rot, wanted, nil); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	emit, stopStream := windSSE(w, r.Context(), 15*time.Second)
	defer stopStream()
	emit("activity", windActivity{Phase: "waiting"})
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock()
	isContinue := req.Continue && len(hist.messages) > 0
	if msg == "" && !isContinue {
		emit("error", map[string]string{"error": "missatge buit"})
		return
	}
	emit("activity", windActivity{Phase: "choosing"})
	// Tria local: membres nomenats (màxim dos) o participació equilibrada;
	// «continua» conserva l’última veu, sense una crida extra al model.
	tRoute := time.Now()
	speakers := s.rotllanaTurnSpeakers(context.Background(), rot, strings.TrimSpace(req.Wind), msg, hist.messages, isContinue)
	routeDur := time.Since(tRoute)
	var tempsVeus []string
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
	var veuLog *turnLog // la veu que parla ara (un registre per veu)
	tornTexte := &tornText{}
	emitTokens := func(kind, text string) {
		switch kind {
		case "activity":
			emitWindActivity(emit, text)
		case "token":
			if veuLog != nil {
				veuLog.token()
			}
			tornTexte.token(text)
			emit("token", map[string]string{"token": text})
		case "phase":
			tornTexte.fita()
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
		// Cada veu té la seva narració: el text de l'anterior ja s'ha desat.
		tornTexte = &tornText{}
		// Nova veu, bombolla nova: l'app tanca l'anterior en veure-ho.
		emit("phase", map[string]string{"phase": "speaker:" + sp.ID})
		emit("activity", windActivity{Phase: "preparing"})
		system, brief := rotllanaChatBrief(s, rot, sp, conversation, msg)
		brief += windDocsBrief
		var res exploracio
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), rotllanaTurnTimeout)
		ctx, doneTurn := s.beginWindTurn("rotllana", rot.ID, ctx)
		ctx = s.windDocsCtx(s.interactiveWindCtx(withLinkBase(ctx, r)))
		tVeu := time.Now()
		s.mu.Lock()
		_, logRole := s.windRoleRef()
		s.mu.Unlock()
		veuLog = startTurnLog("rotllana", rot.ID+":"+sp.ID, logRole)
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
		doneTurn()
		veuLog.finish(err, tornAturat(err), res.parcial, &res.temps)
		out := ambNarracions(tornTexte.narracions(res.text), res.text)
		// Un torn que mor a mitges no és una resposta del vent: cau al
		// xat pla com al clàssic (vegeu winds_rotllanes.go).
		if err == nil && res.parcial && esRespostaMecanica(out) {
			err = errors.New(res.motiu)
			out = ""
		}
		if err != nil {
			if tornAturat(err) {
				// Aturada per la persona: no es fa el pla B ni parlen les altres veus.
				lastErr = err
				break
			}
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
			emit("activity", windActivity{Phase: "fallback"})
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
		reply += s.windDocLinks(tVeu, reply)
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
		tempsVeus = append(tempsVeus, fmt.Sprintf("%s %.1fs (%s)", sp.Name, time.Since(tVeu).Seconds(), res.temps))
		emit("reply", map[string]any{"wind": sp.ID, "history": s.rotllanaHistoryJSON(conversation)})
	}
	// Discussió entre ells: si hi ha hagut més d'una veu i el torn ha anat sencer,
	// la colla continua fins a entendre's o fins que tothom calla.
	if len(veus) >= 2 && !parcial && lastErr == nil && !isContinue && rotllanaVolDebat(msg) {
		var noves []llm.Message
		tDebat := time.Now()
		debateCtx, cancelDebate := context.WithTimeout(context.Background(), rotllanaTurnTimeout)
		debateCtx, doneDebate := s.beginWindTurn("rotllana", rot.ID, debateCtx)
		conversation, noves = s.rotllanaDebat(debateCtx, rot, s.rotMembersOf(rot), conversation, msg, emit)
		stopped := tornAturat(debateCtx.Err())
		doneDebate()
		cancelDebate()
		if stopped {
			hist.messages = conversation
			emit("error", map[string]any{"error": "aturat", "cancelat": true})
			return
		}
		if len(noves) > 0 {
			hist.messages = conversation
			tempsVeus = append(tempsVeus, fmt.Sprintf("debat %d veus %.1fs", len(noves), time.Since(tDebat).Seconds()))
		}
	}
	log.Printf("vents rotllana %s: tria %.1fs · %s", rot.ID, routeDur.Seconds(), strings.Join(tempsVeus, " · "))
	if len(veus) == 0 {
		// Ningú no ha pogut respondre: no es desa res i el client conserva l'esborrany.
		if tornAturat(lastErr) {
			emit("error", map[string]any{"error": "aturat", "cancelat": true})
			return
		}
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
		"actions":       remindActionsSince(s.remindOwner(), speakingWind.ID, turnStart),
		"failed":        failed,
		"failed_reason": redactError(lastErr),
	})
}
