package transcript

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

func call(id, name, args string) llm.ToolCall {
	var c llm.ToolCall
	c.ID, c.Type = id, "function"
	c.Function.Name, c.Function.Arguments = name, args
	return c
}

// conversa té tot el que una sessió real porta: system, mapa del projecte
// enganxat a la tasca, avisos interns, eines i resposta final.
func conversa() []llm.Message {
	mapa := agent.MarcaContext + " abc123] C:/Users/aleix/proj\nfitxers:\nsecret_dir/main.go\ninternal/zz_hidden.go\n"
	return []llm.Message{
		{Role: "system", Content: "SYSTEM PROMPT SECRET"},
		{Role: "user", Content: "Fix the <b>bug</b> in main.go\n\n" + mapa},
		{Role: "assistant", Content: "Let me look.", ToolCalls: []llm.ToolCall{call("c1", "read", `{"path":"main.go"}`), call("c2", "todowrite", `{"todos":[]}`)}},
		{Role: "tool", ToolCallID: "c1", Content: "package main\n// key sk-proj-abcdefghijklmnopqrstuvwxyz0123\n"},
		{Role: "tool", ToolCallID: "c2", Content: "ok"},
		{Role: "user", Content: agent.GuiaEinaText},
		{Role: "assistant", ToolCalls: []llm.ToolCall{call("c3", "bash", `{"command":"go test ./..."}`)}},
		{Role: "tool", ToolCallID: "c3", Content: strings.Repeat("x", 5000)},
		{Role: "assistant", Content: "Fixed it. ```go\nx := 1\n``` done <script>alert(1)</script>"},
		{Role: "user", Content: "thanks\n\n[hook context]\nsome hook said this"},
		{Role: "assistant", Content: "You're welcome."},
	}
}

func TestBuildAgrupaComLaUI(t *testing.T) {
	bl := Build(conversa())
	kinds := []Kind{}
	for _, b := range bl {
		kinds = append(kinds, b.Kind)
	}
	want := []Kind{KindUser, KindWork, KindAnswer, KindUser, KindAnswer}
	if len(kinds) != len(want) {
		t.Fatalf("blocs: %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("blocs: %v", kinds)
		}
	}
	if bl[0].Text != "Fix the <b>bug</b> in main.go" {
		t.Fatalf("usuari: %q", bl[0].Text)
	}
	if n := bl[1].ToolCount(); n != 2 {
		t.Fatalf("el grup ha de tenir read i bash (sense todowrite): %d", n)
	}
	if bl[3].Text != "thanks" {
		t.Fatalf("context de hook: %q", bl[3].Text)
	}
}

func TestExportSenseMapaNiContextIntern(t *testing.T) {
	for name, out := range map[string]string{
		"md":   Markdown(Meta{Title: "t"}, conversa(), Options{}),
		"html": HTML(Meta{Title: "t"}, conversa(), Options{}),
	} {
		for _, bad := range []string{agent.MarcaContext, "secret_dir/main.go", "zz_hidden", "SYSTEM PROMPT SECRET", "NOT executed", "[hook context]", "some hook said", "todowrite"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s conté %q", name, bad)
			}
		}
	}
	// Sessions antigues: el mapa en català també fora.
	old := []llm.Message{{Role: "user", Content: "hola\n\n" + agent.MarcaContextCA + " x]\nfitxer_secret.go"}}
	if out := Markdown(Meta{}, old, Options{}); strings.Contains(out, "fitxer_secret") {
		t.Fatalf("mapa CA: %s", out)
	}
}

func TestMarkdownRender(t *testing.T) {
	out := Markdown(Meta{Title: "My  chat", Workspace: "/w"}, conversa(), Options{Now: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)})
	for _, want := range []string{"# My chat\n", "2026-01-02 03:04", "## User\n\nFix the <b>bug</b>", "<details>\n<summary>Work · 2 tool calls</summary>", "**Tool:** `read`", "\"path\": \"main.go\"", "## Assistant\n\nFixed it.", "[truncated, 3000 more characters]", "You're welcome."} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q a:\n%s", want, out)
		}
	}
	// Sense redact, el contingut va tal qual.
	if !strings.Contains(out, "sk-proj-abcdefghijklmnopqrstuvwxyz0123") {
		t.Error("sense redact no s'ha de tocar res")
	}
	// Una tanca de codi dins d'un resultat no pot trencar el bloc.
	conv := []llm.Message{
		{Role: "assistant", ToolCalls: []llm.ToolCall{call("a", "read", "")}},
		{Role: "tool", ToolCallID: "a", Content: "```\ninside\n```"},
		{Role: "assistant", Content: "end"},
	}
	if md := Markdown(Meta{}, conv, Options{}); !strings.Contains(md, "````text\n```\ninside\n```\n````") {
		t.Errorf("tanca: %s", md)
	}
}

func TestHTMLAutocontingutIEscapat(t *testing.T) {
	out := HTML(Meta{Title: "<x> & y"}, conversa(), Options{Redact: true})
	if strings.Contains(out, "<script") {
		t.Fatal("hi ha un <script> sense escapar")
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") || !strings.Contains(out, "<title>&lt;x&gt; &amp; y</title>") {
		t.Fatal("contingut sense escapar")
	}
	if !strings.Contains(out, "Content-Security-Policy") || !strings.Contains(out, "<style>") || !strings.Contains(out, "<details class=\"work\">") {
		t.Fatal("falten CSP, estil inline o grup plegat")
	}
	// Cap recurs extern: ni src=, ni href= cap a fora, ni @import.
	if regexp.MustCompile(`(?i)(src=|<link|@import|url\()`).MatchString(out) {
		t.Fatal("referència a un recurs extern")
	}
	if strings.Contains(out, "sk-proj-") || strings.Contains(out, "C:/Users/aleix") {
		t.Fatal("redact no aplicat")
	}
	// Amb renderitzador injectat, el text passa per ell.
	md := HTML(Meta{}, conversa(), Options{RenderMarkdown: func(s string) string { return "<p>R</p>" }})
	if !strings.Contains(md, "<div class=\"md\"><p>R</p></div>") {
		t.Fatal("RenderMarkdown ignorat")
	}
}

func TestFilename(t *testing.T) {
	if got := Filename("Fix the bug: «main.go»!", "md"); got != "gregal-fix-the-bug-main-go.md" {
		t.Fatalf("%q", got)
	}
	if got := Filename("", ".html"); got != "gregal-conversation.html" {
		t.Fatalf("%q", got)
	}
}

func TestCleanUser(t *testing.T) {
	if CleanUser(agent.MarcaContext+" x]\nfiles") != "" {
		t.Fatal("un missatge que és només mapa ha de quedar buit")
	}
	if CleanUser("AUTONOMOUS CHECKPOINT:\n- go test: PASS") != "" {
		t.Fatal("checkpoint intern")
	}
	if CleanUser("  hola  ") != "hola" {
		t.Fatal("trim")
	}
}
