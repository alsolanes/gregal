package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// rotllanaStreamFixture prepares three members; the request names Tramuntana
// and Xaloc explicitly. Search and plain fallback are stubbed per voice.
func rotllanaStreamFixture(t *testing.T, loop func(n int32, ctx context.Context, emit func(kind, text string)) (exploracio, error), plain func(ctx context.Context) (string, error)) (*Server, string) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	var calls int32
	savedLoop, savedChatter := rotllanaVoiceLoopStream, petChatter
	rotllanaVoiceLoopStream = func(_ *Server, ctx context.Context, _ string, emit func(kind, text string)) (exploracio, error) {
		return loop(atomic.AddInt32(&calls, 1), ctx, emit)
	}
	petChatter = func(ctx context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, _ []llm.Message) (string, error) {
		return plain(ctx)
	}
	t.Cleanup(func() { waitBackground(); rotllanaVoiceLoopStream, petChatter = savedLoop, savedChatter })
	return s, id
}

func TestRotllanaStreamDebateIsCancellableAndOrdinaryTurnsStop(t *testing.T) {
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	s, id := rotllanaStreamFixture(t, func(n int32, _ context.Context, _ func(string, string)) (exploracio, error) {
		if n%2 == 1 {
			return exploracio{text: "Primera proposta"}, nil
		}
		return exploracio{text: "Segona proposta"}, nil
	}, func(ctx context.Context) (string, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return "", ctx.Err()
	})
	raw := postRotllanaStream(s, id, "fem una web")
	if calls.Load() != 0 || !strings.Contains(raw, "event: done") {
		t.Fatalf("unexpected debate: %s", raw)
	}
	done := make(chan string, 1)
	go func() { done <- postRotllanaStream(s, id, "discutiu entre vosaltres") }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("debate did not start")
	}
	rec := postJSON(t, s.handleWindCancel, `{"rotllana":"`+id+`"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"aturat":true`) {
		t.Fatalf("cancel: %s", rec.Body.String())
	}
	select {
	case raw = <-done:
		if !strings.Contains(raw, `"cancelat":true`) || strings.Contains(raw, "event: done") {
			t.Fatalf("cancel stream: %s", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("debate did not stop")
	}
	disk, err := session.Load(s.sessDir(), "rotllana-"+id)
	if err != nil || len(disk.Convo) != 6 {
		t.Fatalf("completed messages lost: %+v, %v", disk, err)
	}
}

func postRotllanaStream(s *Server, id, message string) string {
	req := httptest.NewRequest("POST", "/api/winds/rotllana/stream", strings.NewReader(`{"rotllana":"`+id+`","message":"@Tramuntana @Xaloc `+message+`"}`))
	rec := httptest.NewRecorder()
	s.handleRotllanaChatStream(rec, req)
	return rec.Body.String()
}

// TestRotllanaStreamKeepsFinishedVoiceWhenLaterVoiceFails: una veu acabada
// queda desada i anunciada abans que parli la següent; si aquesta falla, la
// ronda acaba igualment amb el que s'ha dit i diu qui no ha pogut respondre.
func TestRotllanaStreamKeepsFinishedVoiceWhenLaterVoiceFails(t *testing.T) {
	s, id := rotllanaStreamFixture(t, func(n int32, _ context.Context, emit func(kind, text string)) (exploracio, error) {
		if n == 1 {
			emit("token", "Primer")
			return exploracio{text: "Primer pla per a la cuina"}, nil
		}
		return exploracio{}, context.DeadlineExceeded
	}, func(context.Context) (string, error) { return "", context.DeadlineExceeded })

	raw := postRotllanaStream(s, id, "fem un pressupost")
	reply, second, done := strings.Index(raw, "event: reply"), strings.Index(raw, `"phase":"speaker:xaloc"`), strings.Index(raw, "event: done")
	if reply < 0 || second < 0 || done < 0 || !(reply < second && second < done) {
		t.Fatalf("la veu acabada s'ha d'anunciar abans de la següent (reply=%d xaloc=%d done=%d):\n%s", reply, second, done, raw)
	}
	if !strings.Contains(raw, `"failed":["Xaloc"]`) || strings.Contains(raw, "event: error") {
		t.Fatalf("la ronda havia d'acabar dient qui ha fallat, sense error global:\n%s", raw)
	}
	disk, err := session.Load(s.sessDir(), "rotllana-"+id)
	if err != nil {
		t.Fatal(err)
	}
	if len(disk.Convo) != 2 || disk.Convo[0].Role != "user" || disk.Convo[1].SpeakerID != "tramuntana" {
		t.Fatalf("havia de quedar desat el missatge i la resposta de Tramuntana: %+v", disk.Convo)
	}
}

// TestRotllanaStreamGivesEachVoiceItsOwnDeadline: el termini és per veu. Una
// cerca que l'esgota cau al xat pla amb temps propi, i la veu següent comença
// amb un termini nou.
func TestRotllanaStreamGivesEachVoiceItsOwnDeadline(t *testing.T) {
	savedTurn := rotllanaTurnTimeout
	rotllanaTurnTimeout = 40 * time.Millisecond
	t.Cleanup(func() { rotllanaTurnTimeout = savedTurn })
	s, id := rotllanaStreamFixture(t, func(n int32, ctx context.Context, _ func(kind, text string)) (exploracio, error) {
		if n == 1 {
			<-ctx.Done()
			return exploracio{}, ctx.Err()
		}
		if ctx.Err() != nil {
			return exploracio{}, errors.New("la segona veu ha heretat un termini esgotat")
		}
		return exploracio{text: "Segona veu a temps"}, nil
	}, func(ctx context.Context) (string, error) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "Resposta sense cerca", nil
	})

	raw := postRotllanaStream(s, id, "i el calendari?")
	if strings.Contains(raw, "event: error") || strings.Contains(raw, `"failed":["`) {
		t.Fatalf("cap veu havia de fallar:\n%s", raw)
	}
	if !strings.Contains(raw, "Resposta sense cerca") || !strings.Contains(raw, "Segona veu a temps") {
		t.Fatalf("falten les dues respostes:\n%s", raw)
	}
}

// TestRotllanaStreamNobodyAnswersSavesNothing: si cap veu respon, no es desa
// el missatge (el client el torna a l'esborrany) i arriba un error, no un done.
func TestRotllanaStreamNobodyAnswersSavesNothing(t *testing.T) {
	s, id := rotllanaStreamFixture(t, func(int32, context.Context, func(kind, text string)) (exploracio, error) {
		return exploracio{}, context.DeadlineExceeded
	}, func(context.Context) (string, error) { return "", context.DeadlineExceeded })

	raw := postRotllanaStream(s, id, "hola")
	if !strings.Contains(raw, "event: error") || strings.Contains(raw, "event: done") || strings.Contains(raw, "event: reply") {
		t.Fatalf("volia només un error:\n%s", raw)
	}
	if disk, err := session.Load(s.sessDir(), "rotllana-"+id); err == nil && len(disk.Convo) > 0 {
		t.Fatalf("no s'havia de desar res: %+v", disk.Convo)
	}
}
