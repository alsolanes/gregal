package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/llm"
	"gregal/internal/session"
	"gregal/internal/tools"
	"gregal/internal/verify"
)

// jsonUnmarshal és un àlies per no importar encoding/json a turn.go.
func jsonUnmarshal(raw string, v any) error { return json.Unmarshal([]byte(raw), v) }

// esc escapa el text per parse_mode=HTML de Telegram.
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// parseCommand separa "/ordre@bot arg" en ("/ordre", "arg").
func parseCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", text
	}
	parts := strings.SplitN(text, " ", 2)
	cmd := parts[0]
	if i := strings.Index(cmd, "@"); i > 0 {
		cmd = cmd[:i]
	}
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}
	return strings.ToLower(cmd), arg
}

// help explica les ordres.
func (b *Bot) help(st *chatState) string {
	return strings.Join([]string{
		"≋ <b>gregal</b> · " + b.tr("agent de codi", "coding agent"),
		fmt.Sprintf(b.tr("projecte <code>%s</code> · mode %s · rol <code>%s</code> · sessió <code>%s</code>", "project <code>%s</code> · mode %s · role <code>%s</code> · session <code>%s</code>"),
			esc(b.projectName()), esc(b.modeBadge(st.mode)), esc(st.role), esc(st.name)),
		"",
		"<b>" + b.tr("Escriu un missatge", "Send a message") + "</b>" + b.tr(" i l'agent hi treballa (mode i permisos del config).", " and the agent will work on it (using the configured mode and permissions)."),
		b.tr("Toca els botons o fes servir les ordres:", "Use the buttons or these commands:"),
		"",
		b.tr("/new [nom] — sessió nova · /sessions — llista amb botons", "/new [name] — start a session · /sessions — browse sessions"),
		b.tr("/use &lt;n|nom&gt; — continua una sessió (aquí o al TUI)", "/use &lt;n|name&gt; — resume a session (here or in the TUI)"),
		b.tr("/rename &lt;nom&gt; — canvia el nom · /del &lt;n|nom&gt; — esborra", "/rename &lt;name&gt; — rename · /del &lt;n|name&gt; — delete"),
		b.tr("/clear — buida la conversa · /export — descarrega-la (.md)", "/clear — clear the conversation · /export — download it (.md)"),
		"/mode [code|inspect|chat|goal|autonomous] — " + b.tr("canvia de mode", "change mode"),
		"/role [chat|think|code] — " + b.tr("quin model respon", "choose the responding model role"),
		"/model [name] — " + b.tr("tria el model en viu (es desa per xat+rol)", "choose a live model (saved per chat and role)"),
		"/goal [llista|executa [id]|mostra [id]|esborra &lt;id&gt;] — " + b.tr("objectius", "manage goals"),
		"/retry — " + b.tr("reintenta l'última petició", "retry the last request") + " · /resum — " + b.tr("resumeix el xat", "summarize the chat"),
		"/status — " + b.tr("estat", "status") + " · /stop — " + b.tr("atura", "stop") + " · /ara <code>&lt;task&gt;</code> — " + b.tr("talla la feina anterior i fes això", "cancel the previous work and do this"),
		"/verify — " + b.tr("revisor", "reviewer") + " · /perms — " + b.tr("permisos", "permissions"),
		"/rewind — " + b.tr("desfés els canvis de fitxers de la sessió", "undo this session's file changes"),
		"/rewind N — " + b.tr("torna al checkpoint N", "restore checkpoint N") + " · /plan <code>&lt;task&gt;</code> — " + b.tr("explora sense actuar i proposa un pla", "inspect without making changes and propose a plan"),
		"/whoami — " + b.tr("id d'usuari i de xat", "show user and chat IDs"),
		"",
		b.tr("Per aprovar eines ({write, edit, bash), fes servir els botons. Només l'amo pot decidir.", "Approve tools ({write, edit, bash}) with the buttons. Only the owner can decide."),
	}, "\n")
}

