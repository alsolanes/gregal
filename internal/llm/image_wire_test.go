package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageImageWire(t *testing.T) {
	// Sense imatges: wire idèntic a l'antic (string).
	plain, _ := json.Marshal(Message{Role: "user", Content: "hola"})
	if !strings.Contains(string(plain), `"content":"hola"`) {
		t.Fatalf("wire clàssic trencat: %s", plain)
	}
	// Amb imatges: array de parts text + image_url.
	m, _ := json.Marshal(Message{Role: "user", Content: "què hi ha?",
		Images: []string{"data:image/png;base64,AAA"}})
	var v map[string]any
	if err := json.Unmarshal(m, &v); err != nil {
		t.Fatal(err)
	}
	parts, ok := v["content"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("content ha de ser array de 2 parts: %s", m)
	}
	img := parts[1].(map[string]any)
	if img["type"] != "image_url" {
		t.Fatalf("segona part image_url: %s", m)
	}
	// Estimació: imatge suma.
	if EstimateTokens([]Message{{Role: "user", Content: "hola", Images: []string{"x"}}}) < 1000 {
		t.Fatal("la imatge ha de sumar tokens")
	}
}
