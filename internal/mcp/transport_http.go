package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// HTTPDoer és el que cal d'un *http.Client (els tests en posen un de propi).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// TokenSource dona el bearer d'un servidor remot. Token torna "" si encara
// no n'hi ha cap; Refresh intenta renovar-lo després d'un 401.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Refresh(ctx context.Context) bool
}

func defaultHTTP() *http.Client {
	// Sense Timeout global: l'stream SSE antic és llarg per naturalesa; cada
	// crida porta el seu context amb termini.
	return &http.Client{}
}

// expandHeaders expandeix ${VAR} als valors (el config queda sense secrets).
func expandHeaders(h map[string]string) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = os.ExpandEnv(v)
	}
	return out
}

// httpStatusError és una resposta HTTP d'error.
type httpStatusError struct {
	Code int
	Body string
}

func (e *httpStatusError) Error() string {
	b := strings.TrimSpace(e.Body)
	if len(b) > 300 {
		b = b[:300] + "…"
	}
	if b == "" {
		return fmt.Sprintf("HTTP %d", e.Code)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Code, b)
}

// authErrorFrom llegeix el WWW-Authenticate d'un 401.
func authErrorFrom(resp *http.Response) *AuthRequiredError {
	e := &AuthRequiredError{}
	for _, h := range resp.Header.Values("WWW-Authenticate") {
		params := parseAuthParams(h)
		if v := params["resource_metadata"]; v != "" {
			e.ResourceMetadata = v
		}
		if v := params["scope"]; v != "" {
			e.Scope = v
		}
		if v := params["error_description"]; v != "" {
			e.Detail = v
		}
	}
	return e
}

// parseAuthParams treu els paràmetres clau="valor" d'un WWW-Authenticate.
func parseAuthParams(h string) map[string]string {
	out := map[string]string{}
	s := strings.TrimSpace(h)
	if i := strings.IndexByte(s, ' '); i > 0 && !strings.Contains(s[:i], "=") {
		s = s[i+1:]
	}
	for len(s) > 0 {
		s = strings.TrimLeft(s, " ,")
		eq := strings.IndexByte(s, '=')
		if eq <= 0 {
			break
		}
		key := strings.ToLower(strings.TrimSpace(s[:eq]))
		s = s[eq+1:]
		var val string
		if strings.HasPrefix(s, `"`) {
			end := 1
			for end < len(s) && s[end] != '"' {
				if s[end] == '\\' {
					end++
				}
				end++
			}
			if end > len(s) {
				end = len(s)
			}
			val = strings.ReplaceAll(s[1:min(end, len(s))], `\"`, `"`)
			s = s[min(end+1, len(s)):]
		} else {
			end := strings.IndexByte(s, ',')
			if end < 0 {
				end = len(s)
			}
			val = strings.TrimSpace(s[:end])
			s = s[end:]
		}
		out[key] = val
	}
	return out
}

// --- Streamable HTTP (spec 2025-03-26 / 2025-06-18) ---

type httpTransport struct {
	name    string
	url     string
	hc      HTTPDoer
	headers map[string]string
	tokens  TokenSource

	mu        sync.Mutex
	sessionID string
	protocol  string
}

func (t *httpTransport) setProtocol(v string) {
	t.mu.Lock()
	t.protocol = v
	t.mu.Unlock()
}

func (t *httpTransport) newRequest(ctx context.Context, method string, body []byte) (*http.Request, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.url, rd)
	if err != nil {
		return nil, err
	}
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	t.mu.Lock()
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	if t.protocol != "" {
		req.Header.Set("MCP-Protocol-Version", t.protocol)
	}
	t.mu.Unlock()
	if t.tokens != nil {
		tok, err := t.tokens.Token(ctx)
		if err != nil {
			return nil, err
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	return req, nil
}

// send fa el POST d'un missatge. Amb wantID torna el resultat de la
// resposta amb aquest id (JSON directe o dins d'un stream SSE).
func (t *httpTransport) send(ctx context.Context, msg any, wantID int64, expect bool) (json.RawMessage, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		req, err := t.newRequest(ctx, http.MethodPost, body)
		if err != nil {
			return nil, err
		}
		resp, err := t.hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("mcp %s: %w", t.name, err)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			ae := authErrorFrom(resp)
			drain(resp)
			if attempt == 0 && t.tokens != nil && t.tokens.Refresh(ctx) {
				continue
			}
			return nil, ae
		}
		if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
			t.mu.Lock()
			t.sessionID = sid
			t.mu.Unlock()
		}
		if resp.StatusCode >= 400 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, &httpStatusError{Code: resp.StatusCode, Body: string(raw)}
		}
		defer resp.Body.Close()
		if !expect {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			return nil, nil
		}
		ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if ct == "text/event-stream" {
			return t.readStream(ctx, resp.Body, wantID)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		if err != nil {
			return nil, err
		}
		return pickResponse(t.name, raw, wantID)
	}
}

