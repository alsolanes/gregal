package tools

// El Chrome de l'usuari, a través de l'extensió de Gregal (extension/chrome).
//
// L'extensió obre un websocket cap al backend (/api/browser/extension, només
// des d'un origen chrome-extension://) quan l'usuari activa «Deixa que
// Gregal faci servir aquest navegador». Per aquest canal el backend li
// demana obrir pestanyes al grup «Gregal», enganxar-hi el depurador
// (chrome.debugger) i enviar-hi ordres CDP. Les accions de l'agent són les
// mateixes que amb el Chrome propi: només canvia el transport.
//
// Protocol (JSON per missatge):
//
//	extensió → backend  {"type":"hello","key":"…","version":"…"}   (primer)
//	backend → extensió  {"id":N,"op":"open|cdp|tabs|select|close|detach", …}
//	extensió → backend  {"id":N,"result":…} o {"id":N,"error":"…"}
//	extensió → backend  {"event":"detached","tab":T,"reason":"…"} · {"event":"stop"} · {"event":"ping"}
//
// Si l'usuari cancel·la el depurador (la barra de Chrome) o prem Atura al
// panell, l'agent no s'hi torna a enganxar sol: cal que l'usuari ho torni a
// activar.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type extReply struct {
	Result json.RawMessage
	Err    string
}

type extHub struct {
	mu       sync.Mutex
	wmu      sync.Mutex
	conn     *websocket.Conn
	gen      int
	nextID   int
	pending  map[int]chan extReply
	detached map[int]bool
	stopped  bool
	version  string
	since    time.Time
}

var browserExt = &extHub{pending: map[int]chan extReply{}, detached: map[int]bool{}}

// BrowserExtensionStatus diu si el Chrome de l'usuari està connectat.
func BrowserExtensionStatus() map[string]any {
	h := browserExt
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]any{"connected": h.conn != nil, "stopped": h.stopped}
	if h.conn != nil {
		out["version"] = h.version
		out["since"] = h.since.Format(time.RFC3339)
	}
	return out
}

// ServeBrowserExtension atén la connexió de l'extensió fins que es tanca.
// auth valida la clau del hello (sempre true en mode local sense token).
func ServeBrowserExtension(ws *websocket.Conn, auth func(key string) bool) {
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	var hello struct {
		Type    string `json:"type"`
		Key     string `json:"key"`
		Version string `json:"version"`
	}
	if err := websocket.JSON.Receive(ws, &hello); err != nil || hello.Type != "hello" {
		return
	}
	if auth != nil && !auth(hello.Key) {
		_ = websocket.JSON.Send(ws, map[string]string{"type": "error", "error": "unauthorized"})
		return
	}
	_ = ws.SetReadDeadline(time.Time{})
	h := browserExt
	h.mu.Lock()
	if h.conn != nil {
		h.conn.Close()
		h.failPendingLocked("the extension reconnected")
	}
	h.conn, h.version, h.since = ws, hello.Version, time.Now()
	h.gen++
	gen := h.gen
	h.stopped = false
	h.detached = map[int]bool{}
	h.mu.Unlock()
	_ = websocket.JSON.Send(ws, map[string]string{"type": "ready"})
	for {
		_ = ws.SetReadDeadline(time.Now().Add(90 * time.Second))
		var m struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  string          `json:"error"`
			Event  string          `json:"event"`
			Tab    int             `json:"tab"`
			Reason string          `json:"reason"`
		}
		if err := websocket.JSON.Receive(ws, &m); err != nil {
			break
		}
		h.mu.Lock()
		switch {
		case m.ID != 0:
			if ch, ok := h.pending[m.ID]; ok {
				delete(h.pending, m.ID)
				ch <- extReply{Result: m.Result, Err: m.Error}
			}
		case m.Event == "detached":
			h.detached[m.Tab] = true
			if m.Reason == "canceled_by_user" {
				h.stopped = true
			}
		case m.Event == "stop":
			h.stopped = true
		}
		h.mu.Unlock()
	}
	h.mu.Lock()
	if h.gen == gen {
		h.conn = nil
		h.failPendingLocked("the extension disconnected")
	}
	h.mu.Unlock()
}

func (h *extHub) failPendingLocked(why string) {
	for id, ch := range h.pending {
		ch <- extReply{Err: why}
		delete(h.pending, id)
	}
}

