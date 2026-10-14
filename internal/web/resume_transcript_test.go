package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// La conversa desada porta el mapa del projecte enganxat a la tasca (és
// per al model). En reprendre-la, la bombolla ha de mostrar només el que
// va escriure la persona, i el model ha de continuar tenint el context.
func TestResumeNoMostraElMapaDelProjecte(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	task := "The median test fails. Fix it."
	withMap := task + "\n\n" + agent.MarcaContext + " abcd1234] Directori de treball: /tmp/ws.\nFitxers (1):\nsrc/stats.py\n"
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: "amb-mapa", Role: "code",
		Convo: []llm.Message{
			{Role: "user", Content: withMap},
			{Role: "assistant", Content: "Fixed."},
		},
	}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleResume(rec, httptest.NewRequest(http.MethodPost, "/api/resume", strings.NewReader(`{"name":"amb-mapa"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("resume: %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Transcript []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"transcript"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Transcript) != 2 || got.Transcript[0].Content != task {
		t.Fatalf("transcript visible incorrecte: %+v", got.Transcript)
	}
	if !strings.Contains(s.convo[0].Content, agent.MarcaContext) {
		t.Fatalf("el model ha de conservar el mapa a l'historial: %q", s.convo[0].Content)
	}
}
