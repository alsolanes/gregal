package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// think: no ha d'arribar al proveïdor. chatRequest duia els camps però el
// wire (marshalChatRequest) no els copiava, i l'ordre de no pensar mai
// sortia: el model raonava igual i el torn trigava el doble.
func TestThinkNoArribaAlCable(t *testing.T) {
	var cos map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &cos)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := New()
	ctx := WithThink(context.Background(), "no")
	if _, err := c.Chat(ctx, srv.URL, "", "m", []Message{{Role: "user", Content: "hola"}}, 0, 16); err != nil {
		t.Fatal(err)
	}
	if v, ok := cos["enable_thinking"].(bool); !ok || v {
		t.Fatalf("enable_thinking:false ha de sortir: %v", cos["enable_thinking"])
	}
	kw, _ := cos["chat_template_kwargs"].(map[string]any)
	if v, ok := kw["enable_thinking"].(bool); !ok || v {
		t.Fatalf("chat_template_kwargs.enable_thinking:false ha de sortir: %v", cos["chat_template_kwargs"])
	}

	// Sense think: no, cap dels dos camps (hi ha proveïdors que rebutgen
	// camps desconeguts).
	cos = nil
	if _, err := c.Chat(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "hola"}}, 0, 16); err != nil {
		t.Fatal(err)
	}
	if _, hi := cos["enable_thinking"]; hi {
		t.Fatal("sense think: no, enable_thinking no s'envia")
	}
	if _, hi := cos["chat_template_kwargs"]; hi {
		t.Fatal("sense think: no, chat_template_kwargs no s'envia")
	}
}
