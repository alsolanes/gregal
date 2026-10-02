package telegram

import (
	"context"
	"testing"
)

// Canviar de mode ha de dir amb quin model es respondrà. Era la informació que
// faltava: s'havia de demanar a part amb /model o /rol, i quan el rol apunta a
// un provider local o al núvol la diferència importa molt.
func TestMissatgeDeModeDiuElModel(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	ctx := context.Background()

	bot.handleMessage(ctx, msg(1234567, "/mode"))
	if _, ok := tg.find("respondrà:"); !ok {
		t.Fatalf("el missatge de /mode hauria de dir el model: %+v", tg.all())
	}

	bot.handleMessage(ctx, msg(1234567, "/mode code"))
	if _, ok := tg.find("respondrà:"); !ok {
		t.Fatalf("en canviar de mode hauria de dir el model: %+v", tg.all())
	}
}
