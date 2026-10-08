package telegram

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Exercise the Telegram driver and real motor, not just prompt construction.
func TestTelegramLatestRequestIgnoresOldGlobalChecklist(t *testing.T) {
	for _, request := range []string{"Ep", "Atura això, deixa-ho estar", "Ara explica què és una PWA", "Continua el bench"} {
		t.Run(request, func(t *testing.T) {
			tools.TodoSet([]tools.TodoItem{{Title: "bench antic", Status: "pending"}})
			t.Cleanup(tools.TodoClear)
			previous := tools.TodoList()
			tg := newFakeTG(t)
			model := newFakeLLM(t, replyText("Resposta directa al missatge actual."))
			bot, _, _ := testBot(t, tg, model, nil)
			st := bot.state(1234567)
			oldHistory := []llm.Message{{Role: "user", Content: "Fes un bench"}, {Role: "assistant", Content: "El bench va quedar a mitges."}}
			st.mu.Lock()
			st.convo = append([]llm.Message(nil), oldHistory...)
			st.mu.Unlock()
			bot.handleMessage(context.Background(), msg(1234567, request))
			model.mu.Lock()
			calls := model.calls
			bodies := append([]string(nil), model.bodies...)
			model.mu.Unlock()
			if calls != 1 {
				t.Errorf("correct direct answer must finish in one model call, got %d", calls)
			}
			for _, body := range bodies {
				if strings.Contains(body, agent.GuiaTodosPendents) {
					t.Error("stale checklist injected a synthetic continuation")
				}
			}
			st.mu.Lock()
			preserved := reflect.DeepEqual(st.convo[:len(oldHistory)], oldHistory)
			st.mu.Unlock()
			if !preserved {
				t.Error("conversation history was modified")
			}
			if !reflect.DeepEqual(previous, tools.TodoList()) {
				t.Error("turn cleared or modified another owner's global checklist")
			}
		})
	}
}

// A checklist created by this request must still enforce completion. It must
// never overwrite another owner's pending work in the process-global store.
func TestTelegramCurrentChecklistStillNudges(t *testing.T) {
	tools.TodoSet([]tools.TodoItem{{Title: "altre xat", Status: "pending"}})
	t.Cleanup(tools.TodoClear)
	previous := tools.TodoList()
	tg := newFakeTG(t)
	model := newFakeLLM(t,
		replyToolCall("todowrite", `{"items":[{"title":"pas actual","status":"pending"}]}`),
		replyText("Fet."),
		replyToolCall("todowrite", `{"items":[{"title":"pas actual","status":"done"}]}`),
		replyText("Feina actual acabada."),
	)
	bot, _, _ := testBot(t, tg, model, nil)
	bot.handleMessage(context.Background(), msg(1234567, "Fes la tasca actual amb checklist"))
	model.mu.Lock()
	calls := model.calls
	bodies := append([]string(nil), model.bodies...)
	model.mu.Unlock()
	if calls != 4 || len(bodies) < 3 || !strings.Contains(bodies[2], agent.GuiaTodosPendents) {
		t.Fatalf("current request must get one completion nudge then finish; calls=%d", calls)
	}
	if !reflect.DeepEqual(previous, tools.TodoList()) {
		t.Fatal("todowrite modified another owner's checklist")
	}
	if _, ok := tg.find("Feina actual acabada"); !ok {
		t.Fatal("current task did not finish")
	}
}
