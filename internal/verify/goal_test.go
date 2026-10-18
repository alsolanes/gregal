package verify

import (
	"strings"
	"testing"
)

func TestParseGoalVerdict(t *testing.T) {
	for _, c := range []struct {
		name, text string
		met        bool
		reason     string
	}{
		{"met with evidence", "VERDICT: MET\nCRITERIA:\n- [MET] a :: read a.go\n- [MET] b :: go test passed\nMISSING: none", true, ""},
		{"markdown bold verdict", "**VERDICT: MET**\n- [MET] a :: x", true, ""},
		{"not met", "VERDICT: NOT MET\n- [NOT MET] a :: no\nMISSING: add a", false, ""},
		{"met but a criterion unverified", "VERDICT: MET\n- [MET] a :: x\n- [UNVERIFIED] b :: could not", false, "UNVERIFIED"},
		{"met without evidence", "VERDICT: MET\n- [MET] a ::", false, "no evidence"},
		{"met without criteria", "VERDICT: MET\nlooks good", false, "no criteria"},
		{"no verdict", "I think it is fine", false, "no explicit verdict"},
	} {
		v := ParseGoalVerdict(c.text)
		if v.Met != c.met || (c.reason != "" && !strings.Contains(v.Reason, c.reason)) {
			t.Errorf("%s: %+v", c.name, v)
		}
	}
	v := ParseGoalVerdict("VERDICT: NOT MET\nCRITERIA:\n- [MET] a :: x\n- [NOT MET] b :: y\nMISSING: do b")
	if len(v.Criteria) != 2 || v.Criteria[1].Status != CriterionNotMet || v.Missing != "do b" {
		t.Fatalf("%+v", v)
	}
}

func TestBuildGoalPromptCarriesGoalAndEvidence(t *testing.T) {
	p := BuildGoalPrompt("task: x\ncriteria:\n- y", "CHECKS: none", "en")
	for _, want := range []string{"independent verifier of a goal", "task: x", "CHECKS: none", "VERDICT: NOT MET", "claims, not evidence"} {
		if !strings.Contains(p, want) {
			t.Errorf("missing %q", want)
		}
	}
}
