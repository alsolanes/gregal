package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// rotllanaChatter crea una rotllana de prova amb dos vents i substitueix el
// chatter pel text donat.
func rotllanaChatter(t *testing.T, reply string) (*Server, string) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Mestral","color":"#29ADFF","role":"sistema"}`)
	postJSON(t, s.handleWinds, `{"name":"Marinada","color":"#A3E24E","role":"música"}`)
	created := postJSON(t, s.handleRotllanes, `{"name":"Capvespre","members":["mestral","marinada"]}`)
	var group struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	saved := petChatter
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		return reply, nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = saved })
	return s, group.Rotllana.ID
}

func TestRotllanaOneCallCannotSpeakForAnotherWind(t *testing.T) {
	s, id := rotllanaChatter(t, "Mestral: Mira: https://example.com\nNota: bon dia\nMarinada: veu inventada")
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"comencem?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	disk, err := session.Load(s.sessDir(), "rotllana-"+id)
	if err != nil {
		t.Fatal(err)
	}
	if len(disk.Convo) != 2 || disk.Convo[1].SpeakerID != "mestral" ||
		disk.Convo[1].Content != "Mira: https://example.com\nNota: bon dia" {
		t.Fatalf("one call must produce only the assigned speaker: %+v", disk.Convo)
	}
}

// TestRotllanaBriefIncludesWindBrain: el brief del vent que parla a la
// rotllana (grup o conversa d'un) porta el seu cervell, igual que el xat
// directe — saber el que ja sap no pot dependre de si parla sol o acompanyat.
func TestRotllanaBriefIncludesWindBrain(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)
	if err := companion.AppendWindBrain(s.windStateDir(), "xaloc", []string{"- La Björk treu disc el 2026 [bjork.com]"}); err != nil {
		t.Fatal(err)
	}
	created := postJSON(t, s.handleRotllanes, `{"name":"Solo","members":["xaloc"]}`)
	var group struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	var gotSystem string
	saved := petChatter
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		gotSystem = msgs[0].Content
		return "entèsos", nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = saved })
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+group.Rotllana.ID+`","message":"què saps de la Björk?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(gotSystem, "Björk") || !strings.Contains(gotSystem, "ALREADY KNOW") {
		t.Fatalf("el brief del grup no porta el cervell del vent:\n%s", gotSystem)
	}
}
