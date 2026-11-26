package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
)

// 162: el pas de l'eina wind diu a quins vents es pregunta, amb els ids del
// magatzem de vents i mai els arguments de l'eina.
func TestWindToolActivityNamesTheConsultedWinds(t *testing.T) {
	cases := []struct {
		args string
		want []string
	}{
		{`{"action":"ask","wind":"Llevant","task":"PRIVATE_TASK"}`, []string{"llevant"}},
		{`{"action":"all","task":"PRIVATE_TASK"}`, []string{"tramuntana", "xaloc", "llevant"}},
	}
	for _, c := range cases {
		t.Run(c.want[0]+fmt.Sprint(len(c.want)), func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cwd = t.TempDir()
			makeRotllanaTrio(t, s)
			old := windLoopStream
			windLoopStream = func(_ *Server, _ context.Context, _ string, _ func(kind, text string), _ ...*routedRole) (exploracio, error) {
				return exploracio{text: "Arròs bomba"}, nil
			}
			t.Cleanup(func() { waitBackground(); windLoopStream = old })
			var calls atomic.Int32
			args, _ := json.Marshal(c.args)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				body := "data: {\"choices\":[{\"delta\":{\"content\":\"Fet\"},\"finish_reason\":\"stop\"}]}\n\n"
				if calls.Add(1) == 1 {
					body = `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"r1","function":{"name":"wind","arguments":` + string(args) + `}}]},"finish_reason":"tool_calls"}]}` + "\n\n"
				}
				if _, err := io.WriteString(w, body+"data: [DONE]\n\n"); err != nil {
					t.Error(err)
				}
			}))
			defer provider.Close()
			p := config.Provider{BaseURL: provider.URL + "/v1"}
			s.cfg.Providers["p"] = p
			ctx := s.interactiveWindCtx(s.withWindSteerKey(context.Background(), "wind", gregalWindID))
			var tools []windActivity
			_, err := s.runReadOnlyLoopForRoleStream(ctx, "pregunta", 3, p, s.cfg.Roles["chat"], func(kind, text string) {
				if kind != "activity" {
					return
				}
				if strings.Contains(text, "PRIVATE") {
					t.Errorf("the activity carries the tool arguments: %s", text)
				}
				var a windActivity
				if json.Unmarshal([]byte(text), &a) == nil && a.Phase == "tool" {
					tools = append(tools, a)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(tools) != 1 || tools[0].Tool != "wind" || strings.Join(tools[0].Winds, ",") != strings.Join(c.want, ",") {
				t.Fatalf("tool activity = %+v, want one wind step asking %v", tools, c.want)
			}
		})
	}
}

// 162: després del torn, el fil del vent que pregunta guarda, abans de la seva
// resposta, el que li ha respost el vent consultat; el model no ho torna a rebre.
func TestWindChatKeepsTheConsultedAnswerBeforeTheReply(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.cwd = t.TempDir()
	makeRotllanaTrio(t, s)
	if _, ok := s.ensureGregalWind(); !ok {
		t.Fatal("no Gregal")
	}
	old := windLoopStream
	windLoopStream = func(s *Server, ctx context.Context, brief string, _ func(kind, text string), _ ...*routedRole) (exploracio, error) {
		if strings.Contains(brief, handoffTag) {
			return exploracio{text: "Arròs bomba, 18 minuts"}, nil
		}
		if _, err := s.runWindTool(ctx, `{"action":"ask","wind":"llevant","task":"com faig l'arròs?"}`); err != nil {
			return exploracio{}, err
		}
		return exploracio{text: "Diu que 18 minuts"}, nil
	}
	t.Cleanup(func() { waitBackground(); windLoopStream = old })
	rec := postJSON(t, s.handleWindChatStream, `{"wind":"gregal","message":"com faig l'arròs?"}`)
	if !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("turn: %s", rec.Body.String())
	}
	get := httptest.NewRecorder()
	s.handleWindChat(get, httptest.NewRequest(http.MethodGet, "/api/winds/chat?wind=gregal", nil))
	var out struct {
		History []map[string]any `json:"history"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	h := out.History
	if len(h) < 3 {
		t.Fatalf("history: %+v", h)
	}
	consult, reply := h[len(h)-2], h[len(h)-1]
	if consult["kind"] != "consult" || consult["speaker_id"] != "llevant" || consult["speaker_name"] != "Llevant" || consult["text"] != "Arròs bomba, 18 minuts" {
		t.Fatalf("consult message: %+v", consult)
	}
	if reply["text"] != "Diu que 18 minuts" || reply["kind"] != nil {
		t.Fatalf("reply: %+v", reply)
	}
	hist := windHistory(s, gregalWindID)
	hist.mu.RLock()
	turns := windChatTurns(hist.messages)
	hist.mu.RUnlock()
	for _, m := range turns {
		if strings.Contains(m.Content, "Arròs bomba") {
			t.Fatalf("the consulted answer went back to the model as Gregal's: %q", m.Content)
		}
	}
}
