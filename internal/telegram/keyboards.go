// Package telegram — keyboards.go: teclats en línia i menú d'ordres.
//
// Tota acció habitual (sessions, mode, rol, objectius, estat) es pot fer
// tocant botons, sense memoritzar la sintaxi. Les dades de callback fan
// 64 bytes com a màxim (límit de Telegram).
package telegram

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/goal"
	"gregal/internal/session"
)

// botMenu és el menú d'ordres que veu l'usuari en prémer "/" al client.
func botMenu() []BotCommand {
	return []BotCommand{
		{"a", "tasca per l'agent (als grups)"},
		{"new", "sessió nova"},
		{"sessions", "sessions desades (les del TUI)"},
		{"use", "continua una sessió: /use <n|nom>"},
		{"mode", "canvia de mode: /mode <code|inspect|chat|goal>"},
		{"role", "canvia de rol: /role <chat|think|code>"},
		{"model", "tria el model en viu: /model"},
		{"goal", "objectius: llista, executa, mostra, esborra"},
		{"status", "estat del projecte, model i sessió"},
		{"stop", "atura la feina en curs"},
		{"ara", "talla la feina anterior i fes això: /ara <tasca>"},
		{"rewind", "desfés els canvis de fitxers de la sessió"},
		{"plan", "explora sense actuar i proposa un pla: /plan <tasca>"},
		{"verify", "passa el darrer diff al revisor"},
		{"resum", "resumeix la conversa actual"},
		{"retry", "torna a intentar l'última petició"},
		{"rename", "canvia el nom: /rename <nom>"},
		{"clear", "buida la conversa actual"},
		{"export", "descarrega la sessió (.md)"},
		{"perms", "permisos d'eines actius"},
		{"whoami", "el teu id d'usuari i de xat"},
		{"help", "ajuda"},
	}
}

func (b *Bot) botMenu() []BotCommand {
	menu := botMenu()
	en := []string{
		"send a task to the agent (in groups)", "start a session", "saved sessions (shared with the TUI)",
		"resume a session: /use <n|name>", "change mode: /mode <code|inspect|chat|goal>",
		"change role: /role <chat|think|code>", "choose a live model: /model", "manage goals: /goal, executa, mostra, esborra",
		"project, model, and session status", "stop current work", "cancel previous work and do this: /ara <task>",
		"undo this session's file changes", "inspect without acting and propose a plan: /plan <task>",
		"send the latest diff to the reviewer", "summarize the current conversation", "retry the last request",
		"rename: /rename <name>", "clear the current conversation", "download the session (.md)",
		"active tool permissions", "show your user and chat IDs", "help",
	}
	for i := range menu {
		menu[i].Description = b.tr(menu[i].Description, en[i])
	}
	return menu
}

// sendKB envia text amb teclat en línia.
func (b *Bot) sendKB(ctx context.Context, chatID int64, text string, kb Keyboard) {
	b.logReply(chatID, text)
	for _, chunk := range splitMessage(text, 3900) {
		if _, err := b.api.SendMessage(ctx, chatID, chunk, kb); err != nil {
			b.logf("sendMessage (teclat): %v", err)
			return
		}
		// El teclat només al primer tros.
		kb = nil
	}
}

// helpKB són les dreceres sota l'ajuda i l'estat.
func (b *Bot) helpKB() Keyboard {
	return Keyboard{
		{{Text: "🗂 " + b.tr("Sessions", "Sessions"), Data: "sess:list"}, {Text: "🆕 " + b.tr("Nova", "New"), Data: "new"}},
		{{Text: "◆ " + b.tr("Mode", "Mode"), Data: "mode:list"}, {Text: "🧠 " + b.tr("Rol", "Role"), Data: "role:list"}},
		{{Text: "🧠 " + b.tr("Model", "Model"), Data: "m:list"}},
		{{Text: "◆ " + b.tr("Objectius", "Goals"), Data: "goal:list"}, {Text: "📊 " + b.tr("Estat", "Status"), Data: "status"}},
	}
}

