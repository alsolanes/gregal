package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/session"
)

// --- dobles de la Bot API i del model ---

type sentMsg struct {
	ChatID int64
	Text   string
	KB     Keyboard
	Edit   bool
	MsgID  int
}

type sentDoc struct {
	ChatID   int64
	Filename string
	Size     int
	Caption  string
}

type fakeTG struct {
	srv     *httptest.Server
	mu      sync.Mutex
	updates []map[string]any
	sent    []sentMsg
	docs    []sentDoc
	cbs     []string
	cbTexts []string
	me      string
	meCalls int
	// Fitxer servit per a getFile + descàrrega (fotos en tests).
	filePath  string
	fileBytes []byte
}

func newFakeTG(t *testing.T) *fakeTG {
	t.Helper()
	f := &fakeTG{me: "gregal_test_bot"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		f.mu.Lock()
		defer f.mu.Unlock()
		// Descàrrega de fitxers: GET /file/bot<token>/<path>.
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/file/bot") {
			w.Write(f.fileBytes)
			return
		}
		switch method {
		case "getMe":
			f.meCalls++
			fmt.Fprintf(w, `{"ok":true,"result":{"id":42,"username":"%s","first_name":"Gregal"}}`, f.me)
		case "getUpdates":
			ups := f.updates
			f.updates = nil
			raw, _ := json.Marshal(ups)
			fmt.Fprintf(w, `{"ok":true,"result":%s}`, raw)
		case "sendMessage":
			chat, _ := p["chat_id"].(float64)
			text, _ := p["text"].(string)
			var kb Keyboard
			if rm, ok := p["reply_markup"].(map[string]any); ok {
				raw, _ := json.Marshal(rm["inline_keyboard"])
				_ = json.Unmarshal(raw, &kb)
			}
			if _, ok := p["parse_mode"]; !ok {
				t.Errorf("sendMessage sense parse_mode: %v", p)
			}
			f.sent = append(f.sent, sentMsg{ChatID: int64(chat), Text: text, KB: kb})
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"chat":{"id":%d,"type":"private"}}}`,
				len(f.sent), int64(chat))
		case "editMessageText":
			chat, _ := p["chat_id"].(float64)
			id, _ := p["message_id"].(float64)
			text, _ := p["text"].(string)
			f.sent = append(f.sent, sentMsg{ChatID: int64(chat), Text: text, Edit: true, MsgID: int(id)})
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
		case "getFile":
			fmt.Fprintf(w, `{"ok":true,"result":{"file_path":%q}}`, f.filePath)
		case "sendChatAction", "answerCallbackQuery":
			if method == "answerCallbackQuery" {
				if id, _ := p["callback_query_id"].(string); id != "" {
					f.cbs = append(f.cbs, id)
				}
				if tx, _ := p["text"].(string); tx != "" {
					f.cbTexts = append(f.cbTexts, tx)
				}
			}
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		case "sendDocument": // Multipart: el nom i el subtítol van com a camps del formulari.
			ct := r.Header.Get("Content-Type")
			mr := multipart.NewReader(strings.NewReader(string(body)), ct[strings.Index(ct, "boundary=")+9:])
			var doc sentDoc
			for {
				part, err := mr.NextPart()
				if err != nil {
					break
				}
				b, _ := io.ReadAll(part)
				switch part.FormName() {
				case "chat_id":
					fmt.Sscanf(string(b), "%d", &doc.ChatID)
				case "caption":
					doc.Caption = string(b)
				case "document":
					doc.Filename = part.FileName()
					doc.Size = len(b)
				}
			}
			f.docs = append(f.docs, doc)
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":99}}`)
		default:
			fmt.Fprint(w, `{"ok":true,"result":{}}`)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// last retorna l'últim missatge enviat que contingui el text donat.
func (f *fakeTG) find(sub string) (sentMsg, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.sent) - 1; i >= 0; i-- {
		if strings.Contains(f.sent[i].Text, sub) {
			return f.sent[i], true
		}
	}
	return sentMsg{}, false
}

// findKB cerca l'últim missatge amb botons que contingui el text.
func (f *fakeTG) findKB(sub string) (sentMsg, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.sent) - 1; i >= 0; i-- {
		if len(f.sent[i].KB) > 0 && strings.Contains(f.sent[i].Text, sub) {
			return f.sent[i], true
		}
	}
	return sentMsg{}, false
}

