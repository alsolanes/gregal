package telegram

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanSenseTasca(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("hola"))
	bot, _, _ := testBot(t, tg, llm, map[string]string{})
	bot.handleMessage(context.Background(), msg(1234567, "/plan"))
	if !lastSentContains(tg, "ús: /plan") {
		t.Fatalf("sense tasca cal ús: %v", sentTexts(tg))
	}
}

func TestPlanExploraReadOnlyIPresenta(t *testing.T) {
	tg := newFakeTG(t)
	work := t.TempDir()
	nope := filepath.Join(work, "no.txt")
	llm := newFakeLLM(t,
		replyToolCall("write", `{"path":"`+nope+`","content":"x"}`),
		replyToolCall("read", `{"path":"`+nope+`"}`),
		replyText("1. Fer X\nVERIFICACIÓ: pytest"),
	)
	bot, _, _ := testBot(t, tg, llm, map[string]string{})
	// El cwd del bot és un altre dir: el write hi apuntaria igualment si
	// s'executés (l'exploració no ha d'escriure enlloc).
	bot.handleMessage(context.Background(), msg(1234567, "/plan investiga X"))
	if _, err := os.Stat(nope); err == nil {
		t.Fatal("l'exploració ha escrit: el pla és read-only")
	}
	var card string
	var kb Keyboard
	for _, s := range tg.sent {
		if strings.Contains(s.Text, "PLA") {
			card, kb = s.Text, s.KB
		}
	}
	if card == "" {
		t.Fatalf("sense targeta PLA: %v", sentTexts(tg))
	}
	if !strings.Contains(card, "1. Fer X") {
		t.Fatalf("targeta sense pla: %q", card)
	}
	if !hasButton(kb, "plan:run") || !hasButton(kb, "plan:drop") {
		t.Fatalf("targeta sense botons Executa/Descarta: %+v", kb)
	}
	bot.mu.Lock()
	pending := bot.pendingPlans[1234567]
	bot.mu.Unlock()
	if !strings.Contains(pending, "1. Fer X") {
		t.Fatalf("pla no desat: %q", pending)
	}
}

func TestPlanDropNeteja(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("1. Fer X"))
	bot, _, _ := testBot(t, tg, llm, map[string]string{})
	bot.mu.Lock()
	bot.pendingPlans = map[int64]string{1234567: "1. Fer X"}
	bot.mu.Unlock()
	bot.handleCallback(context.Background(), planCB(1234567, "plan:drop"))
	bot.mu.Lock()
	_, ok := bot.pendingPlans[1234567]
	bot.mu.Unlock()
	if ok {
		t.Fatal("Descarta ha de netejar el pendent")
	}
	if !lastSentContains(tg, "descartat") {
		t.Fatalf("cal confirmar: %v", sentTexts(tg))
	}
}

func TestPlanRunSensePendent(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("hola"))
	bot, _, _ := testBot(t, tg, llm, map[string]string{})
	bot.handleCallback(context.Background(), planCB(1234567, "plan:run"))
	if !lastSentContains(tg, "cap pla pendent") {
		t.Fatalf("sense pendent cal avís: %v", sentTexts(tg))
	}
}

func planCB(chatID int64, data string) *CallbackQuery {
	return &CallbackQuery{ID: "cb1", Data: data, From: User{ID: 1234567},
		Message: &Message{MessageID: 1, Chat: Chat{ID: chatID, Type: "private"}}}
}

func hasButton(kb Keyboard, data string) bool {
	for _, row := range kb {
		for _, b := range row {
			if b.Data == data {
				return true
			}
		}
	}
	return false
}

func sentTexts(tg *fakeTG) []string {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	out := make([]string, 0, len(tg.sent))
	for _, s := range tg.sent {
		out = append(out, s.Text)
	}
	return out
}

func lastSentContains(tg *fakeTG, want string) bool {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	for i := len(tg.sent) - 1; i >= 0; i-- {
		if strings.Contains(tg.sent[i].Text, want) {
			return true
		}
	}
	return false
}

func TestRewindAmbNumero(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("hola"))
	bot, _, _ := testBot(t, tg, llm, map[string]string{})
	bot.handleMessage(context.Background(), msg(1234567, "/rewind 99"))
	if !lastSentContains(tg, "inexistent") {
		t.Fatalf("seq fora de rang cal error: %v", sentTexts(tg))
	}
}
