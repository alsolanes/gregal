package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/team"
)

// Respondre la pregunta d'una colla pel xat en directe segueix la feina fins
// al final. Abans el flux es tancava de seguida («silenci») i la colla
// treballava sense que l'app en mostrés res fins al missatge següent.
func TestCrewAnswerStreamFollowsTheWork(t *testing.T) {
	s, _ := companionTestServer(t)
	a, _ := s.winds.Save(companion.Wind{Name: "Gregal", Role: "Organització"})
	b, _ := s.winds.Save(companion.Wind{Name: "Ratxa", Role: "Disseny"})
	savedWork, savedCoord := crewWork, crewCoordinator
	t.Cleanup(func() {
		waitBackground()
		crewWork, crewCoordinator = savedWork, savedCoord
	})

	crewWork = func(*Server, context.Context, Rotllana, string, team.FlowSpec, bool) (string, error) {
		return "Fet", nil
	}
	calls := 0
	crewCoordinator = func(_ *Server, _ context.Context, _, _ string) (string, error) {
		calls++
		switch calls {
		case 1:
			return `{"action":"ask","question":"Per a quantes persones?","options":["Dues","Sis"]}`, nil
		case 2:
			return `{"action":"work","assignments":[{"wind":"` + b.ID + `","part":"menú"}]}`, nil
		default:
			return `{"action":"finish","conclusion":"Menú per a sis fet."}`, nil
		}
	}
	rec := postJSON(t, s.handleCrewStart, `{"task":"Prepara el sopar","lang":"ca","reuse":["`+a.ID+`","`+b.ID+`"]}`)
	var out struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	waitCrew(t, s, out.Rotllana.ID, "waiting")
	req := httptest.NewRequest("POST", "/api/winds/rotllana/stream", strings.NewReader(`{"rotllana":"`+out.Rotllana.ID+`","message":"Sis"}`))
	stream := httptest.NewRecorder()
	s.handleRotllanaChatStream(stream, req)
	body := stream.Body.String()
	if !strings.Contains(body, "Menú per a sis fet") || !strings.Contains(body, "event: done") {
		t.Fatalf("el flux ha de seguir la colla fins a la conclusió:\n%s", body)
	}
}
