package agent

import (
	"strings"
	"testing"

	"gregal/internal/llm"
)

func TestDropSeenToolImagesKeepsOnlyUnseen(t *testing.T) {
	img := "data:image/png;base64,AAAA"
	h := []llm.Message{
		{Role: "user", Content: "mira", Images: []string{img}},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "a"}}},
		{Role: "tool", ToolCallID: "a", Content: "pàgines", Images: []string{img, img}},
		{Role: "assistant", Content: "hi veig un error a la pàgina 5", ToolCalls: []llm.ToolCall{{ID: "b"}}},
		{Role: "tool", ToolCallID: "b", Content: "nova captura", Images: []string{img}},
	}
	out := DropSeenToolImages(h)
	if len(out[2].Images) != 0 || !strings.Contains(out[2].Content, "2 imatge(s)") {
		t.Fatalf("seen tool images kept: %+v", out[2])
	}
	if len(out[4].Images) != 1 {
		t.Fatal("the newest tool image, not yet seen, must stay")
	}
	if len(out[0].Images) != 1 {
		t.Fatal("images the user attached must stay")
	}
	if len(h[2].Images) != 2 || strings.Contains(h[2].Content, "imatge") {
		t.Fatal("the original history must not change")
	}
	plain := []llm.Message{{Role: "user", Content: "hola"}}
	if &DropSeenToolImages(plain)[0] != &plain[0] {
		t.Fatal("no copy when nothing changes")
	}
}
