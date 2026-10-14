package verify

import (
	"strings"
	"testing"
)

func TestParseScore(t *testing.T) {
	cases := map[string]int{"SCORE: 7\n- ok": 7, "score = 10/10": 10, "**SCORE: 0**": 0, "Some text\nSCORE: 4 / 10": 4}
	for in, want := range cases {
		if got, ok := ParseScore(in); !ok || got != want {
			t.Fatalf("ParseScore(%q) = %d,%v; vull %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"no score", "SCORE: 11", ""} {
		if _, ok := ParseScore(bad); ok {
			t.Fatalf("ParseScore(%q) havia de fallar", bad)
		}
	}
}

func TestBuildScorePrompt(t *testing.T) {
	p := BuildScorePrompt("tasca X", "", "$ go test → PASS", "ca")
	for _, w := range []string{"SCORE: <integer 0-10>", "tasca X", "DIFF: (empty)", "go test → PASS", "RUBRIC"} {
		if !strings.Contains(p, w) {
			t.Fatalf("el prompt no porta %q", w)
		}
	}
}
