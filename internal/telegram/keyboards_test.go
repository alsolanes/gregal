package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/session"
)

// El teclat de rols ha de sortir del config: un rol nou (halogen) només es
// podia triar escrivint /role, perquè la llista era fixa al codi. I el
// reviewer no hi ha de ser mai: no és per triar.
func TestRoleKBSurtDelConfig(t *testing.T) {
	tg := newFakeTG(t)
	llmSrv := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llmSrv, nil)
	bot.cfg.Roles["halogen"] = config.Role{Provider: "chat", Model: "halogen-qwen3.8-flash-next"}
	bot.cfg.Roles["reviewer"] = config.Role{Provider: "chat", Model: "deepseek"}

	var dades []string
	for _, row := range bot.roleKB("halogen") {
		for _, b := range row {
			dades = append(dades, b.Data)
		}
	}
	trobat := false
	for _, d := range dades {
		switch d {
		case "role:halogen":
			trobat = true
		case "role:reviewer":
			t.Fatalf("el reviewer no hauria de ser triable: %v", dades)
		}
	}
	if !trobat {
		t.Fatalf("el rol del config no surt al teclat: %v", dades)
	}
	// Els tres de sempre han de continuar hi, i els primers.
	if len(dades) < 4 || dades[0] != "role:chat" {
		t.Fatalf("s'esperaven els rols de sempre al davant: %v", dades)
	}
}

func cb(chatID, user int64, data string) *CallbackQuery {
	return &CallbackQuery{
		ID:   "cb-" + data,
		From: User{ID: user, Username: "usera"},
		Message: &Message{MessageID: 5, Chat: Chat{ID: chatID, Type: "private"},
			From: &User{ID: 42, Username: "gregal_test_bot"}},
		Data: data,
	}
}

func waitText(t *testing.T, tg *fakeTG, sub string) sentMsg {
	t.Helper()
	m, ok := tg.waitFor(sub, 5*time.Second)
	if !ok {
		t.Fatalf("no ha arribat %q: %+v", sub, tg.all())
	}
	return m
}

func TestModePerBoto(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)

	bot.handleMessage(context.Background(), msg(1234567, "/mode"))
	m, ok := tg.findKB("mode actual")
	if !ok {
		t.Fatalf("sense teclat de mode: %+v", tg.all())
	}
	if len(m.KB) != 1 || len(m.KB[0]) != 5 {
		t.Fatalf("hauria de tenir 5 botons (codi, consulta, xat, objectiu i autònom): %+v", m.KB)
	}
	if m.KB[0][4].Data != "mode:autonomous" {
		t.Fatalf("l'últim botó ha de ser el mode autònom: %+v", m.KB[0])
	}

	bot.handleCallback(context.Background(), cb(1234567, 1234567, "mode:goal"))
	st := bot.state(1234567)
	st.mu.Lock()
	mode := st.mode
	st.mu.Unlock()
	if mode != "goal" {
		t.Fatalf("mode=%q", mode)
	}
	if _, ok := tg.find("objectiu"); !ok {
		t.Fatalf("sense confirmació: %+v", tg.all())
	}
}

func TestBotonsNomésPerLAMo(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)

	bot.handleCallback(context.Background(), cb(1234567, 555, "mode:chat"))
	st := bot.state(1234567)
	st.mu.Lock()
	mode := st.mode
	st.mu.Unlock()
	if mode == "chat" {
		t.Fatal("un usuari sense permís ha canviat el mode")
	}
	tg.mu.Lock()
	texts := append([]string{}, tg.cbTexts...)
	tg.mu.Unlock()
	trobat := false
	for _, x := range texts {
		if strings.Contains(x, "només l'amo") {
			trobat = true
		}
	}
	if !trobat {
		t.Fatalf("sense avís: %v", texts)
	}
}

