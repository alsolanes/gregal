package telegram

import (
	"context"
	"fmt"
	"gregal/internal/agent"
	"gregal/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStartupLearnsAdvertisedModelWindow(t *testing.T) {
	tg := newFakeTG(t)
	stub := newFakeLLM(t, replyText("unused"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"probe-window-model","context_length":4096}]}`)
	}))
	defer server.Close()
	bot, _, _ := testBot(t, tg, stub, nil)
	bot.cfg.Providers["t"] = config.Provider{BaseURL: server.URL}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { bot.Run(ctx); close(done) }()
	found := false
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if win, ok := agent.KnownWindow(server.URL, "probe-window-model"); ok && win == 4096 {
			found = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poller did not stop")
	}
	if !found {
		t.Fatal("Telegram startup never learned provider window")
	}
}
