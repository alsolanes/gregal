package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/llm"
	"gregal/internal/runs"
	"gregal/internal/session"
	"gregal/internal/tools"
)

// roleRef retorna el provider i el rol demanats (o el de xat per defecte).
func (b *Bot) roleRef(name string) (config.Provider, config.Role) {
	if name == "" {
		name = "chat"
	}
	r, ok := b.cfg.Roles[name]
	if !ok {
		r = b.cfg.Roles["chat"]
	}
	return b.cfg.Providers[r.Provider], r
}

// toolLine resumeix una crida d'eina per l'activitat.
func toolLine(c llm.ToolCall) string {
	return toolLineNom(c.Function.Name, c.Function.Arguments)
}

// primeraLinia és la primera fila d'un text (per a l'activitat).
func primeraLinia(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// toolLineNom resumeix una crida d'eina (nom i arguments) per l'activitat.
func toolLineNom(name, args string) string {
	short := ""
	var v map[string]any
	if err := jsonUnmarshal(args, &v); err == nil {
		for _, k := range []string{"path", "command", "pattern"} {
			if s, ok := v[k].(string); ok && s != "" {
				short = s
				break
			}
		}
	}
	ico := map[string]string{"read": "📖", "write": "✍️", "edit": "🩹", "bash": "⚡️", "grep": "🔎", "glob": "🗂"}[name]
	if ico == "" {
		ico = "🔧"
	}
	return ico + " " + name + " " + truncate(short, 90)
}

func (b *Bot) routeReason(why string) string {
	if b.cfg != nil && b.cfg.Lang() == "ca" {
		return why
	}
	switch {
	case strings.HasPrefix(why, "router: amb imatges → rol "):
		return "router: image input → role " + strings.TrimPrefix(why, "router: amb imatges → rol ")
	case strings.HasPrefix(why, "router: sembla una pregunta → rol "):
		role := strings.TrimPrefix(why, "router: sembla una pregunta → rol ")
		lowCost := strings.HasSuffix(role, " (barat)")
		role = strings.TrimSuffix(role, " (barat)")
		if lowCost {
			role += " (low-cost)"
		}
		return "router: looks like a question → role " + role
	case strings.HasPrefix(why, "router: feina pesada → rol "):
		return "router: complex task → role " + strings.TrimPrefix(why, "router: feina pesada → rol ")
	default:
		return why
	}
}

func (b *Bot) eventMessage(e agent.Event) string {
	if e.Clau == "app.repetida" {
		return b.tr("eina repetida", "repeated tool call")
	}
	if b.cfg != nil && b.cfg.Lang() == "ca" {
		return e.Missatge()
	}
	if e.Clau != "" {
		switch e.Clau {
		case "app.ampliaSegueix":
			return fmt.Sprintf("continuing with %d more steps (now %d)", e.Args...)
		case "app.ampliaPerChecklist":
			return fmt.Sprintf("could not read whether to continue (%s), but the checklist is %d/%d: continuing with %d more steps (now %d)", e.Args...)
		case "app.checkpoint":
			return fmt.Sprintf("autonomous · checkpoint %d", e.Args...)
		case "app.passosEsgotats":
			return fmt.Sprintf("all %d available steps used: next is a summary, not completed work", e.Args...)
		case "app.senseAccionsNoves":
			return "the model proposed no new actions (tools were repeated); closing the task…"
		case "app.ratxaVermella":
			return fmt.Sprintf("%d consecutive checks failed: asking the model to inspect the full error and check whether it predates this task", e.Args...)
		case "app.serverReintentCurt":
			return "server error; retrying this step…"
		case "app.todosPendents":
			return fmt.Sprintf("checklist %d/%d still pending; continuing automatically…", e.Args...)
		case "app.tornBuit":
			return fmt.Sprintf("the model ended the turn without a response (step %d/%d); try again or increase max_tokens", e.Args...)
		case "app.verifReintent":
			return "the last check failed; continuing…"
		case "compact.fallida":
			if len(e.Args) == 0 {
				return "compaction failed (will retry next turn)"
			}
			return fmt.Sprintf("compaction failed (will retry next turn): %v", e.Args[0])
		case "compact.fet":
			return fmt.Sprintf("conversation compacted: %d → %d messages + summary (%d characters)", e.Args...)
		case "est.agentCancel":
			return "agent cancelled"
		case "est.cancellat":
			return "cancelled"
		case "app.repetida":
			return "repeated tool call"
		}
	}
	switch e.Text {
	case agent.AvisEinaText:
		return "The model tried to call a tool by writing it as text instead of making a tool call, so it was not executed. Try again; if this keeps happening, this model may not handle tools well and should be switched to the code role."
	case agent.DoomGuide:
		return "REPEATED TOOL: this tool has already been called 3–4 times with the same or nearly the same arguments without progress. Change strategy (read another file, refine the pattern, or summarize what you have) instead of repeating it."
	case "ajornada: primer la pregunta":
		return "deferred: handle the question first"
	}
	if strings.HasPrefix(e.Text, "pregunta malformada: ") {
		return "malformed question: " + strings.TrimPrefix(e.Text, "pregunta malformada: ")
	}
	if strings.HasPrefix(e.Text, "bloquejada: ") {
		return "blocked: " + strings.TrimPrefix(e.Text, "bloquejada: ")
	}
	return e.Missatge()
}

// editInterval és el mínim entre dues edicions del mateix missatge.
// Telegram limita les edicions seguides (respon 429) i cada edició és una
// crida de xarxa; amb una cada 1,2 s el text continua semblant viu i no fem
// deu crides per resposta. El text NO es perd mai: tot queda al buffer, i
// tant la propera edició com l'última (la final, que sempre es fa) el posen
// sencer. Si es limités el text i no només el ritme, l'usuari veuria una
// resposta escapçada — i això seria pitjor que anar una mica més lent.
const editInterval = 1200 * time.Millisecond

// potEditar diu si toca editar ara; si és que sí, ho apunta.
func (b *Bot) potEditar(chatID int64) bool {
	b.editMu.Lock()
	t, vist := b.lastEdit[chatID]
	b.editMu.Unlock()
	if vist && time.Since(t) < editInterval {
		return false
	}
	b.marcaEditat(chatID)
	return true
}

// marcaEditat apunta que acabem de tocar el missatge del xat (enviar-lo o
// editar-lo). Compta l'enviament: Telegram també limita editar un missatge
// just després d'haver-lo enviat, i aquell instant és el que dona la
// resposta immediata a l'usuari.
func (b *Bot) marcaEditat(chatID int64) {
	b.editMu.Lock()
	defer b.editMu.Unlock()
	if b.lastEdit == nil {
		b.lastEdit = map[int64]time.Time{}
	}
	b.lastEdit[chatID] = time.Now()
}

// streamAnswer escriu o edita el missatge de resposta amb el text acumulat
// (markdown del model → HTML de Telegram).
func (b *Bot) streamAnswer(ctx context.Context, chatID int64, id *int, buf *strings.Builder, chunk string) {
	if buf.Len() > 0 {
		buf.WriteString("\n\n")
	}
	buf.WriteString(strings.TrimSpace(chunk))
	text := renderAnswer(buf.String())
	if *id == 0 {
		if m, err := b.api.SendMessage(ctx, chatID, text+" ▌", nil); err == nil {
			*id = m.MessageID
			b.marcaEditat(chatID)
		} else {
			b.logf("sendMessage (resposta): %v", err)
		}
		return
	}
	if !b.potEditar(chatID) {
		// Massa aviat per a una altra edició: el text ja és al buffer i el
		// posarà la propera (o la final, que no es limita mai).
		return
	}
	if err := b.api.EditMessageText(ctx, chatID, *id, text+" ▌", nil); err != nil {
		b.logf("editMessageText: %v", err)
	}
}

// finishAnswer treu el cursor final del missatge de resposta.
// Torna true si la resposta supera el límit i cal enviar-la sencera com a fitxer.
func (b *Bot) finishAnswer(ctx context.Context, chatID int64, id int, buf *strings.Builder) bool {
	if id == 0 || buf.Len() == 0 {
		return false
	}
	final := renderAnswer(buf.String())
	if err := b.api.EditMessageText(ctx, chatID, id, final, nil); err != nil {
		b.logf("editMessageText (final): %v", err)
	}
	return len([]rune(buf.String())) > 3900
}

// addActivity manté un missatge amb les eines que s'han fet servir.
func (b *Bot) addActivity(ctx context.Context, chatID int64, id *int, buf *strings.Builder, line string) {
	buf.WriteString(line + "\n")
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) > 12 {
		lines = append([]string{"…"}, lines[len(lines)-12:]...)
	}
	text := "🏃 " + b.tr("activitat", "activity") + "\n" + esc(strings.Join(lines, "\n"))
	if *id == 0 {
		if m, err := b.api.SendMessage(ctx, chatID, text, nil); err == nil {
			*id = m.MessageID
		}
		return
	}
	if err := b.api.EditMessageText(ctx, chatID, *id, text, nil); err != nil {
		b.logf("editMessageText (activitat): %v", err)
	}
}

