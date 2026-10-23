package telegram

import (
	"gregal/internal/agent"
	"testing"
)

func TestModelOverrideDoesNotInheritDifferentModelWindow(t *testing.T) {
	tg := newFakeTG(t)
	model := newFakeLLM(t, replyText("unused"))
	bot, _, _ := testBot(t, tg, model, nil)
	r := bot.cfg.Roles["code"]
	r.ContextWindow = 65536
	r.Think = "no"
	r.FallbackProvider = "t"
	r.FallbackModel = "backup"
	bot.cfg.Roles["code"] = r
	bot.modelsMu.Lock()
	bot.modelsOv = map[string]map[string]string{"99": {"code": "t/small"}}
	bot.modelsMu.Unlock()
	agent.LearnWindow(bot.cfg.Providers["t"].BaseURL, "small", 4096)
	_, actual := bot.roleRefFor(99, "code")
	if got := agent.Window(bot.cfg, actual); got != 4096 {
		t.Fatalf("small model inherits large model window: %d", got)
	}
	if actual.Think != "no" || actual.FallbackModel != "backup" {
		t.Fatalf("override lost role settings: %+v", actual)
	}
	bot.modelsMu.Lock()
	bot.modelsOv["99"]["code"] = "t/m"
	bot.modelsMu.Unlock()
	_, same := bot.roleRefFor(99, "code")
	if same.ContextWindow != 65536 {
		t.Fatal("same-model explicit configured window was lost")
	}
}
