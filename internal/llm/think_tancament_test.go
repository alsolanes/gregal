package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Amb think: no, glm-5.3-flash (opencode zen) tornava «</think>Arreglat…»:
// la plantilla del model obre el bloc de raonament dins del prompt i el que
// arriba és el tancament sol. L'etiqueta sortia a la resposta de l'usuari.
func TestTancamentThinkSolNoArribaALaResposta(t *testing.T) {
	if cl, th := extractThinkTags("pensant una mica</think>Arreglat."); cl != "Arreglat." || th != "pensant una mica" {
		t.Fatalf("clean=%q think=%q", cl, th)
	}
	if cl, _ := extractThinkTags("</think>\n\nArreglat."); strings.Contains(cl, "think>") {
		t.Fatalf("clean=%q", cl)
	}
	if cl, th := extractThinkTags("<think>a</think>b"); cl != "b" || th != "a" {
		t.Fatalf("el cas de sempre: clean=%q think=%q", cl, th)
	}
	if got := senseThink("res de raonament"); got != "res de raonament" {
		t.Fatalf("sense etiquetes no es toca: %q", got)
	}

	sense := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"</think>\n\nArreglat."},"finish_reason":"stop"}]}`))
	}))
	defer sense.Close()
	out, err := New().Chat(context.Background(), sense.URL, "", "m", []Message{{Role: "user", Content: "x"}}, 0, 64)
	if err != nil || out != "Arreglat." {
		t.Fatalf("sense streaming: %q %v", out, err)
	}

	stream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"</think>\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"Arreglat.\"},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: [DONE]\n\n"))
	}))
	defer stream.Close()
	out, err = New().ChatStream(context.Background(), stream.URL, "", "m", []Message{{Role: "user", Content: "x"}}, 0, 64, nil, nil)
	if err != nil || strings.Contains(out, "think>") || !strings.Contains(out, "Arreglat.") {
		t.Fatalf("amb streaming: %q %v", out, err)
	}
}