// command atén les ordres del bot.
func (b *Bot) command(ctx context.Context, m *Message, st *chatState, text string) {
	cmd, arg := parseCommand(text)
	chatID := m.Chat.ID
	switch cmd {
	case "/start", "/help", "/ajuda":
		b.sendKB(ctx, chatID, b.help(st), b.helpKB())

	case "/whoami", "/qui":
		msg := fmt.Sprintf(b.tr("usuari <code>%d</code> (%s)\nxat <code>%d</code> (%s)", "user <code>%d</code> (%s)\nchat <code>%d</code> (%s)"),
			m.From.ID, esc(m.From.Username), chatID, esc(m.Chat.Type))
		if st.user != "" {
			msg += fmt.Sprintf("\n"+b.tr("ets gregal <code>%s</code> · casa <code>%s</code>", "Gregal account <code>%s</code> · home <code>%s</code>"), esc(st.user), esc(b.dirOf(st)))
		} else {
			msg += "\n" + b.tr("sense compte lligat: projecte global", "no linked account: global project")
		}
		b.send(ctx, chatID, msg)

	case "/new", "/nova":
		b.newSession(ctx, chatID, st, arg)

	case "/sessions", "/sessio":
		b.listSessionsPage(ctx, chatID, st, 0)

	case "/use", "/usa":
		if arg == "" {
			b.send(ctx, chatID, b.tr("ús: /use &lt;número o nom&gt; (mira /sessions)", "usage: /use &lt;number or name&gt; (see /sessions)"))
			return
		}
		b.useSession(ctx, chatID, st, arg)

	case "/rename", "/reanomena":
		if arg == "" {
			b.send(ctx, chatID, b.tr("ús: /rename &lt;nom nou&gt;", "usage: /rename &lt;new name&gt;"))
			return
		}
		b.renameSession(ctx, chatID, st, arg)

	case "/del", "/delete", "/esborra":
		if arg == "" {
			b.send(ctx, chatID, b.tr("ús: /del &lt;número o nom&gt; (mira /sessions)", "usage: /del &lt;number or name&gt; (see /sessions)"))
			return
		}
		b.deleteSession(ctx, chatID, st, arg)

	case "/clear", "/neteja":
		b.clearSession(ctx, chatID, st)

	case "/export", "/descarrega":
		b.exportSession(ctx, chatID, st)

	case "/mode":
		if arg == "" {
			b.sendKB(ctx, chatID, fmt.Sprintf(b.tr("mode actual: %s\n%s\n%s", "current mode: %s\n%s\n%s"),
				esc(b.modeBadge(st.mode)), esc(b.modeHint(st.mode)),
				esc(b.modelLine(chatID, st))), b.modeKB(st.mode))
			return
		}
		b.setMode(ctx, chatID, st, arg, m.Chat.Type, m.From.ID)

	case "/role", "/rol":
		if arg == "" {
			b.sendKB(ctx, chatID, b.roleText(chatID, st), b.roleKB(st.role))
			return
		}
		b.setRole(ctx, chatID, st, arg)

	case "/model", "/models", "/modelos":
		if arg == "" {
			b.modelList(ctx, chatID, st)
			return
		}
		b.modelPick(ctx, chatID, st, arg)

	case "/perms", "/permisos":
		b.send(ctx, chatID, b.permsText())

	case "/retry", "/reintenta":
		b.retry(ctx, chatID, st)

	case "/resum", "/resumeix", "/summarize":
		b.summarize(ctx, chatID, st)

	case "/goal", "/objectiu":
		b.goalCommand(ctx, chatID, st, arg)

	case "/status", "/estat":
		b.status(ctx, chatID, st)

	case "/stop", "/atura":
		b.stop(ctx, chatID, st)

	case "/ara", "/now":
		b.nowCommand(ctx, m, chatID, st, arg)

	case "/rewind", "/desfes":
		if n, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil {
			out, err := tools.Active.RewindTo(n)
			if err != nil {
				b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut desfer el checkpoint: ", "Could not undo checkpoint: ")+esc(err.Error()))
				return
			}
			st.mu.Lock()
			st.convo = truncConvoTG(st.convo, tools.Active.ConvoLenAt(n))
			st.mu.Unlock()
			b.send(ctx, chatID, "↩️ "+b.tr("Punt de control ", "Checkpoint ")+esc(arg)+":\n"+esc(out))
			return
		}
		out, err := tools.Active.Rewind()
		if err != nil {
			b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut desfer: ", "Could not undo changes: ")+esc(err.Error()))
			return
		}
		st.mu.Lock()
		st.convo = truncConvoTG(st.convo, tools.Active.ConvoLenAt(0))
		st.mu.Unlock()
		if strings.TrimSpace(out) == "" {
			out = b.tr("res a desfer", "nothing to undo")
		}
		b.send(ctx, chatID, "↩️ "+b.tr("Canvis desfets (restaura el contingut d'abans de la sessió, no usa git):\n", "Changes undone (restores the content from before this session; does not use git):\n")+esc(out))

	case "/remember", "/recorda":
		if err := config.Remember(arg); err != nil {
			b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut desar l'apunt: ", "Could not save the note: ")+esc(err.Error()))
			return
		}
		b.send(ctx, chatID, "📝 "+b.tr("Apuntat.", "Noted."))

	case "/recall", "/record":
		lines := config.Recall(arg)
		if len(lines) == 0 {
			b.send(ctx, chatID, b.tr("Memòria buida.", "Memory is empty."))
			return
		}
		b.send(ctx, chatID, "📝 "+b.tr("Memòria", "Memory")+":\n"+esc(strings.Join(lines, "\n")))

	case "/forget", "/oblida":
		n, err := config.Forget(arg)
		if err != nil {
			b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut oblidar: ", "Could not forget the note: ")+esc(err.Error()))
			return
		}
		if n == 0 {
			b.send(ctx, chatID, b.tr("No hi havia res amb aquest terme.", "Nothing matched that term."))
			return
		}
		b.send(ctx, chatID, fmt.Sprintf(b.tr("🗑️ Oblidades %d entrades.", "🗑️ Forgot %d entries."), n))

	case "/plan", "/pla":
		b.planCommand(ctx, chatID, st, arg)

	case "/verify", "/revisa":
		b.verify(ctx, chatID, st)

	case "/a", "/agent", "/fes":
		if arg == "" {
			b.send(ctx, chatID, b.tr("ús: /a &lt;tasca per l'agent&gt;", "usage: /a &lt;task for the agent&gt;"))
			return
		}
		b.turn(ctx, chatID, st, arg)

	default:
		b.send(ctx, chatID, b.tr("ordre desconeguda: ", "unknown command: ")+esc(cmd)+" · /help")
	}
}

