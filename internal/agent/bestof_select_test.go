package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBestOfVotesGroupsEquivalentPatches(t *testing.T) {
	a := "diff --git a/m.go b/m.go\n--- a/m.go\n+++ b/m.go\n@@ -1 +1 @@\n-x := 1\n+x := 2 // fix\n"
	b := "diff --git a/m.go b/m.go\n--- a/m.go\n+++ b/m.go\n@@ -1 +1 @@\n-x := 1\n+x  :=  2 // fix\ndiff --git a/m_test.go b/m_test.go\n+++ b/m_test.go\n+func TestX() {}\n"
	c := "diff --git a/m.go b/m.go\n--- a/m.go\n+++ b/m.go\n@@ -1 +1 @@\n-x := 1\n+x := 3\n"
	v := bestOfVotes(map[int]string{0: a, 1: b, 2: c})
	if v[0] != 2 || v[1] != 2 || v[2] != 1 {
		t.Fatalf("vots: %v", v)
	}
}

// Tres intents: dos arriben al valor bo (42) i un al dolent (41). Cada un
// escriu el seu «test». Les proves creuades i el vot han de triar un dels
// bons encara que tots tres passin els seus propis tests.
func TestBestOfCrossTestsPickAgreeingFix(t *testing.T) {
	root := t.TempDir()
	mk := func(i int, val string) BestOfAttempt {
		dir := filepath.Join(root, string(rune('a'+i)))
		os.MkdirAll(filepath.Join(dir, "tests"), 0o755)
		os.WriteFile(filepath.Join(dir, "code.txt"), []byte(val), 0o644)
		name := "tests/t" + string(rune('a'+i)) + ".txt"
		os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(val), 0o644)
		ok := true
		return BestOfAttempt{Index: i + 1, Path: dir, State: BestOfDone, DiffBytes: 10, Files: []string{"code.txt", name}, ChecksPassed: &ok, Added: 2}
	}
	atts := []BestOfAttempt{mk(0, "42"), mk(1, "41"), mk(2, "42")}
	verify := []string{`for f in tests/*.txt; do [ "$(cat "$f")" = "$(cat code.txt)" ] || exit 1; done`}
	if runtime.GOOS == "windows" { // les verificacions corren amb cmd.exe
		verify = []string{`for %f in (tests\*.txt) do @(fc /b %f code.txt >nul || exit 1)`}
	}
	pass, total := bestOfCrossTests(context.Background(), atts, verify)
	for i := range atts {
		atts[i].CrossPassed, atts[i].CrossTotal = pass[atts[i].Index], total[atts[i].Index]
	}
	if atts[0].CrossPassed != 1 || atts[1].CrossPassed != 0 || atts[2].CrossPassed != 1 || atts[1].CrossTotal != 2 {
		t.Fatalf("proves creuades: %+v", atts)
	}
	// Els fitxers dels altres no s'han de quedar al worktree.
	if _, err := os.Stat(filepath.Join(atts[1].Path, "tests", "ta.txt")); err == nil {
		t.Fatal("no s'ha restaurat el worktree")
	}
	if r := BestOfRecommend(atts); r != 1 && r != 3 {
		t.Fatalf("recomanat #%d, hauria de ser un dels que fan 42", r)
	}
}