// pickResponse treu la resposta amb l'id d'un cos JSON (objecte o lot).
func pickResponse(name string, raw []byte, id int64) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	var msgs []rpcMsg
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &msgs); err != nil {
			return nil, err
		}
	} else {
		var m rpcMsg
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("mcp %s: unreadable response: %w", name, err)
		}
		msgs = []rpcMsg{m}
	}
	for _, m := range msgs {
		if m.esResposta() && idIgual(m.ID, id) {
			if m.Error != nil {
				return nil, fmt.Errorf("mcp %s: %s", name, m.Error.Message)
			}
			return m.Result, nil
		}
	}
	return nil, fmt.Errorf("mcp %s: response without id %d", name, id)
}

func (t *httpTransport) readStream(ctx context.Context, body io.Reader, id int64) (json.RawMessage, error) {
	var result json.RawMessage
	var found error = errors.New("mcp " + t.name + ": stream ended without a response")
	done := false
	err := readSSE(ctx, body, func(ev sseEvent) bool {
		if ev.Event != "" && ev.Event != "message" {
			return true
		}
		var m rpcMsg
		if json.Unmarshal([]byte(ev.Data), &m) != nil {
			return true
		}
		if m.esResposta() && idIgual(m.ID, id) {
			done = true
			if m.Error != nil {
				found = fmt.Errorf("mcp %s: %s", t.name, m.Error.Message)
			} else {
				result, found = m.Result, nil
			}
			return false
		}
		if m.Method == "ping" && len(m.ID) > 0 {
			go t.reply(m.ID, map[string]any{})
		}
		return true
	})
	if !done && err != nil {
		return nil, err
	}
	return result, found
}

// reply respon una petició del servidor (només ping).
func (t *httpTransport) reply(id json.RawMessage, result any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = t.send(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "result": result}, 0, false)
}

func (t *httpTransport) request(ctx context.Context, id int64, method string, params any) (json.RawMessage, error) {
	return t.send(ctx, rpcReq{JSONRPC: "2.0", ID: id, Method: method, Params: params}, id, true)
}

func (t *httpTransport) notify(ctx context.Context, method string, params any) error {
	_, err := t.send(ctx, rpcNotif{JSONRPC: "2.0", Method: method, Params: params}, 0, false)
	return err
}

// close acaba la sessió al servidor (DELETE amb l'id), si n'hi ha.
func (t *httpTransport) close() {
	t.mu.Lock()
	sid := t.sessionID
	t.mu.Unlock()
	if sid == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := t.newRequest(ctx, http.MethodDelete, nil)
	if err != nil {
		return
	}
	if resp, err := t.hc.Do(req); err == nil {
		drain(resp)
	}
}

func drain(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	resp.Body.Close()
}

// --- SSE ---

type sseEvent struct {
	Event string
	Data  string
	ID    string
}

