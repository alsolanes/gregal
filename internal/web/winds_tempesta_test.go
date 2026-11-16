package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Tempesta: tres agents en paral·lel, cadascun amb el seu angle; la síntesi
// rep els tres esborranys i la resposta del torn és la síntesi. El client veu
// l'estat de cada agent.
func TestTempestaRunsAgentsInParallelAndMerges(t *testing.T) {
	s := remindTestServer(t)
	oldLoop, oldChat := windLoop, petChatter
	t.Cleanup(func() { waitBackground(); windLoop = oldLoop; petChatter = oldChat })

	var mu sync.Mutex
	angles := map[string]bool{}
	running, maxRunning := 0, 0
	release := make(chan struct{})
	windLoop = func(_ *Server, ctx context.Context, brief string, _ ...*routedRole) (exploracio, error) {
		mu.Lock()
		running++
		if running > maxRunning {
			maxRunning = running
		}
		for _, a := range tempestaAngles {
			if strings.Contains(brief, a.brief) {
				angles[a.Key] = true
			}
		}
		n := running
		mu.Unlock()
		if n == tempestaAgentCount {
			close(release) // tots tres alhora: el darrer obre la porta
		}
		<-release
		mu.Lock()
		running--
		mu.Unlock()
		switch {
		case strings.Contains(brief, "ANGLE: be the sceptic"):
			return exploracio{text: "Compte: des del gener el preu és 12 €, no 10 €."}, nil
		case strings.Contains(brief, "ANGLE: evidence first"):
			return exploracio{text: "Segons la web oficial costa 12 € [web](https://ex.cat)."}, nil
		}
		return exploracio{text: "Costa 10 €."}, nil
	}
	var synthInput string
	petChatter = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, msgs []llm.Message) (string, error) {
		synthInput = msgs[len(msgs)-1].Content
		return "Costa **12 €** des del gener [web](https://ex.cat).", nil
	}

	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"Quant costa l'entrada?","tempesta":true}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	raw := rec.Body.String()

	if maxRunning != tempestaAgentCount || len(angles) != 3 {
		t.Fatalf("agents en paral·lel: %d, angles: %v", maxRunning, angles)
	}
	for _, want := range []string{"DRAFT 1", "DRAFT 2", "DRAFT 3", "12 €", "10 €"} {
		if !strings.Contains(synthInput, want) {
			t.Fatalf("la síntesi no rep %q:\n%s", want, synthInput)
		}
	}
	if !strings.Contains(raw, "event: tempesta") || !strings.Contains(raw, `"state":"done"`) {
		t.Fatalf("el client no veu els agents:\n%s", raw)
	}
	if !strings.Contains(raw, "event: done") || !strings.Contains(raw, "Costa **12 €** des del gener") {
		t.Fatalf("la resposta ha de ser la síntesi:\n%s", raw)
	}
}

// Si la síntesi falla, la resposta és l'esborrany més complet (mai res buit).
func TestTempestaFallsBackToTheBestDraft(t *testing.T) {
	s := remindTestServer(t)
	oldLoop, oldChat := windLoop, petChatter
	t.Cleanup(func() { waitBackground(); windLoop = oldLoop; petChatter = oldChat })
	windLoop = func(_ *Server, _ context.Context, brief string, _ ...*routedRole) (exploracio, error) {
		if strings.Contains(brief, "ANGLE: evidence first") {
			return exploracio{text: "La resposta llarga i amb fonts [web](https://ex.cat), amb tots els detalls."}, nil
		}
		return exploracio{text: "Curta."}, nil
	}
	petChatter = func(context.Context, *llm.Client, *config.Config, config.Provider, config.Role, []llm.Message) (string, error) {
		return "", context.DeadlineExceeded
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"i ara?","tempesta":true}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	if raw := rec.Body.String(); !strings.Contains(raw, "La resposta llarga i amb fonts") {
		t.Fatalf("sense síntesi, l'esborrany més complet:\n%s", raw)
	}
}
