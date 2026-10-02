package tui

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestRenderMD(t *testing.T) {
	out := stripANSI(renderMD("# Títol\n\nHola **món**.\n\n```go\nfmt.Println(1)\n```\n", 80))
	if !strings.Contains(out, "Títol") || !strings.Contains(out, "món") {
		t.Fatalf("falta text: %q", out)
	}
	if !strings.Contains(out, "fmt.Println(1)") {
		t.Fatalf("falta codi: %q", out)
	}
	if strings.Contains(out, "```") {
		t.Fatalf("backticks sense renderitzar: %q", out)
	}
	if got := renderMD("  \n ", 80); got != "  \n " {
		t.Fatalf("buit ha de tornar igual: %q", got)
	}
}
