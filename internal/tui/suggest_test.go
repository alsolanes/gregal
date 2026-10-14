package tui

import (
	"testing"
)

func TestFilterCommands(t *testing.T) {
	got := filterCommands("/a")
	if len(got) != 4 || got[0].name != "agent" || got[1].name != "agents" || got[2].name != "autoreview" || got[3].name != "attach" {
		t.Fatalf("got=%v", got)
	}
	if got := filterCommands("/"); len(got) != 5 {
		t.Fatalf("topall 5: %d", len(got))
	}
	if got := filterCommands("/zzz"); len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
	got = filterCommands("rea")
	if len(got) != 1 || got[0].name != "read" {
		t.Fatalf("sense barra: %v", got)
	}
}
