package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/events"
)

// TestV2ContractHealthEventsAndInteraction comprova el contracte que
// comparteixen web, TUI, Android i VS Code: health anuncia la negociació
// completa, el polling conserva cursor/session/payload i l'SSE reprodueix el
// mateix event sense perdre les dades necessàries per a l'aprovació.
func TestV2ContractHealthEventsAndInteraction(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()

	health := v2Do(t, s, http.MethodGet, "/api/health", "")
	if health.Code != http.StatusOK {
		t.Fatalf("health: %d %s", health.Code, health.Body.String())
	}
	var hp struct {
		Status       string          `json:"status"`
		Protocol     string          `json:"protocol"`
		InstanceID   string          `json:"instance_id"`
		Capabilities map[string]bool `json:"capabilities"`
	}
	if err := json.Unmarshal(health.Body.Bytes(), &hp); err != nil {
		t.Fatal(err)
	}
	if hp.Status != "ok" || hp.Protocol == "" || hp.InstanceID == "" {
		t.Fatalf("health incomplet: %+v", hp)
	}
	for _, capability := range []string{
		"sessions", "runs", "events", "durable_events", "event_stream", "interactive_events",
	} {
		if !hp.Capabilities[capability] {
			t.Fatalf("health no anuncia %s: %+v", capability, hp.Capabilities)
		}
	}

	payload := []byte(`{"key":"k1","call_id":"c1","name":"write","args":"{}"}`)
	if _, err := h.eventStore.Append(events.Event{
		SessionID: s.id, RunID: 41, Kind: "approve_request", Text: "permís", Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}

	page := v2Do(t, s, http.MethodGet, "/api/v2/events?after=0&limit=10&session="+s.id, "")
	if page.Code != http.StatusOK {
		t.Fatalf("events: %d %s", page.Code, page.Body.String())
	}
	var ep struct {
		Events []events.Event `json:"events"`
		Cursor uint64         `json:"cursor"`
		Next   uint64         `json:"next"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &ep); err != nil {
		t.Fatal(err)
	}
	if len(ep.Events) != 1 || ep.Events[0].RunID != 41 || ep.Next <= ep.Cursor {
		t.Fatalf("events incompatible: %+v", ep)
	}
	if string(ep.Events[0].Payload) != string(payload) {
		t.Fatalf("payload interactiu perdut: %s", ep.Events[0].Payload)
	}

	ctx, cancel := context.WithCancel(context.Background())
	capture := newCaptureSSE()
	done := make(chan struct{})
	go func() {
		s.handleEventsStream(capture,
			httptest.NewRequest(http.MethodGet, "/api/v2/events/stream?after=0&session="+s.id, nil).WithContext(ctx))
		close(done)
	}()
	body := waitSSEContains(t, capture, "event: approve_request\n")
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("l'stream no s'ha tancat")
	}
	if !strings.Contains(body, `"run_id":41`) || !strings.Contains(body, `"payload":{"key":"k1"`) {
		t.Fatalf("SSE incompatible amb els clients: %s", body)
	}
}
