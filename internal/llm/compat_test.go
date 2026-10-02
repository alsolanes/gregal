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

// Un proveïdor estricte com opencode zen rebutja amb 400 els camps que no
// coneix (enable_thinking, chat_template_kwargs) i accepta
// reasoning_effort. Amb think: no, la crida ha d'anar bé igualment, amb
// l'ordre que sí que entén, i la segona crida ja no ha d'enviar el que
// va rebutjar (una sola petició, no tres).
func TestCampsOpcionalsAmbProveidorEstricte(t *testing.T) {
	var peticions atomic.Int32
	var ultim map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peticions.Add(1)
		b, _ := io.ReadAll(r.Body)
		var cos map[string]any
		_ = json.Unmarshal(b, &cos)
		for _, camp := range []string{"enable_thinking", "chat_template_kwargs"} {
			if _, hi := cos[camp]; hi {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":{"param":"` + camp + `","type":"invalid_request_error","message":"Upstream request failed: [unknown_parameter] invalid request body: json: unknown field \"` + camp + `\""}}`))
				return
			}
		}
		ultim = cos
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := New()
	ctx := WithThink(context.Background(), "no")
	msgs := []Message{{Role: "user", Content: "hola"}}
	if _, err := c.Chat(ctx, srv.URL, "", "m", msgs, 0, 16); err != nil {
		t.Fatalf("un camp opcional no pot trencar la crida: %v", err)
	}
	if ultim["reasoning_effort"] != "none" {
		t.Fatalf("l'ordre que el proveïdor sí que entén ha d'arribar: %v", ultim)
	}
	peticions.Store(0)
	if _, err := c.Chat(ctx, srv.URL, "", "m", msgs, 0, 16); err != nil {
		t.Fatal(err)
	}
	if n := peticions.Load(); n != 1 {
		t.Fatalf("els camps rebutjats es recorden: %d peticions a la segona crida", n)
	}

	// Un altre endpoint no hereta el que ha rebutjat aquest.
	var rebut map[string]any
	permissiu := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &rebut)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer permissiu.Close()
	if _, err := c.Chat(ctx, permissiu.URL, "", "m", msgs, 0, 16); err != nil {
		t.Fatal(err)
	}
	if _, hi := rebut["chat_template_kwargs"]; !hi {
		t.Fatal("un proveïdor permissiu continua rebent chat_template_kwargs")
	}
}

// Un 400 que no és de camps (context massa llarg) torna tal qual, sense
// reintents ni camps apagats.
func TestCampsOpcionalsAltre400(t *testing.T) {
	var peticions atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peticions.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"This model's maximum context length is 4096 tokens"}}`))
	}))
	defer srv.Close()
	c := New()
	_, err := c.Chat(WithThink(context.Background(), "no"), srv.URL, "", "m", []Message{{Role: "user", Content: "x"}}, 0, 16)
	if err == nil {
		t.Fatal("un 400 de context s'ha de tornar")
	}
	if n := peticions.Load(); n != 1 {
		t.Fatalf("sense reintents per un 400 que no és de camps: %d", n)
	}
	if c.campOff(normBase(srv.URL), campEnableThinking) {
		t.Fatal("un 400 de context no apaga cap camp")
	}
}
