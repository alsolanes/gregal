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

// TestRotllanaSplitsMultiVoiceReply: quan el model fa parlar més d'un vent al
// mateix torn ("Mestral: …\nMarinada: …"), cada veu queda com un missatge
// propi amb la seva identitat — mai enganxades dins d'un sol missatge.
func TestRotllanaSplitsMultiVoiceReply(t *testing.T) {
	s, id := rotllanaChatter(t, "Mestral: endavant!\nMarinada: jo hi sumo una idea.")
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"comencem?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		History []map[string]string `json:"history"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.History) != 3 {
		t.Fatalf("vull 1 missatge d'usuari + 1 per veu, tinc %d: %s", len(out.History), rec.Body.String())
	}
	first, second := out.History[1], out.History[2]
	if first["speaker_id"] != "mestral" || second["speaker_id"] != "marinada" {
		t.Fatalf("veus sense identitat pròpia: %+v %+v", first, second)
	}
	if !strings.Contains(first["text"], "endavant!") || !strings.Contains(second["text"], "jo hi sumo una idea.") {
		t.Fatalf("text de les veus mal partit: %+v %+v", first, second)
	}
	if strings.Contains(first["text"], "Marinada") {
		t.Fatalf("la veu de mestral encara porta text de marinada: %+v", first)
	}

	// Durador: cada veu desada com a missatge propi amb la seva identitat.
	disk, err := session.Load(s.sessDir(), "rotllana-"+id)
	if err != nil {
		t.Fatal(err)
	}
	if len(disk.Convo) != 3 || disk.Convo[1].SpeakerID != "mestral" || disk.Convo[2].SpeakerID != "marinada" {
		t.Fatalf("el durador no té les veus separades: %+v", disk.Convo)
	}

	// Rotació: cada veu compta pel seu vent. Després del primer torn, tots dos
	// han parlat un cop → el proper torn automàtic torna al primer membre.
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		return "jo altre cop", nil
	}
	rec = postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"seguim"}`)
	if rec.Code != 200 {
		t.Fatalf("chat2: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"wind":"mestral"`) {
		t.Errorf("cada veu ha de comptar al torn; hauria de parlar mestral: %s", rec.Body.String())
	}
}

// TestRotllanaSplitsPreambleAndColons: les línies sense prefix van amb el
// vent que parlava el torn; els dos punts normals i les URLs no parteixen res.
func TestRotllanaSplitsPreambleAndColons(t *testing.T) {
	s, id := rotllanaChatter(t, "Endavant tots! Mira: https://example.com\nMarinada: i jo hi sumo.\nNota: bon dia")
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"comencem?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		History []map[string]string `json:"history"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.History) != 3 {
		t.Fatalf("el prefix fals «Nota:» no ha de crear veu: %s", rec.Body.String())
	}
	if out.History[1]["speaker_id"] != "mestral" || !strings.Contains(out.History[1]["text"], "https://example.com") {
		t.Fatalf("el preàmbul pertany a mestral (vent del torn): %+v", out.History[1])
	}
	if out.History[2]["speaker_id"] != "marinada" || !strings.Contains(out.History[2]["text"], "Nota: bon dia") {
		t.Fatalf("els dos punts normals i les línies continuades pertanyen a marinada: %+v", out.History[2])
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
