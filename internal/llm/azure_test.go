package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestEndpointURLKeepsAzureQuery(t *testing.T) {
	cases := map[string]string{
		"https://r.openai.azure.com/openai/v1/":                                            "https://r.openai.azure.com/openai/v1/chat/completions",
		"https://r.openai.azure.com/openai/deployments/gpt5?api-version=2025-04-01-preview": "https://r.openai.azure.com/openai/deployments/gpt5/chat/completions?api-version=2025-04-01-preview",
		"https://api.openai.com/v1":                                                         "https://api.openai.com/v1/chat/completions",
	}
	for in, want := range cases {
		if got := endpointURL(in, "/chat/completions"); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestSetAuthForAzureAndFoundry(t *testing.T) {
	try := func(base, key string) http.Header {
		r, _ := http.NewRequest("POST", endpointURL(base, "/chat/completions"), nil)
		setAuth(r, base, key)
		return r.Header
	}
	if h := try("https://r.openai.azure.com/openai/v1", "abc123"); h.Get("api-key") != "abc123" || h.Get("Authorization") != "" {
		t.Fatalf("Azure API key must go in api-key: %v", h)
	}
	if h := try("https://r.openai.azure.com/openai/v1", "eyJhbGciOi.x.y"); h.Get("Authorization") != "Bearer eyJhbGciOi.x.y" || h.Get("api-key") != "" {
		t.Fatalf("an Entra ID token goes as Bearer: %v", h)
	}
	if h := try("https://api.openai.com/v1", "sk"); h.Get("Authorization") != "Bearer sk" {
		t.Fatalf("other providers keep Bearer: %v", h)
	}
	if h := try("https://r.services.ai.azure.com/models", "k"); h.Get("extra-parameters") != "pass-through" || h.Get("api-key") != "k" {
		t.Fatalf("Foundry Model Inference passes extra parameters through: %v", h)
	}
	if h := try("https://r.services.ai.azure.com/openai/v1", "k"); h.Get("extra-parameters") != "" {
		t.Fatalf("Azure OpenAI v1 on services.ai.azure.com is not the inference API: %v", h)
	}
}

// fakeEffort records the request bodies and answers with the scripted
// responses (status, body) in order; after that, plain success.
func fakeEffort(t *testing.T, script ...[2]string) (*httptest.Server, func() []map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var seen []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var cos map[string]any
		_ = json.Unmarshal(b, &cos)
		mu.Lock()
		seen = append(seen, cos)
		n := len(seen)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if n <= len(script) && script[n-1][0] == "400" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(script[n-1][1]))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []map[string]any { mu.Lock(); defer mu.Unlock(); return append([]map[string]any(nil), seen...) }
}

// Un model que no admet el valor demanat: es fa servir el més proper dels
// que diu que admet, i es recorda.
func TestReasoningEffortUnsupportedValuePicksNearest(t *testing.T) {
	srv, seen := fakeEffort(t, [2]string{"400", `{"error":{"message":"Unsupported value: 'reasoning_effort' does not support 'max' with this model. Supported values are: 'low', 'medium', and 'high'.","param":"reasoning_effort","code":"unsupported_value"}}`})
	c := New()
	ctx := WithThink(context.Background(), "max")
	msgs := []Message{{Role: "user", Content: "hola"}}
	if _, err := c.Chat(ctx, srv.URL, "", "o3", msgs, 0, 64); err != nil {
		t.Fatal(err)
	}
	s := seen()
	if len(s) != 2 || s[1]["reasoning_effort"] != "high" {
		t.Fatalf("expected a retry with the nearest supported value 'high': %v", s)
	}
	if _, err := c.Chat(ctx, srv.URL, "", "o3", msgs, 0, 64); err != nil {
		t.Fatal(err)
	}
	if s := seen(); len(s) != 3 || s[2]["reasoning_effort"] != "high" {
		t.Fatalf("the supported value is remembered (one request): %v", s)
	}
}

// gpt-5.6 a Chat Completions: amb eines només admet reasoning_effort none,
// encara que no s'enviï (per defecte és medium).
func TestReasoningEffortToolsConflictSendsNone(t *testing.T) {
	srv, seen := fakeEffort(t, [2]string{"400", `{"error":{"message":"Function tools with reasoning_effort are not supported for gpt-5.6-sol in /v1/chat/completions. To use function tools, use /v1/responses or set reasoning_effort to 'none'.","type":"invalid_request_error"}}`})
	c := New()
	msgs := []Message{{Role: "user", Content: "llegeix a.go"}}
	specs := []ToolSpec{{Name: "read", Description: "read a file", Parameters: map[string]any{"type": "object"}}}
	if _, _, err := c.ChatWithTools(context.Background(), srv.URL, "", "gpt-5.6-sol", msgs, 0, 64, specs); err != nil {
		t.Fatal(err)
	}
	s := seen()
	if len(s) != 2 || s[1]["reasoning_effort"] != "none" {
		t.Fatalf("expected a retry with reasoning_effort none: %v", s)
	}
	if _, _, err := c.ChatWithTools(context.Background(), srv.URL, "", "gpt-5.6-sol", msgs, 0, 64, specs); err != nil {
		t.Fatal(err)
	}
	if s := seen(); len(s) != 3 || s[2]["reasoning_effort"] != "none" {
		t.Fatalf("the tools+none rule is remembered: %v", s)
	}
	// Sense eines, el mateix model no rep l'ordre de no pensar.
	if _, err := c.Chat(context.Background(), srv.URL, "", "gpt-5.6-sol", msgs, 0, 64); err != nil {
		t.Fatal(err)
	}
	if s := seen(); s[len(s)-1]["reasoning_effort"] != nil {
		t.Fatalf("without tools the model keeps its own effort: %v", s[len(s)-1])
	}
}

// Foundry serveix molts models sota la mateixa URL: que un rebutgi el camp
// no l'ha de treure als altres.
func TestReasoningEffortRejectionIsPerModel(t *testing.T) {
	var mu sync.Mutex
	got := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var cos map[string]any
		_ = json.Unmarshal(b, &cos)
		w.Header().Set("Content-Type", "application/json")
		if cos["model"] == "Llama-4" && cos["reasoning_effort"] != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"Extra parameters ['reasoning_effort'] are not allowed when extra-parameters is not set or set to be 'error'."}}`))
			return
		}
		mu.Lock()
		got[cos["model"].(string)] = cos["reasoning_effort"]
		mu.Unlock()
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	c := New()
	ctx := WithThink(context.Background(), "high")
	msgs := []Message{{Role: "user", Content: "hola"}}
	for _, m := range []string{"Llama-4", "o4-mini"} {
		if _, err := c.Chat(ctx, srv.URL, "", m, msgs, 0, 64); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}
	if got["Llama-4"] != nil || got["o4-mini"] != "high" {
		t.Fatalf("the rejection must stay with the model that rejected it: %v", got)
	}
}
