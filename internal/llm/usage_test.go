package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El consum real del proveïdor (usage) s'ha de poder llegir per calibrar
// l'heurístic EstimateTokens. Abans no es parsejava enlloc.
func TestLastUsageNoStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"hola"}}],"usage":{"prompt_tokens":1234,"completion_tokens":56,"total_tokens":1290,"prompt_tokens_details":{"cached_tokens":1024}}}`)
	}))
	defer srv.Close()

	c := New()
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100); err != nil {
		t.Fatal(err)
	}
	u, ok := c.LastUsage()
	if !ok || u.PromptTokens != 1234 || u.CompletionTokens != 56 || u.TotalTokens != 1290 || u.PromptTokensDetails.CachedTokens != 1024 {
		t.Fatalf("usage=%+v ok=%v", u, ok)
	}
}

// Si el proveïdor no declara usage, l'últim consum no s'ha d'arrossegar de
// la crida anterior (si no, el mesurador mostraria un valor fals).
func TestLastUsageEsNetejaEntreCrides(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"a"}}],"usage":{"prompt_tokens":900,"completion_tokens":1,"total_tokens":901}}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"b"}}]}`)
	}))
	defer srv.Close()

	c := New()
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "1"}}, 0.5, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.LastUsage(); !ok {
		t.Fatal("la primera crida declara usage")
	}
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "2"}}, 0.5, 100); err != nil {
		t.Fatal(err)
	}
	if u, ok := c.LastUsage(); ok {
		t.Fatalf("sense usage no ha de quedar el consum anterior: %+v", u)
	}
}

// Amb eines (no-stream) també.
func TestLastUsageAmbEines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":77,"completion_tokens":5}}`)
	}))
	defer srv.Close()

	c := New()
	if _, _, err := c.ChatWithTools(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100, nil); err != nil {
		t.Fatal(err)
	}
	u, ok := c.LastUsage()
	if !ok || u.PromptTokens != 77 {
		t.Fatalf("usage=%+v ok=%v", u, ok)
	}
}

// Streaming: el consum arriba a l'últim event (choices buides).
func TestLastUsageStream(t *testing.T) {
	srv := sseServer(t, []string{
		`{"choices":[{"delta":{"content":"ho"}}]}`,
		`{"choices":[{"delta":{"content":"la"}}]}`,
		`{"choices":[],"usage":{"prompt_tokens":4242,"completion_tokens":9,"total_tokens":4251}}`,
	})
	defer srv.Close()

	c := New()
	if _, err := c.ChatStream(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100, nil, nil); err != nil {
		t.Fatal(err)
	}
	u, ok := c.LastUsage()
	if !ok || u.PromptTokens != 4242 || u.CompletionTokens != 9 {
		t.Fatalf("usage=%+v ok=%v", u, ok)
	}
}

// El comptador del context suma totes les crides, no només l'última: és el
// que necessita el mode headless per donar el consum real d'un run.
func TestUsageMeterSumaCrides(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		fmt.Fprintf(w, `{"choices":[{"message":{"content":"r%d"}}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"prompt_tokens_details":{"cached_tokens":%d}}}`, n, 1000*n, 10*n, 100*n)
	}))
	defer srv.Close()

	m := &UsageMeter{}
	ctx := WithUsage(context.Background(), m)
	c := New()
	msgs := []Message{{Role: "user", Content: "h"}}
	if _, err := c.Chat(ctx, srv.URL, "", "m", msgs, 0.5, 100); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.ChatWithTools(ctx, srv.URL, "", "m", msgs, 0.5, 100, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Chat(ctx, srv.URL, "", "m", msgs, 0.5, 100); err != nil {
		t.Fatal(err)
	}
	u, calls, withUsage := m.Total()
	if calls != 3 || withUsage != 3 {
		t.Fatalf("calls=%d withUsage=%d", calls, withUsage)
	}
	if u.PromptTokens != 6000 || u.CompletionTokens != 60 || u.PromptTokensDetails.CachedTokens != 600 || u.TotalTokens != 6060 {
		t.Fatalf("suma=%+v", u)
	}
	// Una crida sense el comptador al context no hi suma.
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", msgs, 0.5, 100); err != nil {
		t.Fatal(err)
	}
	if _, calls, _ := m.Total(); calls != 3 {
		t.Fatalf("una crida sense comptador hi ha sumat: calls=%d", calls)
	}
}

