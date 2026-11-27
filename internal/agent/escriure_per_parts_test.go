package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/llm"
)

// limitSortidaFals és el límit de sortida del proveïdor fals, en caràcters
// entre pensament, text i arguments. És fix: doblar max_tokens no el mou,
// com el sostre real d'un proveïdor.
const limitSortidaFals = 1500

// peticioFalsa és el que el model fals mira de cada petició.
type peticioFalsa struct {
	Messages  []llm.Message `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
	Tools     []any         `json:"tools"`
}

// sseFals munta un stream SSE OpenAI-compatible amb els trossos donats.
type sseFals struct{ b strings.Builder }

func (s *sseFals) envia(v any) {
	raw, _ := json.Marshal(v)
	s.b.WriteString("data: " + string(raw) + "\n\n")
}

func (s *sseFals) delta(d map[string]any) {
	s.envia(map[string]any{"choices": []any{map[string]any{"delta": d}}})
}

func (s *sseFals) crida(id, args string) {
	s.delta(map[string]any{"tool_calls": []any{map[string]any{
		"index": 0, "id": id, "type": "function",
		"function": map[string]string{"name": "write", "arguments": args},
	}}})
}

func (s *sseFals) acaba(finish string, tokens int) string {
	s.envia(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": finish}}})
	s.envia(map[string]any{"choices": []any{}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": tokens}})
	s.b.WriteString("data: [DONE]\n\n")
	return s.b.String()
}

// respostaModelFals és un model que, si la conversa no li ha dit res
// d'escriure per parts, redacta el fitxer sencer pensant i el vol escriure
// d'un sol write. Quan la conversa li diu «append: true» (o ja ha començat
// a escriure'l), escriu el tros següent. El límit de sortida es fa com el
// va fer zen al run 364: tallat dins del pensament, el proxy tanca la crida
// amb només el path i diu tool_calls amb completion_tokens = max_tokens;
// tallat dins dels arguments, finish length.
func respostaModelFals(req peticioFalsa, sencer string, trossos []string) string {
	var s sseFals
	perParts, escrits := false, 0
	for _, m := range req.Messages {
		perParts = perParts || (m.Role != "assistant" && strings.Contains(m.Content, "append: true"))
		for _, c := range m.ToolCalls {
			if c.Function.Name == "write" {
				escrits++
				perParts = true
			}
		}
	}
	if len(req.Tools) == 0 || escrits >= len(trossos) {
		s.delta(map[string]any{"content": "index.html escrit sencer"})
		return s.acaba("stop", 5)
	}
	pensament, args := "Redacto el fitxer sencer:\n"+sencer, map[string]any{"path": "index.html", "content": sencer}
	if perParts {
		pensament, args = "Següent tros.", map[string]any{"path": "index.html", "content": trossos[escrits], "append": escrits > 0}
	}
	raw, _ := json.Marshal(args)
	id := fmt.Sprintf("w%d", len(req.Messages))
	switch {
	case len(pensament) >= limitSortidaFals:
		s.delta(map[string]any{"reasoning_content": pensament[:limitSortidaFals]})
		s.crida(id, `{"path": "index.html"}`)
		return s.acaba("tool_calls", req.MaxTokens)
	case len(pensament)+len(raw) > limitSortidaFals:
		s.delta(map[string]any{"reasoning_content": pensament})
		s.crida(id, string(raw[:limitSortidaFals-len(pensament)]))
		return s.acaba("length", req.MaxTokens)
	}
	s.delta(map[string]any{"reasoning_content": pensament})
	s.crida(id, string(raw))
	return s.acaba("tool_calls", 50)
}

// El cas del run 364 d'extrem a extrem, amb el motor de debò: se li demana
// un fitxer més gran que el límit de sortida, el model se'l gasta pensant i
// només arriba el path. El torn ha d'acabar amb el fitxer sencer al disc,
// escrit en diverses crides, sense que ningú hi digui res.
func TestFitxerGranTallatPelLimitAcabaEscritPerParts(t *testing.T) {
	var linies []string
	for i := 1; i <= 40; i++ {
		linies = append(linies, fmt.Sprintf("<p>Línia %02d del joc: %s</p>", i, strings.Repeat("x", 60)))
	}
	sencer := strings.Join(linies, "\n") + "\n"
	var trossos []string
	for i := 0; i < len(linies); i += 10 {
		trossos = append(trossos, strings.Join(linies[i:i+10], "\n")+"\n")
	}
	if len(sencer) <= limitSortidaFals {
		t.Fatalf("el fitxer (%d) ha de passar del límit (%d)", len(sencer), limitSortidaFals)
	}
	var mu sync.Mutex
	var indicacions []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req peticioFalsa
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if last := req.Messages[len(req.Messages)-1]; last.Role == "user" {
			mu.Lock()
			indicacions = append(indicacions, last.Content)
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(w, respostaModelFals(req, sencer, trossos)); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	res, err := RunNonInteractiveExIn(context.Background(), llm.New(), functionalConfig(srv.URL), "Fes el joc a index.html", ModeCode, 20, false, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil || string(got) != sencer {
		t.Fatalf("el fitxer no és sencer (%d de %d bytes, err %v); resposta %q, eines %v", len(got), len(sencer), err, res.Answer, res.Tools)
	}
	if strings.Join(res.Tools, ",") != "write,write,write,write" || res.Answer != "index.html escrit sencer" {
		t.Fatalf("eines=%v resposta=%q, volia quatre write i la resposta final", res.Tools, res.Answer)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(indicacions) < 2 || !strings.Contains(indicacions[1], "in your thinking") {
		t.Fatalf("el reintent després del tall no diu que no redacti el fitxer pensant: %q", indicacions)
	}
}
