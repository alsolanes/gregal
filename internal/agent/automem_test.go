package agent

import (
	"strings"
	"testing"

	"gregal/internal/llm"
)

func TestFilterMemorySuggestions(t *testing.T) {
	existing := []string{"- 2026-01-01: Run go test ./... before committing"}
	got := FilterMemorySuggestions([]string{
		"run go test ./... before committing.", // duplicat de l'existent
		"Use gofmt on changed files only",
		"use gofmt on changed files only", // duplicat entre propostes
		strings.Repeat("x", 300),          // massa llarga
		"Web i18n keys must match in CA and EN",
		"third one is dropped",
	}, existing)
	if len(got) != 2 || got[0] != "Use gofmt on changed files only" || got[1] != "Web i18n keys must match in CA and EN" {
		t.Fatalf("got=%q", got)
	}
}

func TestShouldSuggestMemory(t *testing.T) {
	edited := []llm.Message{{Role: "tool", Name: "edit", Content: "ok: 1 replacement"}}
	failedEdit := []llm.Message{{Role: "tool", Name: "edit", Content: "ERROR: not found"}}
	if !ShouldSuggestMemory(ModeCode, 3, false, edited) {
		t.Fatal("code turn with an applied edit qualifies")
	}
	for name, ok := range map[string]bool{
		"chat":      ShouldSuggestMemory(ModeChat, 5, false, edited),
		"few":       ShouldSuggestMemory(ModeCode, 2, false, edited),
		"cut short": ShouldSuggestMemory(ModeCode, 5, true, edited),
		"no edit":   ShouldSuggestMemory(ModeCode, 5, false, failedEdit),
	} {
		if ok {
			t.Errorf("%s must not suggest", name)
		}
	}
}

func TestNotesDiffAndNormalizeTidy(t *testing.T) {
	before := []string{"- 2026-01-01: a", "- 2026-01-02: b", "- 2026-01-03: b again"}
	after := NormalizeTidy([]string{"- 2026-01-01: a", "b merged", "b merged", "x", "y"}, len(before))
	if len(after) != 3 || !strings.HasSuffix(after[1], ": b merged") || !strings.HasPrefix(after[1], "- 20") {
		t.Fatalf("after=%q", after)
	}
	diff := NotesDiff(before, after[:2])
	for _, want := range []string{"  - 2026-01-01: a", "- - 2026-01-02: b", "- - 2026-01-03: b again", "+ " + after[1]} {
		if !strings.Contains(diff, want) {
			t.Fatalf("diff missing %q:\n%s", want, diff)
		}
	}
}

func TestTranscriptDigestAndRecapTurns(t *testing.T) {
	var call llm.ToolCall
	call.Function.Name, call.Function.Arguments = "read", `{"path":"a.go"}`
	hist := []llm.Message{
		{Role: "system", Content: "SECRET SYSTEM"},
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "ok", ToolCalls: []llm.ToolCall{call}},
		{Role: "tool", Name: "read", Content: "package a"},
		{Role: "user", Content: "second"},
		{Role: "assistant", Content: "done"},
	}
	d := TranscriptDigest(hist, 0)
	if strings.Contains(d, "SECRET") || !strings.Contains(d, "TOOL CALL read") || !strings.Contains(d, "TOOL RESULT read: package a") {
		t.Fatalf("digest=%s", d)
	}
	if got := RecapTurns(hist, 1); len(got) != 2 || got[0].Content != "second" {
		t.Fatalf("last turn=%v", got)
	}
	if tail := TranscriptDigest(hist, 10); !strings.HasPrefix(tail, "…(earlier part omitted)") {
		t.Fatalf("tail=%q", tail)
	}
}
