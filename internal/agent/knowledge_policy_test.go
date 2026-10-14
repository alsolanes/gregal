package agent

import (
	"strings"
	"testing"

	"gregal/internal/knowledge"
	"gregal/internal/tools"
)

// Les eines de coneixement: cercar és lectura (permès a tots els modes);
// desar una nota escriu i demana permís, llevat del mode autònom quan la
// nota va a una col·lecció de projecte o de sessió. Els modes de només
// lectura la deneguen.
func TestKnowledgeToolsPolicy(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	project := t.TempDir()
	w := knowledge.Where{Project: project, Session: procSession}
	if _, err := knowledge.Create(w, "Shared", "", knowledge.ScopeGlobal); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.Create(w, "Repo docs", "", knowledge.ScopeProject); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"knowledge_search", "knowledge_note"} {
		if !tools.EsNativa(n) || !tools.Configurable(n) {
			t.Fatalf("%s ha de ser al registre i configurable", n)
		}
	}
	p := &Policy{ProjectDir: project}
	cases := []struct {
		mode, tool, args, want string
	}{
		{ModeCode, "knowledge_search", `{"query":"x"}`, "allow"},
		{ModeChat, "knowledge_search", `{"query":"x"}`, "allow"},
		{ModeInspect, "knowledge_search", `{"query":"x"}`, "allow"},
		{ModeGoal, "knowledge_search", `{"query":"x"}`, "allow"},
		{ModeCode, "knowledge_note", `{"text":"x"}`, "ask"},
		{ModeChat, "knowledge_note", `{"text":"x"}`, "deny"},
		{ModeInspect, "knowledge_note", `{"text":"x"}`, "deny"},
		{ModeGoal, "knowledge_note", `{"text":"x"}`, "deny"},
		{ModeAutonomous, "knowledge_note", `{"text":"x"}`, "allow"},
		{ModeAutonomous, "knowledge_note", `{"text":"x","collection":"Repo docs"}`, "allow"},
		{ModeAutonomous, "knowledge_note", `{"text":"x","collection":"Shared"}`, "ask"},
		{ModeAutonomous, "knowledge_note", `{"text":"x","collection":"missing"}`, "ask"},
	}
	for _, c := range cases {
		if got, reason := p.Decide(c.mode, c.tool, c.args); got != c.want {
			t.Errorf("%s %s %s = %s (%s), want %s", c.mode, c.tool, c.args, got, reason, c.want)
		}
	}
	// Un permís explícit del config mana.
	strict := &Policy{ProjectDir: project, Tools: map[string]string{"knowledge_note": "deny"}}
	if got, _ := strict.Decide(ModeAutonomous, "knowledge_note", `{"text":"x"}`); got != "deny" {
		t.Errorf("config deny ignorat: %s", got)
	}
	if !ParallelSafe("knowledge_search") || ParallelSafe("knowledge_note") {
		t.Errorf("knowledge_search és de lectura; knowledge_note no")
	}
}

func TestKnowledgeToolsExecAndPrompt(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	project := t.TempDir()
	out, _, err := ExecIn("tab-1", project, "knowledge_note", `{"text":"The deploy window is Thursday 14:00","title":"Deploys"}`)
	if err != nil || !strings.Contains(out, knowledge.SessionNotesName) {
		t.Fatalf("nota: %q %v", out, err)
	}
	out, _, err = ExecIn("tab-1", project, "knowledge_search", `{"query":"deploy window"}`)
	if err != nil || !strings.Contains(out, "Thursday 14:00") {
		t.Fatalf("cerca: %q %v", out, err)
	}
	// Una altra sessió no veu les notes de la primera.
	out, _, _ = ExecIn("tab-2", project, "knowledge_search", `{"query":"deploy window"}`)
	if strings.Contains(out, "Thursday") {
		t.Fatalf("notes de sessió filtrades a una altra sessió: %q", out)
	}
	if ctx := ContextProjecteSessio(project, "tab-1"); !strings.Contains(ctx, "KNOWLEDGE COLLECTIONS") || !strings.Contains(ctx, knowledge.SessionNotesName) {
		t.Fatalf("prompt sense col·leccions: %q", ctx)
	}
	if ctx := ContextProjecte(project); strings.Contains(ctx, knowledge.SessionNotesName) {
		t.Fatalf("sense sessió no hi han de sortir les de sessió: %q", ctx)
	}
}
