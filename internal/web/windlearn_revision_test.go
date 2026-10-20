package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestWindLearnCannotUndoConfirmedMemoryEdit(t *testing.T) {
	for _, replacement := range []string{"", "- Només el coneixement corregit per l'usuari.\n"} {
		t.Run(replacement, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			postJSON(t, s.handleWinds, `{"name":"Marinada","role":"natura"}`)
			wind, _ := s.winds.ByID("marinada")
			if err := companion.ReplaceWindBrain(s.windStateDir(), wind.ID, "- Un fet antic [example.org].\n"); err != nil {
				t.Fatal(err)
			}
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			original := windLearnLoop
			windLearnLoop = func(_ *Server, _ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, _ []llm.Message) (string, error) {
				close(entered)
				<-release
				return "- Un fet pendent que ja s'ha oblidat [example.org].", nil
			}
			defer func() { windLearnLoop = original }()
			go func() { defer close(done); s.windLearn(wind, "Pregunta", "Fet a https://example.org/font") }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("extraction did not start")
			}
			// Another client of the same durable state confirms a manual edit.
			other := &Server{cfgPath: s.cfgPath, goalDir: s.goalDir, winds: s.winds, user: s.user}
			body, _ := json.Marshal(map[string]string{"wind": wind.ID, "text": replacement})
			response := httptest.NewRecorder()
			other.handleWindMemory(response, httptest.NewRequest(http.MethodPost, "/api/winds/memory", strings.NewReader(string(body))))
			close(release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("extraction did not finish")
			}
			if response.Code != http.StatusOK {
				t.Fatalf("memory edit: %d %s", response.Code, response.Body.String())
			}
			if got := s.windBrain(wind.ID); got != replacement {
				t.Fatalf("pending extraction undid confirmed edit: got %q want %q", got, replacement)
			}
		})
	}
}
