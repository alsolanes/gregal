package llm

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
	"time"
)

// servidorTall és un fake chat-completions que desa cada petició i respon
// amb el guió del torn n (1-based).
type servidorTall struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []chatRequest
}

func nouServidorTall(t *testing.T, guio func(w http.ResponseWriter, n int)) *servidorTall {
	t.Helper()
	s := &servidorTall{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(raw, &req)
		s.mu.Lock()
		s.reqs = append(s.reqs, req)
		n := len(s.reqs)
		s.mu.Unlock()
		guio(w, n)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *servidorTall) req(i int) chatRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reqs[i]
}

func (s *servidorTall) n() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.reqs)
}

func sse(w http.ResponseWriter, deltas ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, d := range deltas {
		fmt.Fprintf(w, "data: %s\n\n", d)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func deltaText(s string) string {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": s}}}})
	return string(b)
}

func deltaCall(id, name, args string) string {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{
		"index": 0, "id": id, "function": map[string]string{"name": name, "arguments": args},
	}}}}}})
	return string(b)
}

func fi(reason string) string {
	return `{"choices":[{"delta":{},"finish_reason":"` + reason + `"}]}`
}

// Text en directe tallat per length: el tros ja ensenyat es conserva, es
// demana una continuació amb el text com a missatge de l'assistent i el
// resultat és el text sencer, igual que el que s'ha pintat.
func TestTallTextEsContinuaSensePerdreRes(t *testing.T) {
	srv := nouServidorTall(t, func(w http.ResponseWriter, n int) {
		if n == 1 {
			sse(w, deltaText("Primera meitat, "), fi("length"))
			return
		}
		sse(w, deltaText("segona meitat."), fi("stop"))
	})
	var avisos []string
	ctx := WithRetryHook(context.Background(), func(attempt, total int, wait time.Duration, err error) {
		avisos = append(avisos, RetryNote(attempt, total, wait, err))
	})
	var pantalla strings.Builder
	msgs := []Message{{Role: "user", Content: "explica"}}
	out, calls, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", msgs, 0, 256, specsProva, func(s string) { pantalla.WriteString(s) }, nil)
	if err != nil || out != "Primera meitat, segona meitat." || len(calls) != 0 {
		t.Fatalf("out=%q calls=%v err=%v", out, calls, err)
	}
	if pantalla.String() != out {
		t.Fatalf("pantalla=%q out=%q", pantalla.String(), out)
	}
	if srv.n() != 2 {
		t.Fatalf("peticions=%d, volia 2", srv.n())
	}
	seg := srv.req(1).Messages
	if len(seg) != 3 || seg[1].Role != "assistant" || seg[1].Content != "Primera meitat, " || seg[2].Role != "user" || !strings.Contains(seg[2].Content, "Continue exactly") {
		t.Fatalf("la continuació no porta el tros i la indicació: %+v", seg)
	}
	if len(msgs) != 1 {
		t.Fatalf("l'historial del caller no s'ha de tocar: %+v", msgs)
	}
	if len(avisos) != 1 || !strings.Contains(avisos[0], "continuant 1/3") {
		t.Fatalf("avisos=%q", avisos)
	}
}

// Una crida d'eina tallada a mig JSON no s'executa mai: es descarta, el text
// que l'acompanyava es conserva, i es torna a demanar amb el doble de
// max_tokens i la indicació de partir la feina.
func TestTallCridaIncompletaNoSExecutaISeReintenta(t *testing.T) {
	for _, buffer := range []bool{false, true} {
		t.Run(fmt.Sprintf("buffer=%v", buffer), func(t *testing.T) {
			srv := nouServidorTall(t, func(w http.ResponseWriter, n int) {
				if n == 1 {
					sse(w, deltaText("Escric el fitxer."), deltaCall("bad", "write", `{"path":"a.go","content":"package a\nfunc`), fi("length"))
					return
				}
				sse(w, deltaCall("good", "write", `{"path":"a.go","content":"package a"}`), fi("tool_calls"))
			})
			ctx := context.Background()
			if buffer {
				ctx = WithRecoverTruncation(ctx)
			}
			var avisos []string
			ctx = WithRetryHook(ctx, func(attempt, total int, wait time.Duration, err error) {
				avisos = append(avisos, RetryNote(attempt, total, wait, err))
			})
			var pantalla strings.Builder
			out, calls, err := New().ChatStreamWithTools(ctx, srv.URL, "", "m", []Message{{Role: "user", Content: "fes-ho"}}, 0, 1000, specsProva, func(s string) { pantalla.WriteString(s) }, nil)
			if err != nil {
				t.Fatal(err)
			}
			if out != "Escric el fitxer." || pantalla.String() != out {
				t.Fatalf("out=%q pantalla=%q", out, pantalla.String())
			}
			if len(calls) != 1 || calls[0].ID != "good" {
				t.Fatalf("s'ha escapat una crida a mitges: %+v", calls)
			}
			if srv.n() != 2 || srv.req(0).MaxTokens != 1000 || srv.req(1).MaxTokens != 2000 {
				t.Fatalf("peticions=%d, pressupostos %d→%d", srv.n(), srv.req(0).MaxTokens, srv.req(1).MaxTokens)
			}
			seg := srv.req(1).Messages
			last := seg[len(seg)-1]
			if last.Role != "user" || !strings.Contains(last.Content, "`write`") || !strings.Contains(last.Content, "NOT executed") || !strings.Contains(last.Content, "smaller") {
				t.Fatalf("indicació de reintent=%+v", last)
			}
			for _, m := range seg {
				if len(m.ToolCalls) > 0 {
					t.Fatalf("la crida tallada no pot tornar a l'historial: %+v", m)
				}
			}
			if len(avisos) != 1 || !strings.Contains(avisos[0], "crida a write tallada") || !strings.Contains(avisos[0], "2000") {
				t.Fatalf("avisos=%q", avisos)
			}
		})
	}
}