// verify passa el darrer diff al model revisor.
func (b *Bot) verify(ctx context.Context, chatID int64, st *chatState) {
	if b.cfg.Verify.Mode == "off" {
		b.send(ctx, chatID, b.tr("El revisor està desactivat (<code>verify.mode: off</code>).", "The reviewer is disabled (<code>verify.mode: off</code>)."))
		return
	}
	r, ok := b.cfg.Roles["reviewer"]
	if !ok {
		b.send(ctx, chatID, b.tr("Falta el rol <code>reviewer</code> al config.", "The <code>reviewer</code> role is missing from the config."))
		return
	}
	p := b.cfg.Providers[r.Provider]
	st.mu.Lock()
	convo := append([]llm.Message{}, st.convo...)
	st.mu.Unlock()
	var tail strings.Builder
	if len(convo) > 6 {
		convo = convo[len(convo)-6:]
	}
	for _, m := range convo {
		tail.WriteString(m.Role + ": " + m.Content + "\n")
	}
	_ = b.api.SendChatAction(ctx, chatID, "typing")
	vctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	v, raw, err := verify.Run(vctx, b.client, p, r, tail.String(), tools.GitDiff(b.dirOf(st)))
	if err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("revisor: ", "reviewer: ")+esc(err.Error()))
		return
	}
	if v.Approved {
		b.send(ctx, chatID, "✅ <b>"+b.tr("Aprovat pel revisor", "Approved by reviewer")+"</b>\n"+esc(v.Summary))
		return
	}
	b.send(ctx, chatID, "✗ <b>"+b.tr("Cal revisar", "Review needed")+"</b>\n"+esc(truncate(raw, 1500)))
}

// modeHint explica què fa cada mode.
func (b *Bot) modeHint(mode string) string {
	switch mode {
	case agent.ModeGoal:
		return b.tr("concreta l'objectiu amb preguntes; no escriu res", "clarify the goal with questions; makes no changes")
	case agent.ModeInspect:
		return b.tr("consulta automàtica: lectura segura, sense popups", "automatic inspection: safe reads, no prompts")
	case agent.ModeChat:
		return b.tr("conversa sense inspeccionar el disc", "chat without inspecting files")
	case agent.ModeAutonomous:
		return b.tr("tasca llarga per fites, checkpoints i verificacions", "long-running work with milestones, checkpoints, and verification")
	default:
		return b.tr("eines amb permisos", "tools with permission checks")
	}
}

// newSession obre una sessió nova (desant l'anterior).
func (b *Bot) newSession(ctx context.Context, chatID int64, st *chatState, name string) {
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("tg-%d-%s", chatID, time.Now().Format("0102-1504"))
	}
	b.saveSession(st)
	st.mu.Lock()
	st.name = name
	st.convo = nil
	mode := st.mode
	st.mu.Unlock()
	b.remembers.Clear(tgScope(chatID))
	b.saveSession(st)
	b.sendKB(ctx, chatID, "🆕 "+b.tr("Sessió nova", "New session")+" <code>"+esc(name)+"</code> · "+b.tr("mode", "mode")+" "+esc(b.modeBadge(mode)), b.helpKB())
}

