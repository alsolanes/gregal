package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// rotllanaStreamFixture prepares three members; the request names Tramuntana
// and Xaloc explicitly. Voices run in parallel, so the stub identifies each
// voice by its brief («Now only YOU, <name>»). Plain fallback and the joint
// answer go through petChatter.
func rotllanaStreamFixture(t *testing.T, loop func(who string, ctx context.Context, emit func(kind, text string)) (exploracio, error), plain func(ctx context.Context, msgs []llm.Message) (string, error)) (*Server, string) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	savedLoop, savedChatter := rotllanaVoiceLoopStream, petChatter
	rotllanaVoiceLoopStream = func(_ *Server, ctx context.Context, brief string, emit func(kind, text string), _ ...*routedRole) (exploracio, error) {
		return loop(voiceOf(brief), ctx, emit)
	}
	petChatter = func(ctx context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, msgs []llm.Message) (string, error) {
		return plain(ctx, msgs)
	}
	t.Cleanup(func() { waitBackground(); rotllanaVoiceLoopStream, petChatter = savedLoop, savedChatter })
	return s, id
}

// voiceOf: quin membre parla segons el seu encàrrec.
func voiceOf(brief string) string {
	for _, who := range []string{"Tramuntana", "Xaloc", "Llevant"} {
		if strings.Contains(brief, "Now only YOU, "+who) {
			return who
		}
	}
	return ""
}

// isJointAnswer: la crida que tanca el grup (rotllanaSummary).
func isJointAnswer(msgs []llm.Message) bool {
	return len(msgs) > 0 && strings.Contains(msgs[0].Content, "Close this debate")
}

// Un grup amb dues veus: parlen alhora (cadascuna en lectura i amb el seu
// encàrrec), les veus queden plegades i el que porta el fil en fa SEMPRE una
// resposta conjunta, que és la resposta del torn. Abans la segona veu només
// podia revisar la primera i la conjunta calia demanar-la amb «debateu».
func TestRotllanaGroupAnswersTogetherAndMerges(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}
	running, maxRunning := 0, 0
	release := make(chan struct{})
	s, id := rotllanaStreamFixture(t, func(who string, ctx context.Context, _ func(string, string)) (exploracio, error) {
		mu.Lock()
		seen[who] = true
		running++
		if running > maxRunning {
			maxRunning = running
		}
		if running == 2 {
			close(release)
		}
		mu.Unlock()
		<-release
		if scope, _ := ctx.Value(interactiveWindKey{}).(windTurnScope); scope.mode != "inspect" {
			t.Errorf("%s: les veus en paral·lel van en lectura, mode=%q", who, scope.mode)
		}
		if who == "Xaloc" {
			return exploracio{text: "Risc: falta una prova de càrrega."}, nil
		}
		return exploracio{text: "Proposta quars-193."}, nil
	}, func(_ context.Context, msgs []llm.Message) (string, error) {
		if !isJointAnswer(msgs) {
			return "", errors.New("unexpected plain call")
		}
		if !strings.Contains(msgs[1].Content, "quars-193") || !strings.Contains(msgs[1].Content, "prova de càrrega") {
			t.Errorf("la resposta conjunta necessita les dues veus:\n%s", msgs[1].Content)
		}
		return "Decisió: quars-193, però primer la prova de càrrega.", nil
	})
	raw := postRotllanaStream(s, id, "quina opció és millor?")
	if maxRunning != 2 || !seen["Tramuntana"] || !seen["Xaloc"] {
		t.Fatalf("les dues veus havien de treballar alhora: max=%d seen=%v", maxRunning, seen)
	}
	if !strings.Contains(raw, "event: tempesta") || !strings.Contains(raw, "event: done") || !strings.Contains(raw, "Decisió: quars-193") {
		t.Fatalf("resposta:\n%s", raw)
	}
	disk, err := session.Load(s.sessDir(), "rotllana-"+id)
	if err != nil || len(disk.Convo) != 4 {
		t.Fatalf("missatge, dues veus i la conjunta: %+v %v", disk, err)
	}
	if disk.Convo[1].Kind != "debate" || disk.Convo[2].Kind != "debate" || disk.Convo[3].Kind != "" || !strings.Contains(disk.Convo[3].Content, "Decisió") {
		t.Fatalf("les veus plegades i la conjunta al final: %+v", disk.Convo)
	}
}

