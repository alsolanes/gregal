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
	savedDiscuss, savedWork, savedCoord := crewDiscussLoop, crewWork, crewCoordinator
	t.Cleanup(func() { crewDiscussLoop, crewWork, crewCoordinator = savedDiscuss, savedWork, savedCoord })
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
		return "Web feta: index.html i style.css", nil
	}
	// El coordinador: primer una volta amb tots dos en paral·lel, després acaba.
	calls := 0
	var ratxaID string
	crewCoordinator = func(_ *Server, _ context.Context, _, prompt string) (string, error) {
		calls++
		if calls == 1 {
			if !strings.Contains(prompt, "Fes una web del sopar") || !strings.Contains(prompt, "Jo m'encarrego") {
				t.Errorf("the coordinator must see the task and the discussion: %s", prompt)
			}
			return `{"action":"work","say":"Som-hi en paral·lel","assignments":[{"wind":"` + keeper.ID + `","part":"l'estructura","files":["index.html"]},{"wind":"` + ratxaID + `","part":"els estils","files":["style.css","index.html"]}]}`, nil
		}
		if !strings.Contains(prompt, "Round 1") {
			t.Errorf("the second decision must see the round: %s", prompt)
		}
		return `{"action":"finish","conclusion":"Web feta amb el menú del sopar.","checked":["index.html obre bé"],"unchecked":["mòbil"]}`, nil
	}
	// L'id del vent nou encara no existeix: el coordinador el llegeix de la colla.
	origCoord := crewCoordinator
	crewCoordinator = func(s2 *Server, ctx context.Context, sys, prompt string) (string, error) {
		if ratxaID == "" {
			if list, _ := s2.winds.List(); len(list) > 0 {
				for _, w := range list {
					if w.Name == "Ratxa" {
						ratxaID = w.ID
					}
				}
			}
		}
		return origCoord(s2, ctx, sys, prompt)
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
	waitCrew(t, out.Rotllana.ID, "done")
	// Dos constructors alhora (cada un amb els seus fitxers; index.html no es
	// repeteix) i un revisor que ho mira tot.
	if len(flow.Nodes) != 3 || flow.Output != "review" || flow.Nodes[0].Wind != keeper.ID || flow.Nodes[1].Wind != out.Created[0].ID {
		t.Fatalf("flow = %+v", flow)
	}
	if !strings.Contains(flow.Nodes[1].Instruction, "style.css") || strings.Contains(flow.Nodes[1].Instruction, "style.css, index.html") {
		t.Fatalf("each file belongs to one member: %s", flow.Nodes[1].Instruction)
	}
	if len(flow.Edges) != 2 || flow.Edges[0].To != "review" {
		t.Fatalf("edges = %+v", flow.Edges)
	}
	if !strings.Contains(workTask, "Jo m'encarrego de l'estructura") {
		t.Fatal("the work must carry the agreed design")
	}
	kinds := map[string]int{}
	var said []string
	for _, m := range s.crewHistory(out.Rotllana) {
		said = append(said, m.Content)
		kinds[m.Kind]++
	}
	joined := strings.Join(said, "\n")
	for _, want := range []string{"Fes una web del sopar", "Gregal: Jo m'encarrego", "Ratxa: Jo m'encarrego", "Som-hi en paral·lel", "**Conclusió**", "Web feta amb el menú", "**Comprovat**", "index.html obre bé", "**Sense comprovar**"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("history misses %q:\n%s", want, joined)
		}
	}
	if kinds["debate"] < 3 || kinds["result"] != 1 {
		t.Fatalf("kinds = %v", kinds)
	}
}

