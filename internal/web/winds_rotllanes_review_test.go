package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestRotllanaWantsReview(t *testing.T) {
	long := strings.Repeat("Una resposta ben llarga i concreta. ", 8)
	for name, tc := range map[string]struct {
		directed, cont bool
		intent, reply  string
		want           bool
	}{
		"normal":        {false, false, "", long, true},
		"adreçat":       {true, false, "", long, false},
		"continua":      {false, true, "", long, false},
		"ja és revisió": {false, false, "review", long, false},
		"curta":         {false, false, "", "Fet.", false},
		"pregunta":      {false, false, "", long + " Quin prefereixes?", true},
		"opcions":       {false, false, "", long + "\n```choices\nA\nB\n```", true},
	} {
		if got := rotllanaWantsReview(tc.directed, tc.cont, tc.intent, tc.reply); got != tc.want {
			t.Errorf("%s: got %v want %v", name, got, tc.want)
		}
	}
}

func TestRotllanaReviewerIsAnotherMember(t *testing.T) {
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	rot, _ := s.rotllanaByID(id)
	members := s.rotMembersOf(rot)
	rev, ok := s.rotllanaReviewer(members, members[:1], "revisa això", nil)
	if !ok || rev.ID == members[0].ID {
		t.Fatalf("revisor %+v ok=%v", rev, ok)
	}
	if _, ok := s.rotllanaReviewer(members, members, "x", nil); ok {
		t.Fatal("si tots han parlat no hi ha revisor")
	}
}

// reviewRoundProvider fa de model a una ronda amb revisió: la primera veu
// respon llarg (perquè valgui una revisió), i el revisor i l'autor intenten
// escriure cadascú el seu fitxer abans de respondre.
func reviewRoundProvider(t *testing.T) *httptest.Server {
	t.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream   bool          `json:"stream"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		var all strings.Builder
		toolDone := false
		for _, m := range req.Messages {
			all.WriteString(m.Content)
			if m.Role == "tool" {
				toolDone = true
			}
		}
		file, final := "", strings.Repeat("La truita: ous, patates i oli, a foc lent. ", 6)
		switch {
		case strings.Contains(all.String(), "REPLY TO YOUR PEER"):
			file, final = "resposta-autor.txt", "Xaloc, tens raó: ja hi he afegit la sal."
		case strings.Contains(all.String(), "PEER REVIEW"):
			file, final = "revisor.txt", "Llevant, a la recepta li falta la sal."
		}
		message := map[string]any{"role": "assistant", "content": final}
		if file != "" && !toolDone {
			args, _ := json.Marshal(map[string]string{"path": file, "content": "escrit"})
			message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
				"id": "call-" + file, "type": "function",
				"function": map[string]string{"name": "write", "arguments": string(args)},
			}}}
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			if calls, ok := message["tool_calls"].([]any); ok {
				calls[0].(map[string]any)["index"] = 0
			}
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": message, "finish_reason": "stop"}}})
			if _, err := fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b); err != nil {
				t.Error(err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message}}}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(provider.Close)
	return provider
}

// L'autor d'un torn que podia escriure aplica la revisió amb els mateixos
// permisos; el revisor continua en només lectura.
func TestRotllanaReviewAuthorKeepsWriteReviewerReadOnly(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%v", stream), func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cwd = t.TempDir()
			s.mode = agent.ModeCode
			s.cfg.Providers["p"] = config.Provider{BaseURL: reviewRoundProvider(t).URL + "/v1"}
			oldRoute := rotllanaRouteModel
			rotllanaRouteModel = func(*Server, context.Context, []llm.Message) (string, error) { return `{"speakers":["llevant"]}`, nil }
			t.Cleanup(func() { waitBackground(); rotllanaRouteModel = oldRoute })
			id := makeRotllanaTrio(t, s)
			handler := s.handleRotllanaChat
			if stream {
				handler = s.handleRotllanaChatStream
			}
			rec := postJSON(t, handler, `{"rotllana":"`+id+`","message":"recepta de la truita de patates"}`)
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "falta la sal") || !strings.Contains(rec.Body.String(), "ja hi he afegit la sal") {
				t.Fatalf("hi havia d'haver revisió i rèplica: %s", rec.Body.String())
			}
			if data, err := os.ReadFile(filepath.Join(s.cwd, "resposta-autor.txt")); err != nil || string(data) != "escrit" {
				t.Fatalf("l'autor havia de poder aplicar la revisió: %v %q", err, data)
			}
			if _, err := os.Stat(filepath.Join(s.cwd, "revisor.txt")); !os.IsNotExist(err) {
				t.Fatalf("el revisor és de només lectura: %v", err)
			}
		})
	}
}

// Si el torn de l'autor era de només lectura (veus en paral·lel), la rèplica
// a la revisió també ho és.
func TestRotllanaReviewAuthorOfReadOnlyTurnStaysReadOnly(t *testing.T) {
	replyMode := ""
	s, id := rotllanaStreamFixture(t, func(who string, ctx context.Context, _ func(string, string)) (exploracio, error) {
		return exploracio{text: strings.Repeat(who+" proposa la truita a foc lent. ", 6)}, nil
	}, func(_ context.Context, msgs []llm.Message) (string, error) {
		return strings.Repeat("Decisió conjunta: truita a foc lent. ", 6), nil
	})
	saved := rotllanaVoiceLoopStream
	rotllanaVoiceLoopStream = func(srv *Server, ctx context.Context, brief string, emit func(kind, text string), rr ...*routedRole) (exploracio, error) {
		scope, _ := ctx.Value(interactiveWindKey{}).(windTurnScope)
		switch {
		case strings.Contains(brief, "REPLY TO YOUR PEER"):
			replyMode = scope.mode
			return exploracio{text: "Xaloc, d'acord."}, nil
		case strings.Contains(brief, "PEER REVIEW"):
			return exploracio{text: "Tramuntana, falta la sal."}, nil
		}
		return saved(srv, ctx, brief, emit, rr...)
	}
	oldRoute := rotllanaRouteModel
	rotllanaRouteModel = func(*Server, context.Context, []llm.Message) (string, error) {
		return `{"speakers":["tramuntana","llevant"]}`, nil
	}
	t.Cleanup(func() { rotllanaVoiceLoopStream, rotllanaRouteModel = saved, oldRoute })
	raw := postRotllanaStreamMessage(s, id, "recepta de la truita de patates")
	if replyMode != agent.ModeInspect {
		t.Fatalf("la rèplica d'un autor en lectura ha de seguir en lectura: mode=%q\n%s", replyMode, raw)
	}
}

// La rèplica de l'autor li demana fer el canvi, no descriure'l.
func TestRotllanaAuthorBriefAsksToApplyTheFix(t *testing.T) {
	brief := rotllanaAuthorBrief(companion.Wind{Name: "Llevant"}, companion.Wind{Name: "Xaloc"})
	if !strings.Contains(brief, "make the change yourself") {
		t.Fatalf("brief: %s", brief)
	}
}
