package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gregal/internal/llm"
)

// Una conversa llarga s'ha de resumir per blocs i combinar-los: abans es
// descartava el mig («missatges intermedis omesos») i les decisions que hi
// vivien no arribaven mai al resumidor. Aquest test posa una marca al mig
// que l'antic camí hauria perdut i comprova que algun bloc la inclou.
func TestSummarizeJerarquicNoDescartaElMig(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		body := ""
		if len(req.Messages) > 0 {
			body = req.Messages[0].Content
		}
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		resp := "PART"
		if strings.Contains(body, "Resums parcials") {
			resp = "FINAL"
		}
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, resp)
	}))
	defer srv.Close()

	// 30 missatges de 800 caràcters (~24000 runes): més d'un bloc de 12000.
	// La marca va a l'índex 5, que l'antic tall (>25 → índexs 1..9 omesos)
	// hauria eliminat.
	var convo []llm.Message
	for i := 0; i < 30; i++ {
		c := strings.Repeat("x", 800)
		if i == 5 {
			c = "DECISIO-AL-MIG " + c
		}
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		convo = append(convo, llm.Message{Role: role, Content: c})
	}

	out, err := Summarize(context.Background(), llm.New(), llm.Target{BaseURL: srv.URL, Model: "m"}, nil, convo)
	if err != nil {
		t.Fatal(err)
	}
	if out != "FINAL" {
		t.Fatalf("calia el resum combinat, tenim %q", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) < 3 {
		t.Fatalf("esperava map+reduce (≥3 crides), n'hi ha %d", len(bodies))
	}
	for _, b := range bodies {
		if strings.Contains(b, "DECISIO-AL-MIG") {
			return
		}
	}
	t.Fatal("el mig de la conversa no s'ha enviat cap cop al resumidor")
}

// splitText ha de cobrir tot el text sense perdre res i tallar de
// preferència per salts de línia.
func TestSplitTextCobreixTot(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString(strings.Repeat("a", 30))
		b.WriteString("\n")
	}
	text := b.String()
	parts := splitText(text, 500)
	if len(parts) < 2 {
		t.Fatalf("esperava més d'un tros, %d", len(parts))
	}
	if got := strings.Join(parts, ""); got != text {
		t.Fatalf("els trossos no reconstrueixen el text (%d vs %d)", len(got), len(text))
	}
	for _, p := range parts {
		if len([]rune(p)) > 500 {
			t.Fatalf("tros massa gran: %d", len([]rune(p)))
		}
	}
}
