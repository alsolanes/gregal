package agent

import (
	"fmt"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func cridaWeb(i int) llm.ToolCall {
	c := llm.ToolCall{ID: fmt.Sprintf("w%d", i), Type: "function"}
	c.Function.Name = "web_search"
	// Paraules diferents: el detector de repeticions no ha de saltar.
	temes := strings.Fields("vent pluja neu sol boira llamp gebre calamarsa tramuntana garbi xaloc migjorn ponent llevant mestral marinada gregal")
	c.Function.Arguments = fmt.Sprintf(`{"query":"%s %s"}`, temes[i%len(temes)], strings.Repeat("x", i))
	return c
}

// Research loops (search, fetch, search again for one more detail) are
// what kept a fill-the-PDF task with a local model going for over an hour.
// The engine nudges at ResearchNudgeAt web calls and stops running web
// calls at ResearchCapAt; other tools are never affected.
func TestResearchBudgetNudgesThenCaps(t *testing.T) {
	turn := NouTorn(OpcionsTorn{Mode: ModeAutonomous})
	n := 0
	step := func(k int, extra ...llm.ToolCall) []llm.ToolCall {
		var calls []llm.ToolCall
		for i := 0; i < k; i++ {
			n++
			calls = append(calls, cridaWeb(n))
		}
		calls = append(calls, extra...)
		turn.Hist = append(turn.Hist, llm.Message{Role: "assistant", ToolCalls: calls})
		turn.repCrides(calls)
		ran := turn.perExecutar
		turn.perExecutar = nil
		var res []Execucio
		for _, c := range ran {
			res = append(res, Execucio{Call: c, Sortida: "ok"})
		}
		turn.RepExecucions(res)
		return ran
	}
	guies := func() int {
		k := 0
		for _, m := range turn.Hist {
			if m.Role == "user" && strings.Contains(m.Content, "RESEARCH BUDGET") {
				k++
			}
		}
		return k
	}

	if ran := step(4); len(ran) != 4 || guies() != 0 {
		t.Fatalf("under budget: ran %d, guides %d", len(ran), guies())
	}
	if ran := step(4); len(ran) != 4 || guies() != 1 {
		t.Fatalf("at the nudge: ran %d, guides %d", len(ran), guies())
	}
	step(4)
	if guies() != 1 {
		t.Fatal("the nudge must be given once")
	}
	write := llm.ToolCall{ID: "wr", Type: "function"}
	write.Function.Name = "read"
	write.Function.Arguments = `{"path":"x.txt"}`
	ran := step(4, write)
	if len(ran) != 3 { // 2 web calls fit (12 -> 14), plus the read
		t.Fatalf("past the cap: ran %d calls, want 3", len(ran))
	}
	cut := 0
	for _, m := range turn.Hist {
		if m.Role == "tool" && strings.Contains(m.Content, "research budget of this turn is spent") {
			cut++
		}
	}
	if cut != 2 {
		t.Fatalf("cut web calls=%d, want 2", cut)
	}
}