// readSSE llegeix events fins que fn torna fals, l'stream s'acaba o el
// context mor.
func readSSE(ctx context.Context, body io.Reader, fn func(sseEvent) bool) error {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	var ev sseEvent
	var data []string
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			if len(data) > 0 || ev.Event != "" {
				ev.Data = strings.Join(data, "\n")
				if !fn(ev) {
					return nil
				}
			}
			ev, data = sseEvent{}, nil
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, val, _ := strings.Cut(line, ":")
		val = strings.TrimPrefix(val, " ")
		switch field {
		case "event":
			ev.Event = val
		case "data":
			data = append(data, val)
		case "id":
			ev.ID = val
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(data) > 0 {
		ev.Data = strings.Join(data, "\n")
		fn(ev)
	}
	return io.EOF
}

// --- SSE antic (spec 2024-11-05): GET d'un stream + POST a l'endpoint ---

type sseTransport struct {
	name     string
	hc       HTTPDoer
	headers  map[string]string
	tokens   TokenSource
	endpoint string
	cancel   context.CancelFunc

	mu      sync.Mutex
	pending map[int64]chan rpcMsg
	done    chan struct{}
	err     error
	once    sync.Once
}

func dialSSE(ctx context.Context, name, rawURL string, hc HTTPDoer, headers map[string]string, tokens TokenSource) (*sseTransport, error) {
	base, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	sctx, cancel := context.WithCancel(context.Background())
	t := &sseTransport{name: name, hc: hc, headers: headers, tokens: tokens, cancel: cancel, pending: map[int64]chan rpcMsg{}, done: make(chan struct{})}
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(sctx, http.MethodGet, rawURL, nil)
		if err != nil {
			cancel()
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Accept", "text/event-stream")
		if tokens != nil {
			tok, err := tokens.Token(ctx)
			if err != nil {
				cancel()
				return nil, err
			}
			if tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
		}
		resp, err = hc.Do(req)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("mcp %s: %w", name, err)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			ae := authErrorFrom(resp)
			drain(resp)
			if attempt == 0 && tokens != nil && tokens.Refresh(ctx) {
				continue
			}
			cancel()
			return nil, ae
		}
		break
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		cancel()
		return nil, &httpStatusError{Code: resp.StatusCode, Body: string(raw)}
	}
	endpoint := make(chan string, 1)
	go func() {
		defer resp.Body.Close()
		err := readSSE(sctx, resp.Body, func(ev sseEvent) bool {
			switch ev.Event {
			case "endpoint":
				u, err := base.Parse(strings.TrimSpace(ev.Data))
				if err == nil {
					select {
					case endpoint <- u.String():
					default:
					}
				}
			case "", "message":
				var m rpcMsg
				if json.Unmarshal([]byte(ev.Data), &m) == nil && m.esResposta() {
					t.dispatch(m)
				}
			}
			return true
		})
		if err == nil || errors.Is(err, io.EOF) {
			err = fmt.Errorf("mcp %s: the SSE stream closed", name)
		}
		t.fail(err)
	}()
	select {
	case ep := <-endpoint:
		t.endpoint = ep
		return t, nil
	case <-t.done:
		cancel()
		return nil, t.err
	case <-ctx.Done():
		cancel()
		return nil, fmt.Errorf("mcp %s: no endpoint event from the SSE server: %w", name, ctx.Err())
	}
}

func (t *sseTransport) dispatch(m rpcMsg) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, ch := range t.pending {
		if idIgual(m.ID, id) {
			ch <- m
			delete(t.pending, id)
			return
		}
	}
}

func (t *sseTransport) fail(err error) {
	t.once.Do(func() {
		t.mu.Lock()
		t.err = err
		t.mu.Unlock()
		close(t.done)
	})
}

func (t *sseTransport) post(ctx context.Context, msg any) (*http.Response, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.tokens != nil {
		if tok, _ := t.tokens.Token(ctx); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	resp, err := t.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp %s: %w", t.name, err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		ae := authErrorFrom(resp)
		drain(resp)
		return nil, ae
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, &httpStatusError{Code: resp.StatusCode, Body: string(raw)}
	}
	return resp, nil
}

func (t *sseTransport) request(ctx context.Context, id int64, method string, params any) (json.RawMessage, error) {
	ch := make(chan rpcMsg, 1)
	t.mu.Lock()
	if t.err != nil {
		err := t.err
		t.mu.Unlock()
		return nil, err
	}
	t.pending[id] = ch
	t.mu.Unlock()
	drop := func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}
	resp, err := t.post(ctx, rpcReq{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		drop()
		return nil, err
	}
	// Alguns servidors responen al mateix POST en comptes de per l'stream.
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	resp.Body.Close()
	if r := bytes.TrimSpace(raw); len(r) > 0 && (r[0] == '{' || r[0] == '[') {
		if res, err := pickResponse(t.name, r, id); err == nil {
			drop()
			return res, nil
		}
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return nil, fmt.Errorf("mcp %s: %s", t.name, m.Error.Message)
		}
		return m.Result, nil
	case <-t.done:
		drop()
		return nil, t.err
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

func (t *sseTransport) notify(ctx context.Context, method string, params any) error {
	resp, err := t.post(ctx, rpcNotif{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	drain(resp)
	return nil
}

func (t *sseTransport) close() {
	if t.cancel != nil {
		t.cancel()
	}
	t.fail(fmt.Errorf("mcp %s: closed", t.name))
}
