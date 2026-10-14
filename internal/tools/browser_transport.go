package tools

// Transports CDP de l'eina browser. Les accions (navegar, llegir, clicar,
// escriure, capturar) només saben enviar una ordre CDP i esperar-ne el
// resultat; com arriba al navegador ho decideix el transport:
//
//   - wsTransport: websocket a la pestanya d'un Chrome/Edge engegat amb
//     --remote-debugging-port (headless o finestra).
//   - appTransport: pont HTTP que serveix el procés principal d'Electron
//     (browser_app.go) i que reenvia les ordres a webContents.debugger del
//     panell de navegador de l'app.
//
// Els esdeveniments CDP no es fan servir (les accions consulten l'estat
// amb Runtime.evaluate), així que el contracte és només petició/resposta.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/net/websocket"
)

// cdpTransport és el canal per on viatgen les ordres CDP.
type cdpTransport interface {
	// Call envia l'ordre i n'espera el resultat; s'atura amb el context.
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	// Alive diu si el canal encara serveix.
	Alive() bool
	// Close tanca el canal (i allibera el navegador si cal).
	Close()
}

type cdpMsg struct {
	ID     int             `json:"id"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// wsTransport parla CDP per websocket amb una pestanya de Chrome.
type wsTransport struct {
	mu      sync.Mutex
	ws      *websocket.Conn
	nextID  int
	pending map[int]chan cdpMsg
	dead    chan struct{}
}

func dialWSTransport(wsURL, origin string) (*wsTransport, error) {
	conn, err := websocket.Dial(wsURL, "", origin)
	if err != nil {
		return nil, fmt.Errorf("browser: websocket: %v", err)
	}
	t := &wsTransport{ws: conn, pending: map[int]chan cdpMsg{}, dead: make(chan struct{})}
	go t.llegeix()
	return t, nil
}

// llegeix reparteix les respostes per id; els esdeveniments s'ignoren.
func (t *wsTransport) llegeix() {
	defer close(t.dead)
	for {
		var raw []byte
		if err := websocket.Message.Receive(t.ws, &raw); err != nil {
			t.mu.Lock()
			for id, ch := range t.pending {
				close(ch)
				delete(t.pending, id)
			}
			t.mu.Unlock()
			return
		}
		var m cdpMsg
		if json.Unmarshal(raw, &m) != nil || m.ID == 0 {
			continue
		}
		t.mu.Lock()
		ch, ok := t.pending[m.ID]
		if ok {
			delete(t.pending, m.ID)
		}
		t.mu.Unlock()
		if ok {
			ch <- m
		}
	}
}

func (t *wsTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	t.nextID++
	id := t.nextID
	ch := make(chan cdpMsg, 1)
	t.pending[id] = ch
	t.mu.Unlock()
	forget := func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}
	req := map[string]any{"id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	raw, _ := json.Marshal(req)
	if err := websocket.Message.Send(t.ws, string(raw)); err != nil {
		forget()
		return nil, fmt.Errorf("browser: envia %s: %v", method, err)
	}
	select {
	case m, ok := <-ch:
		forget()
		if !ok {
			return nil, fmt.Errorf("browser: connexió tancada")
		}
		if m.Error != nil {
			return nil, fmt.Errorf("browser: %s: %s", method, m.Error.Message)
		}
		return m.Result, nil
	case <-ctx.Done():
		forget()
		return nil, ctx.Err()
	}
}

func (t *wsTransport) Alive() bool {
	select {
	case <-t.dead:
		return false
	default:
		return true
	}
}

func (t *wsTransport) Close() { t.ws.Close() }