func TestSessioPerBoto(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, sessionsDir, _ := testBot(t, tg, llm, nil)
	ctx := context.Background()

	if _, err := session.Save(sessionsDir, "primera", "chat", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Save(sessionsDir, "segona", "chat", nil); err != nil {
		t.Fatal(err)
	}

	bot.handleMessage(ctx, msg(1234567, "/sessions"))
	m, ok := tg.findKB("Sessions")
	if !ok {
		t.Fatalf("sense teclat de sessions: %+v", tg.all())
	}
	if len(m.KB) == 0 {
		t.Fatal("cap botó de sessió")
	}

	bot.handleCallback(ctx, cb(1234567, 1234567, "sess:segona"))
	st := bot.state(1234567)
	st.mu.Lock()
	name := st.name
	st.mu.Unlock()
	if name != "segona" {
		t.Fatalf("sessió=%q", name)
	}
}

func TestGoalPerBoto(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("objectiu en marxa"))
	bot, _, goalsDir := testBot(t, tg, llm, nil)
	ctx := context.Background()
	g := goal.Goal{ID: "abc123", Project: bot.projectName(), Title: "Fer la cosa", Body: "tasca: Fer la cosa"}
	if err := goal.Save(goalsDir, g); err != nil {
		t.Fatal(err)
	}

	bot.handleMessage(ctx, msg(1234567, "/goal"))
	m, ok := tg.findKB("Objectius")
	if !ok {
		t.Fatalf("sense teclat d'objectius: %+v", tg.all())
	}
	trobat := false
	for _, row := range m.KB {
		for _, b := range row {
			if b.Data == "goal:run:abc123" {
				trobat = true
			}
		}
	}
	if !trobat {
		t.Fatalf("sense botó ▶: %+v", m.KB)
	}

	bot.handleCallback(ctx, cb(1234567, 1234567, "goal:run:abc123"))
	waitText(t, tg, "Executant l'objectiu")
	st := bot.state(1234567)
	st.mu.Lock()
	mode := st.mode
	st.mu.Unlock()
	if mode != "code" {
		t.Fatalf("mode=%q", mode)
	}
	if g2, err := goal.Get(goalsDir, "abc123"); err != nil || g2.Status != goal.StatusFet {
		t.Fatalf("hauria de quedar fet: %+v %v", g2.Status, err)
	}
}

func TestRenameDelClear(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, sessionsDir, _ := testBot(t, tg, llm, nil)
	ctx := context.Background()

	bot.handleMessage(ctx, msg(1234567, "/rename feina-nova"))
	st := bot.state(1234567)
	st.mu.Lock()
	name := st.name
	st.mu.Unlock()
	if name != "feina-nova" {
		t.Fatalf("nom=%q", name)
	}
	if _, err := session.Load(sessionsDir, "feina-nova"); err != nil {
		t.Fatalf("no desada amb el nom nou: %v", err)
	}

	if _, err := session.Save(sessionsDir, "altra", "chat", nil); err != nil {
		t.Fatal(err)
	}
	bot.handleMessage(ctx, msg(1234567, "/del altra"))
	if _, err := session.Load(sessionsDir, "altra"); err == nil {
		t.Fatal("no s'ha esborrat")
	}

	bot.handleMessage(ctx, msg(1234567, "recorda això"))
	bot.handleMessage(ctx, msg(1234567, "/clear"))
	st.mu.Lock()
	n := len(st.convo)
	st.mu.Unlock()
	if n != 0 {
		t.Fatalf("conversa no buida: %d", n)
	}
}

func TestRol(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	ctx := context.Background()

	bot.handleMessage(ctx, msg(1234567, "/role"))
	if _, ok := tg.findKB("Rol actual"); !ok {
		t.Fatalf("sense teclat de rol: %+v", tg.all())
	}
	bot.handleCallback(ctx, cb(1234567, 1234567, "role:think"))
	st := bot.state(1234567)
	st.mu.Lock()
	role := st.role
	st.mu.Unlock()
	if role != "think" {
		t.Fatalf("rol=%q", role)
	}

	bot.handleMessage(ctx, msg(1234567, "/role reviewer"))
	if _, ok := tg.find("només per al revisor"); !ok {
		t.Fatalf("hauria de rebutjar reviewer: %+v", tg.all())
	}
}

func TestRetryIResum(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("primera"), replyText("resum fet"))
	bot, _, _ := testBot(t, tg, llm, nil)
	ctx := context.Background()

	bot.handleMessage(ctx, msg(1234567, "quant és 2+2?"))
	waitText(t, tg, "primera")
	bot.handleMessage(ctx, msg(1234567, "/retry"))
	waitText(t, tg, "primera")
	llm.mu.Lock()
	calls := llm.calls
	llm.mu.Unlock()
	if calls < 2 {
		t.Fatalf("retry no ha cridat el model (%d)", calls)
	}

	bot.handleMessage(ctx, msg(1234567, "/resum"))
	waitText(t, tg, "Resum")
}