// modelLine diu, en una línia, qui respondrà: el rol actiu i el seu model.
// Va als missatges de mode perquè no calgui preguntar-ho a part.
func (b *Bot) modelLine(chatID int64, st *chatState) string {
	st.mu.Lock()
	role := st.role
	st.mu.Unlock()
	return b.tr("respondrà: ", "responding as ") + role + " → " + b.currentRef(chatID, role)
}

// setMode canvia el mode del xat (i el desa al config si és privat).
//
// El mode autònom no és per a tothom: fa anar les eines sense cap aprovació,
// així que només el pot activar qui té dret a treballar desatès (vegeu
// mayRunUnattended). Els botons ja passen per canApprove; el text no, i
// abans qualsevol usuari autoritzat hi podia entrar escrivint l'ordre.
func (b *Bot) setMode(ctx context.Context, chatID int64, st *chatState, mode, chatType string, userID int64) {
	if !agent.ValidMode(mode) {
		b.send(ctx, chatID, b.tr("mode invàlid: ", "invalid mode: ")+esc(mode)+" (code|inspect|chat|goal|autonomous)")
		return
	}
	if mode == agent.ModeAutonomous && !b.mayRunUnattended(userID) {
		b.send(ctx, chatID, b.tr("el mode autònom només el pot activar l'amo: fa anar les eines sense demanar permís.\nPots fer servir /mode code, /mode consulta o /mode objectiu.", "only the owner can enable autonomous mode because it runs tools without asking for permission.\nUse /mode code, /mode inspect, or /mode goal instead."))
		return
	}
	st.mu.Lock()
	st.mode = mode
	st.mu.Unlock()
	// El mode es desa com a preferència del xat privat, però l'autònom NO: és
	// un permís, no un gust. Si es desés, el pròxim xat que s'obrís —el d'un
	// altre usuari— naixeria sense aprovacions, que és exactament el forat
	// que això tanca.
	if chatType == "private" && mode != agent.ModeAutonomous {
		b.cfg.Mode = mode
		if err := b.cfg.Save(b.cfgPath); err != nil {
			b.logf("desar el mode: %v", err)
		}
	}
	b.sendKB(ctx, chatID, b.tr("mode ", "mode ")+esc(b.modeBadge(mode))+" · "+esc(b.modeHint(mode))+"\n"+esc(b.modelLine(chatID, st)), b.modeKB(mode))
}

// roleText descriu el rol actiu i el model que hi ha al darrere.
func (b *Bot) roleText(chatID int64, st *chatState) string {
	st.mu.Lock()
	role := st.role
	st.mu.Unlock()
	p, r := b.roleRefFor(chatID, role)
	txt := fmt.Sprintf("🧠 "+b.tr("Rol actual", "Current role")+": <code>%s</code>\n"+b.tr("model", "model")+" <code>%s/%s</code>\n"+b.tr("lloc", "endpoint")+" <code>%s</code>",
		esc(role), esc(r.Provider), esc(r.Model), esc(p.BaseURL))
	if b.overrideFor(chatID, role) != "" {
		txt += "\n" + b.tr("override d'aquest xat (↺ per tornar al defecte)", "chat override (↺ to restore the default)")
	}
	return txt + "\n" + b.tr("Toca un rol per canviar qui respon, o /model per triar el model.", "Choose a role to change who responds, or use /model to select a model.")
}

// setRole canvia quin rol (model) respon en aquest xat.
func (b *Bot) setRole(ctx context.Context, chatID int64, st *chatState, role string) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "reviewer" {
		b.send(ctx, chatID, b.tr("el rol <code>reviewer</code> és només per al revisor", "the <code>reviewer</code> role is reserved for reviews"))
		return
	}
	if _, ok := b.cfg.Roles[role]; !ok {
		var noms []string
		for n := range b.cfg.Roles {
			if n != "reviewer" {
				noms = append(noms, n)
			}
		}
		sort.Strings(noms)
		b.send(ctx, chatID, b.tr("rol desconegut: ", "unknown role: ")+esc(role)+" ("+esc(strings.Join(noms, "|"))+")")
		return
	}
	st.mu.Lock()
	st.role = role
	st.rolePinned = true
	st.mu.Unlock()
	b.saveSession(st)
	b.sendKB(ctx, chatID, b.roleText(chatID, st), b.roleKB(role))
}

