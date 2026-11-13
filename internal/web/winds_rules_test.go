package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regles en llenguatge natural: es desen netes, entren al context i el
// revisor cita la regla que toca.

func TestWindRulesSaveCleanAndLoad(t *testing.T) {
	s := goalTestServer(t)
	rec := httptest.NewRecorder()
	body := `{"alone":["  Reservar taula   si costa menys de 50 €  ","reservar taula si costa menys de 50 €",""],"ask":["Enviar correus"],"never":["Esborrar res de Documents"]}`
	s.handleWindRules(rec, httptest.NewRequest(http.MethodPost, "/api/winds/regles", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	got := loadWindRules(s.remindOwner())
	if len(got.Alone) != 1 || got.Alone[0] != "Reservar taula si costa menys de 50 €" {
		t.Fatalf("alone = %q", got.Alone)
	}
	if len(got.Ask) != 1 || len(got.Never) != 1 {
		t.Fatalf("rules = %+v", got)
	}
	brief := s.windRulesBrief()
	for _, want := range []string{"Never do:", "- Esborrar res de Documents", "Always ask the person first before", "You may do these on your own"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief sense %q:\n%s", want, brief)
		}
	}
	rec = httptest.NewRecorder()
	s.handleWindRules(rec, httptest.NewRequest(http.MethodGet, "/api/winds/regles", nil))
	if !strings.Contains(rec.Body.String(), "Enviar correus") {
		t.Fatalf("GET = %s", rec.Body)
	}
	_, _ = saveWindRules(s.remindOwner(), windRules{})
	if s.windRulesBrief() != "" {
		t.Fatal("sense regles no hi ha d'haver res al context")
	}
}

func TestWindRulesReviewCitesTheRule(t *testing.T) {
	s := goalTestServer(t)
	if _, err := saveWindRules(s.remindOwner(), windRules{Ask: []string{"Enviar correus"}, Never: []string{"Esborrar res de Documents"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = saveWindRules(s.remindOwner(), windRules{}) })
	saved := windRulesReviewer
	t.Cleanup(func() { windRulesReviewer = saved })
	calls := 0
	windRulesReviewer = func(_ *Server, _ context.Context, system, user string) (string, error) {
		calls++
		act := user[strings.Index(user, "Action:"):]
		switch {
		case strings.Contains(act, `"mail"`):
			return "ASK 2", nil
		case strings.Contains(act, "Documents"):
			return "NEVER 1", nil
		case strings.Contains(act, "mismatch"):
			return "ALONE 1", nil // el número no és d'una regla «alone»: no compta
		}
		return "NONE", nil
	}
	if v := s.windRulesReview(context.Background(), "mail", `{"action":"send","to":"a@b.c"}`); v.Kind != "ask" || v.Rule != "Enviar correus" {
		t.Fatalf("mail = %+v", v)
	}
	if v := s.windRulesReview(context.Background(), "bash", `{"command":"rm ~/Documents/x"}`); v.Kind != "never" {
		t.Fatalf("rm = %+v", v)
	}
	if v := s.windRulesReview(context.Background(), "bash", `{"command":"echo mismatch"}`); v.Kind != "" {
		t.Fatalf("una regla d'un altre tipus no pot valer: %+v", v)
	}
	// Llegir no es revisa (ni gasta cap crida).
	before := calls
	if v := s.windRulesReview(context.Background(), "read", `{"path":"Documents/x"}`); v.Kind != "" || calls != before {
		t.Fatalf("read = %+v, crides %d→%d", v, before, calls)
	}
	if v := s.windRulesReview(context.Background(), "browser", `{"action":"snapshot"}`); v.Kind != "" || calls != before {
		t.Fatalf("browser snapshot = %+v", v)
	}
	// La mateixa acció no es torna a preguntar al model.
	s.windRulesReview(context.Background(), "mail", `{"action":"send","to":"a@b.c"}`)
	if calls != before {
		t.Fatalf("la revisió s'havia de reaprofitar: %d crides", calls)
	}
}

func TestWindRuleHardGate(t *testing.T) {
	if !windRuleHardGate("publish", `{"name":"x"}`, "") {
		t.Fatal("publicar per primer cop és sempre una porta dura")
	}
	if windRuleHardGate("write", `{"path":"a.txt"}`, "") {
		t.Fatal("escriure no és una porta dura")
	}
}
