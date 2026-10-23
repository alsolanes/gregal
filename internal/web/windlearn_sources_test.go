package web

import (
	"context"
	"gregal/internal/config"
	"gregal/internal/llm"
	"strings"
	"testing"
)

func TestWindLearnRejectsInventedOrConversationOnlySources(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Marinada","color":"#EFC69A","role":"natura"}`)
	w, _ := s.winds.ByID("marinada")
	saved := windLearnLoop
	defer func() { waitBackground(); windLearnLoop = saved }()
	windLearnLoop = func(s *Server, ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		return "- La ruta de prova es diu SAFIRA [conversa actual].\n- Un fet amb una font inventada [invented.example].\n- Un coneixement amb una font present [example.org].", nil
	}
	s.windLearn(w, "És una pregunta de prova.", "Aquesta resposta inclou una font: https://example.org/fets")
	brain := s.windBrain(w.ID)
	if strings.Contains(brain, "SAFIRA") || strings.Contains(brain, "invented.example") {
		t.Fatalf("untrusted extraction persisted: %s", brain)
	}
	if !strings.Contains(brain, "example.org") {
		t.Fatalf("legitimate source lost: %s", brain)
	}
}
