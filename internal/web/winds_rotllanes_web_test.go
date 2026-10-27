package web

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// rotllanaWebLoop captura el brief que rep el loop de la rotllana quan es fa
// servir el camí de cerca (runReadOnlyLoop amb eines web), i respon fix.
func rotllanaWebLoop(t *testing.T, reply string) (captured *string) {
	t.Helper()
	captured = new(string)
	saved := rotllanaVoiceLoop
	rotllanaVoiceLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
		*captured = brief
		return exploracio{text: reply}, nil
	}
	t.Cleanup(func() { rotllanaVoiceLoop = saved })
	return captured
}

// TestRotllanaWebSearch: la rotllana és el punt clau dels vents com a
// companys de cerques web: el vent que parla ha de poder buscar al web
// (web_search/web_fetch) com al xat directe, no contestar «de memòria».
func TestRotllanaWebSearch(t *testing.T) {
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
	captured := rotllanaWebLoop(t, "Mestral: he buscat i l'he trobat. [example.com](https://example.com)")
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+group.Rotllana.ID+`","message":"busca'm què és el Mistral"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if *captured == "" {
		t.Fatal("la rotllana no ha rebut cap brief — el camí de cerca no s'ha fet servir")
	}
	for _, want := range []string{"Mestral", "GROUP CHAT", "web_search", "busca'm què és el Mistral"} {
		if !strings.Contains(*captured, want) {
			t.Errorf("brief sense %q:\n%s", want, *captured)
		}
	}
	// La resposta de la cerca queda com a missatge del parlant amb la seva identitat.
	var out struct {
		History []histMsg `json:"history"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.History) != 2 {
		t.Fatalf("vull 1 usuari + 1 resposta amb fonts, tinc %d: %s", len(out.History), rec.Body.String())
	}
	last := out.History[1]
	if last["speaker_id"] != "mestral" || last["text"] != "Mestral: he buscat i l'he trobat. [example.com](https://example.com)" {
		t.Fatalf("resposta sense identitat del parlant: %+v", last)
	}
}

// TestRotllanaWebSearchFallbackToChatter: si el camí de cerca falla, el torn
// no es perd — cau al xat pla (petChatter), com feia abans.
func TestRotllanaWebSearchFallbackToChatter(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)
	created := postJSON(t, s.handleRotllanes, `{"name":"Solo","members":["xaloc"]}`)
	var group struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	forceErr := errors.New("search unavailable")
	saved := rotllanaVoiceLoop
	rotllanaVoiceLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) { return exploracio{}, forceErr }
	t.Cleanup(func() { rotllanaVoiceLoop = saved })
	savedChatter := petChatter
	calls := 0
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		calls++
		return "Xaloc: resposta pla", nil
	}
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		calls++
		return "Xaloc: resposta pla", nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = savedChatter })
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+group.Rotllana.ID+`","message":"hola"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if calls == 0 {
		t.Fatal("el fallback a petChatter no s'ha executat")
	}
}

// TestRotllanaBriefCarriesBrainAndHistory: el brief de la rotllana porta el
// cervell del vent i la conversa prèvia, igual que el xat directe.
func TestRotllanaBriefCarriesBrainAndHistory(t *testing.T) {
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
	if err := saveRotllanaHistory(s, group.Rotllana.ID, []llm.Message{{Role: "user", Content: "recordes quan vam parlar de la Björk?"}}); err != nil {
		t.Fatal(err)
	}
	captured := rotllanaWebLoop(t, "Xaloc: sí, recordo")
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+group.Rotllana.ID+`","message":"què saps de la Björk?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"Björk", "ALREADY KNOW", "recordes quan vam parlar"} {
		if !strings.Contains(*captured, want) {
			t.Errorf("brief sense %q:\n%s", want, *captured)
		}
	}
}