// waitFor espera que aparegui un missatge que contingui el text.
func (f *fakeTG) waitFor(sub string, d time.Duration) (sentMsg, bool) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if m, ok := f.find(sub); ok {
			return m, true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return sentMsg{}, false
}

func (f *fakeTG) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func (f *fakeTG) all() []sentMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMsg{}, f.sent...)
}

// fakeLLM retorna respostes prefixades per torn.
type fakeLLM struct {
	srv     *httptest.Server
	mu      sync.Mutex
	replies []string
	bodies  []string
	calls   int
}

func newFakeLLM(t *testing.T, replies ...string) *fakeLLM {
	t.Helper()
	f := &fakeLLM{replies: replies}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.bodies = append(f.bodies, string(body))
		i := f.calls
		f.calls++
		if i >= len(f.replies) {
			i = len(f.replies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":%s,"finish_reason":"stop"}]}`, f.replies[i])
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func replyText(text string) string {
	raw, _ := json.Marshal(map[string]any{"role": "assistant", "content": text})
	return string(raw)
}

func replyToolCall(name, args string) string {
	raw, _ := json.Marshal(map[string]any{
		"role": "assistant", "content": "",
		"tool_calls": []map[string]any{{
			"id": "c1", "type": "function",
			"function": map[string]any{"name": name, "arguments": args},
		}},
	})
	return string(raw)
}

// testBot munta el bot amb els dobles.
func testBot(t *testing.T, tg *fakeTG, llm *fakeLLM, perms map[string]string) (*Bot, string, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Language:  "ca",
		Providers: map[string]config.Provider{"t": {BaseURL: llm.srv.URL}},
		Roles: map[string]config.Role{
			"chat":  {Provider: "t", Model: "m", MaxTokens: 100},
			"think": {Provider: "t", Model: "mt", MaxTokens: 100},
			"code":  {Provider: "t", Model: "m", MaxTokens: 100},
		},
		Permissions: config.PermissionsCfg{Tools: perms},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 4},
		Mode:        "code",
	}
	goalsDir := filepath.Join(dir, "cfg")
	sessionsDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(goalsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	bot := New(Options{
		Cfg: cfg, CfgPath: filepath.Join(goalsDir, "config.yaml"), Cwd: dir,
		Token: "123:abc", AllowedUsers: []int64{1234567}, AllowGroups: true,
		SessionsDir: sessionsDir, GoalsDir: goalsDir,
	})
	bot.api = &API{Token: "123:abc", BaseURL: tg.srv.URL, HTTP: tg.srv.Client()}
	return bot, sessionsDir, goalsDir
}

func TestTelegramEnglishDefault(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("ok"))
	configured := New(Options{Cfg: &config.Config{Language: "ca"}, SessionsDir: filepath.Join(t.TempDir(), "sessions")})
	if got := configured.api.tr("massa gran", "too large"); got != "massa gran" {
		t.Errorf("Catalan API error text = %q", got)
	}
	if got := NewAPI("token").tr("massa gran", "too large"); got != "too large" {
		t.Errorf("default API error text = %q", got)
	}
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.cfg.Language = ""

	help := bot.help(&chatState{mode: "code", role: "chat", name: "test"})
	for _, want := range []string{"coding agent", "project", "Send a message", "Only the owner can decide"} {
		if !strings.Contains(help, want) {
			t.Errorf("English help missing %q: %s", want, help)
		}
	}
	if strings.Contains(help, "agent de codi") {
		t.Errorf("English help contains Catalan: %s", help)
	}
	if got := bot.modeBadge("autonomous"); got != "≋ autonomous" {
		t.Errorf("English mode badge = %q", got)
	}
	if got := bot.permsText(); !strings.Contains(got, "Permissions") || !strings.Contains(got, "without prompting") {
		t.Errorf("English permissions text = %s", got)
	}
	menu := bot.botMenu()
	if menu[1].Description != "start a session" {
		t.Errorf("English command menu = %q", menu[1].Description)
	}
	modelKB := bot.modelKB(1, "chat", []ModelRef{{Provider: "t", Model: "offline", Unavailable: true}}, nil)
	if !strings.HasSuffix(modelKB[0][0].Text, "(unavailable)") || modelKB[len(modelKB)-1][0].Text != "↺ Default" {
		t.Errorf("English model keyboard = %+v", modelKB)
	}
	if got := bot.planKB(); got[0][0].Text != "▶ Run" || got[0][1].Text != "Discard" {
		t.Errorf("English plan keyboard = %+v", got)
	}
	if got := bot.goalsKB([]goal.Goal{{ID: "g1", Title: "goal"}}); got[len(got)-1][0].Text != "↻ Refresh" {
		t.Errorf("English goal keyboard = %+v", got)
	}
	if got := bot.routeReason("router: feina pesada → rol code"); got != "router: complex task → role code" {
		t.Errorf("English route reason = %q", got)
	}
	if got := bot.eventMessage(agent.Event{Clau: "app.ampliaSegueix", Args: []any{2, 5}}); got != "continuing with 2 more steps (now 5)" {
		t.Errorf("English progress event = %q", got)
	}
	bot.handleMessage(context.Background(), msg(999, "hello"))
	if _, ok := tg.find("Not authorized."); !ok {
		t.Errorf("English unauthorized response missing: %+v", tg.all())
	}
	bot.handleCallback(context.Background(), cb(999, 999, "stop"))
	tg.mu.Lock()
	callbackText := tg.cbTexts[len(tg.cbTexts)-1]
	tg.mu.Unlock()
	if callbackText != "only the owner can decide" {
		t.Errorf("English permission denial = %q", callbackText)
	}
	bot.cfg.Language = "ca"
	if got := bot.modeBadge("autonomous"); got != "≋ autònom" {
		t.Errorf("Catalan mode badge = %q", got)
	}
	if got := bot.botMenu(); got[1].Description != "sessió nova" {
		t.Errorf("Catalan command menu = %q", got[1].Description)
	}
	if got := bot.planKB(); got[0][0].Text != "▶ Executa" || got[0][1].Text != "Descarta" {
		t.Errorf("Catalan plan keyboard = %+v", got)
	}
	if got := bot.eventMessage(agent.Event{Clau: "app.ampliaSegueix", Args: []any{2, 5}}); got != "continuo amb 2 passos més (ara 5)" {
		t.Errorf("Catalan progress event = %q", got)
	}
}

func msg(user int64, text string) *Message {
	return &Message{MessageID: 1, Chat: Chat{ID: user, Type: "private"}, From: &User{ID: user, Username: "usera"}, Text: text}
}

// --- proves ---

func TestNoAutoritzatNoRepCapResposta(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("no hauria de passar"))
	bot, _, _ := testBot(t, tg, llm, nil)

	bot.handleMessage(context.Background(), msg(999, "hola"))

	llm.mu.Lock()
	calls := llm.calls
	llm.mu.Unlock()
	if calls != 0 {
		t.Fatalf("el model no s'hauria d'haver cridat (%d)", calls)
	}
	if _, ok := tg.find("No autoritzat"); !ok {
		t.Fatal("esperava un avís de no autoritzat")
	}
}

