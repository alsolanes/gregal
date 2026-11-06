package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/team"
)

// Una colla que acaba una web la publica el servidor (els membres treballen en
// còpies i no poden): l'enllaç surt a la conclusió.
func TestCrewFinishPublishesTheSite(t *testing.T) {
	s, _ := companionTestServer(t)
	s.cwd = t.TempDir()
	if err := os.MkdirAll(filepath.Join(s.cwd, "web-cv"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.cwd, "web-cv", "index.html"), []byte("<!doctype html><title>CV</title><h1>Hola</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _ := s.winds.Save(companion.Wind{Name: "Gregal", Role: "Organització"})
	b, _ := s.winds.Save(companion.Wind{Name: "Ratxa", Role: "Webs"})
	savedDiscuss, savedWork, savedCoord := crewDiscussLoop, crewWork, crewCoordinator
	t.Cleanup(func() { waitBackground(); crewDiscussLoop, crewWork, crewCoordinator = savedDiscuss, savedWork, savedCoord })
	crewDiscussLoop = func(*Server, context.Context, string) (string, error) { return "D'acord.", nil }
	crewWork = func(*Server, context.Context, Rotllana, string, team.FlowSpec, bool) (string, error) { return "Fet i revisat: approve", nil }
	calls := 0
	crewCoordinator = func(_ *Server, _ context.Context, _, prompt string) (string, error) {
		calls++
		if calls == 1 {
			if !strings.Contains(prompt, "the server publishes it") {
				t.Errorf("the coordinator must know how to publish: %s", prompt)
			}
			return `{"action":"work","assignments":[{"wind":"` + b.ID + `","part":"la web"}]}`, nil
		}
		return `{"action":"finish","conclusion":"Web feta.","publish":{"dir":"web-cv","title":"CV"}}`, nil
	}
	rec := postJSON(t, s.handleCrewStart, `{"task":"Fes-me una web amb el meu CV","lang":"ca","reuse":["`+a.ID+`","`+b.ID+`"]}`)
	var out struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	waitCrew(t, s, out.Rotllana.ID, "done")
	hist := s.crewHistory(out.Rotllana)
	last := ""
	for _, m := range hist {
		if m.Kind == "result" {
			last = m.Content
		}
	}
	if !strings.Contains(last, "Publicada:") || !strings.Contains(last, "/share/") {
		for _, m := range hist {
			t.Logf("%s/%s: %.300s", m.Role, m.Kind, m.Content)
		}
		t.Fatalf("the conclusion must carry the published link:\n%s", last)
	}
	// Fora de la carpeta de treball no es publica res.
	if line := s.crewPublish(context.Background(), crewPublishReq{Dir: "../../"}, true); line != "" {
		t.Fatalf("outside the working folder: %s", line)
	}
}
