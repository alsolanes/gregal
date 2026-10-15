package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRemote és un servidor MCP remot de prova: Streamable HTTP, SSE antic
// opcional i OAuth 2.1 complet (metadades, registre dinàmic, PKCE, refresc).
type fakeRemote struct {
	t          *testing.T
	srv        *httptest.Server
	mu         sync.Mutex
	auth       bool   // demana bearer
	valid      string // token vàlid ara mateix
	sseReply   bool   // tools/call respon per SSE
	legacy     bool   // només SSE antic (el POST a /mcp torna 405)
	challenge  string
	issued     int
	refreshed  int
	registered int
	sessions   int
	lastHeader http.Header
	sseOut     chan string
}

func newFakeRemote(t *testing.T) *fakeRemote {
	f := &fakeRemote{t: t, valid: "tok-1", sseOut: make(chan string, 16)}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", f.handleMCP)
	mux.HandleFunc("/sse", f.handleSSE)
	mux.HandleFunc("/messages", f.handleMessages)
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"resource": f.srv.URL + "/mcp", "authorization_servers": []string{f.srv.URL}, "scopes_supported": []string{"mcp.read"}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer": f.srv.URL, "authorization_endpoint": f.srv.URL + "/authorize", "token_endpoint": f.srv.URL + "/token",
			"registration_endpoint": f.srv.URL + "/register", "code_challenge_methods_supported": []string{"S256"},
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["token_endpoint_auth_method"] != "none" || len(body["redirect_uris"].([]any)) != 1 {
			http.Error(w, "bad registration", 400)
			return
		}
		f.mu.Lock()
		f.registered++
		f.mu.Unlock()
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"client_id": "cid"})
	})
	mux.HandleFunc("/token", f.handleToken)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRemote) url() string { return f.srv.URL + "/mcp" }

func (f *fakeRemote) authorized(w http.ResponseWriter, r *http.Request) bool {
	f.mu.Lock()
	need, valid := f.auth, f.valid
	f.mu.Unlock()
	if !need || r.Header.Get("Authorization") == "Bearer "+valid {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+f.srv.URL+`/.well-known/oauth-protected-resource/mcp", scope="mcp.read"`)
	http.Error(w, "unauthorized", 401)
	return false
}

