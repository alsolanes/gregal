// Package lsp és un client mínim del Language Server Protocol per a
// l'eina `lsp` de l'agent: anar a la definició, trobar referències, hover,
// diagnòstics d'un fitxer i rename segur entre fitxers.
//
// Gregal ja dona els errors després de cada edició amb les eines de línia
// d'ordres (go vet, tsc, pyright/mypy: agent/diag_*.go), que és el que
// opencode recomana ara per defecte (hi té l'LSP desactivat). El que no
// tenia és la navegació semàntica: grep troba el text, no on es fa servir
// una funció ni quina de tres `Run` és la que es crida, i un rename amb grep
// i edit toca comentaris, strings i homònims. Per això aquí només hi ha
// consultes i rename, amb els servidors que l'usuari ja té instal·lats
// (gopls, pyright, typescript-language-server, rust-analyzer, clangd): no
// es baixa res.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client és una connexió amb un servidor de llenguatge.
type Client struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	mu     sync.Mutex // escriptura
	nextID int64

	pmu     sync.Mutex
	pending map[int64]chan resposta
	closed  bool

	dmu   sync.Mutex
	diags map[string][]Diagnostic // URI → diagnòstics publicats
	dver  map[string]int          // URI → quantes publicacions
	dcond *sync.Cond

	done chan struct{}
}

type resposta struct {
	Result json.RawMessage
	Err    *rpcError
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("lsp: %s (%d)", e.Message, e.Code) }

type missatge struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Start engega el servidor i fa la salutació initialize/initialized.
func Start(ctx context.Context, command []string, root string, env []string) (*Client, error) {
	if len(command) == 0 {
		return nil, errors.New("lsp: no command")
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = root
	if len(env) > 0 {
		cmd.Env = env
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{cmd: cmd, in: in, pending: map[int64]chan resposta{}, diags: map[string][]Diagnostic{}, dver: map[string]int{}, done: make(chan struct{})}
	c.dcond = sync.NewCond(&c.dmu)
	go c.llegeix(bufio.NewReader(out))
	params := map[string]any{
		"processId": nil,
		"rootUri":   URI(root),
		"workspaceFolders": []map[string]any{
			{"uri": URI(root), "name": "workspace"},
		},
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"definition":         map[string]any{"linkSupport": true},
				"references":         map[string]any{},
				"hover":              map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
				"rename":             map[string]any{"prepareSupport": false},
				"publishDiagnostics": map[string]any{"relatedInformation": false},
				"synchronization":    map[string]any{"didSave": true},
			},
			"workspace": map[string]any{
				"workspaceEdit":    map[string]any{"documentChanges": true},
				"configuration":    true,
				"workspaceFolders": true,
			},
			"general": map[string]any{"positionEncodings": []string{"utf-16"}},
		},
	}
	if _, err := c.Call(ctx, "initialize", params); err != nil {
		c.Close()
		return nil, err
	}
	if err := c.Notify("initialized", map[string]any{}); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Call fa una petició i n'espera la resposta.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.pmu.Lock()
	if c.closed {
		c.pmu.Unlock()
		return nil, errors.New("lsp: server stopped")
	}
	c.nextID++
	id := c.nextID
	ch := make(chan resposta, 1)
	c.pending[id] = ch
	c.pmu.Unlock()
	if err := c.envia(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		if r.Err != nil {
			return nil, r.Err
		}
		return r.Result, nil
	case <-ctx.Done():
		c.pmu.Lock()
		delete(c.pending, id)
		c.pmu.Unlock()
		_ = c.Notify("$/cancelRequest", map[string]any{"id": id})
		return nil, ctx.Err()
	case <-c.done:
		return nil, errors.New("lsp: server stopped")
	}
}

// Notify envia una notificació.
func (c *Client) Notify(method string, params any) error {
	return c.envia(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *Client) envia(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n", len(raw)); err != nil {
		return err
	}
	_, err = c.in.Write(raw)
	return err
}

// llegeix rep missatges fins que el servidor tanca.
func (c *Client) llegeix(r *bufio.Reader) {
	defer func() {
		c.pmu.Lock()
		c.closed = true
		c.pmu.Unlock()
		close(c.done)
		c.dmu.Lock()
		c.dcond.Broadcast()
		c.dmu.Unlock()
	}()
	for {
		n := -1
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				return
			}
			l = strings.TrimRight(l, "\r\n")
			if l == "" {
				break
			}
			if k, v, ok := strings.Cut(l, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
				n, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		if n < 0 {
			continue
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return
		}
		var m missatge
		if json.Unmarshal(buf, &m) != nil {
			continue
		}
		switch {
		case m.Method != "" && len(m.ID) > 0:
			c.respon(m)
		case m.Method != "":
			c.notificacio(m)
		case len(m.ID) > 0:
			id, err := strconv.ParseInt(string(m.ID), 10, 64)
			if err != nil {
				continue
			}
			c.pmu.Lock()
			ch := c.pending[id]
			delete(c.pending, id)
			c.pmu.Unlock()
			if ch != nil {
				ch <- resposta{Result: m.Result, Err: m.Error}
			}
		}
	}
}

// respon les peticions del servidor que un client mínim ha de contestar.
func (c *Client) respon(m missatge) {
	var result any
	switch m.Method {
	case "workspace/configuration":
		var p struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(m.Params, &p)
		res := make([]any, len(p.Items))
		result = res
	case "workspace/workspaceFolders":
		result = []any{}
	default: // window/workDoneProgress/create, client/registerCapability…
		result = nil
	}
	_ = c.envia(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": result})
}

func (c *Client) notificacio(m missatge) {
	if m.Method != "textDocument/publishDiagnostics" {
		return
	}
	var p struct {
		URI         string       `json:"uri"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if json.Unmarshal(m.Params, &p) != nil {
		return
	}
	k := normURI(p.URI)
	c.dmu.Lock()
	c.diags[k] = p.Diagnostics
	c.dver[k]++
	c.dcond.Broadcast()
	c.dmu.Unlock()
}

// diagVersio és quantes publicacions de diagnòstics ha rebut un fitxer.
func (c *Client) diagVersio(uri string) int {
	c.dmu.Lock()
	defer c.dmu.Unlock()
	return c.dver[normURI(uri)]
}

// EsperaDiagnostics espera una publicació posterior a `despres` (o fins
// al límit) i torna els diagnòstics del fitxer.
func (c *Client) EsperaDiagnostics(uri string, despres int, limit time.Duration) ([]Diagnostic, bool) {
	k := normURI(uri)
	fi := time.Now().Add(limit)
	t := time.AfterFunc(limit, func() {
		c.dmu.Lock()
		c.dcond.Broadcast()
		c.dmu.Unlock()
	})
	defer t.Stop()
	c.dmu.Lock()
	defer c.dmu.Unlock()
	for c.dver[k] <= despres && time.Now().Before(fi) {
		select {
		case <-c.done:
			return c.diags[k], false
		default:
		}
		c.dcond.Wait()
	}
	return c.diags[k], c.dver[k] > despres
}

// Close atura el servidor (shutdown/exit, i si no respon, el mata).
func (c *Client) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = c.Call(ctx, "shutdown", nil)
	_ = c.Notify("exit", nil)
	_ = c.in.Close()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
	}
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
}
