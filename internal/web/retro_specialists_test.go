package web

import (
	"reflect"
	"testing"
)

func TestRetroRespondersForMentionsAllAndAuto(t *testing.T) {
	c := &retroChannel{Lang: "ca", Agents: retroWithSpecialists([]retroAgent{
		{ID: "coordinator", Name: "Coordinator"}, {ID: "researcher", Name: "Researcher"},
	}, "ca")}
	all := []string{"coordinator", "researcher", "design", "product", "code", "security"}
	cases := map[string]struct {
		ids  []string
		auto bool
	}{
		"a nivell estètic estaria bé alguna millora visual?": {all, true},
		"@seguretat mira-ho tu":                               {[]string{"security"}, false},
		"@disseny i @codi, què en penseu?":                    {[]string{"design", "code"}, false},
		"@tots què en penseu?":                                {all, false},
	}
	for text, want := range cases {
		ids, auto := retroRespondersFor(text, c)
		if !reflect.DeepEqual(ids, want.ids) || auto != want.auto {
			t.Errorf("%q: got %v auto=%v, want %v auto=%v", text, ids, auto, want.ids, want.auto)
		}
	}
	if h := retroTopicHints("a nivell estètic estaria bé alguna millora visual?", c); !reflect.DeepEqual(h, []string{"design"}) {
		t.Errorf("design hint expected, got %v", h)
	}
}

func TestRetroSpecialistsAddedOnceAndLocalized(t *testing.T) {
	a := retroWithSpecialists([]retroAgent{{ID: "coordinator"}}, "en")
	a = retroWithSpecialists(a, "en")
	if len(a) != 5 {
		t.Fatalf("got %d agents, want 5 (no duplicates)", len(a))
	}
	if a[1].Name != "Design" || a[4].Name != "Security" || !retroIsSpecialist(&a[4]) || retroIsSpecialist(&a[0]) {
		t.Fatalf("unexpected specialists: %+v", a)
	}
	if retroSpecialists("ca")[0].Name != "Disseny" {
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

// «Resum ràpid»: no espera el model, dona el resum automàtic de l'evidència.
func TestRetroQuickStartSkipsTheModel(t *testing.T) {
	fake := &retroFake{}
	s := retroTestServer(t, fake)
	before := fake.calls.Load()
	c := retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{"quick":true}`))
	if fake.calls.Load() != before {
		t.Fatalf("quick start must not call the model (%d calls)", fake.calls.Load()-before)
	}
	if !c.Degraded || len(c.Messages) == 0 {
		t.Fatalf("quick start returns the automatic summary: degraded=%v msgs=%d", c.Degraded, len(c.Messages))
	}
}
