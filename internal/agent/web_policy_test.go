package agent

import "testing"

// Les eines web són només lectura: permís automàtic a tots els modes,
// inclòs consulta (sense popups) i xat/objectiu (read-only).
func TestWebToolsSensePopups(t *testing.T) {
	p := DefaultPolicy()
	for _, tool := range []string{"web_search", "web_fetch"} {
		if d, _ := p.For(tool, `{}`); d != "allow" {
			t.Fatalf("%s For=%q, cal allow", tool, d)
		}
		for _, mode := range []string{ModeCode, ModeChat, ModeInspect, ModeGoal} {
			if d, _ := p.Decide(mode, tool, `{}`); d != "allow" {
				t.Fatalf("%s Decide(%s)=%q, cal allow", tool, mode, d)
			}
		}
	}
	if d, _ := p.Decide(ModeInspect, "web_fetch", `{"url":"http://127.0.0.1:4000/x"}`); d != "allow" {
		t.Fatalf("la política no filtra hosts: Decide=%q (el guard és a Exec)", d)
	}
}
