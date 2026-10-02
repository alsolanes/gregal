package tui

import (
	"strings"
	"testing"
)

func TestStatsExplicaContextRutaIActivitat(t *testing.T) {
	m := planTestModel(t)
	m.rolePinned = true
	m.usedTokens, m.tokUp, m.tokDown = 1200, 900, 300
	m.pushToolCall("read", `{"path":"a.go"}`)
	m.pushToolResult("read", "ok", false)
	out := stripANSI(m.statsBlock())
	for _, want := range []string{"context", "conversa", "fixada", "1 eines"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stats no conté %q:\n%s", want, out)
		}
	}
}

func TestAsciiViewTreuGlifsEspecials(t *testing.T) {
	out := asciiView("≋ ✓ ✗ │ █ 🔒 …")
	for _, bad := range []string{"≋", "✓", "✗", "│", "█", "🔒", "…"} {
		if strings.Contains(out, bad) {
			t.Fatalf("encara conté %q: %s", bad, out)
		}
	}
}
