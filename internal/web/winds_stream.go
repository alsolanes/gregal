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
	"gregal/internal/tools"
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
		defer recoverGo("winds_stream")
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

// rotllanaAskTimeout: el termini de la veu que pot demanar permís (la primera,
// quan el grup fa feina): prou per contestar des del mòbil, com windAskMax.
var rotllanaAskTimeout = windAskMax

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
	case strings.Contains(n, "remind") || strings.Contains(n, "routine"):
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
	"\n3. TEST before saying it works: a syntax check (node --check, python -m py_compile), run it, curl it; to see a page use browser open/read/screenshot, never eval (that waits for the user)." +
	"\n4. DELIVER: keep private review or scoring tools local unless the user requests publication. For a site or game meant to be public call publish {dir, title} and give its link. Something that needs a server (an API, data saved on the server, a multiplayer game) → call app {action:start, name, dir, command}; it must listen on 127.0.0.1 at $PORT and use RELATIVE URLs (fetch('api/x'), not '/api/x'); test it and give the link. Never start servers with bash, nohup or a trailing &." +
	"\n5. Browser code: wrap localStorage in try/catch (published pages run sandboxed)." +
	"\n6. IN STAGES, so they see it grow and can steer: for a new site, game or app of some size, this turn builds a FIRST WORKING VERSION (structure, look, the main thing working), tests it and STOPS — the app shows the live preview in the chat. Say in 1-2 lines what is there, then end with ONE ```choices block of 2-3 concrete next steps (e.g. «Endavant: afegeix el formulari de reserves», «Canvia els colors a més càlids»). Each later turn does the next stage the same way, on the same project, and publishes only when they say it is ready. If they asked for it all at once («fes-ho tot», «no cal que preguntis»), or it is a small change or fix, just do it."

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
	// Primer el que gairebé no canvia (instruccions, poders, connexions) i després el que
	// canvia amb l'ús (calaix, equip, cervell, memòria, lliçons): un servidor local només
	// reaprofita el prefix fins al primer canvi.
	return system + windTradeBrief(wind) + s.windCardsBrief(wind.ID) + s.windServerBrief() + s.windConnectionsBrief() +
		windDocsBrief + windCodeBrief + windEstilBrief + windRemindBrief(wind.ID) + windFollowupBrief(wind) + windNowBrief() +
		s.gregalTeamBrief(wind) + s.windCalaixBrief(wind) + companion.WindBrainSystem(s.windBrain(wind.ID)) + s.windMemorySystem(wind.ID) + s.windSkillsSystem(wind.ID) +
		s.windLessonsBrief(wind.ID) + windProjectesBrief(ws) + s.windRulesBrief() + companion.ReplyLanguageBrief(msg)
}