// answer implementa els mètodes MCP.
func (f *fakeRemote) answer(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return map[string]any{"protocolVersion": ProtocolVersion, "serverInfo": map[string]any{"name": "fake", "version": "1"},
			"capabilities": map[string]any{"tools": map[string]any{}, "resources": map[string]any{}, "prompts": map[string]any{}}}, nil
	case "tools/list":
		return map[string]any{"tools": []any{map[string]any{"name": "eco", "description": "echo", "inputSchema": map[string]any{"type": "object"}}}}, nil
	case "tools/call":
		var p struct {
			Arguments map[string]any `json:"arguments"`
		}
		json.Unmarshal(params, &p)
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": fmt.Sprintf("ECO:%v", p.Arguments["text"])}}}, nil
	case "resources/list":
		return map[string]any{"resources": []any{map[string]any{"uri": "file:///notes.md", "name": "notes"}}}, nil
	case "resources/read":
		return map[string]any{"contents": []any{map[string]any{"uri": "file:///notes.md", "text": "# Notes"}}}, nil
	case "prompts/list":
		return map[string]any{"prompts": []any{map[string]any{"name": "review", "description": "Review code", "arguments": []any{map[string]any{"name": "file", "required": true}, map[string]any{"name": "focus"}}}}}, nil
	case "prompts/get":
		var p struct {
			Arguments map[string]string `json:"arguments"`
		}
		json.Unmarshal(params, &p)
		return map[string]any{"messages": []any{map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": "Review " + p.Arguments["file"] + " for " + p.Arguments["focus"]}}}}, nil
	}
	return nil, &rpcError{Code: -32601, Message: "nope"}
}

func (f *fakeRemote) handleMCP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	legacy := f.legacy
	f.lastHeader = r.Header.Clone()
	f.mu.Unlock()
	if legacy {
		http.Error(w, "method not allowed", 405)
		return
	}
	if r.Method == http.MethodDelete {
		w.WriteHeader(204)
		return
	}
	if !f.authorized(w, r) {
		return
	}
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	json.NewDecoder(r.Body).Decode(&msg)
	if len(msg.ID) == 0 {
		w.WriteHeader(202)
		return
	}
	if msg.Method != "initialize" && r.Header.Get("Mcp-Session-Id") != "sess-1" {
		http.Error(w, "missing session", 400)
		return
	}
	res, rerr := f.answer(msg.Method, msg.Params)
	resp := map[string]any{"jsonrpc": "2.0", "id": msg.ID}
	if rerr != nil {
		resp["error"] = rerr
	} else {
		resp["result"] = res
	}
	if msg.Method == "initialize" {
		f.mu.Lock()
		f.sessions++
		f.mu.Unlock()
		w.Header().Set("Mcp-Session-Id", "sess-1")
	}
	f.mu.Lock()
	sse := f.sseReply && msg.Method == "tools/call"
	f.mu.Unlock()
	if sse {
		w.Header().Set("Content-Type", "text/event-stream")
		b, _ := json.Marshal(resp)
		fmt.Fprintf(w, ": comment\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (f *fakeRemote) handleSSE(w http.ResponseWriter, r *http.Request) {
	if !f.authorized(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	fl := w.(http.Flusher)
	fmt.Fprintf(w, "event: endpoint\ndata: /messages?sid=1\n\n")
	fl.Flush()
	for {
		select {
		case m := <-f.sseOut:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", m)
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (f *fakeRemote) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("sid") != "1" {
		http.Error(w, "bad sid", 400)
		return
	}
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	json.NewDecoder(r.Body).Decode(&msg)
	w.WriteHeader(202)
	if len(msg.ID) == 0 {
		return
	}
	res, rerr := f.answer(msg.Method, msg.Params)
	resp := map[string]any{"jsonrpc": "2.0", "id": msg.ID}
	if rerr != nil {
		resp["error"] = rerr
	} else {
		resp["result"] = res
	}
	b, _ := json.Marshal(resp)
	f.sseOut <- string(b)
}

func (f *fakeRemote) handleToken(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge || r.Form.Get("code") != "the-code" || r.Form.Get("client_id") != "cid" || r.Form.Get("resource") != f.srv.URL+"/mcp" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
			return
		}
		f.issued++
		json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-1", "refresh_token": "ref-1", "token_type": "Bearer", "expires_in": 3600})
	case "refresh_token":
		if r.Form.Get("refresh_token") != "ref-1" {
			w.WriteHeader(400)
			return
		}
		f.refreshed++
		f.valid = "tok-2"
		json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-2", "token_type": "Bearer", "expires_in": 3600})
	default:
		w.WriteHeader(400)
	}
}

func TestStreamableHTTP(t *testing.T) {
	f := newFakeRemote(t)
	c, err := DialCtx(context.Background(), "remot", Server{URL: f.url()}, DialOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !c.Caps.Tools || !c.Caps.Resources || !c.Caps.Prompts {
		t.Fatalf("caps=%+v", c.Caps)
	}
	tools, err := c.Tools()
	if err != nil || len(tools) != 1 || tools[0].Spec.Name != "mcp_remot_eco" {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
	out, err := tools[0].Call(`{"text":"hola"}`)
	if err != nil || out != "ECO:hola" {
		t.Fatalf("call=%q err=%v", out, err)
	}
	f.mu.Lock()
	h := f.lastHeader
	f.mu.Unlock()
	if h.Get("Mcp-Session-Id") != "sess-1" || h.Get("MCP-Protocol-Version") != ProtocolVersion {
		t.Fatalf("capçaleres de sessió: %v", h)
	}
}

func TestStreamableHTTPRespostaSSE(t *testing.T) {
	f := newFakeRemote(t)
	f.sseReply = true
	c, err := DialCtx(context.Background(), "remot", Server{URL: f.url()}, DialOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out, err := c.CallTool(context.Background(), "eco", `{"text":"stream"}`)
	if err != nil || out != "ECO:stream" {
		t.Fatalf("call=%q err=%v", out, err)
	}
}

func TestCapcaleresIExpansio(t *testing.T) {
	f := newFakeRemote(t)
	t.Setenv("GREGAL_TEST_TEAM", "blau")
	c, err := DialCtx(context.Background(), "remot", Server{URL: f.url(), Headers: map[string]string{"X-Team": "${GREGAL_TEST_TEAM}"}}, DialOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	f.mu.Lock()
	got := f.lastHeader.Get("X-Team")
	f.mu.Unlock()
	if got != "blau" {
		t.Fatalf("capçalera expandida: %q", got)
	}
}

func TestSSEAnticDeReserva(t *testing.T) {
	f := newFakeRemote(t)
	f.legacy = true
	// Transport "" amb una URL que només sap SSE: el POST torna 405 i es
	// prova el GET de l'stream a la mateixa URL... aquí, a /sse.
	c, err := DialCtx(context.Background(), "antic", Server{URL: f.srv.URL + "/sse", Transport: "sse"}, DialOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out, err := c.CallTool(context.Background(), "eco", `{"text":"vell"}`)
	if err != nil || out != "ECO:vell" {
		t.Fatalf("call=%q err=%v", out, err)
	}
}

func TestSSEAnticAutodetectat(t *testing.T) {
	f := newFakeRemote(t)
	// Un servidor antic a una sola URL: POST → 405, GET → stream.
	mux := http.NewServeMux()
	mux.HandleFunc("/only", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			http.Error(w, "no", 405)
			return
		}
		f.handleSSE(w, r)
	})
	mux.HandleFunc("/messages", f.handleMessages)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := DialCtx(context.Background(), "antic", Server{URL: srv.URL + "/only"}, DialOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, ok := c.tr.(*sseTransport); !ok {
		t.Fatalf("hauria de ser SSE antic: %T", c.tr)
	}
}

func TestOAuthFluxComplet(t *testing.T) {
	f := newFakeRemote(t)
	f.auth = true
	store := NewTokenStore(filepath.Join(t.TempDir(), "mcp-oauth.json"))
	m := SetupWith(map[string]Server{"remot": {URL: f.url()}}, Options{Store: store, NoRegister: true})
	defer m.Close()
	st := m.Status()
	if len(st) != 1 || st[0]["status"] != StatusNeedsAuth || st[0]["auth"] != "required" {
		t.Fatalf("sense token, cal autoritzar: %+v", st)
	}
	redirect := "http://127.0.0.1:9/api/mcp/oauth/callback"
	authURL, err := m.Connect(context.Background(), "remot", redirect)
	if err != nil || authURL == "" {
		t.Fatalf("connect: %q %v", authURL, err)
	}
	u, _ := url.Parse(authURL)
	q := u.Query()
	if u.Path != "/authorize" || q.Get("client_id") != "cid" || q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != redirect ||
		q.Get("resource") != f.url() || q.Get("scope") != "mcp.read" || q.Get("state") == "" || q.Get("response_type") != "code" {
		t.Fatalf("URL d'autorització: %s", authURL)
	}
	f.mu.Lock()
	f.challenge = q.Get("code_challenge")
	f.mu.Unlock()
	// El navegador torna al callback amb el codi.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", OAuthCallbackPath+"?code=the-code&state="+url.QueryEscape(q.Get("state")), nil)
	server, err := m.HandleCallback(rec, req)
	if err != nil || server != "remot" || rec.Code != 200 {
		t.Fatalf("callback: %q %v %d %s", server, err, rec.Code, rec.Body.String())
	}
	st = m.Status()
	if st[0]["status"] != StatusConnected || st[0]["auth"] != "authorized" || st[0]["tools"] != 1 {
		t.Fatalf("connectat després del callback: %+v", st)
	}
	out, err := m.CallTool(context.Background(), "remot", "eco", `{"text":"auth"}`)
	if err != nil || out != "ECO:auth" {
		t.Fatalf("call=%q err=%v", out, err)
	}
	// El state és d'un sol ús.
	if _, err := m.HandleCallback(httptest.NewRecorder(), req); err == nil {
		t.Fatal("un state reutilitzat ha de fallar")
	}
	// Tokens desats amb 0600.
	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos del magatzem: %v", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(store.path)
	if !strings.Contains(string(raw), "ref-1") {
		t.Fatalf("refresh token desat: %s", raw)
	}
	// Status no exposa tokens ni capçaleres.
	b, _ := json.Marshal(m.Status())
	if strings.Contains(string(b), "tok-1") || strings.Contains(string(b), "ref-1") {
		t.Fatalf("Status filtra secrets: %s", b)
	}
	// Reconnectar reaprofita el client registrat i el token.
	if _, err := m.Connect(context.Background(), "remot", redirect); err != nil {
		t.Fatal(err)
	}
	if f.registered != 1 {
		t.Fatalf("registre dinàmic un sol cop: %d", f.registered)
	}
}

func TestOAuthRefrescDespresDe401(t *testing.T) {
	f := newFakeRemote(t)
	f.auth = true
	store := NewTokenStore(filepath.Join(t.TempDir(), "mcp-oauth.json"))
	key := tokenKey("remot", Server{URL: f.url()})
	store.put(key, tokenRecord{AccessToken: "tok-1", RefreshToken: "ref-1", ClientID: "cid", TokenEndpoint: f.srv.URL + "/token", Resource: f.url()})
	f.valid = "tok-old" // el servidor ja no accepta tok-1: 401 → refresc → tok-2
	m := SetupWith(map[string]Server{"remot": {URL: f.url()}}, Options{Store: store, NoRegister: true})
	defer m.Close()
	if st := m.Status(); st[0]["status"] != StatusConnected {
		t.Fatalf("el refresc havia de salvar la connexió: %+v (errors %v)", st, m.ErrorsSnapshot())
	}
	if f.refreshed != 1 {
		t.Fatalf("refrescos: %d", f.refreshed)
	}
	if r, _ := store.get(key); r.AccessToken != "tok-2" || r.RefreshToken != "ref-1" {
		t.Fatalf("token renovat i refresh conservat: %+v", r)
	}
}

func TestOAuthTokenCaducatEsRefresca(t *testing.T) {
	f := newFakeRemote(t)
	store := NewTokenStore(filepath.Join(t.TempDir(), "mcp-oauth.json"))
	store.put("k", tokenRecord{AccessToken: "vell", RefreshToken: "ref-1", Expiry: time.Now().Add(-time.Minute), ClientID: "cid", TokenEndpoint: f.srv.URL + "/token"})
	ts := &storeTokens{store: store, key: "k", hc: http.DefaultClient}
	tok, err := ts.Token(context.Background())
	if err != nil || tok != "tok-2" {
		t.Fatalf("token caducat → refresc: %q %v", tok, err)
	}
}

func TestRecursosIPrompts(t *testing.T) {
	f := newFakeRemote(t)
	m := SetupWith(map[string]Server{"docs": {URL: f.url()}}, Options{Store: NewTokenStore(filepath.Join(t.TempDir(), "s.json")), NoRegister: true})
	defer m.Close()
	list, err := m.ResourceList(context.Background(), "")
	if err != nil || !strings.Contains(list, "file:///notes.md") {
		t.Fatalf("list=%q err=%v", list, err)
	}
	body, err := m.ResourceRead(context.Background(), "docs", "file:///notes.md")
	if err != nil || body != "# Notes" {
		t.Fatalf("read=%q err=%v", body, err)
	}
	ps := m.Prompts()
	if len(ps) != 1 || ps[0].Command() != "mcp__docs__review" {
		t.Fatalf("prompts=%+v", ps)
	}
	out, ok, err := m.ExpandPromptCommand(context.Background(), "/mcp__docs__review main.go focus=security")
	if !ok || err != nil || out != "Review main.go for security" {
		t.Fatalf("expand=%q ok=%v err=%v", out, ok, err)
	}
	if _, ok, err := m.ExpandPromptCommand(context.Background(), "/mcp__docs__review"); !ok || err == nil {
		t.Fatal("falta l'argument obligatori")
	}
	if _, ok, _ := m.ExpandPromptCommand(context.Background(), "/compact"); ok {
		t.Fatal("no és un prompt MCP")
	}
}

func TestPromptArgs(t *testing.T) {
	p := Prompt{Arguments: []PromptArgument{{Name: "file"}, {Name: "focus"}}}
	got := PromptArgs(p, "a.go perf and memory")
	if got["file"] != "a.go" || got["focus"] != "perf and memory" {
		t.Fatalf("%v", got)
	}
	got = PromptArgs(p, "focus=x a.go")
	if got["file"] != "a.go" || got["focus"] != "x" {
		t.Fatalf("%v", got)
	}
}

func TestParseAuthParams(t *testing.T) {
	got := parseAuthParams(`Bearer realm="mcp", resource_metadata="https://x/.well-known/oauth-protected-resource", scope="a b", error=invalid_token`)
	if got["resource_metadata"] != "https://x/.well-known/oauth-protected-resource" || got["scope"] != "a b" || got["error"] != "invalid_token" || got["realm"] != "mcp" {
		t.Fatalf("%v", got)
	}
}

func TestAddRemove(t *testing.T) {
	f := newFakeRemote(t)
	m := SetupWith(map[string]Server{}, Options{Store: NewTokenStore(filepath.Join(t.TempDir(), "s.json")), NoRegister: true})
	defer m.Close()
	if err := m.Add(context.Background(), "nou", Server{URL: f.url()}); err != nil {
		t.Fatal(err)
	}
	if len(m.ToolsSnapshot()) != 1 {
		t.Fatalf("tools=%v", m.ToolsSnapshot())
	}
	m.Remove("nou")
	if len(m.ToolsSnapshot()) != 0 || len(m.Status()) != 0 {
		t.Fatalf("tret: %v %v", m.ToolsSnapshot(), m.Status())
	}
}

func TestOAuthLoopbackDelTUI(t *testing.T) {
	f := newFakeRemote(t)
	f.auth = true
	m := SetupWith(map[string]Server{"remot": {URL: f.url()}}, Options{Store: NewTokenStore(filepath.Join(t.TempDir(), "s.json")), NoRegister: true})
	defer m.Close()
	lb, err := m.StartLoopback()
	if err != nil {
		t.Fatal(err)
	}
	defer lb.Close()
	if !strings.HasPrefix(lb.RedirectURI, "http://127.0.0.1:") || !strings.HasSuffix(lb.RedirectURI, OAuthCallbackPath) {
		t.Fatalf("redirect: %s", lb.RedirectURI)
	}
	authURL, err := m.Connect(context.Background(), "remot", lb.RedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.Parse(authURL)
	f.mu.Lock()
	f.challenge = q.Query().Get("code_challenge")
	f.mu.Unlock()
	resp, err := http.Get(lb.RedirectURI + "?code=the-code&state=" + url.QueryEscape(q.Query().Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	select {
	case r := <-lb.Done:
		if r.Err != nil || r.Server != "remot" {
			t.Fatalf("loopback: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sense callback")
	}
	if st := m.Status(); st[0]["status"] != StatusConnected {
		t.Fatalf("%+v", st)
	}
}

// TestNewManagerConnectaEnSegonPla: NewManager no toca la xarxa i cada
// ConnectServer porta les eines del seu servidor encara que un altre
// segueixi penjat (és el que fa el TUI en arrencar).
func TestNewManagerConnectaEnSegonPla(t *testing.T) {
	f := newFakeRemote(t)
	penjat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // no respon mai
	}))
	defer penjat.Close()
	start := time.Now()
	m := NewManager(map[string]Server{"docs": {URL: f.url()}, "penjat": {URL: penjat.URL + "/mcp"}},
		Options{Store: NewTokenStore(filepath.Join(t.TempDir(), "s.json")), NoRegister: true})
	defer m.Close()
	if time.Since(start) > time.Second {
		t.Fatal("NewManager no ha d'esperar cap servidor")
	}
	if got := m.Connecting(); len(got) != 2 {
		t.Fatalf("connecting=%v", got)
	}
	if s := m.Summary(); !strings.Contains(s, "2 connectant") {
		t.Fatalf("summary=%q", s)
	}
	for _, row := range m.Status() {
		if row["status"] != StatusConnecting {
			t.Fatalf("status=%v", row)
		}
	}
	if err := m.ConnectServer("docs"); err != nil {
		t.Fatal(err)
	}
	if got := m.Connecting(); len(got) != 1 || got[0] != "penjat" {
		t.Fatalf("connecting=%v", got)
	}
	if len(m.ToolsSnapshot()) == 0 {
		t.Fatal("les eines de docs han d'aparèixer sense esperar penjat")
	}
	m.Remove("penjat")
	if got := m.Connecting(); len(got) != 0 {
		t.Fatalf("connecting després de Remove=%v", got)
	}
}