func TestSalutacioViaRapidaSenseModel(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("no hauria de passar"))
	bot, _, _ := testBot(t, tg, llm, nil)

	bot.handleMessage(context.Background(), msg(1234567, "hola"))

	llm.mu.Lock()
	calls := llm.calls
	llm.mu.Unlock()
	if calls != 0 {
		t.Fatalf("un hola no pot cridar el model (%d)", calls)
	}
	if _, ok := tg.find("Sóc el Gregal"); !ok {
		t.Fatalf("esperava la salutació directa: %+v", tg.all())
	}
}

func TestTornDeAgentResponIDesaLaSessio(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("hola, sóc el gregal"))
	bot, sessionsDir, _ := testBot(t, tg, llm, nil)

	// "què tal?" és cortesia i va per la via ràpida (sense model): per
	// exercir el torn d'agent cal una tasca de debò.
	bot.handleMessage(context.Background(), msg(1234567, "resumeix el projecte en una línia"))

	nova := false
	for _, m := range tg.all() {
		if !m.Edit && strings.Contains(m.Text, "sóc el gregal") {
			nova = true
		}
	}
	if !nova {
		t.Fatalf("no s'ha enviat la resposta com a missatge nou: %+v", tg.all())
	}
	// El cursor d'streaming s'ha de netejar a l'últim estat del missatge.
	if m, ok := tg.find("sóc el gregal"); ok && strings.HasSuffix(m.Text, "▌") {
		t.Errorf("ha quedat el cursor d'streaming a l'últim missatge: %q", m.Text)
	}
	infos, err := session.List(sessionsDir)
	if err != nil || len(infos) != 1 {
		t.Fatalf("sessió no desada: %v %+v", err, infos)
	}
	s, err := session.Load(sessionsDir, infos[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Convo) < 2 {
		t.Fatalf("conversa massa curta: %+v", s.Convo)
	}
}

