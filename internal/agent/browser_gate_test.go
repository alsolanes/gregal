package agent

import (
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

func browserCall(id, args string) llm.ToolCall {
	c := llm.ToolCall{ID: id}
	c.Function.Name, c.Function.Arguments = "browser", args
	return c
}

func TestTornBrowserGate(t *testing.T) {
	defer tools.ForgetBrowserSites("gate-test")
	tr := &Torn{o: OpcionsTorn{Sessio: "gate-test", Mode: ModeCode, Pol: DefaultPolicy()}}

	// Acceptar un diàleg és irreversible: pregunta forçada (ni aprovació
	// automàtica ni revisor).
	pl := CallPlan{}
	tr.browserGate(browserCall("c1", `{"action":"dialog","accept":true}`), &pl)
	if !pl.Ask || !pl.ForceAsk || !tr.forcats["c1"] {
		t.Fatalf("dialog accept: %+v %v", pl, tr.forcats)
	}

	// Actuar en un lloc nou del perfil propi: pregunta normal.
	pl = CallPlan{}
	tr.browserGate(browserCall("c2", `{"action":"batch","steps":[{"action":"open","url":"https://botiga.cat/"},{"action":"click","target":"e2"}]}`), &pl)
	if !pl.Ask || pl.ForceAsk {
		t.Fatalf("lloc nou: %+v", pl)
	}

	// Llegir: la política ja ho deixa i el gate no hi posa res.
	pl = CallPlan{}
	tr.browserGate(browserCall("c3", `{"action":"open","url":"https://botiga.cat/"}`), &pl)
	if pl.Ask {
		t.Fatalf("llegir: %+v", pl)
	}

	// El Chrome de l'usuari: llegir passa la política dels modes de lectura
	// (el permís de lloc el posa el gate).
	if d, _ := DefaultPolicy().Decide(ModePlan, "browser", `{"action":"read","chrome":true}`); d != "allow" {
		t.Fatalf("pla + chrome read: %s", d)
	}
	if d, _ := DefaultPolicy().Decide(ModePlan, "browser", `{"action":"batch","steps":[{"action":"click","target":"e1"}]}`); d != "deny" {
		t.Fatalf("pla + lot que actua: %s", d)
	}
}