// truncateFromStart retalla pel començament (els missatges llargs de Telegram).
func truncateFromStart(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…\n" + s[len(s)-max:]
}

// turn encua una tasca a la cua compartida. La clau de sessió és estable per
// xat de Telegram, independentment del nom visible de la conversa; així tots
// els missatges del mateix xat queden serialitzats i comparteixen cancel·lació.
func (b *Bot) turn(ctx context.Context, chatID int64, st *chatState, text string, images ...string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	st.mu.Lock()
	// Conservem el lock fins haver registrat l'ID pendent. L'executor pot
	// arrencar immediatament després de Submit; com que turnRun agafa el mateix
	// lock, /stop no pot observar una finestra on el torn existeix a la cua
	// però encara no figura ni com a actiu ni com a pendent.
	wasBusy := st.busy || st.runID != 0 || len(st.pendingRuns) > 0
	run, _, err := b.queue.Submit(func(runCtx context.Context, r runs.Run) error {
		return b.turnRun(runCtx, chatID, st, text, images, r)
	}, runs.Run{
		Session:   tgScope(chatID),
		Workspace: runs.WorkspaceKey(b.dirOf(st)),
		Priority:  runs.PriorityInteractive,
	})
	if err != nil {
		st.mu.Unlock()
		b.send(ctx, chatID, "⚠️ "+b.tr("No he pogut encuar el torn: ", "Could not queue the turn: ")+truncate(err.Error(), 250))
		return
	}
	st.pendingRuns = append(st.pendingRuns, run.ID)
	st.mu.Unlock()
	if wasBusy {
		b.sendKB(ctx, chatID, "⏳ "+b.tr("Torn encuat · treballaré en aquest xat per ordre d'arribada · /ara <code>&lt;tasca&gt;</code> si ha de passar abans", "Turn queued · I’ll work through this chat in order · use /ara <code>&lt;task&gt;</code> to move it ahead"), b.stopKB())
		return
	}
	// Mantén el contracte del handler anterior per al primer torn: qui crida
	// directament turn rep el resultat quan acaba, mentre que les peticions que
	// arriben amb el xat ocupat queden desacoblades a la cua.
	<-b.queue.Done(run.ID)
}

