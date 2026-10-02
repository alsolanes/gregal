package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gregal/internal/service"
)

func TestEventsStreamReprènAmbCursorILastEventID(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v2/events/stream" || r.URL.Query().Get("session") != "feina" {
			t.Fatalf("ruta o sessió incorrectes: %s", r.URL.String())
		}
		if calls == 1 {
			if r.URL.Query().Get("after") != "4" || r.Header.Get("Last-Event-ID") != "4" {
				t.Fatalf("primer cursor incorrecte: query=%s last=%q", r.URL.RawQuery, r.Header.Get("Last-Event-ID"))
			}
		} else if r.URL.Query().Get("after") != "5" || r.Header.Get("Last-Event-ID") != "5" {
			t.Fatalf("reconnexió sense cursor: query=%s last=%q", r.URL.RawQuery, r.Header.Get("Last-Event-ID"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			fmt.Fprintf(w, "id: %d\nevent: text\ndata: {\"kind\":\"text\",\"text\":\"event %d\"}\n\n", 4+calls, 4+calls)
			f.Flush()
			return
		}
		t.Fatal("el servidor de proves no suporta Flush")
	}))
	defer srv.Close()
	c := New(srv.URL, "secret", "inst")
	first, err := c.EventsStream(context.Background(), 4, 100, "feina")
	if err != nil || len(first.Events) != 1 || first.Events[0].ID != 5 {
		t.Fatalf("primer stream: %+v %v", first, err)
	}
	second, err := c.EventsStream(context.Background(), first.Next, 100, "feina")
	if err != nil || len(second.Events) != 1 || second.Events[0].ID != 6 {
		t.Fatalf("reconnexió stream: %+v %v", second, err)
	}
}

func TestEventsStreamFallbackRutaAntiga(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := New(srv.URL, "", "inst")
	_, err := c.EventsStream(context.Background(), 0, 100, "default")
	if !errors.Is(err, ErrEventsStreamUnsupported) {
		t.Fatalf("volia fallback segur, error=%v", err)
	}
}

func TestEventsSessionsICancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("token absent")
		}
		switch r.URL.Path {
		case "/api/v2/events":
			if r.URL.Query().Get("after") != "4" || r.URL.Query().Get("session") != "feina" {
				t.Errorf("query: %s", r.URL.RawQuery)
			}
			_, _ = fmt.Fprint(w, `{"events":[{"id":5,"kind":"assistant","text":"fet"}],"cursor":4,"next":5}`)
		case "/api/sessions/live":
			_, _ = fmt.Fprint(w, `{"sessions":[{"id":"feina","busy":true}]}`)
		case "/api/agent/cancel":
			if r.Header.Get("X-Gregal-Session") != "feina" {
				t.Errorf("sessió absent")
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "secret", "inst")
	p, err := c.Events(context.Background(), 4, 10, "feina")
	if err != nil || len(p.Events) != 1 || p.Next != 5 {
		t.Fatalf("events: %+v %v", p, err)
	}
	sessions, err := c.Sessions(context.Background())
	if err != nil || len(sessions) != 1 || !sessions[0].Busy {
		t.Fatalf("sessions: %+v %v", sessions, err)
	}
	if err := c.Cancel(context.Background(), "feina"); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitRunRunICancelRun(t *testing.T) {
	var sessio, mode, clau, ws string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("token absent")
		}
		sessio = r.Header.Get("X-Gregal-Session")
		switch {
		case r.URL.Path == "/api/v2/runs" && r.Method == http.MethodPost:
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			mode, clau, ws = body["mode"], body["idempotency_key"], body["workspace"]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run":{"id":9,"state":"queued","session":"feina"}}`))
		case r.URL.Path == "/api/v2/runs/9" && r.Method == http.MethodGet:
			_, _ = fmt.Fprint(w, `{"run":{"id":9,"state":"running","started_at":"2026-09-20T10:00:00Z"}}`)
		case r.URL.Path == "/api/v2/runs/9/cancel" && r.Method == http.MethodPost:
			_, _ = fmt.Fprint(w, `{"ok":true,"cancelled":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "secret", "inst")
	ctx := context.Background()

	run, err := c.SubmitRun(ctx, "feina", "fes la feina", "code", "msg-1", `C:\proj`)
	if err != nil || run.ID != 9 || run.State != "queued" {
		t.Fatalf("submit: %+v %v", run, err)
	}
	if sessio != "feina" || mode != "code" || clau != "msg-1" || ws != `C:\proj` {
		t.Fatalf("capçalera o cos erroni: sessio=%q mode=%q clau=%q ws=%q", sessio, mode, clau, ws)
	}

	run, err = c.Run(ctx, 9)
	if err != nil || run.State != "running" {
		t.Fatalf("run: %+v %v", run, err)
	}
	if err := c.CancelRun(ctx, 9); err != nil {
		t.Fatal(err)
	}

	// Un encàrrec buit no s'ha d'enviar mai.
	if _, err := c.SubmitRun(ctx, "feina", "   ", "", "", ""); err == nil {
		t.Fatal("la tasca buida s'hauria de rebutjar al client")
	}
}

func TestHealthValidaProtocolIdentitatIToken(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","protocol":"gregal.v1","instance_id":"inst-1","capabilities":{"events":true}}`))
	}))
	defer srv.Close()
	if _, err := service.Publish(service.Descriptor{Protocol: Protocol, Instance: "inst-1", Address: srv.URL, Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	c, h, err := Discover(context.Background())
	if err != nil || c == nil || h.InstanceID != "inst-1" {
		t.Fatalf("discover: client=%+v health=%+v err=%v", c, h, err)
	}
}

func TestDiscoverRebutjaInstanciaCanviada(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","protocol":"gregal.v1","instance_id":"inst-nova"}`))
	}))
	defer srv.Close()
	if _, err := service.Publish(service.Descriptor{Protocol: Protocol, Instance: "inst-vella", Address: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Discover(context.Background()); err == nil {
		t.Fatal("cal rebutjar el canvi d'instància")
	}
}
