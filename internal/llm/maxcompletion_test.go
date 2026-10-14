package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Azure OpenAI amb un model de raonament: rebutja max_tokens (i una
// temperatura que no sigui la de per defecte). La crida ha d'anar bé, amb
// max_completion_tokens, i la següent ja no ha de tornar a fallar.
func TestReasoningModelGetsMaxCompletionTokensAndDefaultTemperature(t *testing.T) {
	var peticions atomic.Int32
	var ultim map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peticions.Add(1)
		b, _ := io.ReadAll(r.Body)
		var cos map[string]any
		_ = json.Unmarshal(b, &cos)
		if _, hi := cos["max_tokens"]; hi {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.","type":"invalid_request_error","param":"max_tokens","code":"unsupported_parameter"}}`))
			return
		}
		if _, hi := cos["temperature"]; hi {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"Unsupported value: 'temperature' does not support 0.2 with this model. Only the default (1) value is supported.","type":"invalid_request_error","param":"temperature","code":"unsupported_value"}}`))
			return
		}
		ultim = cos
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := New()
	msgs := []Message{{Role: "user", Content: "hola"}}
	out, err := c.Chat(context.Background(), srv.URL, "", "gpt-5-mini", msgs, 0.2, 512)
	if err != nil || out != "ok" {
		t.Fatalf("the call must succeed after adjusting: %q %v", out, err)
	}
	if ultim["max_completion_tokens"] != float64(512) {
		t.Fatalf("max_completion_tokens must carry the budget: %v", ultim)
	}
	peticions.Store(0)
	if _, err := c.Chat(context.Background(), srv.URL, "", "gpt-5-mini", msgs, 0.2, 512); err != nil {
		t.Fatal(err)
	}
	if n := peticions.Load(); n != 1 {
		t.Fatalf("the adjustments are remembered per endpoint and model: %d requests on the second call", n)
	}

	// Un altre model al mateix endpoint continua amb max_tokens.
	var altre map[string]any
	normal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &altre)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer normal.Close()
	if _, err := c.Chat(context.Background(), normal.URL, "", "gpt-4o", msgs, 0.2, 64); err != nil {
		t.Fatal(err)
	}
	if altre["max_tokens"] != float64(64) || altre["max_completion_tokens"] != nil {
		t.Fatalf("models that accept max_tokens keep it: %v", altre)
	}
}
