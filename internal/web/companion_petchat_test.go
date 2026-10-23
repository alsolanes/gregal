package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// POST /api/companion/chat {pet,message}: resposta amb la veu de la mascota,
// historial propi per mascota, i neteja de cometes.
func TestPetChatRepliesInVoice(t *testing.T) {
	s, _ := companionTestServer(t)

	saved := petChatter
	var gotSystem string
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		gotSystem = msgs[0].Content
		return "«Buf, quin dia!»", nil // el model ha posat cometes: cal netejar
	}
	t.Cleanup(func() { waitBackground(); petChatter = saved })

	rec := postJSON(t, s.handleCompanionPetChat, `{"pet":"mel","message":"què recomanes avui?"}`)
	if rec.Code != 200 {
		t.Fatalf("pet chat: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Reply string `json:"reply"`
		Pet   string `json:"pet"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Pet != "mel" {
		t.Errorf("pet: %q", out.Pet)
	}
	if strings.Contains(out.Reply, "«") || strings.Contains(out.Reply, "»") {
		t.Errorf("les cometes d'eco s'han de netejar: %q", out.Reply)
	}
	if !strings.Contains(gotSystem, "Mel") {
		t.Error("el system ha de portar la persona de la Mel")
	}
}

// Segona volta: l'historial creix (la mascota recorda el que s'ha dit).
func TestPetChatHistoryGrows(t *testing.T) {
	s, _ := companionTestServer(t)

	var calls int
	var lastHist []llm.Message
	saved := petChatter
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		calls++
		lastHist = msgs
		return "resposta " + strings.Repeat("x", 20), nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = saved })

	postJSON(t, s.handleCompanionPetChat, `{"pet":"pixel","message":"hola"}`)
	postJSON(t, s.handleCompanionPetChat, `{"pet":"pixel","message":"com va?"}`)
	if calls != 2 {
		t.Fatalf("esperava 2 crides, tinc %d", calls)
	}
	// l'historial de la segona ha de contenir la primera pregunta i resposta
	joined := ""
	for _, m := range lastHist {
		joined += m.Content + "|"
	}
	if !strings.Contains(joined, "hola") || !strings.Contains(joined, "com va?") {
		t.Error("l'historial ha de recordar ambdós torns")
	}
	// mascotes diferents, historials diferents
	postJSON(t, s.handleCompanionPetChat, `{"pet":"mica","message":"tot bé?"}`)
	lastMica := lastHist
	joinedMica := ""
	for _, m := range lastMica {
		joinedMica += m.Content + "|"
	}
	if strings.Contains(joinedMica, "hola") {
		t.Error("l'historial de la Mica no pot tenir la conversa d'en Pixel")
	}
}

// pet desconegut → 400; missatge buit → 400.
func TestPetChatValidation(t *testing.T) {
	s, _ := companionTestServer(t)
	rec := postJSON(t, s.handleCompanionPetChat, `{"pet":"fantasma","message":"hola"}`)
	if rec.Code != 400 {
		t.Errorf("pet desconegut: %d", rec.Code)
	}
	rec = postJSON(t, s.handleCompanionPetChat, `{"pet":"mel","message":"   "}`)
	if rec.Code != 400 {
		t.Errorf("missatge buit: %d", rec.Code)
	}
}

func TestPetChatPersistsAndCanBeLoadedAfterRestart(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	saved := petChatter
	petChatter = func(context.Context, *llm.Client, *config.Config, config.Provider, config.Role, []llm.Message) (string, error) {
		return "Continuem demà amb el projecte.", nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = saved })

	first, _ := companionTestServer(t)
	rec := postJSON(t, first.handleCompanionPetChat, `{"pet":"pixel","message":"Recorda el fil del projecte"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: %d %s", rec.Code, rec.Body.String())
	}

	// Un servidor nou simula el procés després d'un reinici.
	restarted, _ := companionTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/companion/chat?pet=pixel", nil)
	got := httptest.NewRecorder()
	restarted.handleCompanionPetChat(got, req)
	if got.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", got.Code, got.Body.String())
	}
	var out struct {
		Pet     string `json:"pet"`
		History []struct {
			From string `json:"from"`
			Text string `json:"text"`
		} `json:"history"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Pet != "pixel" || len(out.History) != 2 {
		t.Fatalf("history: pet=%q messages=%d (%s)", out.Pet, len(out.History), got.Body.String())
	}
	if out.History[0].From != "user" || !strings.Contains(out.History[0].Text, "Recorda el fil") || out.History[1].From != "pet" {
		t.Fatalf("unexpected persisted messages: %+v", out.History)
	}
}
