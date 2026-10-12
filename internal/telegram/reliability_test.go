package telegram

import (
	"context"
	"fmt"
	"gregal/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/session"
)

func TestReliabilityHeartbeatWhileModelBlocked(t *testing.T) {
	tg := newFakeTG(t)
	model := newFakeLLM(t, replyText("done"))
	bot, _, _ := testBot(t, tg, model, nil)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprintf(w, `{"choices":[{"message":%s,"finish_reason":"stop"}]}`, replyText("done"))
	}))
	defer blocked.Close()
	defer close(release)
	bot.cfg.Providers["t"] = config.Provider{BaseURL: blocked.URL}
	bot.heartbeatInterval = 20 * time.Millisecond
	done := make(chan struct{})
	go func() {
		bot.turn(context.Background(), 1234567, bot.state(1234567), "Comprova el servidor")
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("model not started")
	}
	if _, ok := tg.waitFor("encara treballant", 200*time.Millisecond); !ok {
		t.Fatal("no heartbeat while provider blocked")
	}
	// The release defer lets the real turn finish before httptest cleanup.
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("turn did not finish")
		}
	})
}

func TestReliabilityEmptyFinalIsNotSilentSuccess(t *testing.T) {
	tg := newFakeTG(t)
	model := newFakeLLM(t, replyToolCall("read", `{"path":"missing.txt"}`), replyText(""), replyText(""))
	bot, _, _ := testBot(t, tg, model, nil)
	bot.turn(context.Background(), 1234567, bot.state(1234567), "Llegeix missing.txt")
	errors := 0
	for _, m := range tg.all() {
		if strings.HasPrefix(m.Text, "⚠️") {
			errors++
		}
	}
	if errors != 1 {
		t.Fatalf("expected one actionable error, got %d: %+v", errors, tg.all())
	}
}

func TestReliabilityMalformedToolFinalIsVisible(t *testing.T) {
	tg := newFakeTG(t)
	model := newFakeLLM(t, replyText("<function=unknown>"))
	bot, _, _ := testBot(t, tg, model, nil)
	bot.turn(context.Background(), 1234567, bot.state(1234567), "Llegeix el fitxer")
	if _, ok := tg.find("⚠️"); !ok {
		t.Fatalf("sanitized empty final silently disappeared: %+v", tg.all())
	}
}

func TestReliabilityModelStepHonorsConfiguredTimeout(t *testing.T) {
	tg := newFakeTG(t)
	model := newFakeLLM(t, replyText("unused"))
	bot, _, _ := testBot(t, tg, model, nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-time.After(2 * time.Second):
			fmt.Fprintf(w, `{"choices":[{"message":%s,"finish_reason":"stop"}]}`, replyText("too late"))
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	bot.cfg.Providers["t"] = config.Provider{BaseURL: server.URL}
	bot.cfg.Agent.ModelTimeoutS = 1
	start := time.Now()
	bot.turn(context.Background(), 1234567, bot.state(1234567), "Comprova el servidor")
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("configured 1-second deadline ignored: %s", elapsed)
	}
	if _, ok := tg.find("⚠️"); !ok {
		t.Fatal("timeout was not reported")
	}
}

func TestReliabilitySmallTalkReleasesRunAndPersists(t *testing.T) {
	tg := newFakeTG(t)
	model := newFakeLLM(t, replyText("unused"))
	bot, dir, _ := testBot(t, tg, model, nil)
	st := bot.state(1234567)
	bot.turn(context.Background(), 1234567, st, "Hola")
	st.mu.Lock()
	busy, runID, name := st.busy, st.runID, st.name
	st.mu.Unlock()
	if busy || runID != 0 {
		t.Errorf("greeting leaves phantom active run: busy=%v id=%d", busy, runID)
	}
	if _, err := os.Stat(filepath.Join(dir, name+".json")); err != nil {
		t.Errorf("greeting session not persisted: %v", err)
	}
}

func TestReliabilityToolTranscriptSurvivesTurn(t *testing.T) {
	tg := newFakeTG(t)
	model := newFakeLLM(t, replyToolCall("read", `{"path":"missing.txt"}`), replyText("No existeix."))
	bot, dir, _ := testBot(t, tg, model, nil)
	st := bot.state(1234567)
	bot.turn(context.Background(), 1234567, st, "Llegeix missing.txt i explica què hi ha")
	st.mu.Lock()
	name := st.name
	st.mu.Unlock()
	saved, err := session.Load(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	hasCall, hasResult := false, false
	for _, m := range saved.Convo {
		if len(m.ToolCalls) > 0 {
			hasCall = true
		}
		if m.Role == "tool" {
			hasResult = true
		}
	}
	if !hasCall || !hasResult {
		t.Fatalf("turn lost actual tool evidence: calls=%v results=%v history=%+v", hasCall, hasResult, saved.Convo)
	}
}