// permsText resumeix els permisos d'eines actius.
func (b *Bot) permsText() string {
	var lines []string
	lines = append(lines, "🔐 <b>"+b.tr("Permisos", "Permissions")+"</b> ("+b.tr("per eina", "per tool")+": allow = "+b.tr("sense confirmar", "without prompting")+", ask = "+b.tr("amb botons", "with buttons")+", deny = "+b.tr("no", "never")+")")
	tools := []string{"read", "write", "edit", "bash", "grep", "glob"}
	for _, t := range tools {
		v := b.policy.Tools[t]
		if v == "" {
			v = "ask"
		}
		lines = append(lines, fmt.Sprintf("· <code>%s</code>: %s", esc(t), esc(v)))
	}
	if len(b.policy.BashAllow) > 0 {
		lines = append(lines, b.tr("bash permesos: <code>", "allowed bash commands: <code>")+esc(strings.Join(b.policy.BashAllow, "</code>, <code>"))+"</code>")
	}
	if len(b.policy.BashDeny) > 0 {
		lines = append(lines, b.tr("bash denegats: <code>", "blocked bash commands: <code>")+esc(strings.Join(b.policy.BashDeny, "</code>, <code>"))+"</code>")
	}
	lines = append(lines, "", b.tr("Es configuren al <code>config.yaml</code> (permissions).", "Configured in <code>config.yaml</code> (permissions)."))
	return strings.Join(lines, "\n")
}

// stop atura la feina en curs del xat.
func (b *Bot) stop(ctx context.Context, chatID int64, st *chatState) {
	st.mu.Lock()
	cancel, busy, runID := st.cancel, st.busy, st.runID
	var queuedID int64
	if !busy && cancel == nil && len(st.pendingRuns) > 0 {
		queuedID = st.pendingRuns[len(st.pendingRuns)-1]
		st.pendingRuns = st.pendingRuns[:len(st.pendingRuns)-1]
	}
	st.mu.Unlock()
	if busy && cancel != nil {
		cancel()
		b.send(ctx, chatID, "🛑 "+b.tr("Aturat", "Stopped"))
		return
	}
	if runID != 0 && b.queue.Cancel(runID) {
		b.send(ctx, chatID, "🛑 "+b.tr("Aturat", "Stopped"))
		return
	}
	if queuedID != 0 && b.queue.Cancel(queuedID) {
		b.send(ctx, chatID, "🛑 "+b.tr("Torn de la cua cancel·lat", "Queued turn cancelled"))
		return
	}
	b.send(ctx, chatID, b.tr("No hi ha res en marxa.", "Nothing is running."))
}

// nowCommand implementa /ara (àlies /now): «talla el que estàs fent i fes
// això ara». Cancel·la el torn actiu i tota la cua del xat i encua la tasca
// nova. Sense text, només talla (equivalent a /stop però amb la cua sencera).
func (b *Bot) nowCommand(ctx context.Context, m *Message, chatID int64, st *chatState, arg string) {
	arg = strings.TrimSpace(arg)
	// Talla-ho tot: el torn en marxa i qualsevol pendent d'aquest xat.
	tallats := b.queue.CancelSession(tgScope(chatID))
	st.mu.Lock()
	st.pendingRuns = nil
	// neteja l'estat local: el defer de turnRun farà el mateix quan vegi
	// el context tallat, però aquí assegurem que /status no digui «treballant»
	// mentre la tasca nova espera.
	st.mu.Unlock()
	switch {
	case tallats > 0 && arg != "":
		b.send(ctx, chatID, fmt.Sprintf(b.tr("🛑 Talla el que feia (%d) · ara: %s", "🛑 Cancelled previous work (%d) · now: %s"), tallats, esc(truncate(arg, 120))))
	case tallats > 0:
		b.send(ctx, chatID, fmt.Sprintf(b.tr("🛑 Talla el que feia (%d)", "🛑 Cancelled previous work (%d)"), tallats))
		return
	case arg != "":
		// res en marxa: és simplement una tasca nova
	default:
		b.send(ctx, chatID, b.tr("No hi ha res en marxa.", "Nothing is running."))
		return
	}
	b.turn(ctx, chatID, st, arg)
}

// resolveSessionName tradueix número del llistat o nom a nom de sessió.
func (b *Bot) resolveSessionName(arg string) (string, error) {
	if n, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil {
		infos := b.sortedSessions()
		if n < 1 || n > len(infos) {
			return "", fmt.Errorf("%s", b.tr("número fora de rang · /sessions", "number out of range · /sessions"))
		}
		return infos[n-1].Name, nil
	}
	s, err := session.Load(b.sessionsDir, arg)
	if err != nil {
		return "", err
	}
	return s.Name, nil
}

