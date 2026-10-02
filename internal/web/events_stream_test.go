package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/events"
)

// captureSSE permet provar un handler que no acaba fins que es cancel·la el
// context. httptest.ResponseRecorder no notifica cada Flush i no és segur de
// llegir mentre el handler hi escriu.
type captureSSE struct {
	header  http.Header
	mu      sync.Mutex
	body    strings.Builder
	changed chan struct{}
}

func newCaptureSSE() *captureSSE {
	return &captureSSE{header: make(http.Header), changed: make(chan struct{}, 1)}
}

func (c *captureSSE) Header() http.Header { return c.header }

func (c *captureSSE) WriteHeader(int) {}

func (c *captureSSE) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.body.Write(p)
	c.mu.Unlock()
	select {
	case c.changed <- struct{}{}:
	default:
	}
	return len(p), nil
}

func (c *captureSSE) Flush() {}

func (c *captureSSE) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.body.String()
}

func waitSSEContains(t *testing.T, c *captureSSE, want string) string {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		if got := c.String(); strings.Contains(got, want) {
			return got
		}
		select {
		case <-c.changed:
		case <-deadline.C:
			t.Fatalf("SSE no ha emès %q; body=%q", want, c.String())
		}
	}
}

func TestEventsStreamReprènAmbCursor(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()
	first, err := h.eventStore.Append(events.Event{SessionID: s.id, Kind: "text", Text: "primer"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v2/events/stream?after=0", nil).WithContext(ctx)
	capture := newCaptureSSE()
	done := make(chan struct{})
	go func() {
		s.handleEventsStream(capture, req)
		close(done)
	}()
	body := waitSSEContains(t, capture, `"text":"primer"`)
	if !strings.Contains(body, "id: "+itoa(int64(first.ID))+"\n") || !strings.Contains(body, "event: text\n") {
		t.Fatalf("format SSE incomplet: %q", body)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("l'SSE no ha tancat en cancel·lar el context")
	}

	second, err := h.eventStore.Append(events.Event{SessionID: s.id, Kind: "done", Text: "segon"})
	if err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v2/events/stream", nil).WithContext(ctx2)
	req2.Header.Set("Last-Event-ID", itoa(int64(first.ID)))
	capture2 := newCaptureSSE()
	done2 := make(chan struct{})
	go func() {
		s.handleEventsStream(capture2, req2)
		close(done2)
	}()
	body2 := waitSSEContains(t, capture2, `"text":"segon"`)
	if strings.Contains(body2, `"text":"primer"`) || !strings.Contains(body2, "id: "+itoa(int64(second.ID))+"\n") {
		t.Fatalf("la reconnexió no ha reprès des de Last-Event-ID: %q", body2)
	}
	cancel2()
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("el segon SSE no ha tancat en cancel·lar el context")
	}
}

func TestEventsStreamMantéAuthDelMiddleware(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.SetToken("secret")
	h := s.Hub()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/events/stream", nil)
	rec := httptest.NewRecorder()
	s.requireUser(h.mux()).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("stream sense token: status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestEventsStreamReprodueixPayloadInteractiu(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	h := s.Hub()
	if _, err := h.eventStore.Append(events.Event{
		SessionID: s.id, Kind: "approve_request", Text: "permís",
		Payload: []byte(`{"key":"7","call_id":"call-1","name":"shell","args":"ls"}`),
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	capture := newCaptureSSE()
	done := make(chan struct{})
	go func() {
		s.handleEventsStream(capture, httptest.NewRequest(http.MethodGet, "/api/v2/events/stream?after=0", nil).WithContext(ctx))
		close(done)
	}()
	body := waitSSEContains(t, capture, "event: approve_request\n")
	if !strings.Contains(body, `"payload":{"key":"7","call_id":"call-1","name":"shell","args":"ls"}`) {
		t.Fatalf("SSE sense payload interactiu: %q", body)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("l'SSE no ha tancat en cancel·lar")
	}
}
