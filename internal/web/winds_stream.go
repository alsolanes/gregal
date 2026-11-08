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
var windLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string), routed ...*routedRole) (exploracio, error) {
	p, role := s.windRoleRef()
	if len(routed) > 0 && routed[0] != nil {
		p, role = routed[0].p, routed[0].role
	}
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
var rotllanaVoiceLoopStream = func(s *Server, ctx context.Context, brief string, emit func(kind, text string), routed ...*routedRole) (exploracio, error) {
	p, role := s.windRoleRef()
	if len(routed) > 0 && routed[0] != nil {
		p, role = routed[0].p, routed[0].role
	}
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
	"\n4. DELIVER: keep private review or scoring tools local unless the user requests publication. For a site or game meant to be public call publish {dir, title} and give its link. Something that needs a server (an API, data saved on the server, a multiplayer game) → call app {action:start, name, dir, command}; it must listen on 127.0.0.1 at $PORT and use RELATIVE URLs (fetch('api/x'), not '/api/x'); test it and give the link. Never start servers with bash, nohup or a trailing &." +
	"\n5. Browser code: wrap localStorage in try/catch (published pages run sandboxed)." +
	"\n6. REPORT at the end (here you may use up to 8 lines): what you built, the folder, the link, and how you checked it. If something could not be done, say exactly what and why."

// windEstilBrief és com s'informa d'una feina feta (fitxers, ordres,
// publicar): el que la persona llegeix al mòbil. Sense això els vents
// tornaven taules de SHA, branques i comptes de línies. Com s'escriu una
// resposta normal ja ho diu el system del vent (companion.windAnswerGuide).
const windEstilBrief = "\n\nAFTER DOING WORK (files, commands, publishing): 2-5 short lines with what you did, where it is (the link), how you checked it and what is left. No diff counters, no branch names or commit SHAs unless they asked, no file listings. If something could not be done, say it in one plain line — never claim it as done."

// windChatSystemPrompt és el system d'un torn de xat directe: qui és el vent,
// com respon, què sap i què pot fer. Primer el que no canvia d'un torn a
// l'altre (la persona, les regles) i al final el que sí (hora, lliçons,
// carpeta, idioma), perquè els proveïdors en puguin reaprofitar el prefix.
func windChatSystemPrompt(s *Server, wind companion.Wind, msg string) string {
	ws, _ := s.ventWorkspace()
	system := companion.WindChatSystem(wind, s.personalMemory(), s.projectName(), s.windGoalTitles(), s.lastSessionDigest(), s.windUserName())
	return system + companion.WindBrainSystem(s.windBrain(wind.ID)) + s.windMemorySystem(wind.ID) + s.windSkillsSystem(wind.ID) + s.windCardsBrief(wind.ID) + s.windServerBrief() + s.windConnectionsBrief() +
		windDocsBrief + windCodeBrief + windEstilBrief + windRemindBrief(wind.ID) + windFollowupBrief(wind) + s.windLessonsBrief(wind.ID) + windProjectesBrief(ws) + companion.ReplyLanguageBrief(msg)
}

// windChatBrief és el mateix torn en un sol text (encàrrecs, fitxes i el
// registre dels tests); el xat directe envia windChatMessages.
func windChatBrief(s *Server, wind companion.Wind, previous []llm.Message, msg string) string {
	return windBriefFromSystem(windChatSystemPrompt(s, wind, msg), previous, msg)
}

func windBriefFromSystem(system string, previous []llm.Message, msg string) string {
	return system + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + agent.TranscriptDigest(previous, 9600) + "\n\nTask: the user asks:\n" + msg
}

// rotllanaChatBrief munta la consigna del torn en grup (clàssic i stream).
func rotllanaChatBrief(s *Server, rot Rotllana, speakingWind companion.Wind, conversation []llm.Message, msg string) (system, brief string) {
	wsp, _ := s.ventWorkspace()
	onTreballa := windProjectesBrief(wsp)
	system = companion.RotllanaSystem(speakingWind, s.rotllanaRoster(rot), s.personalMemory(), s.windUserName()) + companion.WindBrainSystem(s.windBrain(speakingWind.ID)) + s.windMemorySystem(speakingWind.ID) + s.windSkillsSystem(speakingWind.ID) + s.windCardsBrief(speakingWind.ID) + s.windServerBrief() + s.windConnectionsBrief()
	// Cada pas de cada veu torna a enviar la conversa: 10k caràcters recents basten per
	// seguir el fil. Amb el nom de qui parla: cada missatge d'assistant és una veu
	// diferent. El pressupost baixa a 9.400 perquè els noms i la consigna de
	// conversa han de continuar cabent al sostre de 20k del brief.
	transcript := agent.TranscriptDigestSpeakers(conversation, 9400)
	brief = system + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + transcript + "\n\nTask: the user asks:\n" + msg + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first (to read any URL use web_fetch, never curl: shell downloads are blocked); if the page needs JavaScript or fetch fails, open it with the browser tool (open/goto/read/screenshot only — look, never click or type) and read it there. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer — no search needed. Never invent facts. When a picture genuinely helps, find it with web_images and embed it as ![title](image-url) on its own line; never invent an image URL." +
		"\n\nSpeak only as yourself. For a requested deliverable, start working if you have what you need. If one missing choice would materially change it, ask that one question with concrete options." + windRemindBrief(speakingWind.ID) + windFollowupBrief(speakingWind) + windCodeBrief + s.windLessonsBrief(speakingWind.ID) + onTreballa + companion.ReplyLanguageBrief(msg)
	return system, brief
}

// windTurnTimeout és el temps d'un torn del xat directe d'un vent. Amb
// l'auto-continuació (plan.go) un torn pot encadenar tres rondes de passos:
// fer, provar i publicar un joc amb un model lent no hi cabia en 5 minuts
// («context deadline exceeded» a mitja feina).
var windTurnTimeout = 15 * time.Minute

// windTurnOutput keeps HTTP response framing out of the turn logic.
func windTurnOutput(w http.ResponseWriter, r *http.Request, stream bool) (emit func(string, any), fail func(int, string), closeStream func()) {
	if stream {
		emit, closeStream = windSSE(w, r.Context(), 15*time.Second)
		return emit, func(_ int, message string) { emit("error", map[string]string{"error": message}) }, closeStream
	}
	return func(event string, payload any) {
		if event == "done" || event == "error" {
			writeJSON(w, payload)
		}
	}, func(code int, message string) { http.Error(w, message, code) }, func() {}
}

// handleWindChatStream és el xat directe amb emissió en directe.
func (s *Server) handleWindChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	s.handleWindTurn(w, r, true)
}

func (s *Server) handleWindTurn(w http.ResponseWriter, r *http.Request, stream bool) {
	var req struct {
		Wind     string `json:"wind"`
		Session  string `json:"session"`
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
	hist, ok := s.requestedWindHistory(w, wind.ID, firstNonEmpty(req.Session, r.URL.Query().Get("session")))
	if !ok {
		return
	}
	emit, fail, stopStream := windTurnOutput(w, r, stream)
	defer stopStream()
	emit("activity", windActivity{Phase: "waiting"})
	// torn serialitza les escriptures d'aquest vent; mu queda lliure per
	// als GET (RLock), que mai no esperen el model.
	turn := windDirectTurn(s, wind.ID)
	turn.Lock()
	defer turn.Unlock()
	hist.mu.RLock()
	if hist.deleted && hist.name != "company-"+wind.ID {
		hist.mu.RUnlock()
		fail(http.StatusNotFound, "no such session")
		return
	}
	base := append([]llm.Message(nil), hist.messages...)
	gotGen := hist.gen
	hist.mu.RUnlock()
	previous := base
	if len(previous) > 40 {
		previous = previous[len(previous)-40:]
	}
	if msg == "" && !(req.Continue && len(previous) > 0) {
		fail(http.StatusBadRequest, "missatge buit")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), windTurnTimeout)
	defer cancel()
	ctx, doneTurn := s.beginWindTurn("wind", wind.ID, ctx)
	defer doneTurn()
	ctx = s.interactiveWindCtx(s.withWindSteerKey(withLinkBase(ctx, r), "wind", wind.ID))
	if s.crewChatIntent(msg) == "review" {
		ctx = s.readOnlyWindCtx(ctx)
	} else {
		ctx = s.windDocsCtx(ctx)
	}

	// «Aprèn a fer jocs web»: un estudi amb pràctica (winds_expert.go), no
	// un torn de xat.
	if tema, ok := learnRequest(msg); ok && !req.Continue {
		reply := s.startLearnFromChat(wind, tema)
		history := append(append([]llm.Message(nil), base...),
			llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()},
			llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix()})
		history, err := s.commitWindTurn(wind, hist, gotGen, history)
		if err != nil {
			fail(http.StatusInternalServerError, "no s'ha pogut desar la conversa")
			return
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
	ctx = tl.withUsage(ctx)
	var res exploracio
	var err error
	emitTokens := func(kind, text string) {
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
		default:
			forwardWindAsk(emit, kind, text)
		}
	}
	if isContinue {
		s.mu.Lock()
		p, role := s.windRoleRef()
		s.mu.Unlock()
		messages := windContinueHistory(s, wind, previous)
		if stream {
			res, err = s.runReadOnlyLoopForRoleStream(ctx, "", windTurnSteps, p, role, emitTokens, messages)
		} else {
			res, err = s.runReadOnlyLoopForRole(ctx, "", windTurnSteps, p, role, messages)
		}
	} else {
		system := windChatSystemPrompt(s, wind, msg)
		brief := windBriefFromSystem(system, previous, msg)
		ctx = withWindPrompt(agent.WithSkillsDirs(ctx, s.windSkillsDir(wind.ID)), windTurnMessages(system, previous, msg))
		var routed *routedRole
		if p, role, routeWhy := s.windRouteRole(msg); routeWhy != "" {
			log.Printf("vents router %s: %s", wind.ID, routeWhy)
			routed = &routedRole{p: p, role: role}
			tl.setRole(role)
		}
		if stream {
			res, err = windLoopStream(s, ctx, brief, emitTokens, routed)
		} else {
			res, err = windLoop(s, ctx, brief, routed)
		}
	}
	tl.finish(err, tornAturat(err), res.parcial, &res.temps)
	if err != nil {
		if tornAturat(err) {
			emit("error", map[string]any{"error": "aturat", "cancelat": true})
			return
		}
		fail(http.StatusBadGateway, redactError(err))
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
	history := append([]llm.Message(nil), base...)
	if !isContinue {
		history = append(history, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}
	// El que el vent ha escrit abans de les eines també és conversa: si només
	// es desa la resposta final, en recarregar el fil desapareix.
	for _, n := range tornTexte.narracions(reply) {
		history = append(history, llm.Message{Role: "assistant", Content: n, Ts: time.Now().Unix()})
	}
	history = append(history, llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix()})
	history, err = s.commitWindTurn(wind, hist, gotGen, history)
	if err != nil {
		fail(http.StatusInternalServerError, "no s'ha pogut desar la conversa")
		return
	}
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
	s.handleRotllanaTurn(w, r, true)
}

func (s *Server) handleRotllanaTurn(w http.ResponseWriter, r *http.Request, stream bool) {
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
	if wanted := strings.TrimSpace(req.Wind); wanted != "" {
		if _, err := s.nextWindInHistory(rot, wanted, nil); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	// La colla d'aquest fil espera resposta: el missatge la reprèn.
	if !req.Continue && msg != "" {
		switch s.crewTakeAnswer(rot, msg) {
		case crewAnswerConsumed:
			// La colla es reprèn: se'n segueix la feina en directe com quan comença.
			if v, ok := crewJobs.Load(crewRegistryKey(s, rot.ID)); ok {
				s.followCrewJob(w, r, rot, v.(*crewJob), len(s.crewHistory(rot)), stream)
				return
			}
			emit, _, closeStream := windTurnOutput(w, r, stream)
			defer closeStream()
			emit("done", map[string]any{"reply": map[string]string{"wind": "", "text": ""}, "silenci": true, "history": s.rotllanaHistoryJSON(s.crewHistory(rot))})
			return
		case crewAnswerFailed:
			errText := "could not resume the crew; the answer may not have been saved. Please retry."
			if s.cfg.Lang() == "ca" {
				errText = "No s'ha pogut reprendre la colla; potser la resposta no s'ha desat. Torna-ho a provar."
			}
			http.Error(w, errText, http.StatusInternalServerError)
			return
		}
	}
	if crewChatBusyFor(s, rot.ID) {
		http.Error(w, s.crewBusyMessage(), http.StatusConflict)
		return
	}
	intent := s.crewChatIntent(msg)
	if !req.Continue && strings.TrimSpace(req.Wind) == "" && intent == "work" {
		s.crewChat(w, r, rot, msg, stream)
		return
	}
	if msg == "" && !req.Continue {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	emit, fail, stopStream := windTurnOutput(w, r, stream)
	defer stopStream()
	emit("activity", windActivity{Phase: "waiting"})
	hist := rotllanaHistory(s, rot.ID)
	// torn serialitza les escriptures del fil; mu queda lliure per als GET.
	hist.torn.Lock()
	defer hist.torn.Unlock()
	hist.mu.RLock()
	base := append([]llm.Message(nil), hist.messages...)
	gotGen := hist.gen
	hist.mu.RUnlock()
	commitRotllana := func(next []llm.Message) []llm.Message {
		next = podaHistorialVent(next)
		hist.mu.Lock()
		if hist.gen == gotGen {
			hist.messages = next
			hist.gen++
			gotGen = hist.gen
		}
		out := append([]llm.Message(nil), hist.messages...)
		hist.mu.Unlock()
		return out
	}
	isContinue := req.Continue && len(base) > 0
	if msg == "" && !isContinue {
		fail(http.StatusBadRequest, "missatge buit")
		return
	}
	emit("activity", windActivity{Phase: "choosing"})
	// Tria local: membres nomenats (màxim dos) o participació equilibrada;
	// «continua» conserva l’última veu, sense una crida extra al model.
	tRoute := time.Now()
	speakers := s.rotllanaTurnSpeakers(context.Background(), rot, strings.TrimSpace(req.Wind), msg, base, isContinue)
	routeDur := time.Since(tRoute)
	var tempsVeus []string
	if len(speakers) == 0 && (strings.TrimSpace(req.Wind) != "" || isContinue) {
		speakingWind, err := s.nextWindInHistory(rot, strings.TrimSpace(req.Wind), base)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		speakers = []companion.Wind{speakingWind}
	}
	conversation := append([]llm.Message(nil), base...)
	if !isContinue {
		conversation = append(conversation, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}

	// Silenci: ningú hi té res a dir. Es desa el missatge i prou.
	if len(speakers) == 0 {
		if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
			fail(http.StatusInternalServerError, "no s'ha pogut desar la conversa")
			return
		}
		commitRotllana(conversation)
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
		default:
			forwardWindAsk(emit, kind, text)
		}
	}
	type veu struct {
		wind companion.Wind
		text string
	}
	var veus []veu
	members := s.rotMembersOf(rot)
	turnCtx, doneTurn := s.beginWindTurn("rotllana", rot.ID, context.Background())
	defer doneTurn()
	emit("phase", map[string]string{"phase": "thinking"})
	var failed []string
	var lastErr error
	parcial := false
	for si, sp := range speakers {
		if turnCtx.Err() != nil {
			break
		}
		// Cada veu té la seva narració: el text de l'anterior ja s'ha desat.
		tornTexte = &tornText{}
		// Nova veu, bombolla nova: l'app tanca l'anterior en veure-ho.
		emit("phase", map[string]string{"phase": "speaker:" + sp.ID})
		emit("activity", windActivity{Phase: "preparing"})
		system, brief := rotllanaChatBrief(s, rot, sp, conversation, msg)
		brief += windDocsBrief
		perspective := rotllanaPerspective(sp, si, len(speakers))
		brief += perspective
		system += perspective
		var res exploracio
		var err error
		ctx, cancel := context.WithTimeout(turnCtx, rotllanaTurnTimeout)
		ctx = s.interactiveWindCtx(s.withWindSteerKey(withLinkBase(ctx, r), "rotllana", rot.ID))
		if si > 0 || intent == "review" {
			ctx = s.readOnlyWindCtx(ctx)
		} else {
			ctx = s.windDocsCtx(ctx)
		}
		tVeu := time.Now()
		s.mu.Lock()
		_, logRole := s.windRoleRef()
		s.mu.Unlock()
		veuLog = startTurnLog("rotllana", rot.ID+":"+sp.ID, logRole)
		ctx = veuLog.withUsage(ctx)
		if isContinue && si == 0 {
			s.mu.Lock()
			p, role := s.windRoleRef()
			s.mu.Unlock()
			if stream {
				res, err = s.runReadOnlyLoopForRoleStream(ctx, "", 6, p, role, emitTokens, conversation)
			} else {
				res, err = s.runReadOnlyLoopForRole(ctx, "", 6, p, role, conversation)
			}
		} else {
			ctx = agent.WithSkillsDirs(ctx, s.windSkillsDir(sp.ID))
			if stream {
				res, err = rotllanaVoiceLoopStream(s, ctx, brief, emitTokens)
			} else {
				var routed *routedRole
				if p, role, why := s.windRouteRole(msg); why != "" {
					routed = &routedRole{p: p, role: role}
					veuLog.setRole(role)
				}
				res, err = rotllanaVoiceLoop(s, ctx, brief, routed)
			}
		}
		if err == nil {
			parcial = parcial || res.parcial
		}
		cancel()
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
			fctx, fcancel := context.WithTimeout(turnCtx, rotllanaFallbackTimeout)
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
		reply := rotllanaOwnReply(stripQuoteTrim(out), members, sp)
		if reply == "" {
			continue
		}
		reply += s.windDocLinks(tVeu, reply)
		conversation = append(conversation, llm.Message{Role: "assistant", Content: reply, SpeakerID: sp.ID, SpeakerName: sp.Name, Ts: time.Now().Unix()})
		veus = append(veus, veu{sp, reply})
		// Cada veu acabada es desa de seguida (amb el missatge de l'usuari, la
		// primera vegada): si una veu posterior falla, aquesta no es perd.
		if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
			fail(http.StatusInternalServerError, "no s'ha pogut desar la conversa")
			return
		}
		conversation = commitRotllana(conversation)
		tempsVeus = append(tempsVeus, fmt.Sprintf("%s %.1fs (%s)", sp.Name, time.Since(tVeu).Seconds(), res.temps))
		if stream {
			emit("reply", map[string]any{"wind": sp.ID, "history": s.rotllanaHistoryJSON(conversation)})
		}
	}
	var summaryReply string
	if len(veus) >= 2 && !parcial && lastErr == nil && !isContinue && rotllanaVolDebat(msg) {
		ctx, cancel := context.WithTimeout(turnCtx, rotllanaTurnTimeout)
		emit("phase", map[string]string{"phase": "speaker:" + speakingWind.ID})
		emit("activity", windActivity{Phase: "thinking"})
		if summary, err := s.rotllanaSummary(ctx, rot, speakingWind, conversation, msg); err != nil {
			lastErr = err
			failed = append(failed, speakingWind.Name)
		} else if summary != "" {
			final := append(conversation, llm.Message{Role: "assistant", Content: summary, SpeakerID: speakingWind.ID, SpeakerName: speakingWind.Name, Ts: time.Now().Unix()})
			if err := saveRotllanaHistory(s, rot.ID, final); err != nil {
				lastErr = err
				failed = append(failed, speakingWind.Name)
			} else {
				conversation = commitRotllana(final)
				summaryReply = summary
			}
		}
		cancel()
	}
	if turnCtx.Err() != nil {
		emit("error", map[string]any{"error": "aturat", "cancelat": true})
		return
	}
	log.Printf("vents rotllana %s: tria %.1fs · %s", rot.ID, routeDur.Seconds(), strings.Join(tempsVeus, " · "))
	if len(veus) == 0 {
		// Preserve the draft on failure. A deliberate silence is not an error.
		if lastErr != nil {
			fail(http.StatusBadGateway, redactError(lastErr))
		} else {
			emit("done", map[string]any{"reply": map[string]string{}, "silenci": true, "history": s.rotllanaHistoryJSON(base)})
		}
		return
	}
	if strings.TrimSpace(msg) != "" {
		for _, v := range veus {
			s.startWindMemoryLearn(v.wind, msg, v.text)
		}
	}
	emit("done", map[string]any{
		"reply":         map[string]string{"wind": veus[0].wind.ID, "text": firstNonEmpty(summaryReply, veus[0].text)},
		"parcial":       parcial,
		"history":       s.rotllanaHistoryJSON(conversation),
		"actions":       remindActionsSince(s.remindOwner(), speakingWind.ID, turnStart),
		"failed":        failed,
		"failed_reason": redactError(lastErr),
	})
}