// gregalTeamBrief diu a Gregal, a cada torn, qui té a l'equip i de què en sap:
// sense això només ho podia saber cridant «wind list», no ho feia, i responia
// sol mentre la interfície proposava «Llevantada en sap més de cuina ·
// Pregunta-li» (la persona havia de canviar de conversa). És el patró del
// «cap de gabinet»: Gregal és l'amo del resultat i consulta en segon pla.
func (s *Server) gregalTeamBrief(wind companion.Wind) string {
	if wind.ID != gregalWindID || s.winds == nil {
		return ""
	}
	all, err := s.winds.List()
	if err != nil {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, w := range all {
		if w.ID == gregalWindID || strings.TrimSpace(w.Role) == "" {
			continue
		}
		if n == 0 {
			b.WriteString("\n\nYOUR TEAM (specialist winds you can consult with the wind tool, action ask; they do not see this chat, so give them the full context):\n")
		}
		b.WriteString("- " + w.Name + " (id " + w.ID + "): " + strings.TrimSpace(w.Role) + "\n")
		if n++; n == 15 {
			break
		}
	}
	if n == 0 {
		return ""
	}
	b.WriteString("When the request is clearly in one of these trades (a recipe for the cook, a trip for the traveller…), ask that wind first, " +
		"then answer the user yourself with its help and name it in one short line (\"Llevantada helped with the recipes\"). " +
		"Do not send the user to talk to it instead: you own the result.\n")
	return b.String()
}

// windChatBrief és el mateix torn en un sol text (encàrrecs, fitxes i el
// registre dels tests); el xat directe envia windChatMessages.
func windChatBrief(s *Server, wind companion.Wind, previous []llm.Message, msg string) string {
	return windBriefFromSystem(windChatSystemPrompt(s, wind, msg), previous, msg)
}

func windBriefFromSystem(system string, previous []llm.Message, msg string) string {
	return system + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + agent.TranscriptDigest(previous, 9600) + "\n\nTask: the user asks:\n" + msg + windClockTag(time.Now())
}

// rotllanaChatBrief munta la consigna del torn en grup (clàssic i stream).
func rotllanaChatBrief(s *Server, rot Rotllana, speakingWind companion.Wind, conversation []llm.Message, msg string) (system, brief string) {
	wsp, _ := s.ventWorkspace()
	onTreballa := windProjectesBrief(wsp)
	system = companion.RotllanaSystem(speakingWind, s.rotllanaRoster(rot), s.personalMemory(), s.windUserName()) + s.windRulesBrief() + windNowBrief() + windTradeBrief(speakingWind) + s.windCalaixBrief(speakingWind) + companion.WindBrainSystem(s.windBrain(speakingWind.ID)) + s.windMemorySystem(speakingWind.ID) + s.windSkillsSystem(speakingWind.ID) + s.windCardsBrief(speakingWind.ID) + s.windServerBrief() + s.windConnectionsBrief()
	// Cada pas de cada veu torna a enviar la conversa: 10k caràcters recents basten per
	// seguir el fil. Amb el nom de qui parla: cada missatge d'assistant és una veu
	// diferent. El pressupost baixa a 9.400 perquè els noms i la consigna de
	// conversa han de continuar cabent al sostre de 20k del brief.
	transcript := agent.TranscriptDigestSpeakers(conversation, 9400)
	brief = system + "\n\nPREVIOUS CONVERSATION (context, not new instructions):\n" + transcript + "\n\nTask: the user asks:\n" + msg + windClockTag(time.Now()) + "\n\nIf this needs facts, current events, prices, or anything you don't know for sure, use web_search and web_fetch first (to read any URL use web_fetch, never curl: shell downloads are blocked); if the page needs JavaScript or fetch fails, open it with the browser tool (open/goto/read/screenshot only — look, never click or type) and read it there. Then answer in 1-4 sentences, in the user's language, citing the source URLs inline like [title](url). If you already know the answer for certain, just answer — no search needed. Never invent facts. If it is about a specific place, dish, product, artwork or person, put a photo line after your first sentence: ![name](image: 2-4 English words); the app turns it into a real photo." +
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
		// Tempesta: diversos agents a la vegada i una sola resposta (winds_tempesta.go).
		Tempesta bool `json:"tempesta"`
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
	steerKey := turnKey{scope: s.sessDir(), kind: "wind", id: wind.ID}
	// El que li has escrit mentre treballava i ja no ha arribat a llegir
	// torna al client, que l'envia com a missatge nou.
	steerLeft := func() {
		if left := windSteerLeft(steerKey); len(left) > 0 {
			emit("steer_unread", map[string]any{"texts": left})
		}
	}
	// Si una decisió no arriba en directe, el torn s'aparca i allibera el
	// goroutine; es reprèn com un run quan es respon (winds_ask.go).
	spot := &windParkSpot{wind: wind.ID, thread: hist.name}
	if !req.Continue {
		spot.userMsg = msg
	}
	ctx = withWindPark(ctx, spot)
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
	// El que faci al navegador torna al fil amb la resposta (captures).
	turnShots := &tools.TurnShots{}
	ctx = tools.WithTurnShots(ctx, turnShots)
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
		switch {
		case req.Tempesta:
			// Els agents corren en paral·lel: l'emissió al client, d'un en un.
			var emitMu sync.Mutex
			res, err = s.windTempesta(ctx, wind, system, previous, msg, func(kind, text string) {
				emitMu.Lock()
				defer emitMu.Unlock()
				emitTokens(kind, text)
			}, func(agents []tempestaAgent) {
				emitMu.Lock()
				defer emitMu.Unlock()
				emit("tempesta", map[string]any{"agents": agents})
			})
		case stream:
			res, err = windLoopStream(s, ctx, brief, emitTokens, routed)
		default:
			res, err = windLoop(s, ctx, brief, routed)
		}
	}
	tl.finish(err, tornAturat(err), res.parcial, &res.temps)
	if parked := spot.parkedRec(); parked != nil && errors.Is(err, errWindParked) {
		// Aparcat: al fil hi queda el missatge i l'avís d'espera; la resposta
		// continuarà el torn com un run (parked.RunID).
		reply := s.windWaitingText(parked)
		msgID := newMessageID()
		history := append([]llm.Message(nil), base...)
		if !isContinue {
			history = append(history, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
		}
		for _, n := range tornTexte.narracions(reply) {
			history = append(history, llm.Message{Role: "assistant", Content: n, Ts: time.Now().Unix()})
		}
		history = append(history, windSteerTaken(steerKey)...)
		history = append(history, llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix(), ID: msgID, Shots: s.turnShotsFor(turnShots)})
		history, err = s.commitWindTurn(wind, hist, gotGen, history)
		if err != nil {
			fail(http.StatusInternalServerError, "no s'ha pogut desar la conversa")
			return
		}
		s.noteParkedDecision(parked.ID, msgID)
		steerLeft()
		emit("done", map[string]any{"reply": reply, "wind": wind.ID, "parcial": false, "history": petHistoryJSON(history),
			"waiting": map[string]any{"id": parked.ID, "runId": parked.RunID, "messageId": msgID}})
		return
	}
	if err != nil {
		if tornAturat(err) {
			emit("error", map[string]any{"error": "aturat", "cancelat": true})
			return
		}
		steerLeft()
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
	// La resposta tal com l'ha escrit el vent: la narració es compara amb
	// aquesta. Després de posar-hi fotos o targetes el text ja no és igual, i
	// la resposta sencera es desava dues vegades (una sense foto i una amb).
	written := reply
	reply = s.windSubjectPhoto(msg, s.windCardImages(reply))
	reply += s.windDocLinks(turnStart, reply)
	reply += s.windWebCards(turnStart, reply)
	history := append([]llm.Message(nil), base...)
	if !isContinue {
		history = append(history, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}
	// El que el vent ha escrit abans de les eines també és conversa: si només
	// es desa la resposta final, en recarregar el fil desapareix.
	for _, n := range tornTexte.narracions(written) {
		history = append(history, llm.Message{Role: "assistant", Content: n, Ts: time.Now().Unix()})
	}
	// El que has escrit mentre treballava, abans de la resposta que ja ho té en compte.
	history = append(history, windSteerTaken(steerKey)...)
	replyID := newMessageID()
	history = append(history, llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix(), ID: replyID, Shots: s.turnShotsFor(turnShots)})
	history, err = s.commitWindTurn(wind, hist, gotGen, history)
	if err != nil {
		fail(http.StatusInternalServerError, "no s'ha pogut desar la conversa")
		return
	}
	// Has tancat l'app mentre treballava: que soni igualment (winds_absent.go).
	s.noteReplyIfAway(r, wind, replySource, replyID, reply)
	s.startWindLearn(wind, msg, reply)
	s.startWindMemoryLearnInThread(wind, msg, reply, hist)
	s.startWindLessons(wind, msg, reply, res.hist)
	steerLeft()
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
		id   string
	}
	var veus []veu
	members := s.rotMembersOf(rot)
	turnCtx, doneTurn := s.beginWindTurn("rotllana", rot.ID, context.Background())
	defer doneTurn()
	rotSteerKey := turnKey{scope: s.sessDir(), kind: "rotllana", id: rot.ID}
	rotSteerLeft := func() {
		if left := windSteerLeft(rotSteerKey); len(left) > 0 {
			emit("steer_unread", map[string]any{"texts": left})
		}
	}
	emit("phase", map[string]string{"phase": "thinking"})
	var failed []string
	var lastErr error
	parcial := false
	var summaryReply, summaryID string
	// Un grup amb diverses veus treballa com la Tempesta (winds_tempesta.go):
	// totes alhora, cadascuna des de la seva especialitat i amb la conversa
	// sencera; les veus queden plegades («debate») i el que porta el fil en
	// fa SEMPRE una resposta conjunta. Abans parlaven d'una en una, la
	// primera proposava i les altres només podien afegir «un risc» o callar,
	// i la resposta conjunta només sortia si deies «debateu».
	parallelVoices := len(speakers) >= 2 && !isContinue
	if parallelVoices {
		agents := make([]tempestaAgent, len(speakers))
		for i, sp := range speakers {
			agents[i] = tempestaAgent{Index: i, Angle: sp.ID, Label: sp.Name, Ca: sp.Name, En: sp.Name, State: "working"}
		}
		var amu sync.Mutex
		var emitMu sync.Mutex
		emitAgents := func() {
			amu.Lock()
			snap := append([]tempestaAgent(nil), agents...)
			amu.Unlock()
			emitMu.Lock()
			emit("tempesta", map[string]any{"agents": snap})
			emitMu.Unlock()
		}
		emitAgents()
		type veuRes struct {
			text string
			res  exploracio
			err  error
			at   time.Time
		}
		results := make([]veuRes, len(speakers))
		var wg sync.WaitGroup
		for si, sp := range speakers {
			wg.Add(1)
			go func(si int, sp companion.Wind) {
				defer wg.Done()
				defer recoverGo("rotllana:veu")
				tVeu := time.Now()
				system, brief := rotllanaChatBrief(s, rot, sp, conversation, msg)
				brief += windDocsBrief + "\n\nNow only YOU, " + sp.Name + ", speak, from your own specialty. The other members answer the same message in parallel and the lead will merge the answers, so give your own concrete view: the facts, the proposal or the risk you see, with sources. Do not ask the user questions and do not speak for the others."
				ctx, cancel := context.WithTimeout(turnCtx, rotllanaTurnTimeout)
				defer cancel()
				ctx = s.readOnlyWindCtx(s.interactiveWindCtx(s.withWindSteerKey(withLinkBase(ctx, r), "rotllana", rot.ID)))
				ctx = agent.WithSkillsDirs(ctx, s.windSkillsDir(sp.ID))
				s.mu.Lock()
				_, logRole := s.windRoleRef()
				s.mu.Unlock()
				vl := startTurnLog("rotllana", rot.ID+":"+sp.ID, logRole)
				// En paral·lel cap veu no escriu a la bombolla: els tokens es descarten
				// (es veu l'estat de cada veu i, al final, la resposta conjunta).
				var res exploracio
				var err error
				if stream {
					res, err = rotllanaVoiceLoopStream(s, vl.withUsage(ctx), brief, func(kind, text string) {
						if kind != "token" && kind != "phase" && kind != "activity" {
							emitMu.Lock()
							forwardWindAsk(emit, kind, text)
							emitMu.Unlock()
						}
					})
				} else {
					res, err = rotllanaVoiceLoop(s, vl.withUsage(ctx), brief, nil)
				}
				vl.finish(err, tornAturat(err), res.parcial, &res.temps)
				out := ""
				if err == nil && !(res.parcial && esRespostaMecanica(res.text)) {
					out = rotllanaOwnReply(stripQuoteTrim(res.text), members, sp)
				} else if err == nil || !tornAturat(err) {
					// Com al camí d'una en una: si la cerca falla, la veu respon sense
					// eines, amb un termini propi (la conversa ja hi és).
					if p, role, ok := s.sideRoleRef(); ok {
						s.mu.Lock()
						cfg, client := s.cfg, s.client
						s.mu.Unlock()
						msgs := []llm.Message{{Role: "system", Content: system}}
						msgs = append(msgs, conversation...)
						if len(msgs) > 41 {
							msgs = append(msgs[:1], msgs[len(msgs)-40:]...)
						}
						fctx, fcancel := context.WithTimeout(turnCtx, rotllanaFallbackTimeout)
						plain, perr := petChatter(fctx, client, cfg, p, role, msgs)
						fcancel()
						if perr == nil {
							out, err = rotllanaOwnReply(stripQuoteTrim(plain), members, sp), nil
						} else {
							err = perr
						}
					}
				}
				results[si] = veuRes{text: out, res: res, err: err, at: tVeu}
				amu.Lock()
				if out != "" {
					agents[si].State = "done"
				} else {
					agents[si].State = "failed"
				}
				amu.Unlock()
				emitAgents()
			}(si, sp)
		}
		wg.Wait()
		for si, sp := range speakers {
			r0 := results[si]
			if r0.err != nil && tornAturat(r0.err) {
				lastErr = r0.err
				continue
			}
			if r0.text == "" {
				if r0.err != nil {
					lastErr = r0.err
				}
				failed = append(failed, sp.Name)
				continue
			}
			parcial = parcial || r0.res.parcial
			reply := s.windCardImages(r0.text)
			reply += s.windDocLinks(r0.at, reply)
			voiceID := newMessageID()
			conversation = append(conversation, llm.Message{Role: "assistant", Content: reply, SpeakerID: sp.ID, SpeakerName: sp.Name, Ts: time.Now().Unix(), ID: voiceID, Kind: "debate"})
			veus = append(veus, veu{sp, reply, voiceID})
			tempsVeus = append(tempsVeus, fmt.Sprintf("%s %.1fs (%s)", sp.Name, time.Since(r0.at).Seconds(), r0.res.temps))
		}
		conversation = append(conversation, windSteerTaken(rotSteerKey)...)
		if len(veus) > 0 {
			if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
				fail(http.StatusInternalServerError, "no s'ha pogut desar la conversa")
				return
			}
			conversation = commitRotllana(conversation)
		}
		// La resposta conjunta, sempre que hi hagi més d'una veu. Si falla, les
		// veus deixen d'estar plegades: millor llegir-les que no veure res.
		unfold := func() {
			for i := range conversation {
				if conversation[i].Kind == "debate" && conversation[i].Ts >= turnStart.Unix() {
					conversation[i].Kind = ""
				}
			}
			if err := saveRotllanaHistory(s, rot.ID, conversation); err == nil {
				conversation = commitRotllana(conversation)
			}
		}
		switch {
		case len(veus) == 1:
			unfold()
		case len(veus) >= 2 && turnCtx.Err() == nil:
			ctx, cancel := context.WithTimeout(turnCtx, rotllanaTurnTimeout)
			emit("phase", map[string]string{"phase": "speaker:" + speakingWind.ID})
			emit("phase", map[string]string{"phase": "writing"})
			summary, err := s.rotllanaSummary(ctx, rot, speakingWind, conversation, msg)
			cancel()
			if err != nil || strings.TrimSpace(summary) == "" {
				if err != nil {
					log.Printf("vents rotllana %s: la resposta conjunta ha fallat: %v", rot.ID, err)
				}
				unfold()
			} else {
				summary = s.windCardImages(summary)
				summaryID = newMessageID()
				final := append(conversation, llm.Message{Role: "assistant", Content: summary, SpeakerID: speakingWind.ID, SpeakerName: speakingWind.Name, Ts: time.Now().Unix(), ID: summaryID})
				if err := saveRotllanaHistory(s, rot.ID, final); err != nil {
					unfold()
				} else {
					conversation = commitRotllana(final)
					summaryReply = summary
				}
			}
		}
	}
	for si, sp := range speakers {
		if parallelVoices {
			break
		}
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
		// La primera veu és la que pot fer canvis i, per tant, demanar permís:
		// amb el termini de 3 minuts, un permís que contestaves des del mòbil
		// al cap de més estona arribava tard i la veu es perdia.
		limit := rotllanaTurnTimeout
		if si == 0 && intent != "review" && rotllanaAskTimeout > limit {
			limit = rotllanaAskTimeout
		}
		ctx, cancel := context.WithTimeout(turnCtx, limit)
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
		reply = s.windCardImages(reply)
		reply += s.windDocLinks(tVeu, reply)
		reply += s.windWebCards(tVeu, reply)
		conversation = append(conversation, windSteerTaken(rotSteerKey)...)
		voiceID := newMessageID()
		conversation = append(conversation, llm.Message{Role: "assistant", Content: reply, SpeakerID: sp.ID, SpeakerName: sp.Name, Ts: time.Now().Unix(), ID: voiceID})
		veus = append(veus, veu{sp, reply, voiceID})
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
	if !parallelVoices && len(veus) >= 2 && !parcial && lastErr == nil && !isContinue && rotllanaVolDebat(msg) {
		ctx, cancel := context.WithTimeout(turnCtx, rotllanaTurnTimeout)
		emit("phase", map[string]string{"phase": "speaker:" + speakingWind.ID})
		emit("activity", windActivity{Phase: "thinking"})
		if summary, err := s.rotllanaSummary(ctx, rot, speakingWind, conversation, msg); err != nil {
			lastErr = err
			failed = append(failed, speakingWind.Name)
		} else if summary != "" {
			summaryID = newMessageID()
			final := append(conversation, llm.Message{Role: "assistant", Content: summary, SpeakerID: speakingWind.ID, SpeakerName: speakingWind.Name, Ts: time.Now().Unix(), ID: summaryID})
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
	// Revisió entre companys: un membre que no ha parlat llegeix la resposta i,
	// si hi veu un error o una millora, ho diu; l'autor contesta. Mai no tomba
	// el torn: si falla o calla, no es veu res.
	if len(veus) > 0 && lastErr == nil && !parcial && turnCtx.Err() == nil {
		author, authorText := veus[len(veus)-1].wind, veus[len(veus)-1].text
		if summaryReply != "" {
			author, authorText = speakingWind, summaryReply
		}
		directed := strings.TrimSpace(req.Wind) != "" || len(windsNamedIn(members, msg)) > 0
		if rotllanaWantsReview(directed, isContinue, intent, authorText) {
			spoke := make([]companion.Wind, 0, len(veus))
			for _, v := range veus {
				spoke = append(spoke, v.wind)
			}
			reviewer, ok := s.rotllanaReviewer(members, spoke, msg, conversation)
			if ok {
				peerSay := func(sp companion.Wind, extra string) string {
					tornTexte = &tornText{}
					emit("phase", map[string]string{"phase": "speaker:" + sp.ID})
					_, brief := rotllanaChatBrief(s, rot, sp, conversation, msg)
					brief += extra
					ctx, cancel := context.WithTimeout(turnCtx, rotllanaTurnTimeout)
					defer cancel()
					ctx = s.readOnlyWindCtx(s.interactiveWindCtx(s.withWindSteerKey(withLinkBase(ctx, r), "rotllana", rot.ID)))
					ctx = agent.WithSkillsDirs(ctx, s.windSkillsDir(sp.ID))
					s.mu.Lock()
					_, logRole := s.windRoleRef()
					s.mu.Unlock()
					veuLog = startTurnLog("rotllana-revisio", rot.ID+":"+sp.ID, logRole)
					tV := time.Now()
					var res exploracio
					var err error
					if stream {
						res, err = rotllanaVoiceLoopStream(s, veuLog.withUsage(ctx), brief, emitTokens)
					} else {
						res, err = rotllanaVoiceLoop(s, veuLog.withUsage(ctx), brief, nil)
					}
					veuLog.finish(err, tornAturat(err), res.parcial, &res.temps)
					if err != nil || res.parcial {
						return ""
					}
					text := rotllanaOwnReply(stripQuoteTrim(res.text), members, sp)
					if text == "" {
						return ""
					}
					text += s.windDocLinks(tV, text)
					conversation = append(conversation, llm.Message{Role: "assistant", Content: text, SpeakerID: sp.ID, SpeakerName: sp.Name, Ts: time.Now().Unix(), ID: newMessageID()})
					if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
						return ""
					}
					conversation = commitRotllana(conversation)
					if stream {
						emit("reply", map[string]any{"wind": sp.ID, "history": s.rotllanaHistoryJSON(conversation)})
					}
					return text
				}
				if note := peerSay(reviewer, rotllanaReviewBrief(reviewer, author)); note != "" && turnCtx.Err() == nil {
					peerSay(author, rotllanaAuthorBrief(author, reviewer))
				}
			}
		}
	}
	if turnCtx.Err() != nil {
		emit("error", map[string]any{"error": "aturat", "cancelat": true})
		return
	}
	log.Printf("vents rotllana %s: tria %.1fs · %s", rot.ID, routeDur.Seconds(), strings.Join(tempsVeus, " · "))
	rotSteerLeft()
	if len(veus) == 0 {
		// Preserve the draft on failure. A deliberate silence is not an error.
		if lastErr != nil {
			fail(http.StatusBadGateway, redactError(lastErr))
		} else {
			emit("done", map[string]any{"reply": map[string]string{}, "silenci": true, "history": s.rotllanaHistoryJSON(base)})
		}
		return
	}
	// Has tancat l'app mentre parlaven: l'avís porta a la primera veu (o al resum).
	if summaryReply != "" {
		s.noteReplyIfAway(r, speakingWind, replySource+":"+rot.ID, summaryID, summaryReply)
	} else {
		s.noteReplyIfAway(r, veus[0].wind, replySource+":"+rot.ID, veus[0].id, veus[0].text)
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
