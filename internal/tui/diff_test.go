package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffLinesCanviSimple(t *testing.T) {
	ops := diffLines(splitLines("a\nb\nc"), splitLines("a\nB\nc"))
	var kinds string
	for _, o := range ops {
		kinds += string(o.kind)
	}
	if kinds != " -+ " {
		t.Fatalf("seqüència %q", kinds)
	}
}

func TestDiffBlockFitxerNou(t *testing.T) {
	out := diffBlock("nou.txt", "", "hola\nmon", 0)
	if !strings.Contains(out, "≋ diff nou.txt") || !strings.Contains(out, "+ hola") {
		t.Fatalf("diff nou:\n%s", out)
	}
	if strings.Contains(out, "−") {
		t.Fatalf("cap baixa en fitxer nou:\n%s", out)
	}
}

func TestDiffBlockRetalla(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30; i++ {
		b.WriteString("línia\n")
	}
	out := diffBlock("gran.txt", "cap", b.String(), 10)
	if !strings.Contains(out, "retallat") {
		t.Fatalf("hauria de retallar:\n%s", out)
	}
}

func TestWaveLine(t *testing.T) {
	if waveLine(3) != "" || !strings.Contains(waveLine(10), "≋≋") {
		t.Fatalf("waveLine")
	}
}

func TestToolFilePath(t *testing.T) {
	if got := toolFilePath("write", `{"path":"a/b.go"}`); got != "a/b.go" {
		t.Fatalf("write: %q", got)
	}
	if got := toolFilePath("bash", `{"command":"ls"}`); got != "" {
		t.Fatalf("bash no té fitxer: %q", got)
	}
	if got := toolFilePath("edit", `malformat`); got != "" {
		t.Fatalf("json dolent: %q", got)
	}
}

func TestFitInput(t *testing.T) {
	m := planTestModel(t)
	m.input.SetValue("una línia")
	m.fitInput()
	if m.input.Height() != 1 {
		t.Fatalf("1 línia → h=%d", m.input.Height())
	}
	m.input.SetValue("a\nb\nc")
	m.fitInput()
	if m.input.Height() != 3 {
		t.Fatalf("3 línies → h=%d", m.input.Height())
	}
}

func TestExecToolWithDiffReal(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	args := fmt.Sprintf(`{"path":%q,"old_string":"b\n","new_string":"B!\n"}`, p)
	out, diff, _, err := execToolWithDiff("edit", args)
	if err != nil {
		t.Fatalf("edit: %v (%s)", err, out)
	}
	if !strings.Contains(diff, "− b") || !strings.Contains(diff, "+ B!") {
		t.Fatalf("diff mut:\n%s", diff)
	}
	// Sense canvis = sense diff.
	_, diff2, _, err := execToolWithDiff("read", fmt.Sprintf(`{"path":%q}`, p))
	if err != nil || diff2 != "" {
		t.Fatalf("read ha de tornar sense diff: %v %q", err, diff2)
	}
}

func TestSplitLinesSenseFantasma(t *testing.T) {
	if got := splitLines("a\nb\n"); len(got) != 2 {
		t.Fatalf("newline final no és línia: %q", got)
	}
}
