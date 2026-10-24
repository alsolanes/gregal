package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/team"
)

func TestCrewProposalReusesFitAndCreatesDistinctNewWinds(t *testing.T) {
	s, _ := companionTestServer(t)
	front, _ := s.winds.Save(companion.Wind{Name: "Tramuntana", Role: "Frontend", Color: "#F7B2D9", Specialty: "frontend"})
	saved := crewPlanner
	t.Cleanup(func() { crewPlanner = saved })
	crewPlanner = func(*Server, context.Context, string, string) (string, error) {
		return "Sure:\n```json\n" + `{"plan":"Tramuntana builds, the new ones check it","reuse":[{"id":"` + front.ID + `","why":"web"},{"id":"ghost"}],` +
			`"create":[{"name":"Tramuntana","role":"Accessibilitat","kit":"UX","persona":"Checks contrast."},{"name":"Remolí","role":"Proves","kit":"nope","persona":""},{"name":"","role":""}]}` + "\n```", nil
	}
	winds, _ := s.winds.List()
	p := s.crewPropose(context.Background(), "Fes una web accessible", true, winds)
	if len(p.Reuse) != 1 || p.Reuse[0].ID != front.ID || p.Reuse[0].Name != "Tramuntana" {
		t.Fatalf("reuse = %+v", p.Reuse)
	}
	if len(p.Create) != 2 {
		t.Fatalf("create = %+v", p.Create)
	}
	colors := map[string]bool{"#F7B2D9": true}
	names := map[string]bool{"tramuntana": true}
	for _, c := range p.Create {
		if colors[c.Color] || names[strings.ToLower(c.Name)] || len(c.Style) != 3 || c.Persona == "" {
			t.Fatalf("new wind not distinct or incomplete: %+v", c)
		}
		colors[c.Color], names[strings.ToLower(c.Name)] = true, true
	}
	if p.Create[0].Kit != "ux" || p.Create[1].Kit != "" {
		t.Fatalf("kits = %q %q", p.Create[0].Kit, p.Create[1].Kit)
	}
}

func TestCrewProposalFallsBackWithoutModel(t *testing.T) {
	s, _ := companionTestServer(t)
	s.winds.Save(companion.Wind{Name: "Ponent", Role: "Llibres i cultura"})
	saved := crewPlanner
	t.Cleanup(func() { crewPlanner = saved })
	crewPlanner = func(*Server, context.Context, string, string) (string, error) { return "no json here", nil }
	winds, _ := s.winds.List()
	p := s.crewPropose(context.Background(), "Recomana llibres de cultura catalana", true, winds)
	if len(p.Reuse) != 1 || p.Reuse[0].Name != "Ponent" || p.Plan == "" {
		t.Fatalf("fallback = %+v", p)
	}
}