// Aturar el grup mentre les veus treballen: no hi ha done i no es desa res.
func TestRotllanaStreamGroupIsCancellable(t *testing.T) {
	started := make(chan struct{}, 2)
	s, id := rotllanaStreamFixture(t, func(_ string, ctx context.Context, _ func(string, string)) (exploracio, error) {
		started <- struct{}{}
		<-ctx.Done()
		return exploracio{}, ctx.Err()
	}, func(ctx context.Context, _ []llm.Message) (string, error) { return "", ctx.Err() })
	done := make(chan string, 1)
	go func() { done <- postRotllanaStream(s, id, "fem una web") }()
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("les veus no han començat")
		}
	}
	rec := postJSON(t, s.handleWindCancel, `{"rotllana":"`+id+`"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"aturat":true`) {
		t.Fatalf("cancel: %s", rec.Body.String())
	}
	select {
	case raw := <-done:
		if !strings.Contains(raw, `"cancelat":true`) || strings.Contains(raw, "event: done") {
			t.Fatalf("cancel stream: %s", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el grup no s'ha aturat")
	}
}

func postRotllanaStream(s *Server, id, message string) string {
	return postRotllanaStreamMessage(s, id, "@Tramuntana @Xaloc "+message)
}

func postRotllanaStreamMessage(s *Server, id, message string) string {
	req := httptest.NewRequest("POST", "/api/winds/rotllana/stream", strings.NewReader(`{"rotllana":"`+id+`","message":"`+message+`"}`))
	rec := httptest.NewRecorder()
	s.handleRotllanaChatStream(rec, req)
	return rec.Body.String()
}

// Si una veu falla (i el pla B també), la que ha respost queda desada i
// visible (sense plegar: no hi ha res a ajuntar) i el torn diu qui no ha pogut.
func TestRotllanaStreamKeepsFinishedVoiceWhenAnotherFails(t *testing.T) {
	s, id := rotllanaStreamFixture(t, func(who string, _ context.Context, _ func(kind, text string)) (exploracio, error) {
		if who == "Tramuntana" {
			return exploracio{text: "Primer pla per a la cuina"}, nil
		}
		return exploracio{}, context.DeadlineExceeded
	}, func(context.Context, []llm.Message) (string, error) { return "", context.DeadlineExceeded })

	raw := postRotllanaStream(s, id, "fem un pressupost")
	if !strings.Contains(raw, `"failed":["Xaloc"]`) || strings.Contains(raw, "event: error") || !strings.Contains(raw, "event: done") {
		t.Fatalf("la ronda havia d'acabar dient qui ha fallat, sense error global:\n%s", raw)
	}
	disk, err := session.Load(s.sessDir(), "rotllana-"+id)
	if err != nil {
		t.Fatal(err)
	}
	if len(disk.Convo) != 2 || disk.Convo[0].Role != "user" || disk.Convo[1].SpeakerID != "tramuntana" || disk.Convo[1].Kind != "" {
		t.Fatalf("havia de quedar desat el missatge i la resposta de Tramuntana, visible: %+v", disk.Convo)
	}
}

func TestRotllanaJSONKeepsFinishedVoiceWhenAnotherFails(t *testing.T) {
	s, id := rotllanaStreamFixture(t, nil, func(context.Context, []llm.Message) (string, error) {
		return "", context.DeadlineExceeded
	})
	saved := rotllanaVoiceLoop
	t.Cleanup(func() { waitBackground(); rotllanaVoiceLoop = saved })
	rotllanaVoiceLoop = func(_ *Server, _ context.Context, brief string, _ ...*routedRole) (exploracio, error) {
		if voiceOf(brief) == "Tramuntana" {
			return exploracio{text: "Primer pla per a la cuina"}, nil
		}
		return exploracio{}, context.DeadlineExceeded
	}
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"@Tramuntana @Xaloc, fem un pressupost"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"failed":["Xaloc"]`) {
		t.Fatalf("JSON partial success: %d %s", rec.Code, rec.Body.String())
	}
	disk, err := session.Load(s.sessDir(), "rotllana-"+id)
	if err != nil || len(disk.Convo) != 2 || disk.Convo[1].Content != "Primer pla per a la cuina" {
		t.Fatalf("the finished voice must be durable: %+v, %v", disk.Convo, err)
	}
}

// El termini és per veu: una cerca que l'esgota cau al xat pla amb temps
// propi, i l'altra veu, que corre alhora, té el seu.
func TestRotllanaStreamGivesEachVoiceItsOwnDeadline(t *testing.T) {
	savedTurn := rotllanaTurnTimeout
	rotllanaTurnTimeout = 40 * time.Millisecond
	t.Cleanup(func() { rotllanaTurnTimeout = savedTurn })
	var plainCalls atomic.Int32
	s, id := rotllanaStreamFixture(t, func(who string, ctx context.Context, _ func(kind, text string)) (exploracio, error) {
		if who == "Tramuntana" {
			<-ctx.Done()
			return exploracio{}, ctx.Err()
		}
		return exploracio{text: "Segona veu a temps"}, nil
	}, func(ctx context.Context, msgs []llm.Message) (string, error) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if isJointAnswer(msgs) {
			return "Les dues coses alhora.", nil
		}
		plainCalls.Add(1)
		return "Resposta sense cerca", nil
	})

	raw := postRotllanaStream(s, id, "i el calendari?")
	if strings.Contains(raw, "event: error") || strings.Contains(raw, `"failed":["`) {
		t.Fatalf("cap veu havia de fallar:\n%s", raw)
	}
	if plainCalls.Load() != 1 || !strings.Contains(raw, "Resposta sense cerca") || !strings.Contains(raw, "Segona veu a temps") {
		t.Fatalf("falten les dues respostes:\n%s", raw)
	}
}

// TestRotllanaStreamNobodyAnswersSavesNothing: si cap veu respon, no es desa
// el missatge (el client el torna a l'esborrany) i arriba un error, no un done.
func TestRotllanaStreamNobodyAnswersSavesNothing(t *testing.T) {
	s, id := rotllanaStreamFixture(t, func(string, context.Context, func(kind, text string)) (exploracio, error) {
		return exploracio{}, context.DeadlineExceeded
	}, func(context.Context, []llm.Message) (string, error) { return "", context.DeadlineExceeded })

	raw := postRotllanaStream(s, id, "hola")
	if !strings.Contains(raw, "event: error") || strings.Contains(raw, "event: done") || strings.Contains(raw, "event: reply") {
		t.Fatalf("volia només un error:\n%s", raw)
	}
	if disk, err := session.Load(s.sessDir(), "rotllana-"+id); err == nil && len(disk.Convo) > 0 {
		t.Fatalf("no s'havia de desar res: %+v", disk.Convo)
	}
}