// turnRun executa una tasca amb l'agent i va publicant el progrés al xat.
// El context rebut és el de runs.Queue: cancel·lar des de la cua talla també
// la crida de model i les eines en curs.
func (b *Bot) turnRun(ctx context.Context, chatID int64, st *chatState, text string, images []string, run runs.Run) error {
	st.mu.Lock()
	st.busy = true
	st.runID = run.ID
	for i, id := range st.pendingRuns {
		if id == run.ID {
			st.pendingRuns = append(st.pendingRuns[:i], st.pendingRuns[i+1:]...)
			break
		}
	}
	// Mapa del projecte amb la tasca (fitxers, git, fitxers anomenats).
	st.convo = append(st.convo, llm.Message{Role: "user", Content: agent.TascaAmbMapa(st.mode, b.dirOf(st), text, st.convo), Images: images})
	mode, role := st.mode, st.role
	pinned := st.rolePinned
	convo := append([]llm.Message{}, st.convo...)
	st.mu.Unlock()
	// Cortesia pura ("hola"): resposta directa sense model ni eines.
	if reply, ok := agent.SmallTalk(text); ok && len(images) == 0 {
		b.send(ctx, chatID, reply)
		st.mu.Lock()
		st.convo = append(st.convo, llm.Message{Role: "assistant", Content: reply})
		st.busy = false
		st.mu.Unlock()
		return nil
	}
	if !pinned {
		if routed, why := b.cfg.Route(text, len(images) > 0, role); routed != role && why != "" {
			role = routed
			b.send(ctx, chatID, "🔀 "+b.routeReason(why))
		}
	}

	tctx, cancel := context.WithCancel(ctx)
	st.mu.Lock()
	st.cancel = cancel
	st.mu.Unlock()
	defer func() {
		cancel()
		st.mu.Lock()
		st.busy = false
		st.cancel = nil
		if st.runID == run.ID {
			st.runID = 0
		}
		st.mu.Unlock()
	}()

	// "escrivint…" mentre dura el torn.
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = b.api.SendChatAction(tctx, chatID, "typing")
			}
		}
	}()
	defer close(done)

	sys := agent.PromptFor(b.cfg.SystemPrompt(), mode)
	if info := agent.ContextProjecte(b.dirOf(st)); info != "" {
		sys += "\n\n" + info
	}

	var answerID, activityID int
	var answer, activity strings.Builder
	var lastReply string
	var turnErr error
	toolCount := 0
	activitat := func(line string) { b.addActivity(tctx, chatID, &activityID, &activity, line) }

	// Els reintents del client es veuen a l'activitat: abans l'espera era
	// muda i semblava que el bot s'havia penjat.
	tctx = llm.WithRetryHook(tctx, func(attempt, total int, wait time.Duration, err error) {
		activitat("↻ " + llm.RetryNote(attempt, total, wait, err))
	})

	// El motor decideix; aquí només es fa la feina (cridar el model,
	// executar eines, demanar permís) i es publica al xat. Abans el bot
	// portava una tercera màquina d'estats pròpia, sense compactació ni
	// síntesi, i cada arranjament del bucle s'havia de fer tres vegades.
	rolActual := func() (config.Provider, config.Role) { return b.roleRefFor(chatID, role) }
	// think: no del rol i comptador de tokens reals (topall de cost autònom).
	if _, r := rolActual(); r.Think != "" {
		tctx = llm.WithThink(tctx, r.Think)
	}
	uso := &llm.UsageMeter{}
	tctx = llm.WithUsage(tctx, uso)
	torn := agent.NouTorn(agent.OpcionsTorn{
		Usage: uso,
		Cfg:   b.cfg, Pol: b.polOf(st), Mode: mode, Tasca: text, MaxSteps: b.cfg.Agent.MaxSteps,
		Hist:      convo,
		SysPrompt: func() string { return sys },
		Finestra: func() (int, int) {
			_, r := rolActual()
			win := agent.Window(b.cfg, r)
			return win, agent.PromptBudget(win, r.MaxTokens)
		},
		Rol: func() (string, string) { _, r := rolActual(); return r.Provider, r.Model },
		Recordat: func(name, args string) bool {
			return b.remembers.Allowed(tgScope(chatID), agent.Sig(name, args))
		},
	})
	pinta := func(evs []agent.Event) {
		for _, e := range evs {
			switch e.Tipus {
			case agent.EvCrida:
				toolCount++
				activitat(toolLineNom(e.Eina, e.EinaArgs))
			case agent.EvResultat:
				if e.Fallada {
					activitat("✗ " + e.Eina + " · " + truncate(primeraLinia(b.eventMessage(e)), 70))
				}
			case agent.EvResposta:
				lastReply = e.Text
				b.streamAnswer(tctx, chatID, &answerID, &answer, e.Text)
			case agent.EvError:
				b.send(ctx, chatID, "⚠️ "+truncate(b.eventMessage(e), 300))
			case agent.EvAvis, agent.EvNota:
				if t := b.eventMessage(e); strings.TrimSpace(t) != "" {
					activitat("· " + truncate(t, 120))
				}
			}
		}
	}
	avisaFallback := func(model string) {
		b.send(ctx, chatID, b.tr("↪ Primari caigut · respon el fallback ", "↪ Primary failed · using fallback ")+esc(model))
	}

