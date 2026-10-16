package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRepGuardDetectsParagraphLoop(t *testing.T) {
	var g repGuard
	para := "El model torna a dir exactament el mateix paràgraf una vegada i una altra perquè ha entrat en un bucle de repetició sense sortida. "
	s := "Introducció normal. "
	looped := false
	for i := 0; i < 30 && !looped; i++ {
		s += para
		looped = g.veu(s)
	}
	if !looped {
		t.Fatal("no ha detectat el bucle")
	}
	out := g.talla(s)
	if strings.Count(out, "bucle de repetició") > 2 || !strings.HasSuffix(out, NotaRepeticio) {
		t.Fatalf("tall: %q", out)
	}
}

func TestRepGuardIgnoresNormalText(t *testing.T) {
	var g repGuard
	s := ""
	for i := 0; i < 200; i++ {
		s += fmt.Sprintf("Línia %d: un text diferent cada vegada amb el número %d.\n", i, i*7)
		if g.veu(s) {
			t.Fatalf("fals positiu a la línia %d", i)
		}
	}
}

func TestStreamCutsRepetition(t *testing.T) {
	para := "Repeteixo aquesta frase llarga perquè estic encallat en un bucle i no sé com sortir-ne de cap manera possible. "
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for i := 0; i < 400; i++ {
			ev := map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": para}}}}
			b, _ := json.Marshal(ev)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return // el client ha tallat
			}
			fl.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	c := New()
	text, _, err := c.ChatStreamWithTools(context.Background(), srv.URL, "", "m", []Message{{Role: "user", Content: "hola"}}, 0, 100000, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(text, NotaRepeticio) || strings.Count(text, "encallat") > 2 {
		t.Fatalf("no s'ha tallat: %d caràcters", len(text))
	}
}
