package web

import (
	"strings"
	"testing"
)

func TestMeasurePromptFindsWhereThePrefixBreaks(t *testing.T) {
	a := []promptBlock{{"base", "AAAA"}, {"now", "12:00"}, {"memory", "mmmm"}}
	b := []promptBlock{{"base", "AAAA"}, {"now", "12:01"}, {"memory", "mmmm"}}
	first := measurePrompt(a, "", false)
	if first.PreviousSeen || first.Chars != 13 || first.Tokens != 3 {
		t.Fatalf("first=%+v", first)
	}
	st := measurePrompt(b, joinPromptBlocks(a), true)
	if st.BreaksAt != "now" || st.StableChars != 8 {
		t.Fatalf("expected break at now after 8 chars, got %+v", st)
	}
	same := measurePrompt(a, joinPromptBlocks(a), true)
	if same.BreaksAt != "" || same.StableChars != same.Chars {
		t.Fatalf("identical prompts must be fully stable: %+v", same)
	}
	if s := st.String(); !strings.Contains(s, "breaks_at=now") || !strings.Contains(s, "base=4") {
		t.Fatalf("line=%q", s)
	}
}

func TestChatSystemBlocksJoinToTheSamePrompt(t *testing.T) {
	// El desglossament no pot canviar el que s'envia al model.
	blocks := []promptBlock{{"a", "x"}, {"b", ""}, {"c", "y"}}
	if got := joinPromptBlocks(blocks); got != "xy" {
		t.Fatalf("got %q", got)
	}
}