// renameSession canvia el nom de la sessió actual.
func (b *Bot) renameSession(ctx context.Context, chatID int64, st *chatState, nou string) {
	st.mu.Lock()
	vell, role, convo := st.name, st.role, st.convo
	st.mu.Unlock()
	s, err := session.Load(b.sessionsDir, vell)
	if err == nil && s.Role != "" {
		role = s.Role
	}
	if _, err := session.Save(b.sessionsDir, nou, role, convo); err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut desar la sessió: ", "Could not save the session: ")+esc(err.Error()))
		return
	}
	_ = session.Delete(b.sessionsDir, vell)
	st.mu.Lock()
	st.name = strings.TrimSpace(nou)
	st.mu.Unlock()
	b.send(ctx, chatID, fmt.Sprintf("✏️ "+b.tr("<code>%s</code> → <code>%s</code>", "Renamed <code>%s</code> → <code>%s</code>"), esc(vell), esc(nou)))
}

// deleteSession esborra una sessió desada.
func (b *Bot) deleteSession(ctx context.Context, chatID int64, st *chatState, arg string) {
	name, err := b.resolveSessionName(arg)
	if err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut trobar la sessió: ", "Could not find the session: ")+esc(err.Error()))
		return
	}
	st.mu.Lock()
	cur := st.name
	st.mu.Unlock()
	if name == cur {
		b.send(ctx, chatID, b.tr("no puc esborrar la sessió activa · /new i després /del <code>", "cannot delete the active session · use /new, then /del <code>")+esc(name)+"</code>")
		return
	}
	if err := session.Delete(b.sessionsDir, name); err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut esborrar la sessió: ", "Could not delete the session: ")+esc(err.Error()))
		return
	}
	b.send(ctx, chatID, "🗑 "+b.tr("Esborrada", "Deleted")+" <code>"+esc(name)+"</code>")
}

// clearSession buida la conversa actual (conservant el nom).
func (b *Bot) clearSession(ctx context.Context, chatID int64, st *chatState) {
	st.mu.Lock()
	st.convo = nil
	name := st.name
	role := st.role
	st.mu.Unlock()
	b.saveSession(st)
	b.send(ctx, chatID, "🧹 "+b.tr("Conversa buidada", "Conversation cleared")+" · <code>"+esc(name)+"</code> · "+b.tr("rol", "role")+" <code>"+esc(role)+"</code>")
}

// exportSession envia la sessió com a .md.
func (b *Bot) exportSession(ctx context.Context, chatID int64, st *chatState) {
	st.mu.Lock()
	name, role, convo := st.name, st.role, append([]llm.Message{}, st.convo...)
	st.mu.Unlock()
	var sb strings.Builder
	sb.WriteString("# " + name + "\n\n")
	sb.WriteString(b.tr("projecte ", "project ") + b.dirOf(st) + " · " + b.tr("rol ", "role ") + role + " · " + strconv.Itoa(len(convo)) + " " + b.tr("missatges", "messages") + "\n\n")
	for _, m := range convo {
		if m.Role == "user" {
			sb.WriteString("## " + b.tr("tu", "you") + "\n\n" + m.Content + "\n\n")
		} else if m.Role == "assistant" && strings.TrimSpace(m.Content) != "" {
			sb.WriteString("## gregal\n\n" + m.Content + "\n\n")
		}
	}
	if err := b.api.SendDocument(ctx, chatID, name+".md", []byte(sb.String()),
		"📄 "+b.tr("Sessió", "Session")+" <code>"+esc(name)+"</code> ("+strconv.Itoa(len(convo))+" "+b.tr("missatges", "messages")+")"); err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("exportació: ", "export: ")+esc(err.Error()))
	}
}

// retry torna a enviar l'última petició de l'usuari.
func (b *Bot) retry(ctx context.Context, chatID int64, st *chatState) {
	st.mu.Lock()
	var last string
	for i := len(st.convo) - 1; i >= 0; i-- {
		if st.convo[i].Role == "user" && strings.TrimSpace(st.convo[i].Content) != "" {
			last = st.convo[i].Content
			break
		}
	}
	st.mu.Unlock()
	if last == "" {
		b.send(ctx, chatID, b.tr("no hi ha cap petició per reintentar", "there is no request to retry"))
		return
	}
	b.turn(ctx, chatID, st, last)
}