// Si el tall arriba just després de crides senceres, es donen per bones:
// no cal cap continuació.
func TestTallAmbCridesSenceresLesAccepta(t *testing.T) {
	srv := nouServidorTall(t, func(w http.ResponseWriter, n int) {
		sse(w, deltaCall("c1", "read", `{"path":"x"}`), fi("length"))
	})
	_, calls, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m", nil, 0, 100, specsProva, nil, nil)
	if err != nil || len(calls) != 1 || calls[0].ID != "c1" || srv.n() != 1 {
		t.Fatalf("calls=%+v n=%d err=%v", calls, srv.n(), err)
	}
}

// Si cada intent torna una crida tallada, al sostre es lliura el text amb
// una nota i cap crida: mai s'executa res a mitges.
func TestTallCridaSempreTalladaAlSostre(t *testing.T) {
	srv := nouServidorTall(t, func(w http.ResponseWriter, n int) {
		sse(w, deltaText(fmt.Sprintf("t%d ", n)), deltaCall("bad", "write", `{"content":"`), fi("length"))
	})
	out, calls, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m", nil, 0, 1000, specsProva, nil, nil)
	if err != nil || len(calls) != 0 {
		t.Fatalf("calls=%+v err=%v", calls, err)
	}
	if !strings.HasPrefix(out, "t1 t2 t3 t4 ") || !strings.Contains(out, "crida a write ha quedat incompleta") {
		t.Fatalf("out=%q", out)
	}
	if srv.n() != MaxContinuacions+1 || srv.req(srv.n()-1).MaxTokens != 8000 {
		t.Fatalf("peticions=%d, últim pressupost %d", srv.n(), srv.req(srv.n()-1).MaxTokens)
	}
}

// Si la continuació falla per una altra raó, el text ja escrit es lliura
// amb el motiu en comptes de convertir-se en un error que el llençaria.
func TestTallContinuacioFallidaConservaElText(t *testing.T) {
	srv := nouServidorTall(t, func(w http.ResponseWriter, n int) {
		if n == 1 {
			sse(w, deltaText("Feina feta"), fi("length"))
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
	})
	out, _, err := New().ChatStreamWithTools(context.Background(), srv.URL, "", "m", nil, 0, 100, specsProva, nil, nil)
	if err != nil || !strings.HasPrefix(out, "Feina feta") || !strings.Contains(out, "continuació ha fallat") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

// Via headless (-p) i Telegram: ChatWithToolsFO no-streaming.
func TestTallViaNoStreamingAmbFailover(t *testing.T) {
	srv := nouServidorTall(t, func(w http.ResponseWriter, n int) {
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"Comença","tool_calls":[{"id":"bad","type":"function","function":{"name":"edit","arguments":"{\"old\":\"a"}}]},"finish_reason":"length"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"","tool_calls":[{"id":"ok","type":"function","function":{"name":"edit","arguments":"{\"old\":\"a\"}"}}]},"finish_reason":"tool_calls"}]}`)
	})
	out, calls, _, err := New().ChatWithToolsFO(context.Background(), Target{BaseURL: srv.URL, Model: "m"}, nil, nil, 0, 512, specsProva, nil)
	if err != nil || out != "Comença" || len(calls) != 1 || calls[0].ID != "ok" {
		t.Fatalf("out=%q calls=%+v err=%v", out, calls, err)
	}
}

// Via nativa Anthropic: stop_reason max_tokens amb un tool_use a mitges.
func TestTallAnthropicToolUseIncomplet(t *testing.T) {
	ns := newNativeServer(t, ProviderAnthropic, func(w http.ResponseWriter, n int, body map[string]any) {
		if n == 1 {
			writeSSE(w,
				map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
				map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}},
				map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "Faig servir glob."}},
				map[string]any{"type": "content_block_stop", "index": 0},
				map[string]any{"type": "content_block_start", "index": 1, "content_block": map[string]any{"type": "tool_use", "id": "toolu_bad", "name": "glob", "input": map[string]any{}}},
				map[string]any{"type": "content_block_delta", "index": 1, "delta": map[string]any{"type": "input_json_delta", "partial_json": `{"pattern":`}},
				map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "max_tokens"}},
				map[string]any{"type": "message_stop"},
			)
			return
		}
		writeSSE(w, antStreamToolTurn()...)
	})
	var pantalla strings.Builder
	out, calls, err := New().ChatStreamWithTools(context.Background(), ns.URL+"/v1", "k", "m", []Message{{Role: "user", Content: "hi"}}, 0, 100, specsProva, func(s string) { pantalla.WriteString(s) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Faig servir glob.") || pantalla.String() != out {
		t.Fatalf("out=%q pantalla=%q", out, pantalla.String())
	}
	if len(calls) != 1 || calls[0].ID != "toolu_01" {
		t.Fatalf("calls=%+v", calls)
	}
	if mt, _ := ns.body(1)["max_tokens"].(float64); mt != 200 {
		t.Fatalf("max_tokens del reintent=%v, volia 200", ns.body(1)["max_tokens"])
	}
}
