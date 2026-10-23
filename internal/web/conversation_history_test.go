package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

type historyRoute struct{ kind, path, field, id, durable string }

var historyRoutes = []historyRoute{
	{"wind", "/api/winds/chat?wind=xaloc", "wind", "xaloc", "company-xaloc"},
	{"rotllana", "/api/winds/rotllana?rotlla=shared-thread", "rotllana", "shared-thread", "rotllana-shared-thread"},
	{"pet", "/api/companion/chat?pet=mel", "pet", "mel", "company-mel"},
}

func (route historyRoute) handler(s *Server) http.HandlerFunc {
	switch route.kind {
	case "wind":
		return s.handleWindChat
	case "rotllana":
		return s.handleRotllanaChat
	default:
		return s.handleCompanionPetChat
	}
}
func (route historyRoute) post(s *Server, msg string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{route.field: route.id, "message": msg})
	out := httptest.NewRecorder()
	route.handler(s)(out, httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(string(body))))
	return out
}
func (route historyRoute) get(s *Server) *httptest.ResponseRecorder {
	out := httptest.NewRecorder()
	route.handler(s)(out, httptest.NewRequest(http.MethodGet, route.path, nil))
	return out
}
func historyFixture(t *testing.T, route historyRoute, shared bool) (a, b, other *Server) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", filepath.Join(t.TempDir(), "memory.md"))
	hub := testHubWithUsers(t)
	if shared {
		a = hub.SessionFor(nil, "desktop")
		b = hub.SessionFor(&User{Name: "local", Tothom: true}, "mobile")
		other = hub.SessionFor(&User{Name: "anna"}, "desktop")
	} else {
		a = hub.SessionFor(&User{Name: "anna"}, "desktop")
		b = hub.SessionFor(&User{Name: "anna"}, "mobile")
		other = hub.SessionFor(&User{Name: "bert"}, "desktop")
	}
	for _, s := range []*Server{a, b, other} {
		if _, err := s.winds.Save(companion.Wind{Name: "Xaloc"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.rotllanes.path(), []byte(`{"rotllanes":[{"id":"shared-thread","name":"Shared","members":["xaloc"]}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		if route.kind == "wind" {
			// Disable asynchronous learning, without touching the shared hub config.
			cfg := *s.cfg
			cfg.Roles = map[string]config.Role{}
			s.cfg = &cfg
		}
	}
	return
}
func historySeam(t *testing.T, call func(string) (string, error)) {
	t.Helper()
	oldWind, oldPet := windLoop, petChatter
	windLoop = func(_ *Server, _ context.Context, brief string) (exploracio, error) {
		text, err := call(brief)
		return exploracio{text: text}, err
	}
	petChatter = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, msgs []llm.Message) (string, error) {
		var b strings.Builder
		for _, m := range msgs {
			b.WriteString(m.Content)
			b.WriteByte('\n')
		}
		return call(b.String())
	}
	t.Cleanup(func() { waitBackground(); windLoop = oldWind; petChatter = oldPet })
}
func assertHistory(t *testing.T, out *httptest.ResponseRecorder, turns int) {
	t.Helper()
	var got struct {
		History []map[string]string `json:"history"`
	}
	if out.Code != 200 {
		t.Fatalf("history HTTP %d: %s", out.Code, out.Body.String())
	}
	if err := json.Unmarshal(out.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.History) != turns*2 {
		t.Errorf("history has %d messages; want %d: %s", len(got.History), turns*2, out.Body.String())
	}
	for i := 1; i <= turns; i++ {
		for _, want := range []string{fmt.Sprintf("PRIVATE-QUESTION-%d", i), fmt.Sprintf("PRIVATE-ANSWER-%d", i)} {
			if !strings.Contains(out.Body.String(), want) {
				t.Errorf("history lost %q: %s", want, out.Body.String())
			}
		}
	}
}

func TestConversationHistorySharedAcrossPrewarmedSessions(t *testing.T) {
	for _, route := range historyRoutes {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/shared=%v", route.kind, shared), func(t *testing.T) {
				a, b, other := historyFixture(t, route, shared)
				for _, s := range []*Server{a, b, other} {
					assertHistory(t, route.get(s), 0)
				}
				calls := 0
				historySeam(t, func(prompt string) (string, error) {
					calls++
					for i := 1; i < calls; i++ {
						for _, want := range []string{fmt.Sprintf("PRIVATE-QUESTION-%d", i), fmt.Sprintf("PRIVATE-ANSWER-%d", i)} {
							if !strings.Contains(prompt, want) {
								t.Errorf("model turn %d missing previous context %q", calls, want)
							}
						}
					}
					return fmt.Sprintf("PRIVATE-ANSWER-%d", calls), nil
				})
				for i, s := range []*Server{a, b, a} {
					assertHistory(t, route.post(s, fmt.Sprintf("PRIVATE-QUESTION-%d", i+1)), i+1)
					assertHistory(t, route.get(a), i+1)
					assertHistory(t, route.get(b), i+1)
				}
				assertHistory(t, route.get(other), 0)
				saved, err := session.Load(a.sessDir(), route.durable)
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(map[string]any{"history": petHistoryJSON(saved.Convo)})
				disk := httptest.NewRecorder()
				disk.Write(raw)
				assertHistory(t, disk, 3)
				if _, err := session.Load(other.sessDir(), route.durable); err == nil {
					t.Error("foreign user has private durable history")
				}
			})
		}
	}
}