bucle:
	for {
		if tctx.Err() != nil {
			pinta(torn.Cancella())
			break
		}
		p := torn.Seguent()
		pr, r := rolActual()
		switch p.Ordre {
		case agent.OrdrePasModel:
			content, calls, _, err := b.client.ChatWithToolsFO(llm.WithThink(tctx, agent.ThinkPerPas(r.Think, p)), b.cfg.PrimTarget(pr, r), b.cfg.FallbackTarget(r), p.Hist,
				r.Temperature, r.MaxTokens, agent.SpecsAll(), avisaFallback)
			// El text que acompanya crides d'eina també es publica: és el
			// model explicant què farà. La resposta final arriba per EvResposta.
			if err == nil && len(calls) > 0 && strings.TrimSpace(content) != "" {
				b.streamAnswer(tctx, chatID, &answerID, &answer, content)
			}
			pinta(torn.RepPas(content, calls, err))

		case agent.OrdreCompacta:
			room := agent.MakeRoom(tctx, b.client, b.cfg.PrimTarget(pr, r), b.cfg.FallbackTarget(r), sys, torn.Hist, p.Budget)
			pinta(torn.RepCompactacio(room))

		case agent.OrdreExecuta:
			outs := agent.RunCalls(p.Calls, nil, func(_ int, c llm.ToolCall) agent.Execucio {
				// Amb el context del torn: /stop atura també l'eina a mitges,
				// no només la següent crida al model.
				o, imgs, err := agent.ExecCtx(tctx, "", b.dirOf(st), c.Function.Name, c.Function.Arguments)
				if err != nil {
					o = "ERROR: " + err.Error()
				}
				return agent.Execucio{Call: c, Sortida: o, Imatges: imgs}
			})
			pinta(torn.RepExecucions(outs))

		case agent.OrdreAprova:
			c := p.Call
			ok := b.askApproval(tctx, chatID, c.Function.Name, c.Function.Arguments, agent.Sig(c.Function.Name, c.Function.Arguments))
			pinta(torn.RepAprovacio(ok, "EINA DENEGADA per l'usuari."))

		case agent.OrdrePregunta:
			// Sense selector al xat: el model la formula com a text.
			pinta(torn.RepPregunta("PREGUNTA SENSE UI: no hi ha interfície per triar opcions aquí. Formula la pregunta en text a la resposta final i continua amb el teu millor criteri."))

		case agent.OrdreAmplia:
			actx, cancelA := context.WithTimeout(llm.WithThink(tctx, agent.ThinkPerPas(r.Think, p)), agent.TimeoutDecisio)
			resp, _, aerr := b.client.ChatFO(actx, b.cfg.PrimTarget(pr, r), b.cfg.FallbackTarget(r), p.Hist, r.Temperature, r.MaxTokens, avisaFallback)
			cancelA()
			pinta(torn.RepAmpliacio(resp, aerr))

		case agent.OrdreSintesi:
			fctx, cancelF := context.WithTimeout(llm.WithThink(tctx, agent.ThinkPerPas(r.Think, p)), agent.TimeoutDecisio)
			final, _, ferr := b.client.ChatFO(fctx, b.cfg.PrimTarget(pr, r), b.cfg.FallbackTarget(r), p.Hist, r.Temperature, r.MaxTokens, avisaFallback)
			cancelF()
			pinta(torn.RepSintesi(final, ferr))

		case agent.OrdreCheckpoint:
			// El bot no corre comprovacions pròpies: el checkpoint es marca
			// i el torn continua.
			pinta(torn.RepCheckpoint("", ""))

		case agent.OrdreError:
			b.send(ctx, chatID, "⚠️ "+truncate(p.Text, 300))
			turnErr = fmt.Errorf("agent: %s", p.Text)
			break bucle

		default: // OrdreAcaba, OrdreRes
			break bucle
		}
	}
	steps := torn.Passos()

	b.finishAnswer(ctx, chatID, answerID, &answer)
	// Resposta llarga: el missatge queda retallat i la versió sencera va en fitxer.
	if len([]rune(answer.String())) > 3900 && strings.TrimSpace(answer.String()) != "" {
		if err := b.api.SendDocument(ctx, chatID, "resposta.md", []byte(answer.String()),
			"📄 "+b.tr("Resposta sencera (", "Full response (")+itoa(len([]rune(answer.String())))+" "+b.tr("caràcters", "characters")+")"); err != nil {
			b.logf("sendDocument (resposta): %v", err)
		}
	}

	st.mu.Lock()
	if strings.TrimSpace(lastReply) != "" {
		st.convo = append(st.convo, llm.Message{Role: "assistant", Content: lastReply})
	}
	tools.Active.MarkConvo(len(st.convo))
	name := st.name
	st.mu.Unlock()
	b.saveSession(st)

	// Mode objectiu: es desa l'objectiu proposat.
	if mode == agent.ModeGoal {
		if g, ok := goal.Parse(lastReply, b.dirOf(st)); ok {
			if err := goal.Save(b.goalDirSafe(), g); err == nil {
				b.send(ctx, chatID, fmt.Sprintf("◆ "+b.tr("Objectiu desat · <code>%s</code>\n/goal executa per posar-lo en marxa (mode code)", "Goal saved · <code>%s</code>\nUse /goal executa to run it (code mode)"), esc(g.ID)))
			}
		}
	}
	if toolCount > 0 || answer.Len() > 0 {
		b.send(ctx, chatID, fmt.Sprintf("— %s · "+b.tr("sessió", "session")+" <code>%s</code> · <code>%s</code> · %d "+b.tr("pas(sos)", "step(s)")+" · %d "+b.tr("eina(es)", "tool(s)"),
			esc(b.modeBadge(mode)), esc(name), esc(shortModel(b.currentRef(chatID, role))), steps, toolCount))
	}
	if ctx.Err() != nil {
		return runs.ErrCancelled
	}
	return turnErr
}

// saveSession desa la conversa al magatzem compartit amb el TUI.
func (b *Bot) saveSession(st *chatState) {
	st.mu.Lock()
	name, role, convo := st.name, st.role, st.convo
	st.mu.Unlock()
	if strings.TrimSpace(name) == "" {
		return
	}
	if _, err := session.Save(b.sessionsDir, name, role, convo); err != nil {
		b.logf("saveSession: %v", err)
	}
}

// modeBadge retorna l'etiqueta del mode.
func (b *Bot) modeBadge(mode string) string {
	switch mode {
	case agent.ModeGoal:
		return "◆ " + b.tr("objectiu", "goal")
	case agent.ModeInspect:
		return "◉ " + b.tr("consulta", "inspect")
	case agent.ModeChat:
		return "◇ " + b.tr("xat", "chat")
	case agent.ModeAutonomous:
		return "≋ " + b.tr("autònom", "autonomous")
	default:
		return "◆ " + b.tr("codi", "code")
	}
}

// truncConvoTG retalla l'historial a n (E2b; -1 = sense info).
func truncConvoTG(convo []llm.Message, n int) []llm.Message {
	if n < 0 || n > len(convo) {
		return convo
	}
	return convo[:n]
}