// modeKB tria els modes (l'actual marcat amb ●). L'autònom hi surt com un
// botó més perquè el bot pugui treballar desatès amb el model local; va a
// l'últim lloc perquè és el que més canvia el comportament i no s'ha de
// tocar per accident buscant els de sempre.
func (b *Bot) modeKB(current string) Keyboard {
	mark := func(m string) string {
		if m == current {
			return "● "
		}
		return ""
	}
	return Keyboard{{
		{Text: mark(agent.ModeCode) + "◆ " + b.tr("Codi", "Code"), Data: "mode:" + agent.ModeCode},
		{Text: mark(agent.ModeInspect) + "◉ " + b.tr("Consulta", "Inspect"), Data: "mode:" + agent.ModeInspect},
		{Text: mark(agent.ModeChat) + "◇ " + b.tr("Xat", "Chat"), Data: "mode:" + agent.ModeChat},
		{Text: mark(agent.ModeGoal) + "◆ " + b.tr("Objectiu", "Goal"), Data: "mode:" + agent.ModeGoal},
		{Text: mark(agent.ModeAutonomous) + "≋ " + b.tr("Autònom", "Autonomous"), Data: "mode:" + agent.ModeAutonomous},
	}}
}

// roleKB tria el rol actiu (el model que respon).
//
// La llista surt del config, no del codi: abans era ["chat","think","code"]
// fix i un rol nou (halogen) no sortia mai al teclat tot i que /role
// l'acceptava — des de la pantalla semblava que no existís. Els tres primers
// es mantenen en el seu ordre per no moure els botons de sempre; la resta hi
// van al darrere, ordenats, i el reviewer no s'hi posa mai (no és per triar).
func (b *Bot) roleKB(current string) Keyboard {
	var noms []string
	vist := map[string]bool{}
	for _, n := range []string{"chat", "think", "code"} {
		if r, ok := b.cfg.Roles[n]; ok && strings.TrimSpace(r.Model) != "" {
			noms = append(noms, n)
			vist[n] = true
		}
	}
	var resta []string
	for n, r := range b.cfg.Roles {
		if vist[n] || n == "reviewer" || strings.TrimSpace(r.Model) == "" {
			continue
		}
		resta = append(resta, n)
	}
	sort.Strings(resta)
	noms = append(noms, resta...)

	var kb Keyboard
	var row []InlineButton
	for _, n := range noms {
		r := b.cfg.Roles[n]
		label := n
		if n == current {
			label = "● " + n
		}
		row = append(row, InlineButton{Text: label + " (" + r.Model + ")", Data: "role:" + n})
		if len(row) == 2 {
			kb = append(kb, row)
			row = nil
		}
	}
	if len(row) > 0 {
		kb = append(kb, row)
	}
	return kb
}

// sessionsKB llista les sessions amb botons (8 per pàgina).
func (b *Bot) sessionsKB(infos []session.Info, current string, page int) Keyboard {
	const per = 8
	var kb Keyboard
	start := page * per
	if start > len(infos) {
		start = 0
	}
	end := start + per
	if end > len(infos) {
		end = len(infos)
	}
	for _, s := range infos[start:end] {
		label := s.Name
		if s.Name == current {
			label = "▸ " + s.Name
		}
		kb = append(kb, []InlineButton{{Text: "▸ " + truncate(label, 40), Data: "sess:" + s.Name}})
	}
	var nav []InlineButton
	if page > 0 {
		nav = append(nav, InlineButton{Text: "◀", Data: "sess:page:" + itoa(page-1)})
	}
	if end < len(infos) {
		nav = append(nav, InlineButton{Text: "▶", Data: "sess:page:" + itoa(page+1)})
	}
	if len(nav) > 0 {
		kb = append(kb, nav)
	}
	kb = append(kb, []InlineButton{{Text: "🆕 " + b.tr("Nova sessió", "New session"), Data: "new"}})
	return kb
}

// goalsKB llista els objectius: ▶ executa, 🔍 mostra.
func (b *Bot) goalsKB(goals []goal.Goal) Keyboard {
	var kb Keyboard
	for _, g := range goals {
		if len(kb) >= 10 {
			break
		}
		marca := "· "
		if g.Status == goal.StatusFet {
			marca = "✓ "
		}
		titol := truncate(marca+g.Title, 28)
		kb = append(kb, []InlineButton{
			{Text: "▶ " + titol, Data: "goal:run:" + g.ID},
			{Text: "🔍", Data: "goal:show:" + g.ID},
		})
	}
	kb = append(kb, []InlineButton{{Text: "↻ " + b.tr("Actualitza", "Refresh"), Data: "goal:list"}})
	return kb
}

