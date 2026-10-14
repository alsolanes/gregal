package web

import (
	"reflect"
	"testing"
)

func TestRetroRespondersForTopicsMentionsAndAll(t *testing.T) {
	c := &retroChannel{Lang: "ca", Agents: retroWithSpecialists([]retroAgent{
		{ID: "coordinator", Name: "Coordinator"}, {ID: "researcher", Name: "Researcher"},
	}, "ca")}
	cases := map[string][]string{
		"què has fet exactament?":             {"coordinator"},
		"és segur guardar el token aquí?":     {"coordinator", "security"},
		"why didn't you run the tests?":       {"coordinator", "code"},
		"fa el que t'he demanat? i és segur?": {"coordinator", "product", "security"},
		"@seguretat mira-ho tu":               {"security"},
		"@researcher què vas trobar?":         {"researcher"},
		"@tots què en penseu?":                {"coordinator", "researcher", "product", "code", "security"},
	}
	for text, want := range cases {
		if got := retroRespondersFor(text, c); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", text, got, want)
		}
	}
}

func TestRetroSpecialistsAddedOnceAndLocalized(t *testing.T) {
	a := retroWithSpecialists([]retroAgent{{ID: "coordinator"}}, "en")
	a = retroWithSpecialists(a, "en")
	if len(a) != 4 {
		t.Fatalf("got %d agents, want 4 (no duplicates)", len(a))
	}
	if a[3].Name != "Security" || !retroIsSpecialist(&a[3]) || retroIsSpecialist(&a[0]) {
		t.Fatalf("unexpected specialists: %+v", a)
	}
	if retroSpecialists("ca")[0].Name != "Funcionalitat" {
		t.Fatal("Catalan names expected")
	}
}

func TestRetroStandupSpecialistSpeaksAtMostOnce(t *testing.T) {
	c := &retroChannel{Agents: retroWithSpecialists([]retroAgent{{ID: "coordinator", Name: "Coordinator"}}, "en"),
		Evidence: []retroEvidence{{ID: "e1", Agent: "coordinator", Title: "read a.go"}}}
	js := `{"messages":[{"agent":"coordinator","text":"read a.go","receipts":["e1"]},
	  {"agent":"security","text":"no secrets in e1","receipts":["e1"]},
	  {"agent":"security","text":"second take","receipts":["e1"]}],"reactions":[]}`
	msgs, _, err := retroValidateStandup(js, c)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, m := range msgs {
		if m.Author == "security" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("security spoke %d times, want 1", n)
	}
}

func TestRetroFallbackSkipsSpecialists(t *testing.T) {
	c := &retroChannel{Lang: "en", Agents: retroWithSpecialists([]retroAgent{{ID: "coordinator", Name: "Coordinator"}}, "en")}
	for _, m := range retroFallbackStandup(c, nil) {
		if m.Author != "coordinator" {
			t.Fatalf("fallback made %s speak without evidence", m.Author)
		}
	}
}