func (h *extHub) call(ctx context.Context, op string, payload map[string]any) (json.RawMessage, error) {
	h.mu.Lock()
	conn := h.conn
	if conn == nil {
		h.mu.Unlock()
		return nil, errExtMissing
	}
	h.nextID++
	id := h.nextID
	ch := make(chan extReply, 1)
	h.pending[id] = ch
	h.mu.Unlock()
	msg := map[string]any{"id": id, "op": op}
	for k, v := range payload {
		msg[k] = v
	}
	h.wmu.Lock()
	err := websocket.JSON.Send(conn, msg)
	h.wmu.Unlock()
	if err != nil {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
		return nil, fmt.Errorf("browser: Chrome: %v", err)
	}
	select {
	case r := <-ch:
		if r.Err != "" {
			return nil, fmt.Errorf("browser: Chrome: %s", r.Err)
		}
		return r.Result, nil
	case <-ctx.Done():
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
		return nil, ctx.Err()
	}
}

var errExtMissing = fmt.Errorf("browser: your Chrome is not connected. Ask the user to open the Gregal extension's side panel in Chrome and turn on «Let Gregal use this browser» (the desktop app must be running); or work without chrome:true in Gregal's own browser")

var errExtStopped = fmt.Errorf("browser: the user stopped Gregal in their Chrome. Do not retry: ask the user whether to continue, and they will turn it back on from the extension")

func (h *extHub) transport(tab int) *extTransport {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.detached, tab)
	return &extTransport{hub: h, tab: tab, gen: h.gen}
}

func (h *extHub) open(ctx context.Context, u string) (*extTransport, int, error) {
	h.mu.Lock()
	stopped := h.stopped
	h.mu.Unlock()
	if stopped {
		return nil, 0, errExtStopped
	}
	raw, err := h.call(ctx, "open", map[string]any{"url": u})
	if err != nil {
		return nil, 0, err
	}
	var r struct {
		Tab int `json:"tab"`
	}
	if json.Unmarshal(raw, &r) != nil || r.Tab == 0 {
		return nil, 0, fmt.Errorf("browser: Chrome did not open the tab")
	}
	return h.transport(r.Tab), r.Tab, nil
}

func (h *extHub) tabs(ctx context.Context) ([]tabInfo, error) {
	raw, err := h.call(ctx, "tabs", nil)
	if err != nil {
		return nil, err
	}
	var list []struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	out := make([]tabInfo, 0, len(list))
	for _, t := range list {
		out = append(out, tabInfo{ID: strconv.Itoa(t.ID), Title: t.Title, URL: t.URL, Type: "page"})
	}
	return out, nil
}

func (h *extHub) selectTab(ctx context.Context, tab int) (*extTransport, error) {
	if _, err := h.call(ctx, "select", map[string]any{"tab": tab}); err != nil {
		return nil, err
	}
	return h.transport(tab), nil
}

func (h *extHub) closeTab(ctx context.Context, tab int) error {
	_, err := h.call(ctx, "close", map[string]any{"tab": tab})
	return err
}

// extTransport és el canal CDP amb una pestanya del Chrome de l'usuari.
type extTransport struct {
	hub *extHub
	tab int
	gen int
}

func (t *extTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if params == nil {
		params = map[string]any{}
	}
	return t.hub.call(ctx, "cdp", map[string]any{"tab": t.tab, "method": method, "params": params})
}

func (t *extTransport) Alive() bool {
	t.hub.mu.Lock()
	defer t.hub.mu.Unlock()
	return t.hub.conn != nil && t.hub.gen == t.gen && !t.hub.detached[t.tab] && !t.hub.stopped
}

// Close deixa anar la pestanya (el depurador); la pestanya queda oberta
// perquè l'usuari vegi on ha quedat la feina.
func (t *extTransport) Close() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = t.hub.call(ctx, "detach", map[string]any{"tab": t.tab})
	}()
}

// openExtBrowser obre una pestanya al grup «Gregal» del Chrome de l'usuari.
func openExtBrowser() (*Browser, error) {
	h := browserExt
	h.mu.Lock()
	connected, stopped := h.conn != nil, h.stopped
	h.mu.Unlock()
	if !connected {
		return nil, errExtMissing
	}
	if stopped {
		return nil, errExtStopped
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t, tab, err := h.open(ctx, "about:blank")
	if err != nil {
		return nil, err
	}
	b := &Browser{refs: map[int]string{}, mode: BrowserModeChrome, t: t, tabID: strconv.Itoa(tab)}
	if err := b.prepara(); err != nil {
		t.Close()
		return nil, err
	}
	return b, nil
}

// ctx és el context del pas en marxa (o un de fons).
func (b *Browser) ctx() context.Context {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.actx != nil {
		return b.actx
	}
	return context.Background()
}
