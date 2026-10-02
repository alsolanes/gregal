package telegram

import (
	"context"
	"testing"
	"time"
)

// PNG mínim vàlid (1x1) per a les proves de foto.
var testPNG = []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82,
	0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 144, 119, 83, 222}

func photoMsg(user int64, caption string) *Message {
	return &Message{MessageID: 2, Chat: Chat{ID: user, Type: "private"},
		From: &User{ID: user, Username: "usera"}, Caption: caption,
		Photo: []PhotoSize{
			{FileID: "petita", Width: 90, Height: 90},
			{FileID: "grossa", Width: 800, Height: 800},
		}}
}

// La foto arriba al model com a image_url i el torn respon.
func TestFotoAlTorn(t *testing.T) {
	tg := newFakeTG(t)
	tg.filePath = "photos/foto.png"
	tg.fileBytes = testPNG
	llm := newFakeLLM(t, replyText("veig un punt"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.handleMessage(context.Background(), photoMsg(1234567, "què hi ha?"))
	if _, ok := tg.waitFor("veig un punt", 10*time.Second); !ok {
		t.Fatal("el torn no ha respost")
	}
	if !llm.sawImage() {
		t.Fatal("el model no ha rebut cap part image_url")
	}
}

// Si getFile falla, avís visible i cap crida al model.
func TestFotoSenseGetFile(t *testing.T) {
	tg := newFakeTG(t)
	tg.filePath = "" // getFile sense file_path
	llm := newFakeLLM(t, replyText("no hauria de passar"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.handleMessage(context.Background(), photoMsg(1234567, ""))
	if _, ok := tg.waitFor("No he pogut obtenir la foto", 10*time.Second); !ok {
		t.Fatal("cal avís visible")
	}
	if llm.calls != 0 {
		t.Fatalf("cap crida al model amb foto fallida (calls=%d)", llm.calls)
	}
}

// La nota de veu s'anuncia com a no suportada, sense cridar el model.
func TestVeuAvisa(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("no hauria de passar"))
	bot, _, _ := testBot(t, tg, llm, nil)
	m := msg(1234567, "")
	m.Voice = &Voice{FileID: "v1", Duration: 5}
	bot.handleMessage(context.Background(), m)
	if _, ok := tg.waitFor("notes de veu encara no estan suportades", 10*time.Second); !ok {
		t.Fatal("cal avís de veu")
	}
	if llm.calls != 0 {
		t.Fatalf("cap crida al model amb veu (calls=%d)", llm.calls)
	}
}
