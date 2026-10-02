package tui

import (
	"strings"
	"testing"

	"gregal/internal/tools"
)

func TestReviewMenuAgrupaHunks(t *testing.T) {
	f := tools.DiffFile{Path: "a.go", Hunks: []tools.DiffHunk{{Index: 0, Added: 2, Removed: 1, Patch: "patch"}, {Index: 1, Added: 1}}}
	mn := reviewMenu([]tools.DiffFile{f})
	if mn == nil || len(mn.items) != 2 || !strings.Contains(mn.items[0].text, "a.go") {
		t.Fatalf("menú=%+v", mn)
	}
	_, sub := mn.run(&Model{}, mn.items[0].val)
	if sub == nil || len(sub.items) != 3 {
		t.Fatalf("submenú=%+v", sub)
	}
}

func TestReviewDiscardPassaPerConfirmacio(t *testing.T) {
	f := tools.DiffFile{Path: "a.go"}
	h := tools.DiffHunk{Added: 1, Patch: "patch"}
	m := Model{cwd: t.TempDir()}
	sub := reviewHunkMenu(f, h)
	_, _ = sub.run(&m, "discard")
	if m.pending == nil || !strings.Contains(m.pending.desc, "a.go") {
		t.Fatal("descartar ha de demanar confirmació")
	}
}
