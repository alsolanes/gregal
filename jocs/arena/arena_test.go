package arena

import (
	"encoding/json"
	"testing"
)

func TestParseAnswerNeverReturnsInvalidJSON(t *testing.T) {
	cases := map[string]string{
		`{"decision":{"play":"run_inside"},"why":"x","rule":"r"}`: `{"play":"run_inside"}`,
		"Sure!\n```json\n{\"decision\": {\"call\": \"cover2\",}, \"why\": \"deep\"}\n```": `{"call": "cover2"}`,
		`{"play":"screen","target":"rb"}`:                            `{"play":"screen","target":"rb"}`,
		`{'decision': {'play': 'punt'}}`:                             `{"play": "punt"}`,
		`no json at all`:                                             ``,
		`{"decision": {"play": "run" "x"}}`:                          ``,
	}
	for in, want := range cases {
		a := ParseAnswer(in)
		if len(a.Decision) > 0 && !json.Valid(a.Decision) {
			t.Fatalf("%q → JSON invàlid %q", in, a.Decision)
		}
		if string(a.Decision) != want {
			t.Errorf("%q → %q, volia %q", in, a.Decision, want)
		}
	}
}