// summarize demana al rol de xat un resum de la conversa (sense desar-lo).
func (b *Bot) summarize(ctx context.Context, chatID int64, st *chatState) {
	st.mu.Lock()
	convo := append([]llm.Message{}, st.convo...)
	st.mu.Unlock()
	if len(convo) == 0 {
		b.send(ctx, chatID, b.tr("res a resumir encara", "nothing to summarize yet"))
		return
	}
	var sb strings.Builder
	for _, m := range convo {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		sb.WriteString(m.Role + ": " + truncate(m.Content, 500) + "\n")
	}
	p, r := b.roleRefFor(chatID, "chat")
	_ = b.api.SendChatAction(ctx, chatID, "typing")
	sctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	out, _, err := b.client.ChatFO(sctx, b.cfg.PrimTarget(p, r), b.cfg.FallbackTarget(r),
		[]llm.Message{{Role: "user", Content: b.tr("Resumeix aquesta conversa en català, curt i per punts. Conversa:\n", "Summarize this conversation briefly in English as bullet points. Conversation:\n") + sb.String()}},
		r.Temperature, 600,
		func(model string) {
			b.send(ctx, chatID, b.tr("↪ Primari caigut · resumeix el fallback ", "↪ Primary failed · summarizing with fallback ")+esc(model))
		})
	if err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("Error del model: ", "Model error: ")+truncate(err.Error(), 300))
		return
	}
	if strings.TrimSpace(out) == "" {
		b.send(ctx, chatID, b.tr("el model no ha tornat resum", "the model returned no summary"))
		return
	}
	b.send(ctx, chatID, "📝 <b>"+b.tr("Resum", "Summary")+"</b>\n"+renderAnswer(out))
}

// useSession carrega una sessió pel número del llistat o pel nom.
func (b *Bot) useSession(ctx context.Context, chatID int64, st *chatState, arg string) {
	name, err := b.resolveSessionName(arg)
	if err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut trobar la sessió: ", "Could not find the session: ")+esc(err.Error()))
		return
	}
	s, err := session.Load(b.sessionsDir, name)
	if err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut carregar la sessió: ", "Could not load the session: ")+esc(err.Error()))
		return
	}
	b.saveSession(st)
	st.mu.Lock()
	st.name = s.Name
	st.convo = s.Convo
	if s.Role != "" {
		st.role = s.Role
	}
	msgs := len(st.convo)
	mode := st.mode
	st.mu.Unlock()
	b.sendKB(ctx, chatID, fmt.Sprintf(b.tr("▸ Segueixo la sessió <code>%s</code> (%d missatges) · mode %s · escriu i continuem", "▸ Resumed session <code>%s</code> (%d messages) · mode %s · send a message to continue"),
		esc(s.Name), msgs, esc(b.modeBadge(mode))), b.helpKB())
}

// status mostra l'estat del bot.
func (b *Bot) status(ctx context.Context, chatID int64, st *chatState) {
	st.mu.Lock()
	name, mode, role, msgs, busy := st.name, st.mode, st.role, len(st.convo), st.busy
	st.mu.Unlock()
	p, r := b.roleRefFor(chatID, role)
	goals, _ := goal.List(b.goalDirSafe(), b.projectName())
	estat := "🟢 " + b.tr("parat", "idle")
	if busy {
		estat = "🟡 " + b.tr("treballant…", "working…")
	}
	b.sendKB(ctx, chatID, strings.Join([]string{
		"📊 <b>" + b.tr("estat", "status") + "</b> · " + estat,
		b.tr("projecte", "project") + " <code>" + esc(b.projectName()) + "</code>",
		b.tr("mode", "mode") + " " + esc(b.modeBadge(mode)) + " · " + b.tr("rol", "role") + " <code>" + esc(role) + "</code>",
		b.tr("model", "model") + " <code>" + esc(r.Provider) + "/" + esc(r.Model) + "</code>",
		b.tr("lloc", "endpoint") + " <code>" + esc(p.BaseURL) + "</code>",
		b.tr("sessió", "session") + " <code>" + esc(name) + "</code> (" + strconv.Itoa(msgs) + " " + b.tr("missatges", "messages") + ")",
		b.tr("objectius del projecte: ", "project goals: ") + strconv.Itoa(len(goals)),
	}, "\n"), b.statusKB())
}