// statusKB refresca o atura des de l'estat.
func (b *Bot) statusKB() Keyboard {
	return Keyboard{{
		{Text: "↻ " + b.tr("Actualitza", "Refresh"), Data: "status"},
		{Text: "⏹ " + b.tr("Atura", "Stop"), Data: "stop"},
		{Text: "✅ " + b.tr("Revisa", "Review"), Data: "verify"},
	}}
}

// stopKB és el botó sota "encara treballo". El botó «Ara» obre el composer
// amb /ara ja escrit: qui el pitja només hi afegeix la tasca.
func (b *Bot) stopKB() Keyboard {
	return Keyboard{{
		{Text: "⏹ " + b.tr("Atura", "Stop"), Data: "stop"},
		{Text: "⚡ " + b.tr("Ara…", "Now…"), Data: "ara"},
	}}
}

// chatOf extreu el xat del callback (nil si no n'hi ha).
func chatOf(c *CallbackQuery) int64 {
	if c.Message != nil {
		return c.Message.Chat.ID
	}
	return 0
}

// handleCallback atén tots els botons excepte els d'aprovació (ap:).
func (b *Bot) handleCallback(ctx context.Context, c *CallbackQuery) {
	data := c.Data
	if strings.HasPrefix(data, "ap:") {
		b.handleApproval(ctx, c)
		return
	}
	chatID := chatOf(c)
	if chatID == 0 {
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("error intern", "internal error"))
		return
	}
	// Les accions que només llegeixen les pot fer tothom; les que canvien
	// l'estat del xat, només els autoritzats.
	readonly := data == "status" || data == "help" ||
		strings.HasPrefix(data, "sess:list") || strings.HasPrefix(data, "sess:page:") ||
		data == "goal:list" || strings.HasPrefix(data, "goal:show:") ||
		data == "mode:list" || data == "role:list"
	if !readonly && !b.canApprove(&c.From) {
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("només l'amo pot decidir", "only the owner can decide"))
		return
	}
	_ = b.api.AnswerCallbackQuery(ctx, c.ID, "")
	st := b.state(chatID)

	switch {
	case data == "help":
		b.sendKB(ctx, chatID, b.help(st), b.helpKB())
	case data == "new":
		b.newSession(ctx, chatID, st, "")
	case data == "status":
		b.status(ctx, chatID, st)
	case data == "stop":
		b.stop(ctx, chatID, st)
	case data == "ara":
		// Obre el composer amb /ara escrit: la tasca s'hi afegeix i s'envia.
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("escriu la tasca nova", "enter the new task"))
		b.send(ctx, chatID, "⚡ "+b.tr("Escriu la tasca nova: /ara &lt;tasca&gt; — tallaré la feina anterior", "Enter a task: /ara &lt;task&gt; — previous work will be cancelled"))
	case data == "verify":
		b.verify(ctx, chatID, st)
	case data == "mode:list":
		b.sendKB(ctx, chatID, b.tr("Mode actual: ", "Current mode: ")+esc(b.modeBadge(st.mode))+"\n"+esc(b.modeHint(st.mode)), b.modeKB(st.mode))
	case data == "role:list":
		b.sendKB(ctx, chatID, b.roleText(chatID, st), b.roleKB(st.role))
	case strings.HasPrefix(data, "mode:"):
		b.setMode(ctx, chatID, st, strings.TrimPrefix(data, "mode:"), chatTypeOf(c), c.From.ID)
	case strings.HasPrefix(data, "role:"):
		b.setRole(ctx, chatID, st, strings.TrimPrefix(data, "role:"))
	case data == "m:list":
		b.modelList(ctx, chatID, st)
	case data == "m:clear":
		st.mu.Lock()
		role := st.role
		st.mu.Unlock()
		b.setOverride(chatID, role, "")
		b.sendKB(ctx, chatID, "↺ "+b.tr("Model per defecte del rol", "Role default model")+"\n"+b.roleText(chatID, st), b.roleKB(role))
	case strings.HasPrefix(data, "m:"):
		b.modelTap(ctx, chatID, st, data)
	case data == "sess:list" || strings.HasPrefix(data, "sess:page:"):
		page := 0
		if strings.HasPrefix(data, "sess:page:") {
			page, _ = strconv.Atoi(strings.TrimPrefix(data, "sess:page:"))
		}
		b.listSessionsPage(ctx, chatID, st, page)
	case strings.HasPrefix(data, "sess:"):
		b.useSession(ctx, chatID, st, strings.TrimPrefix(data, "sess:"))
	case data == "goal:list":
		b.goalList(ctx, chatID)
	case strings.HasPrefix(data, "goal:show:"):
		b.goalShow(ctx, chatID, strings.TrimPrefix(data, "goal:show:"))
	case strings.HasPrefix(data, "goal:run:"):
		b.goalRun(ctx, chatID, st, strings.TrimPrefix(data, "goal:run:"))
	case data == "plan:run":
		b.planRun(ctx, chatID, st)
	case data == "plan:drop":
		b.planDrop(ctx, chatID)
	default:
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("botó desconegut", "unknown button"))
	}
}