// Streaming: la petició demana stream_options.include_usage i l'usage de
// l'últim event se suma. Si el proveïdor el repeteix acumulat a cada event,
// compta l'últim, no la suma de tots.
func TestUsageMeterStream(t *testing.T) {
	var demanat []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			StreamOptions *struct {
				IncludeUsage bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		demanat = append(demanat, body.StreamOptions != nil && body.StreamOptions.IncludeUsage)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ch := range []string{
			`{"choices":[{"delta":{"content":"ho"}}],"usage":{"prompt_tokens":500,"completion_tokens":1}}`,
			`{"choices":[{"delta":{"content":"la"},"finish_reason":"stop"}],"usage":{"prompt_tokens":500,"completion_tokens":2}}`,
			`{"choices":[],"usage":{"prompt_tokens":500,"completion_tokens":3,"total_tokens":503}}`,
		} {
			fmt.Fprintf(w, "data: %s\n\n", ch)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	m := &UsageMeter{}
	ctx := WithUsage(context.Background(), m)
	c := New()
	msgs := []Message{{Role: "user", Content: "h"}}
	if _, _, err := c.ChatStreamWithTools(ctx, srv.URL, "", "m", msgs, 0.5, 100, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ChatStream(ctx, srv.URL, "", "m", msgs, 0.5, 100, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(demanat) != 2 || !demanat[0] || !demanat[1] {
		t.Fatalf("stream_options.include_usage no s'ha demanat: %v", demanat)
	}
	u, calls, withUsage := m.Total()
	if calls != 2 || withUsage != 2 || u.PromptTokens != 1000 || u.CompletionTokens != 6 {
		t.Fatalf("suma=%+v calls=%d withUsage=%d", u, calls, withUsage)
	}
}

// Sense usage del proveïdor, la crida compta però no suma tokens: és el
// senyal perquè qui llegeix torni a l'estimació.
func TestUsageMeterSenseUsage(t *testing.T) {
	srv := sseServer(t, []string{`{"choices":[{"delta":{"content":"hola"},"finish_reason":"stop"}]}`})
	defer srv.Close()
	m := &UsageMeter{}
	ctx := WithUsage(context.Background(), m)
	if _, err := New().ChatStream(ctx, srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100, nil, nil); err != nil {
		t.Fatal(err)
	}
	u, calls, withUsage := m.Total()
	if calls != 1 || withUsage != 0 || !u.Buida() {
		t.Fatalf("suma=%+v calls=%d withUsage=%d", u, calls, withUsage)
	}
}

// Un servidor estricte que rebutja stream_options no ha de trencar la crida:
// es torna a provar sense el camp i aquell endpoint ja no el rep més.
func TestStreamOptionsRebutjat(t *testing.T) {
	var peticions, ambCamp int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peticions++
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "stream_options") {
			ambCamp++
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"Unrecognized request argument supplied: stream_options"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	c := New()
	msgs := []Message{{Role: "user", Content: "h"}}
	for i := 0; i < 2; i++ {
		got, _, err := c.ChatStreamWithTools(context.Background(), srv.URL, "", "m", msgs, 0.5, 100, nil, nil, nil)
		if err != nil || got != "ok" {
			t.Fatalf("crida %d: got=%q err=%v", i, got, err)
		}
	}
	if peticions != 3 || ambCamp != 1 {
		t.Fatalf("peticions=%d ambCamp=%d (esperat 3 i 1)", peticions, ambCamp)
	}
}

// Un 400 d'una altra mena no es reintenta: arriba tal qual al caller.
func TestStreamOptionsAltre400(t *testing.T) {
	peticions := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peticions++
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"context length exceeded"}}`)
	}))
	defer srv.Close()
	_, err := New().ChatStream(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "h"}}, 0.5, 100, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "context length exceeded") {
		t.Fatalf("err=%v", err)
	}
	if peticions != 1 {
		t.Fatalf("peticions=%d", peticions)
	}
}

// Un comptador penjat sobre un altre suma als dos (run dins d'un run).
func TestUsageMeterEncadenat(t *testing.T) {
	pare, fill := &UsageMeter{}, &UsageMeter{}
	ctx := WithUsage(WithUsage(context.Background(), pare), fill)
	UsageFrom(ctx).Add(Usage{PromptTokens: 7, CompletionTokens: 3})
	if u, c, _ := pare.Total(); c != 1 || u.PromptTokens != 7 {
		t.Fatalf("pare=%+v calls=%d", u, c)
	}
	if u, c, _ := fill.Total(); c != 1 || u.CompletionTokens != 3 {
		t.Fatalf("fill=%+v calls=%d", u, c)
	}
}