// goalCommand gestiona els objectius des de Telegram.
func (b *Bot) goalCommand(ctx context.Context, chatID int64, st *chatState, arg string) {
	sub := ""
	id := ""
	if arg != "" {
		parts := strings.SplitN(arg, " ", 2)
		sub = strings.ToLower(parts[0])
		if len(parts) > 1 {
			id = strings.TrimSpace(parts[1])
		}
	}
	switch sub {
	case "", "llista":
		b.goalList(ctx, chatID)

	case "mostra":
		b.goalShow(ctx, chatID, id)

	case "executa", "executar", "run":
		b.goalRun(ctx, chatID, st, id)

	case "esborra", "borra", "delete":
		g, err := b.resolveGoal(id)
		if err != nil {
			b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut trobar l'objectiu: ", "Could not find the goal: ")+esc(err.Error()))
			return
		}
		if err := goal.Delete(b.goalDirSafe(), g.ID); err != nil {
			b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut esborrar l'objectiu: ", "Could not delete the goal: ")+esc(err.Error()))
			return
		}
		b.send(ctx, chatID, "🗑 "+b.tr("Esborrat", "Deleted")+" <code>"+esc(g.ID)+"</code>")

	default:
		b.send(ctx, chatID, b.tr("ús: /goal [llista|executa [id]|mostra [id]|esborra &lt;id&gt;]", "usage: /goal [llista|executa [id]|mostra [id]|esborra &lt;id&gt;] (list, run, show, or delete goals)"))
	}
}

// goalList mostra els objectius amb botons ▶/🔍.
func (b *Bot) goalList(ctx context.Context, chatID int64) {
	goals, _ := goal.List(b.goalDirSafe(), b.projectName())
	if len(goals) == 0 {
		b.send(ctx, chatID, b.tr("Cap objectiu encara.\nPosa'm en mode objectiu (/mode goal), explica què vols fer i et faré les preguntes que calguin.", "No goals yet.\nSwitch to goal mode (/mode goal), describe what you want to do, and I’ll ask any necessary questions."))
		return
	}
	var lines []string
	lines = append(lines, "◆ <b>"+b.tr("Objectius", "Goals")+"</b> · "+esc(b.projectName())+" · "+b.tr("toca ▶ per executar", "tap ▶ to run"))
	for _, g := range goals[:minInt(len(goals), 10)] {
		marca := "·"
		if g.Status == goal.StatusFet {
			marca = "✓"
		}
		lines = append(lines, fmt.Sprintf("%s <code>%s</code> %s", marca, esc(g.ID), esc(truncate(g.Title, 60))))
	}
	lines = append(lines, "", "/goal mostra &lt;id&gt; · /goal esborra &lt;id&gt; · "+b.tr("mostra o esborra un objectiu", "show or delete a goal"))
	b.sendKB(ctx, chatID, strings.Join(lines, "\n"), b.goalsKB(goals))
}

// goalShow mostra un objectiu sencer.
func (b *Bot) goalShow(ctx context.Context, chatID int64, id string) {
	g, err := b.resolveGoal(id)
	if err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut trobar l'objectiu: ", "Could not find the goal: ")+esc(err.Error()))
		return
	}
	kb := Keyboard{{{Text: "▶ " + b.tr("Executa", "Run"), Data: "goal:run:" + g.ID}, {Text: "🗂 " + b.tr("Objectius", "Goals"), Data: "goal:list"}}}
	b.sendKB(ctx, chatID, fmt.Sprintf("◆ <b>%s</b> · <code>%s</code>\n\n%s", esc(g.Title), esc(g.ID), esc(g.Body)), kb)
}

// goalRun executa un objectiu en mode codi.
func (b *Bot) goalRun(ctx context.Context, chatID int64, st *chatState, id string) {
	g, err := b.resolveGoal(id)
	if err != nil {
		b.send(ctx, chatID, "⚠️ "+b.tr("No s'ha pogut trobar l'objectiu: ", "Could not find the goal: ")+esc(err.Error()))
		return
	}
	g.Status = goal.StatusFet
	_ = goal.Save(b.goalDirSafe(), g)
	st.mu.Lock()
	st.mode = agent.ModeCode
	st.mu.Unlock()
	b.send(ctx, chatID, fmt.Sprintf("◆ "+b.tr("Executant l'objectiu <code>%s</code> en mode codi\n%s", "Running goal <code>%s</code> in code mode\n%s"), esc(g.ID), esc(g.Title)))
	b.turn(ctx, chatID, st, goal.Task(g))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// resolveGoal troba l'objectiu per id o el més recent.
func (b *Bot) resolveGoal(id string) (goal.Goal, error) {
	dir := b.goalDirSafe()
	if id != "" {
		return goal.Get(dir, id)
	}
	goals, err := goal.List(dir, b.projectName())
	if err != nil {
		return goal.Goal{}, err
	}
	if len(goals) == 0 {
		return goal.Goal{}, fmt.Errorf("cap objectiu desat")
	}
	return goals[0], nil
}