func TestOrdresBasiques(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, sessionsDir, _ := testBot(t, tg, llm, nil)
	ctx := context.Background()

	bot.handleMessage(ctx, msg(1234567, "/whoami"))
	if _, ok := tg.find("1234567"); !ok {
		t.Fatal("/whoami no ha respost l'id")
	}

	bot.handleMessage(ctx, msg(1234567, "/new prova"))
	if _, ok := tg.find("Sessió nova"); !ok {
		t.Fatal("/new no ha confirmat")
	}
	if _, err := session.Load(sessionsDir, "prova"); err != nil {
		t.Fatalf("la sessió nova no és al disc: %v", err)
	}

	bot.handleMessage(ctx, msg(1234567, "/mode goal"))
	if _, ok := tg.find("objectiu"); !ok {
		t.Fatal("/mode goal no ha confirmat")
	}

	bot.handleMessage(ctx, msg(1234567, "/status"))
	if _, ok := tg.find("estat"); !ok {
		t.Fatal("/status no ha respost")
	}

	bot.handleMessage(ctx, msg(1234567, "/sessions"))
	if _, ok := tg.find("Sessions"); !ok {
		t.Fatal("/sessions no ha llistat")
	}

	bot.handleMessage(ctx, msg(1234567, "/use 1"))
	if _, ok := tg.find("Segueixo la sessió"); !ok {
		t.Fatalf("/use no ha canviat de sessió: %+v", tg.all())
	}
}

