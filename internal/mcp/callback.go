package mcp

import (
	"context"
	"errors"
	"html"
	"net"
	"net/http"
	"time"
)

// HandleCallback atén la tornada del navegador des del servidor
// d'autorització (?code=…&state=…): acaba l'autorització i pinta una
// pàgina curta. El state (aleatori, d'un sol ús) és el que l'autentica:
// la ruta no porta el token de Gregal perquè ve d'una redirecció externa.
func (m *Manager) HandleCallback(w http.ResponseWriter, r *http.Request) (string, error) {
	q := r.URL.Query()
	var server string
	var err error
	if e := q.Get("error"); e != "" {
		// Es consumeix el state igualment perquè no quedi penjat.
		m.mu.Lock()
		if p := m.pending[q.Get("state")]; p != nil {
			server = p.Server
			delete(m.pending, q.Get("state"))
		}
		m.mu.Unlock()
		msg := e
		if d := q.Get("error_description"); d != "" {
			msg += ": " + d
		}
		err = errors.New(msg)
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		server, err = m.CompleteAuth(ctx, q.Get("state"), q.Get("code"))
		cancel()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	title, body := "Gregal: connector authorized", "You can close this window and go back to Gregal."
	if server != "" {
		title = "Gregal: " + server + " connected"
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		title, body = "Gregal: authorization failed", err.Error()
	}
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>` + html.EscapeString(title) +
		`</title><body style="font-family:system-ui,sans-serif;max-width:36rem;margin:4rem auto;line-height:1.5"><h1 style="font-size:1.3rem">` +
		html.EscapeString(title) + `</h1><p>` + html.EscapeString(body) + `</p><script>setTimeout(function(){try{window.close()}catch(e){}},1500)</script></body>`))
	return server, err
}

// Loopback és un servidor efímer al loopback per rebre el callback OAuth
// als clients que no tenen web (el TUI).
type Loopback struct {
	RedirectURI string
	// Done rep el resultat del primer callback.
	Done <-chan LoopbackResult
	srv  *http.Server
}

// LoopbackResult és el que torna el callback.
type LoopbackResult struct {
	Server string
	Err    error
}

// StartLoopback obre 127.0.0.1:<port lliure> i hi serveix OAuthCallbackPath.
func (m *Manager) StartLoopback() (*Loopback, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	done := make(chan LoopbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(OAuthCallbackPath, func(w http.ResponseWriter, r *http.Request) {
		server, err := m.HandleCallback(w, r)
		select {
		case done <- LoopbackResult{Server: server, Err: err}:
		default:
		}
	})
	lb := &Loopback{
		RedirectURI: "http://" + ln.Addr().String() + OAuthCallbackPath,
		Done:        done,
		srv:         &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second},
	}
	go lb.srv.Serve(ln)
	return lb, nil
}

// Close tanca el servidor del loopback.
func (l *Loopback) Close() {
	if l != nil && l.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = l.srv.Shutdown(ctx)
	}
}