// chatTypeOf diu el tipus de xat del callback.
func chatTypeOf(c *CallbackQuery) string {
	if c.Message != nil {
		return c.Message.Chat.Type
	}
	return "private"
}

// handleApproval és l'antiga branca ap: (permisos d'eines).
func (b *Bot) handleApproval(ctx context.Context, c *CallbackQuery) {
	parts := strings.Split(c.Data, ":")
	if len(parts) != 3 {
		return
	}
	always := parts[2] == "always"
	ok := parts[2] == "ok" || always
	if !b.canApprove(&c.From) {
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("només l'amo pot decidir", "only the owner can decide"))
		return
	}
	b.mu.Lock()
	ar, found := b.approve[parts[1]]
	if found {
		delete(b.approve, parts[1])
	}
	b.mu.Unlock()
	if !found {
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("ja no cal", "no longer needed"))
		return
	}
	if always && ok {
		b.remembers.Allow(ar.scope, ar.sig)
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("permès sempre en aquesta sessió", "always allowed for this session"))
	} else if ok {
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("permès", "allowed"))
	} else {
		_ = b.api.AnswerCallbackQuery(ctx, c.ID, b.tr("denegat", "denied"))
	}
	ar.ch <- approveRes{ok: ok, always: always}
}

// sortedSessions retorna les sessions ordenades (recents primer).
func (b *Bot) sortedSessions() []session.Info {
	infos, _ := session.List(b.sessionsDir)
	sort.Slice(infos, func(i, j int) bool { return infos[i].SavedAt.After(infos[j].SavedAt) })
	return infos
}

// listSessionsPage mostra una pàgina de sessions amb botons.
func (b *Bot) listSessionsPage(ctx context.Context, chatID int64, st *chatState, page int) {
	infos := b.sortedSessions()
	if len(infos) == 0 {
		b.send(ctx, chatID, b.tr("Cap sessió desada encara · /new per començar-ne una", "No saved sessions yet · use /new to start one"))
		return
	}
	st.mu.Lock()
	cur := st.name
	st.mu.Unlock()
	var lines []string
	lines = append(lines, "🗂 <b>"+b.tr("Sessions", "Sessions")+"</b> ("+b.tr("mateix magatzem que el TUI · toca per continuar-ne una", "shared with the TUI · tap one to resume")+")")
	const per = 8
	start := page * per
	if start >= len(infos) {
		start, page = 0, 0
	}
	end := start + per
	if end > len(infos) {
		end = len(infos)
	}
	for i, s := range infos[start:end] {
		marca := "  "
		if s.Name == cur {
			marca = "▸ "
		}
		lines = append(lines, fmt.Sprintf("%s%d. <code>%s</code> · %d "+b.tr("msgs", "messages")+" · %s",
			marca, start+i+1, esc(s.Name), s.Msgs, s.SavedAt.Format("02/01 15:04")))
	}
	b.sendKB(ctx, chatID, strings.Join(lines, "\n"), b.sessionsKB(infos, cur, page))
}
