package telegram

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gregal/internal/tools"
)

func TestTelegramNextTurnDoesNotInheritUnfinishedChecklist(t *testing.T) {
	tools.TodoClear()
	t.Cleanup(tools.TodoClear)
	tg := newFakeTG(t)
	model := newFakeLLM(t,
		replyToolCall("todowrite", `{"items":[{"title":"tasca interrompuda","status":"pending"}]}`),
		replyText("Encara pendent."), replyText("Encara pendent."), replyText("Encara pendent."),
		replyText("Ep!"),
	)
	bot, _, _ := testBot(t, tg, model, nil)
	bot.handleMessage(context.Background(), msg(1234567, "Fes una feina amb passos pendents"))
	model.mu.Lock()
	firstCalls := model.calls
	model.mu.Unlock()
	if firstCalls != 4 {
		t.Fatalf("expected pending-task retry cap, got %d calls", firstCalls)
	}
	bot.handleMessage(context.Background(), msg(1234567, "Ep"))
	model.mu.Lock()
	totalCalls := model.calls
	model.mu.Unlock()
	if totalCalls != firstCalls+1 {
		t.Fatalf("next turn reopened prior checklist: %d extra calls", totalCalls-firstCalls)
	}
	if _, ok := tg.find("Ep!"); !ok {
		t.Fatal("greeting answer missing")
	}
}

func TestTurnChecklistsAreIsolatedConcurrentSnapshots(t *testing.T) {
	var a, b turnChecklist
	var wg sync.WaitGroup
	for _, job := range []struct {
		c     *turnChecklist
		title string
	}{{&a, "chat-a"}, {&b, "chat-b"}} {
		wg.Add(1)
		go func(c *turnChecklist, title string) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_, handled, err := c.exec("todowrite", `{"items":[{"title":"`+title+`","status":"completed"}]}`)
				if !handled || err != nil {
					t.Errorf("write failed: handled=%v err=%v", handled, err)
					return
				}
				snapshot := c.list()
				if len(snapshot) != 1 || snapshot[0].Title != title || snapshot[0].Status != "done" {
					t.Errorf("cross-chat checklist or invalid normalization: %+v", snapshot)
					return
				}
				snapshot[0].Title = "mutated caller copy"
				if c.list()[0].Title != title {
					t.Error("snapshot aliases checklist")
				}
			}
		}(job.c, job.title)
	}
	wg.Wait()
	out, handled, err := a.exec("todoread", "{}")
	if !handled || err != nil || !strings.Contains(out, "chat-a") || strings.Contains(out, "chat-b") {
		t.Fatalf("read isolated checklist: %q %v %v", out, handled, err)
	}
	before := a.list()
	if _, handled, err := a.exec("todowrite", "invalid-json"); !handled || err == nil {
		t.Fatal("malformed todo arguments must fail")
	}
	if !reflect.DeepEqual(before, a.list()) {
		t.Fatal("invalid write damaged existing checklist")
	}
	if _, handled, err := a.exec("read", "{}"); handled || err != nil {
		t.Fatal("non-checklist tool should use the normal executor")
	}
}
