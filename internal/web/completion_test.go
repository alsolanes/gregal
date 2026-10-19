package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInlineCompletionUsesIsolatedCodeRoleAndDoesNotTouchTranscript(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	var request map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"return value + 1;\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer upstream.Close()
	s := hubTestServer(t)
	provider := s.cfg.Providers["p"]
	provider.BaseURL = upstream.URL
	s.cfg.Providers["p"] = provider
	codeRole := s.cfg.Roles["code"]
	codeRole.Model = "completion-model"
	s.cfg.Roles["code"] = codeRole
	workspace := t.TempDir()
	before := len(s.convo)
	body, err := json.Marshal(inlineCompletionRequest{
		Language: "go", Prefix: "func add(value int) int {\n return value + ", Suffix: "\n}", Workspace: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := v2Do(t, s, http.MethodPost, "/api/vscode/completion", string(body))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "return value + 1;") {
		t.Fatalf("completion=%d %s", rec.Code, rec.Body.String())
	}
	if len(s.convo) != before {
		t.Fatalf("completion changed chat transcript: before=%d after=%d", before, len(s.convo))
	}
	if request["model"] != "completion-model" || request["max_tokens"] != float64(inlineCompletionTokens) {
		t.Fatalf("completion did not use the short code-model request: %+v", request)
	}
	messages, ok := request["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("completion did not use isolated prompt: %+v", request)
	}
	userMessage := messages[1].(map[string]any)
	for _, fragment := range []string{"Language: go", "Code before cursor", "Code after cursor", "<|cursor|>"} {
		if !strings.Contains(userMessage["content"].(string), fragment) {
			t.Errorf("prompt missing %q: %s", fragment, userMessage["content"])
		}
	}
}
