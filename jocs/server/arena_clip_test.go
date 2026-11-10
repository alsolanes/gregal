package server

import "testing"

func TestClipSentences(t *testing.T) {
	long := "Challenger brings a loose playbook. Bot Analyst arrives with a full scouting report and a matchup table for every read. The key: if the economy holds, the analyst's structured defaults will grind out rounds"
	for _, c := range []struct {
		in   string
		max  int
		want string
	}{
		{"Short and sweet.", 100, "Short and sweet."},
		{long, 120, "Challenger brings a loose playbook. Bot Analyst arrives with a full scouting report and a matchup table for every read."},
		{"First one. Second one is cut mid sentence and goes on…", 700, "First one."},
		{"Nothing to cut at all because this is one very long sentence without any stop", 40, "Nothing to cut at all because this is…"},
		{"  spaced\n\nout  text. ", 100, "spaced out text."},
	} {
		if got := clipSentences(c.in, c.max); got != c.want {
			t.Errorf("clipSentences(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}