func waitCrew(t *testing.T, rotID, step string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		v, _ := crewJobs.Load(rotID)
		if v != nil && v.(*crewJob).get() == step {
			return
		}
		if time.Now().After(deadline) {
			got := ""
			if v != nil {
				got = v.(*crewJob).get()
			}
			t.Fatalf("crew job did not reach %q (now %q)", step, got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestCrewAsksAndResumes: la colla fa una pregunta, s'atura, i la resposta de
// l'usuari al fil la reprèn (sense passar pel xat de sempre).
func TestCrewAsksAndResumes(t *testing.T) {
	s, _ := companionTestServer(t)
	a, _ := s.winds.Save(companion.Wind{Name: "Gregal", Role: "Organització"})
	b, _ := s.winds.Save(companion.Wind{Name: "Ratxa", Role: "Disseny"})
	savedDiscuss, savedWork, savedCoord := crewDiscussLoop, crewWork, crewCoordinator
	t.Cleanup(func() { crewDiscussLoop, crewWork, crewCoordinator = savedDiscuss, savedWork, savedCoord })
	crewDiscussLoop = func(*Server, context.Context, string) (string, error) { return "D'acord.", nil }
	worked := 0
	crewWork = func(*Server, context.Context, Rotllana, string, team.FlowSpec, bool) (string, error) {
		worked++
		return "Fet", nil
	}
	calls := 0
	crewCoordinator = func(_ *Server, _ context.Context, _, prompt string) (string, error) {
		calls++
		switch calls {
		case 1:
			return `{"action":"ask","question":"Per a quantes persones és el sopar?"}`, nil
		case 2:
			if !strings.Contains(prompt, "The user answered the crew's question: Sis") {
				t.Errorf("the answer must reach the coordinator: %s", prompt)
			}
			return `{"action":"work","assignments":[{"wind":"` + b.ID + `","part":"menú per a sis"}]}`, nil
		default:
			return `{"action":"finish","conclusion":"Menú per a sis."}`, nil
		}
	}
	rec := postJSON(t, s.handleCrewStart, `{"task":"Prepara el sopar","lang":"ca","reuse":["`+a.ID+`","`+b.ID+`"]}`)
	var out struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	waitCrew(t, out.Rotllana.ID, "waiting")
	if worked != 0 {
		t.Fatal("asking must stop the crew before any work")
	}
	if st, ok := loadCrewState(s, out.Rotllana.ID); !ok || !st.Waiting || st.Questions != 1 {
		t.Fatalf("the question must be saved: %+v", st)
	}
	if got := crewStepDesat(s, out.Rotllana.ID); got != "waiting" {
		t.Fatalf("after a restart it still waits, got %q", got)
	}
	hist := s.crewHistory(out.Rotllana)
	if last := hist[len(hist)-1]; last.Kind != "question" || !strings.Contains(last.Content, "quantes persones") {
		t.Fatalf("question: %+v", last)
	}
	// La resposta al fil reprèn la colla.
	rot, _ := s.rotllanaByID(out.Rotllana.ID)
	if !s.crewTakeAnswer(rot, "Sis") {
		t.Fatal("the answer must be taken by the waiting crew")
	}
	waitCrew(t, out.Rotllana.ID, "done")
	if worked != 1 {
		t.Fatalf("work rounds = %d", worked)
	}
	if s.crewTakeAnswer(rot, "una altra cosa") {
		t.Fatal("a finished crew does not take messages")
	}
	if _, ok := loadCrewState(s, out.Rotllana.ID); ok {
		t.Fatal("the state is removed when the crew finishes")
	}
}

func TestCrewSanitiseDecision(t *testing.T) {
	members := []companion.Wind{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	st := &crewState{Task: "x"}
	// Res de vàlid a la primera: construeix el primer membre.
	if d := crewSanitiseDecision(crewDecision{Action: "dance"}, st, members); d.Action != "work" || d.Assignments[0].Wind != "a" {
		t.Fatalf("fallback: %+v", d)
	}
	// Membres inventats fora; fitxers repetits o que surten del projecte, fora.
	d := crewSanitiseDecision(crewDecision{Action: "work", Assignments: []crewAssignment{
		{Wind: "zz", Part: "p"}, {Wind: "a", Files: []string{"x.go", "../fora.go"}}, {Wind: "b", Files: []string{"x.go", "y.go"}},
	}}, st, members)
	if len(d.Assignments) != 2 || len(d.Assignments[0].Files) != 1 || d.Assignments[1].Files[0] != "y.go" {
		t.Fatalf("assignments: %+v", d.Assignments)
	}
	// Dades inventades abans de la primera volta: es pregunta, encara que el model digui «work».
	fresh := &crewState{Task: "x", Ca: true}
	if d := crewSanitiseDecision(crewDecision{Action: "work", Assumed: []string{"8 convidats", "100 €"}, Assignments: []crewAssignment{{Wind: "a"}}}, fresh, members); d.Action != "ask" || !strings.Contains(d.Question, "8 convidats; 100 €") {
		t.Fatalf("assumed: %+v", d)
	}
	// Després d'una resposta (o d'una volta), ja no.
	fresh.Questions = 1
	if d := crewSanitiseDecision(crewDecision{Action: "work", Assumed: []string{"x"}, Assignments: []crewAssignment{{Wind: "a"}}}, fresh, members); d.Action != "work" {
		t.Fatalf("after an answer: %+v", d)
	}
	// Sense voltes, acaba; sense preguntes, no pregunta.
	st.Round, st.Questions = crewMaxRounds, crewMaxQuestions
	if d := crewSanitiseDecision(crewDecision{Action: "work", Assignments: []crewAssignment{{Wind: "a"}}}, st, members); d.Action != "finish" {
		t.Fatalf("rounds: %+v", d)
	}
	if d := crewSanitiseDecision(crewDecision{Action: "ask", Question: "?"}, st, members); d.Action != "finish" {
		t.Fatalf("questions: %+v", d)
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
