package web

import (
	"encoding/json"
	"strings"
	"testing"

	"gregal/internal/events"
)

// En obrir el retro, cada especialista revisa en paral·lel: publica el
// veredicte i com a molt tres troballes (gravetat normalitzada, rebuts
// filtrats, arranjament), sempre després del resum; el que no hi aplica
// calla i el que falla no trenca res.
func TestRetroParallelReviewsPublishFindings(t *testing.T) {
	fake := &retroFake{reviews: true}
	s := retroTestServer(t, fake)
	c := retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{}`))
	if c.Degraded {
		t.Fatal("a failing reviewer must not degrade the channel")
	}
	var qa []retroMessage
	standupDone := false
	for _, m := range c.Messages {
		switch m.Author {
		case "qa":
			if !standupDone {
				t.Fatal("reviews must come after the stand-up")
			}
			qa = append(qa, m)
		case "coordinator", "builder", "reviewer":
			if len(qa) > 0 {
				t.Fatalf("stand-up message after a review: %+v", m)
			}
			standupDone = true
		default:
			t.Fatalf("%s should stay silent (applies=false or invalid JSON): %+v", m.Author, m)
		}
	}
	if len(qa) != 4 {
		t.Fatalf("qa: verdict + 3 findings expected, got %d: %+v", len(qa), qa)
	}
	if qa[0].Kind != retroKindVerdict || qa[0].Text != "The parser has no tests." {
		t.Fatalf("verdict first: %+v", qa[0])
	}
	f := qa[1]
	if f.Kind != retroKindFinding || f.Severity != "high" || f.Title != "Parser helper untested" ||
		strings.Join(f.Receipts, ",") != "e6" || f.FixSuggestion == "" {
		t.Fatalf("finding: %+v", f)
	}
	if qa[2].Severity != "low" {
		t.Fatalf("unknown severity becomes low: %+v", qa[2])
	}
	if n := retroFindingCounts(c); n["high"] != 1 || n["low"] != 2 {
		t.Fatalf("counts: %v", n)
	}
	// El revisor ha vist el diff i només els agents que han fet la feina.
	body := fake.body(t, "(id qa)")
	if !strings.Contains(body, "calc.go") || strings.Contains(body, "- security (") {
		t.Fatal("review prompt: evidence of the workers only")
	}
	// Les respostes posteriors veuen les troballes.
	if ctx := retroRenderChannel(c, ""); !strings.Contains(ctx, "FINDING (high) Parser helper untested") {
		t.Fatalf("reply context lacks findings:\n%s", ctx)
	}
	// Una troballa amb arranjament es pot engegar com un torn.
	if rec := retroPost(t, s, "/api/retro/fix", `{"message_id":"`+f.ID+`"}`); rec.Code != 202 && rec.Code != 200 {
		t.Fatalf("fix from a finding: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRetroValidateReview(t *testing.T) {
	c := &retroChannel{Agents: retroWithSpecialists(nil, "en"), Evidence: []retroEvidence{{ID: "e1"}}}
	if m, err := retroValidateReview(`{"applies":false,"verdict":"n/a"}`, c, "design"); err != nil || len(m) != 0 {
		t.Fatalf("applies=false must be silent: %v %v", m, err)
	}
	if _, err := retroValidateReview(``, c, "design"); err == nil {
		t.Fatal("empty output is an error (so it is retried)")
	}
	m, err := retroValidateReview(`{"verdict":"ok","findings":[{"severity":"medium","text":"x","receipts":["e1","e7"]},{"title":"","text":""}]}`, c, "code")
	if err != nil || len(m) != 2 || m[1].Severity != "medium" || strings.Join(m[1].Receipts, ",") != "e1" {
		t.Fatalf("got %+v %v", m, err)
	}
}

// Només revisa qui hi té on mirar: Disseny si hi ha interfície, Codi si hi
// ha fitxers, QA si hi ha fitxers o ordres; Funcionalitat i Seguretat sempre.
func TestRetroReviewersOnlyWhereTheyApply(t *testing.T) {
	ids := func(ev ...retroEvidence) string {
		c := &retroChannel{Title: "Run #3", Agents: retroWithSpecialists([]retroAgent{{ID: "coordinator"}}, "en"), Evidence: ev}
		var out []string
		for _, a := range retroReviewers(c) {
			out = append(out, a.ID)
		}
		return strings.Join(out, ",")
	}
	if got := ids(retroEvidence{Kind: "summary", Title: "final answer"}); got != "product,security" {
		t.Errorf("plain answer: %s", got)
	}
	if got := ids(retroEvidence{Kind: "file", Title: "internal/calc.go (+3 −1)"}); got != "qa,product,code,security" {
		t.Errorf("backend change: %s", got)
	}
	if got := ids(retroEvidence{Kind: "file", Title: "internal/web/app/retro.css (+9 −0)"}); got != "design,qa,product,code,security" {
		t.Errorf("UI change: %s", got)
	}
	if got := ids(retroEvidence{Kind: "file", Title: "internal/web/retro.go (+9 −0)"}); got != "qa,product,code,security" {
		t.Errorf("Go under web/ is backend: %s", got)
	}
	if got := ids(retroEvidence{Kind: "file", Title: "internal/web/app/retro.js (+9 −0)"}); !strings.HasPrefix(got, "design,") {
		t.Errorf("front-end JS: %s", got)
	}
	if got := ids(retroEvidence{Kind: "test", Title: "go test ./... ✓"}); got != "qa,product,security" {
		t.Errorf("only commands: %s", got)
	}
}

// El títol (la tasca que jutgen els revisors) surt del run_started del torn,
// no de la conversa carregada ara.
func TestRetroRunTitleFromRunStarted(t *testing.T) {
	evs := []events.Event{
		{RunID: 4, Kind: "run_started", Payload: json.RawMessage(`{"task":"other task"}`)},
		{RunID: 5, Kind: "run_started", Payload: json.RawMessage(`{"task":"Read every file and summarise it"}`)},
	}
	if got := (&Server{}).retroRunTitle(evs, 5, 9); got != "Read every file and summarise it" {
		t.Fatalf("title=%q", got)
	}
	if got := (&Server{}).retroRunTitle(nil, 3, 9); got != "Run #3" {
		t.Fatalf("fallback title=%q", got)
	}
}
