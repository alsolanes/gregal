package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Observes flushed events without reading a ResponseRecorder concurrently.
type windEventRecorder struct {
	*httptest.ResponseRecorder
	events chan string
}

func (w *windEventRecorder) Write(b []byte) (int, error) {
	select {
	case w.events <- string(b):
	default:
	}
	return w.ResponseRecorder.Write(b)
}

func TestRotllanaContinueKeepsLastSpeaker(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			s, _ := companionTestServer(t)
			id := makeRotllanaTrio(t, s)
			rot, _ := s.rotllanaByID(id)
			history := []llm.Message{{Role: "user", Content: "previous"}, {Role: "assistant", SpeakerID: "llevant", SpeakerName: "Llevant", Content: "Llevant: Partial reply"}}
			h := rotllanaHistory(s, id)
			h.messages = history
			if err := saveRotllanaHistory(s, id, history); err != nil {
				t.Fatal(err)
			}
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					http.NotFound(w, r)
					return
				}
				var req struct {
					Stream bool `json:"stream"`
				}
				json.NewDecoder(r.Body).Decode(&req)
				if req.Stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Continued\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				} else {
					fmt.Fprint(w, `{"choices":[{"message":{"content":"Continued"}}]}`)
				}
			}))
			defer provider.Close()
			s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
			handler := s.handleRotllanaChat
			if stream {
				handler = s.handleRotllanaChatStream
			}
			rec := postJSON(t, handler, `{"rotllana":"`+rot.ID+`","continue":true}`)
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"wind":"llevant"`) || strings.Contains(rec.Body.String(), `"wind":"tramuntana"`) {
				t.Fatalf("continue must keep the last speaker: %s", rec.Body.String())
			}
			if len(h.messages) != 3 || h.messages[2].SpeakerID != "llevant" {
				t.Fatalf("history lost continuation identity: %+v", h.messages)
			}
		})
	}
}

func TestWindLoopActivityTracksRealStagesWithoutPrivateText(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"PRIVATE_REASONING\"}}]}\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"r1","function":{"name":"glob","arguments":"{\"pattern\":\"PRIVATE_ARGUMENT\"}"}}]},"finish_reason":"tool_calls"}]}`+"\n\n")
		} else {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Final reply\"},\"finish_reason\":\"stop\"}]}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	p := config.Provider{BaseURL: provider.URL + "/v1"}
	s.cfg.Providers["p"] = p
	var activity []string
	res, err := s.runReadOnlyLoopForRoleStream(context.Background(), "PRIVATE_TASK", 3, p, s.cfg.Roles["chat"], func(kind, text string) {
		if kind == "activity" {
			activity = append(activity, text)
		}
	})
	if err != nil || res.text != "Final reply" {
		t.Fatalf("loop err=%v reply=%q", err, res.text)
	}
	var stages []string
	for _, text := range activity {
		var a map[string]any
		if err := json.Unmarshal([]byte(text), &a); err != nil {
			t.Fatal(err)
		}
		for key := range a {
			if key != "phase" && key != "tool" && key != "step" {
				t.Fatalf("private/unexpected field: %s", text)
			}
		}
		if strings.Contains(text, "PRIVATE") || strings.Contains(text, s.cwd) {
			t.Fatalf("private activity: %s", text)
		}
		if tool, ok := a["tool"]; ok && tool != "glob" {
			t.Fatalf("not a tool NAME: %s", text)
		}
		stages = append(stages, fmt.Sprintf("%v:%v", a["phase"], a["step"]))
	}
	want := "model:1,thinking:1,tool:1,tool_done:1,model:2,responding:2"
	if got := strings.Join(stages, ","); got != want {
		t.Fatalf("activity=%s want=%s", got, want)
	}
}

func TestWindSSEHeartbeatStopsWithHandler(t *testing.T) {
	rec := &windEventRecorder{httptest.NewRecorder(), make(chan string, 64)}
	emit, stop := windSSE(rec, context.Background(), time.Millisecond)
	emit("activity", windActivity{Phase: "waiting"})
	<-rec.events
	select {
	case event := <-rec.events:
		if !strings.Contains(event, "event: ping") || strings.Contains(event, "activity") {
			t.Fatalf("heartbeat invented progress: %s", event)
		}
	case <-time.After(time.Second):
		t.Fatal("no heartbeat")
	}
	stop()
	stop() // idempotent, and no heartbeat writes after return
	for len(rec.events) > 0 {
		<-rec.events
	}
	select {
	case event := <-rec.events:
		t.Fatalf("write after stream stop: %s", event)
	case <-time.After(5 * time.Millisecond):
	}
}

func TestWindLoopCancelledContextDoesNotCallProvider(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); http.Error(w, "unexpected", 500) }))
	defer provider.Close()
	s := goalTestServer(t)
	p := config.Provider{BaseURL: provider.URL + "/v1"}
	s.cfg.Providers["p"] = p
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.runReadOnlyLoopForRoleStream(ctx, "test", 2, p, s.cfg.Roles["chat"], func(string, string) {})
	if err == nil || calls.Load() != 0 {
		t.Fatalf("cancelled turn err=%v requests=%d", err, calls.Load())
	}
}

func TestWindStreamAnnouncesWaitingBeforeConversationLock(t *testing.T) {
	for _, group := range []bool{false, true} {
		t.Run(map[bool]string{false: "wind", true: "rotllana"}[group], func(t *testing.T) {
			s, _ := companionTestServer(t)
			id := makeRotllanaTrio(t, s)
			oldWind, oldGroup := windLoopStream, rotllanaVoiceLoopStream
			loop := func(_ *Server, _ context.Context, _ string, _ func(string, string), _ ...*routedRole) (exploracio, error) {
				return exploracio{text: "Resposta"}, nil
			}
			windLoopStream, rotllanaVoiceLoopStream = loop, loop
			defer func() { waitBackground(); windLoopStream, rotllanaVoiceLoopStream = oldWind, oldGroup }()
			var unlock func()
			var handler http.HandlerFunc
			var body string
			if group {
				h := rotllanaHistory(s, id)
				h.mu.Lock()
				unlock, handler = h.mu.Unlock, s.handleRotllanaChatStream
				body = `{"rotllana":"` + id + `","message":"hola"}`
			} else {
				h := windHistory(s, "tramuntana")
				h.mu.Lock()
				unlock, handler = h.mu.Unlock, s.handleWindChatStream
				body = `{"wind":"tramuntana","message":"hola"}`
			}
			rec := &windEventRecorder{httptest.NewRecorder(), make(chan string, 64)}
			done := make(chan struct{})
			go func() {
				handler(rec, httptest.NewRequest("POST", "/stream", strings.NewReader(body)))
				close(done)
			}()
			defer func() { unlock(); <-done }()
			select {
			case event := <-rec.events:
				if !strings.Contains(event, "event: activity") || !strings.Contains(event, `"phase":"waiting"`) {
					t.Fatalf("first event must honestly announce lock wait: %s", event)
				}
			case <-time.After(time.Second):
				t.Fatal("no immediate event while conversation lock is held")
			}
		})
	}
}