func TestAprovacioAmbBotonsPermet(t *testing.T) {
	tg := newFakeTG(t)
	work := t.TempDir()
	target := filepath.Join(work, "nota.txt")
	llm := newFakeLLM(t,
		replyToolCall("write", fmt.Sprintf(`{"path":%q,"content":"dades"}`, target)),
		replyText("fet"),
	)
	bot, _, _ := testBot(t, tg, llm, map[string]string{"write": "ask"})

	go bot.handleMessage(context.Background(), msg(1234567, "escriu el fitxer"))

	peticio, ok := func() (sentMsg, bool) {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if m, ok := tg.findKB("Permís"); ok {
				return m, true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return sentMsg{}, false
	}()
	if !ok {
		t.Fatalf("no s'ha demanat permís: %+v", tg.all())
	}
	if len(peticio.KB) != 1 || len(peticio.KB[0]) != 3 {
		t.Fatalf("botons inesperats: %+v", peticio.KB)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("el fitxer no s'hauria d'haver escrit abans d'aprovar")
	}

	data := peticio.KB[0][0].Data // ap:<id>:ok
	bot.handleCallback(context.Background(), &CallbackQuery{ID: "cb1", From: User{ID: 1234567}, Data: data})

	// L'escriptura passa al fil del torn; esperem que hi sigui.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, err := os.ReadFile(target); err == nil {
			if string(b) != "dades" {
				t.Fatalf("contingut inesperat: %q", string(b))
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("el fitxer no s'ha escrit després d'aprovar: %+v", tg.all())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if m, ok := tg.waitFor("✅ permès", 5*time.Second); !ok {
		t.Fatalf("no s'ha marcat el permís: %+v", tg.all())
	} else if strings.Contains(m.Text, "\n") {
		t.Fatalf("rebut massa llarg: %q", m.Text)
	}
}

func TestAprovacioDenegadaNoEscriu(t *testing.T) {
	tg := newFakeTG(t)
	work := t.TempDir()
	target := filepath.Join(work, "no.txt")
	llm := newFakeLLM(t,
		replyToolCall("write", fmt.Sprintf(`{"path":%q,"content":"no"}`, target)),
		replyText("d'acord"),
	)
	bot, _, _ := testBot(t, tg, llm, map[string]string{"write": "ask"})

	go bot.handleMessage(context.Background(), msg(1234567, "escriu"))
	var peticio sentMsg
	{
		deadline := time.Now().Add(5 * time.Second)
		found := false
		for time.Now().Before(deadline) && !found {
			peticio, found = tg.findKB("Permís")
			time.Sleep(20 * time.Millisecond)
		}
		if !found {
			t.Fatalf("no s'ha demanat permís: %+v", tg.all())
		}
	}
	bot.handleCallback(context.Background(), &CallbackQuery{ID: "cb", From: User{ID: 1234567}, Data: peticio.KB[0][2].Data})

	m, ok := tg.waitFor("⛔️ denegat", 5*time.Second)
	if !ok {
		t.Fatalf("no s'ha marcat la denegació: %+v", tg.all())
	}
	// El rebut ha de ser compacte (una línia) perquè no enterri la resposta.
	if strings.Contains(m.Text, "\n") {
		t.Fatalf("rebut massa llarg: %q", m.Text)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("el fitxer no s'hauria d'haver escrit")
	}
}

func TestNomesLAmoPotAprovar(t *testing.T) {
	tg := newFakeTG(t)
	work := t.TempDir()
	target := filepath.Join(work, "x.txt")
	llm := newFakeLLM(t,
		replyToolCall("write", fmt.Sprintf(`{"path":%q,"content":"x"}`, target)),
		replyText("fi"),
	)
	bot, _, _ := testBot(t, tg, llm, map[string]string{"write": "ask"})
	go bot.handleMessage(context.Background(), msg(1234567, "escriu"))
	var peticio sentMsg
	{
		deadline := time.Now().Add(5 * time.Second)
		found := false
		for time.Now().Before(deadline) && !found {
			peticio, found = tg.findKB("Permís")
			time.Sleep(20 * time.Millisecond)
		}
		if !found {
			t.Fatalf("no s'ha demanat permís: %+v", tg.all())
		}
	}

	// Un altre membre del grup prem el botó: s'ignora.
	bot.handleCallback(context.Background(), &CallbackQuery{ID: "cb", From: User{ID: 555}, Data: peticio.KB[0][0].Data})
	if _, err := os.Stat(target); err == nil {
		t.Fatal("un usuari sense permís no pot aprovar escriptures")
	}
	tg.mu.Lock()
	n := len(tg.cbs)
	tg.mu.Unlock()
	if n == 0 {
		t.Fatal("s'hauria d'haver respost el callback")
	}
}

func TestModeObjectiuDesaElBloc(t *testing.T) {
	tg := newFakeTG(t)
	bloc := "Ja en tinc prou.\n\n```goal\ntasca: Afegir un comptador\ncontext: al TUI\ncriteris:\n- es veu\npassos:\n- tocar la barra\nriscos:\n- cap\n```\n"
	llm := newFakeLLM(t, replyText(bloc))
	bot, _, goalsDir := testBot(t, tg, llm, nil)
	bot.cfg.Mode = "goal"

	bot.handleMessage(context.Background(), msg(1234567, "vull un comptador"))

	goals, err := goal.List(goalsDir, filepath.Base(bot.cwd))
	if err != nil || len(goals) != 1 {
		t.Fatalf("objectiu no desat: %v %+v", err, goals)
	}
	if goals[0].Title != "Afegir un comptador" {
		t.Fatalf("títol inesperat: %q", goals[0].Title)
	}
	if _, ok := tg.find("Objectiu desat"); !ok {
		t.Fatalf("no s'ha avisat de l'objectiu: %+v", tg.all())
	}
}

func TestGoalExecutaCanviaAModeCodi(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("objectiu en marxa"))
	bot, _, goalsDir := testBot(t, tg, llm, nil)
	g := goal.Goal{ID: "abc123", Project: filepath.Base(bot.cwd), Title: "Fer la cosa", Body: "tasca: Fer la cosa\npassos:\n- primer"}
	if err := goal.Save(goalsDir, g); err != nil {
		t.Fatal(err)
	}
	st := bot.state(1234567)
	st.mode = "goal"

	bot.handleMessage(context.Background(), msg(1234567, "/goal executa abc123"))

	st.mu.Lock()
	mode := st.mode
	st.mu.Unlock()
	if mode != "code" {
		t.Fatalf("hauria de passar a code, tinc %q", mode)
	}
	if _, ok := tg.find("Executant l'objectiu"); !ok {
		t.Fatal("no s'ha anunciat l'execució")
	}
	if g2, err := goal.Get(goalsDir, "abc123"); err != nil || g2.Status != goal.StatusFet {
		t.Fatalf("l'objectiu hauria de quedar com a fet: %v %v", g2.Status, err)
	}
}

func TestBucleDePollingProcessaUnUpdate(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("resposta del bucle"))
	bot, _, _ := testBot(t, tg, llm, nil)

	tg.mu.Lock()
	tg.updates = []map[string]any{{
		"update_id": 7,
		"message": map[string]any{
			"message_id": 1,
			"chat":       map[string]any{"id": 1234567, "type": "private"},
			"from":       map[string]any{"id": 1234567, "username": "usera"},
			"text":       "hola des del bucle",
		},
	}}
	tg.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { _ = bot.Run(ctx); close(done) }()

	deadline := time.After(2500 * time.Millisecond)
	for {
		if _, ok := tg.find("resposta del bucle"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("el bucle no ha respost: %+v", tg.all())
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestTokenMaiAlMissatgeDError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":false,"description":"Unauthorized"}`)
	}))
	defer srv.Close()
	api := &API{Token: "123:SECRET", BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := api.Me(context.Background())
	if err == nil {
		t.Fatal("esperava error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("el token ha sortit al missatge: %v", err)
	}
}

func TestSplitMessage(t *testing.T) {
	long := strings.Repeat("a", 5000)
	parts := splitMessage(long, 1000)
	if len(parts) != 5 {
		t.Fatalf("parts=%d", len(parts))
	}
	for _, p := range parts {
		if len(p) > 1000 {
			t.Fatalf("part massa llarga: %d", len(p))
		}
	}
	if got := splitMessage("", 10); len(got) != 1 {
		t.Fatalf("text buit: %+v", got)
	}
}

func TestAprovacioSempreRecordaSessio(t *testing.T) {
	tg := newFakeTG(t)
	work := t.TempDir()
	t1 := filepath.Join(work, "a.txt")
	t2 := filepath.Join(work, "b.txt")
	llm := newFakeLLM(t,
		replyToolCall("write", fmt.Sprintf(`{"path":%q,"content":"a"}`, t1)),
		replyToolCall("write", fmt.Sprintf(`{"path":%q,"content":"b"}`, t2)),
		replyText("fet"),
	)
	bot, _, _ := testBot(t, tg, llm, map[string]string{"write": "ask"})

	go bot.handleMessage(context.Background(), msg(1234567, "escriu dos fitxers"))
	var peticio sentMsg
	{
		deadline := time.Now().Add(5 * time.Second)
		found := false
		for time.Now().Before(deadline) && !found {
			peticio, found = tg.findKB("Permís")
			time.Sleep(20 * time.Millisecond)
		}
		if !found {
			t.Fatalf("no s'ha demanat permís: %+v", tg.all())
		}
	}
	// Sempre = segon botó: la segona escriptura idèntica ja no ha de demanar.
	bot.handleCallback(context.Background(), &CallbackQuery{ID: "cb", From: User{ID: 1234567}, Data: peticio.KB[0][1].Data})

	if _, ok := tg.waitFor("🧠 permès sempre (sessió)", 5*time.Second); !ok {
		t.Fatalf("sense rebut de sempre: %+v", tg.all())
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		b1, e1 := os.ReadFile(t1)
		b2, e2 := os.ReadFile(t2)
		if e1 == nil && e2 == nil && string(b1) == "a" && string(b2) == "b" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("les dues escriptures haurien de passar: %+v", tg.all())
		}
		time.Sleep(20 * time.Millisecond)
	}
	n := 0
	for _, m := range tg.all() {
		if strings.Contains(m.Text, "Permís ·") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("només la primera hauria de demanar permís (%d peticions): %+v", n, tg.all())
	}
}

// sawImage diu si alguna crida al model duia parts d'imatge.
func (f *fakeLLM) sawImage() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.bodies {
		if strings.Contains(b, "image_url") {
			return true
		}
	}
	return false
}
