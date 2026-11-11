package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/llm"
)

// El reply del JSON surt descodificat a trossos, encara que els fragments
// tallin escapaments, \u i lletres de més d'un byte.
func TestCapReplyStreamDescodificaTrossos(t *testing.T) {
	raw := `{"action":"self","winds":[],"task":"","reply":"Els \"boneshakers\" van\nsortir a França 🚲 i ja."}`
	for _, size := range []int{1, 2, 3, 7, len(raw)} {
		var got strings.Builder
		c := &capReplyStream{emit: func(s string) { got.WriteString(s) }}
		for i := 0; i < len(raw); i += size {
			end := i + size
			if end > len(raw) {
				end = len(raw)
			}
			c.feed(raw[i:end])
		}
		if want := "Els \"boneshakers\" van\nsortir a França 🚲 i ja."; got.String() != want {
			t.Fatalf("trossos de %d: %q", size, got.String())
		}
	}
}

// Una entrega a un vent no surt en directe: la frase arriba amb el done.
func TestCapReplyStreamNomesSelf(t *testing.T) {
	var got strings.Builder
	c := &capReplyStream{emit: func(s string) { got.WriteString(s) }}
	c.feed(`{"action":"wind","winds":["w-1"],"reply":"Això és per a la Marinada"}`)
	if got.Len() != 0 {
		t.Fatalf("una entrega ha sortit en directe: %q", got.String())
	}
	got.Reset()
	c = &capReplyStream{emit: func(s string) { got.WriteString(s) }}
	c.feed("Hola! Sóc aquí.")
	if got.String() != "Hola! Sóc aquí." {
		t.Fatalf("prosa: %q", got.String())
	}
}

// Un JSON tallat (pressupost exhaurit) dona el text, mai el JSON cru.
func TestCapJSONTallatNoSurtCru(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	old := capDeciderActual()
	t.Cleanup(func() { setCapDecider(old) })
	setCapDecider(func(*Server, context.Context, []llm.Message) (string, error) {
		return `{"action":"self","winds":[],"task":"","reply":"Els germans Michaux van fer el \"vélocipède\" i despr`, nil
	})
	out := capPost(t, s, `{"message":"explica'm la bicicleta","lang":"ca"}`)
	reply, _ := out["reply"].(string)
	if out["action"] != "self" || strings.Contains(reply, `\"`) || strings.Contains(reply, `"action"`) || !strings.Contains(reply, `"vélocipède"`) {
		t.Fatalf("reply: %q", reply)
	}
}

// El flux passa els trossos del reply i acaba amb el mateix done que el POST.
func TestCapStreamEmetTokensIDone(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	old := capDeciderActual()
	t.Cleanup(func() { setCapDecider(old) })
	setCapDecider(func(_ *Server, ctx context.Context, _ []llm.Message) (string, error) {
		raw := `{"action":"self","reply":"Un motor gira perquè els imants s'empenyen."}`
		if tok := capTokenOf(ctx); tok != nil {
			for i := 0; i < len(raw); i += 9 {
				end := min(i+9, len(raw))
				tok(raw[i:end])
			}
		}
		return raw, nil
	})
	rec := httptest.NewRecorder()
	s.handleCapStream(rec, httptest.NewRequest("POST", "/api/winds/cap/stream", strings.NewReader(`{"message":"com va un motor?","lang":"ca"}`)))
	body := rec.Body.String()
	if strings.Count(body, "event: token") < 3 || !strings.Contains(body, "event: done") || !strings.Contains(body, `"action":"self"`) {
		t.Fatalf("flux:\n%s", body)
	}
	if strings.Contains(body, `"token":"{`) {
		t.Fatalf("el JSON cru ha sortit al flux:\n%s", body)
	}
}