func TestCrewStartCreatesWindsDiscussesAndWorks(t *testing.T) {
	s, _ := companionTestServer(t)
	keeper, _ := s.winds.Save(companion.Wind{Name: "Gregal", Role: "Organització", Color: "#C1B4DE"})
	savedDiscuss, savedWork := crewDiscussLoop, crewWork
	t.Cleanup(func() { crewDiscussLoop, crewWork = savedDiscuss, savedWork })
	crewDiscussLoop = func(_ *Server, _ context.Context, brief string) (string, error) {
		if !strings.Contains(brief, "Discussió de disseny") {
			t.Errorf("brief misses the discussion prompt")
		}
		return "Jo m'encarrego de l'estructura.", nil
	}
	var flow team.FlowSpec
	var workTask string
	crewWork = func(_ *Server, _ context.Context, _ Rotllana, task string, f team.FlowSpec, _ bool) (string, error) {
		flow, workTask = f, task
		return "Web feta: index.html", nil
	}
	rec := postJSON(t, s.handleCrewStart, `{"task":"Fes una web del sopar","lang":"ca","reuse":["`+keeper.ID+`"],"create":[{"name":"Ratxa","role":"Disseny web","kit":"frontend","persona":"Draws pretty pages."}]}`)
	if rec.Code != 200 {
		t.Fatalf("start = %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Rotllana Rotllana         `json:"rotllana"`
		Created  []companion.Wind `json:"created"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Created) != 1 || out.Created[0].Name != "Ratxa" || out.Created[0].Specialty != "frontend" || out.Created[0].Color == "#C1B4DE" {
		t.Fatalf("created = %+v", out.Created)
	}
	if _, ok := s.winds.ByID(out.Created[0].ID); !ok {
		t.Fatal("the summoned wind must stay available for later tasks")
	}
	if len(out.Rotllana.Members) != 2 || out.Rotllana.Members[0] != keeper.ID {
		t.Fatalf("members = %v", out.Rotllana.Members)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		v, _ := crewJobs.Load(out.Rotllana.ID)
		if v != nil && v.(*crewJob).get() == "done" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("crew job did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if flow.Output != "reviewer" || flow.Nodes[0].Wind != keeper.ID || flow.Nodes[1].Wind != out.Created[0].ID {
		t.Fatalf("flow = %+v", flow)
	}
	if !strings.Contains(workTask, "Jo m'encarrego de l'estructura") {
		t.Fatal("the work must carry the agreed design")
	}
	hist := s.crewHistory(out.Rotllana)
	var said []string
	for _, m := range hist {
		said = append(said, m.Content)
	}
	joined := strings.Join(said, "\n")
	for _, want := range []string{"Fes una web del sopar", "Gregal: Jo m'encarrego", "Ratxa: Jo m'encarrego", "Som-hi", "Fet. Web feta"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("history misses %q:\n%s", want, joined)
		}
	}
}

func TestCrewColourIsFarFromExistingOnes(t *testing.T) {
	used := map[string]bool{"#A8D8EA": true, "#9CC4E8": true, "#F4B6C2": true}
	got := nextCrewColor(used)
	for u := range used {
		if colorDistance(got, u) < 3000 {
			t.Fatalf("%s is too close to %s", got, u)
		}
	}
}

func TestCrewRejectedReview(t *testing.T) {
	for _, answer := range []string{"Web\n\nOutstanding review feedback:\nmissing file", "## Veredicte: ❌ REBUTJAT", `{"approved": false}`} {
		if !crewRejected(answer) {
			t.Fatalf("not seen as rejected: %q", answer)
		}
	}
	if crewRejected("Web feta: index.html amb el calendari") {
		t.Fatal("a finished deliverable is not a rejection")
	}
}

func TestCrewReusesTheWindThatAlreadyKnowsTheTrade(t *testing.T) {
	s, _ := companionTestServer(t)
	brisa, _ := s.winds.Save(companion.Wind{Name: "Brisa", Role: "Horticultura de balcó", Color: "#F7B2D9"})
	s.winds.Save(companion.Wind{Name: "Mestral", Role: "Visualització i revisió"})
	winds, _ := s.winds.List()
	p := crewSanitise(crewProposal{Create: []crewNew{{Name: "Garbinell", Role: "Horticultura i plagues del tomàquet"}, {Name: "Lupa", Role: "Revisió de qualitat"}}}, winds, "plagues", true)
	if len(p.Reuse) != 1 || p.Reuse[0].ID != brisa.ID {
		t.Fatalf("Brisa should be reused: %+v", p)
	}
	if len(p.Create) != 1 || p.Create[0].Name != "Lupa" {
		t.Fatalf("a generic word must not merge winds: %+v", p.Create)
	}
}

func TestCrewClosingReadsTheVerdict(t *testing.T) {
	text, changes := crewClosing("```json\n{\"verdict\": \"changes_requested\", \"resum\": \"Falta la cinquena fitxa.\"}\n```\n\nFitxers: formatges.html")
	if !changes || text != "Falta la cinquena fitxa.\n\nFitxers: formatges.html" {
		t.Fatalf("got %q %v", text, changes)
	}
	text, changes = crewClosing("{\"verdict\": \"canvis_menors\", \"resum\": \"Compleix la petició.\"}")
	if changes || text != "Compleix la petició." {
		t.Fatalf("minor changes are a pass: %q %v", text, changes)
	}
	if text, changes := crewClosing("He escrit lisboa.md amb l'itinerari."); changes || text != "He escrit lisboa.md amb l'itinerari." {
		t.Fatalf("plain text: %q %v", text, changes)
	}
}
