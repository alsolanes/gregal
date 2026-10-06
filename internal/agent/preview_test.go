package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPreviewRequest(t *testing.T) {
	if decision, _ := DefaultPolicy().For("preview", `{}`); decision != "allow" {
		t.Fatalf("preview policy: %s", decision)
	}
	policy := &Policy{Tools: map[string]string{"preview": "deny"}}
	if decision, _ := policy.For("preview", `{}`); decision != "deny" {
		t.Fatal("preview bypasses explicit deny")
	}
	for _, kind := range []string{"html", "markdown", "url"} {
		content := "# Plan"
		if kind == "url" {
			content = "http://127.0.0.1:3000/"
		}
		args, _ := json.Marshal(map[string]string{"kind": kind, "content": content})
		out, _, err := Exec("preview", string(args))
		if err != nil || !json.Valid([]byte(out)) {
			t.Fatalf("%s: %s %v", kind, out, err)
		}
	}
	for _, content := range []string{"https://example.com/", "file:///etc/passwd", "http://user:pass@localhost:3000/", "http://localhost.example.com/", ""} {
		args, _ := json.Marshal(map[string]string{"kind": "url", "content": content})
		if _, _, err := Exec("preview", string(args)); err == nil {
			t.Fatalf("accepted %q", content)
		}
	}
	args, _ := json.Marshal(map[string]string{"kind": "html", "content": strings.Repeat("x", 200001)})
	if _, _, err := Exec("preview", string(args)); err == nil {
		t.Fatal("accepted oversized content")
	}
}