func TestExportIPerms(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("hola"))
	bot, _, _ := testBot(t, tg, llm, nil)
	ctx := context.Background()

	bot.handleMessage(ctx, msg(1234567, "apunta això"))
	waitText(t, tg, "hola")
	bot.handleMessage(ctx, msg(1234567, "/export"))
	deadline := time.Now().Add(5 * time.Second)
	var doc sentDoc
	for time.Now().Before(deadline) {
		tg.mu.Lock()
		if len(tg.docs) > 0 {
			doc = tg.docs[len(tg.docs)-1]
		}
		tg.mu.Unlock()
		if doc.Filename != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.HasSuffix(doc.Filename, ".md") || doc.Size == 0 {
		t.Fatalf("document inesperat: %+v", doc)
	}

	bot.handleMessage(ctx, msg(1234567, "/perms"))
	if _, ok := tg.find("bash"); !ok {
		t.Fatalf("sense permisos: %+v", tg.all())
	}
}

func TestMenuDOrdres(t *testing.T) {
	tg := newFakeTG(t)
	menu := botMenu()
	if len(menu) < 10 {
		t.Fatalf("menú massa curt: %d", len(menu))
	}
	vistes := map[string]bool{}
	for _, c := range menu {
		if c.Command == "" || c.Description == "" {
			t.Fatalf("ordre buida: %+v", c)
		}
		vistes[c.Command] = true
	}
	for _, want := range []string{"new", "sessions", "mode", "role", "goal", "status", "stop", "verify", "export", "help"} {
		if !vistes[want] {
			t.Fatalf("falta /%s al menú", want)
		}
	}
	if err := (&API{Token: "123:abc", BaseURL: tg.srv.URL, HTTP: tg.srv.Client()}).SetMyCommands(context.Background(), menu); err != nil {
		t.Fatalf("setMyCommands: %v", err)
	}
}

func TestNomDelBotAmbCau(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)

	m := msg(1234567, "hola @gregal_test_bot, em sents?")
	m.Chat = Chat{ID: 1234567, Type: "group"}
	if !bot.addressed(m) {
		t.Fatal("hauria de detectar la menció")
	}
	if !bot.addressed(m) {
		t.Fatal("segona crida")
	}
	tg.mu.Lock()
	n := tg.meCalls
	tg.mu.Unlock()
	if n != 1 {
		t.Fatalf("getMe cridat %d vegades (hauria de ser 1)", n)
	}

	// Resposta a un missatge del bot també compta.
	m2 := msg(1234567, "i això?")
	m2.Chat = Chat{ID: 1234567, Type: "group"}
	m2.ReplyTo = &Message{From: &User{ID: 42, Username: "GREGAL_TEST_BOT"}}
	if !bot.addressed(m2) {
		t.Fatal("hauria de detectar la resposta al bot")
	}
}

func TestRespostaMarkdownRenderitzada(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("**hola** amb `codi` i [enllaç](https://exemple.test)"))
	bot, _, _ := testBot(t, tg, llm, nil)

	bot.handleMessage(context.Background(), msg(1234567, "saluda"))
	m := waitText(t, tg, "hola")
	if !strings.Contains(m.Text, "<b>hola</b>") || !strings.Contains(m.Text, "<code>codi</code>") {
		t.Fatalf("sense renderitzar: %q", m.Text)
	}
	if strings.Contains(m.Text, "**hola**") {
		t.Fatalf("markdown cru: %q", m.Text)
	}
}

func TestRespostaLlargaVaEnFitxer(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText(strings.Repeat("paraula ", 900)))
	bot, _, _ := testBot(t, tg, llm, nil)

	bot.handleMessage(context.Background(), msg(1234567, "escriu molt"))
	deadline := time.Now().Add(8 * time.Second)
	var doc sentDoc
	for time.Now().Before(deadline) {
		tg.mu.Lock()
		for _, d := range tg.docs {
			if d.Filename == "resposta.md" {
				doc = d
			}
		}
		tg.mu.Unlock()
		if doc.Filename != "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if doc.Filename == "" {
		t.Fatalf("sense fitxer de resposta: %+v", tg.all())
	}
	if doc.Size < 3900 {
		t.Fatalf("fitxer massa curt: %+v", doc)
	}
}

func TestMissatgeEditatEsProcessa(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("resposta del bucle"))
	bot, _, _ := testBot(t, tg, llm, nil)

	tg.mu.Lock()
	tg.updates = []map[string]any{{
		"update_id": 9,
		"edited_message": map[string]any{
			"message_id": 2,
			"chat":       map[string]any{"id": 1234567, "type": "private"},
			"from":       map[string]any{"id": 1234567, "username": "usera"},
			"text":       "/status",
		},
	}}
	tg.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { _ = bot.Run(ctx); close(done) }()

	deadline := time.After(2500 * time.Millisecond)
	for {
		if _, ok := tg.find("estat"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("l'editat no s'ha processat: %+v", tg.all())
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-done
}
