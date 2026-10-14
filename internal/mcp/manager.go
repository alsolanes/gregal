package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// Estats d'un servidor (Status).
const (
	StatusConnected = "connected"
	StatusError     = "error"
	StatusNeedsAuth = "needs_auth"
)

// Manager arrenca N servidors i registra les seves tools a l'agent. Es pot
// tocar en calent (afegir, treure, reconnectar, autoritzar) des de la
// pàgina de connectors: tot passa per mu.
type Manager struct {
	mu      sync.Mutex
	servers map[string]Server
	clients map[string]*Client
	// Tools i Errors són instantànies (compatibilitat amb el codi que les
	// llegia directament); fes servir ToolsSnapshot/ErrorsSnapshot.
	Tools    []Tool
	Errors   map[string]string
	needAuth map[string]bool
	prompts  map[string][]Prompt
	toolsBy  map[string][]Tool
	pending  map[string]*pendingAuth

	store    *TokenStore
	hc       HTTPDoer
	register bool
}

var current atomic.Pointer[Manager]

// Current és el gestor de l'última crida a Setup (nil si cap).
func Current() *Manager { return current.Load() }

// FromConfig adapta el config al paquet mcp.
func FromConfig(servers map[string]config.MCPServerCfg) map[string]Server {
	out := make(map[string]Server, len(servers))
	for name, s := range servers {
		out[name] = ServerFromConfig(s)
	}
	return out
}

// ServerFromConfig adapta un servidor del config.
func ServerFromConfig(s config.MCPServerCfg) Server {
	srv := Server{Command: s.Command, Args: s.Args, Env: s.Env, URL: s.URL, Transport: s.Transport, Headers: s.Headers, ReadOnly: s.ReadOnly}
	if s.OAuth != nil {
		srv.OAuth = &OAuthConfig{ClientID: s.OAuth.ClientID, ClientSecret: s.OAuth.ClientSecret, Scopes: s.OAuth.Scopes}
	}
	return srv
}

// Options configura un gestor (tests: magatzem i client HTTP propis).
type Options struct {
	Store *TokenStore
	HTTP  HTTPDoer
	// NoRegister: no registra res a l'agent (tests).
	NoRegister bool
}

// Setup arrenca els servidors del config i registra les tools a l'agent.
// No falla mai: els errors queden a Errors (consultables amb /mcp).
func Setup(servers map[string]Server) *Manager {
	return SetupWith(servers, Options{})
}

// SetupWith és Setup amb opcions.
func SetupWith(servers map[string]Server, o Options) *Manager {
	m := &Manager{
		servers: map[string]Server{}, clients: map[string]*Client{}, Errors: map[string]string{},
		needAuth: map[string]bool{}, prompts: map[string][]Prompt{}, pending: map[string]*pendingAuth{},
		toolsBy: map[string][]Tool{},
		store:   o.Store, hc: o.HTTP, register: !o.NoRegister,
	}
	if m.store == nil {
		m.store = NewTokenStore("")
	}
	if m.hc == nil {
		m.hc = defaultHTTP()
	}
	for name, srv := range servers {
		m.servers[name] = srv
	}
	// En paral·lel: un servidor remot lent no ha de retardar els altres.
	var wg sync.WaitGroup
	for name := range servers {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			m.connect(ctx, name)
		}(name)
	}
	wg.Wait()
	m.refreshTools()
	if m.register {
		current.Store(m)
	}
	return m
}

func (m *Manager) dialOpts(name string, srv Server) DialOpts {
	o := DialOpts{HTTP: m.hc}
	if srv.Remote() {
		o.Tokens = &storeTokens{store: m.store, key: tokenKey(name, srv), hc: m.hc}
	}
	return o
}

// connect (re)connecta un servidor i en desa l'estat. Sense lock: només
// toca els mapes sota mu.
func (m *Manager) connect(ctx context.Context, name string) error {
	m.mu.Lock()
	srv, ok := m.servers[name]
	old := m.clients[name]
	delete(m.clients, name)
	m.mu.Unlock()
	if old != nil {
		old.Close()
	}
	if !ok {
		return fmt.Errorf("mcp: unknown server %q", name)
	}
	c, err := DialCtx(ctx, name, srv, m.dialOpts(name, srv))
	var prompts []Prompt
	var tools []Tool
	if err == nil {
		if c.Caps.Tools {
			tools, err = c.ToolsCtx(ctx)
		}
		if err == nil && c.Caps.Prompts {
			prompts, _ = c.ListPrompts(ctx) // els prompts són opcionals
		}
		if err != nil {
			c.Close()
			c = nil
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		delete(m.prompts, name)
		delete(m.toolsBy, name)
		if IsAuthRequired(err) {
			m.needAuth[name] = true
			m.Errors[name] = "authorization required"
		} else {
			m.needAuth[name] = false
			m.Errors[name] = err.Error()
		}
		return err
	}
	m.clients[name] = c
	delete(m.Errors, name)
	m.needAuth[name] = false
	m.prompts[name] = prompts
	m.toolsBy[name] = tools
	return nil
}

// refreshTools refà la llista d'eines i el registre de l'agent.
func (m *Manager) refreshTools() {
	m.mu.Lock()
	names := make([]string, 0, len(m.clients))
	resources := false
	byName := map[string][]Tool{}
	readOnly := map[string]bool{}
	for n, c := range m.clients {
		readOnly[n] = m.servers[n].ReadOnly
		names = append(names, n)
		if c.Caps.Resources {
			resources = true
		}
		byName[n] = append([]Tool(nil), m.toolsBy[n]...)
	}
	m.mu.Unlock()
	sort.Strings(names)
	var all []Tool
	for _, n := range names {
		tools := byName[n]
		for i := range tools {
			server, remote := n, tools[i].remote
			tools[i].Call = func(argsJSON string) (string, error) {
				return m.CallTool(context.Background(), server, remote, argsJSON)
			}
		}
		all = append(all, tools...)
	}
	m.mu.Lock()
	m.Tools = all
	m.mu.Unlock()
	if !m.register {
		return
	}
	keep := map[string]bool{}
	for _, t := range all {
		keep[t.Spec.Name] = true
	}
	if resources {
		keep[ResourceListTool] = true
		keep[ResourceReadTool] = true
	}
	agent.UnregisterExtra(func(name string) bool { return strings.HasPrefix(name, "mcp_") && !keep[name] })
	var ro []string
	for _, t := range all {
		if readOnly[t.server] {
			ro = append(ro, t.Spec.Name)
		}
	}
	agent.SetReadOnlyMCPTools(ro)
	for _, t := range all {
		server, remote := t.server, t.remote
		agent.RegisterExtraCtx(t.Spec, func(ctx context.Context, args string) (string, error) {
			return m.CallTool(ctx, server, remote, args)
		})
	}
	if resources {
		m.registerResourceTools()
	}
}

// CallTool crida una eina d'un servidor pel client viu (el reconnectat,
// si s'ha reconnectat).
func (m *Manager) CallTool(ctx context.Context, server, tool, argsJSON string) (string, error) {
	c := m.client(server)
	if c == nil {
		return "", fmt.Errorf("mcp %s: not connected", server)
	}
	out, err := c.CallTool(ctx, tool, argsJSON)
	if IsAuthRequired(err) {
		m.mu.Lock()
		m.needAuth[server] = true
		m.Errors[server] = "authorization required"
		m.mu.Unlock()
		return "", fmt.Errorf("mcp %s: authorization expired; reconnect it from the Connectors page (or /mcp connect %s)", server, server)
	}
	return out, err
}

func (m *Manager) client(server string) *Client {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[server]
}

// ToolsSnapshot torna una còpia de les eines.
func (m *Manager) ToolsSnapshot() []Tool {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Tool(nil), m.Tools...)
}

// ErrorsSnapshot torna una còpia dels errors per servidor.
func (m *Manager) ErrorsSnapshot() map[string]string {
	out := map[string]string{}
	if m == nil {
		return out
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.Errors {
		out[k] = v
	}
	return out
}

// Status returns the configured servers and their current state without
// exposing commands, headers, tokens or environment secrets.
func (m *Manager) Status() []map[string]any {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make(map[string]bool, len(m.servers)+len(m.Errors))
	for n := range m.servers {
		names[n] = true
	}
	for n := range m.clients {
		names[n] = true
	}
	for n := range m.Errors {
		names[n] = true
	}
	keys := make([]string, 0, len(names))
	for n := range names {
		keys = append(keys, n)
	}
	sort.Strings(keys)
	toolCount := map[string]int{}
	for _, t := range m.Tools {
		toolCount[t.server]++
	}
	out := make([]map[string]any, 0, len(keys))
	for _, name := range keys {
		srv := m.servers[name]
		row := map[string]any{"name": name, "transport": srv.TransportName(), "tools": toolCount[name], "prompts": len(m.prompts[name])}
		if srv.ReadOnly {
			row["readonly"] = true
		}
		switch {
		case m.clients[name] != nil:
			row["status"] = StatusConnected
			row["resources"] = m.clients[name].Caps.Resources
		case m.needAuth[name]:
			row["status"] = StatusNeedsAuth
		default:
			row["status"] = StatusError
		}
		if err := m.Errors[name]; err != "" {
			row["error"] = err
		}
		if srv.Remote() {
			if u, err := url.Parse(srv.URL); err == nil {
				row["host"] = u.Host
			}
			_, has := m.store.get(tokenKey(name, srv))
			switch {
			case m.needAuth[name]:
				row["auth"] = "required"
			case has:
				row["auth"] = "authorized"
			default:
				row["auth"] = "none"
			}
		}
		out = append(out, row)
	}
	return out
}

// Summary retorna "2 servidors, 7 eines" o "inactiu".
func (m *Manager) Summary() string {
	if m == nil {
		return "inactiu"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.clients) == 0 && len(m.Errors) == 0 {
		return "inactiu"
	}
	s := fmt.Sprintf("%d servidor(s), %d eina(es)", len(m.clients), len(m.Tools))
	if len(m.Errors) > 0 {
		s += fmt.Sprintf(" (%d error(s))", len(m.Errors))
	}
	return s
}

// Close atura tots els servidors.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	clients := m.clients
	m.clients = map[string]*Client{}
	m.mu.Unlock()
	for _, c := range clients {
		c.Close()
	}
}

// SpecsOf retorna les specs registrades (per tests).
func (m *Manager) SpecsOf() []llm.ToolSpec {
	var out []llm.ToolSpec
	for _, t := range m.ToolsSnapshot() {
		out = append(out, t.Spec)
	}
	return out
}

// Reconnect torna a connectar un servidor (Connect sense OAuth nou).
func (m *Manager) Reconnect(ctx context.Context, name string) error {
	err := m.connect(ctx, name)
	m.refreshTools()
	return err
}

// Connect (re)connecta un servidor. Si demana autorització, torna l'URL
// on l'usuari ha d'autoritzar Gregal; el navegador tornarà a
// redirectURI (OAuthCallbackPath) i CompleteAuth acabarà la feina.
func (m *Manager) Connect(ctx context.Context, name, redirectURI string) (authURL string, err error) {
	err = m.connect(ctx, name)
	m.refreshTools()
	if err == nil || !IsAuthRequired(err) {
		return "", err
	}
	m.mu.Lock()
	srv := m.servers[name]
	m.mu.Unlock()
	var ae *AuthRequiredError
	errors.As(err, &ae)
	u, p, berr := beginAuth(ctx, m.hc, m.store, name, srv, redirectURI, ae)
	if berr != nil {
		m.mu.Lock()
		m.Errors[name] = "authorization required: " + berr.Error()
		m.mu.Unlock()
		return "", berr
	}
	m.mu.Lock()
	// Autoritzacions velles (>15 min) fora: no s'acumulen.
	for st, old := range m.pending {
		if time.Since(old.Created) > 15*time.Minute {
			delete(m.pending, st)
		}
	}
	m.pending[p.State] = p
	m.mu.Unlock()
	return u, nil
}

// CompleteAuth acaba una autorització amb el codi del callback: canvia el
// codi per tokens, els desa i reconnecta el servidor.
func (m *Manager) CompleteAuth(ctx context.Context, state, code string) (string, error) {
	m.mu.Lock()
	p := m.pending[state]
	delete(m.pending, state)
	m.mu.Unlock()
	if p == nil {
		return "", errors.New("unknown or expired OAuth state")
	}
	if strings.TrimSpace(code) == "" {
		return p.Server, errors.New("the authorization server returned no code")
	}
	rec, err := exchangeCode(ctx, m.hc, p, code)
	if err != nil {
		return p.Server, err
	}
	if err := m.store.put(p.Key, rec); err != nil {
		return p.Server, err
	}
	return p.Server, m.Reconnect(ctx, p.Server)
}

// Disconnect oblida el token d'un servidor i el desconnecta.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	srv, ok := m.servers[name]
	c := m.clients[name]
	delete(m.clients, name)
	delete(m.toolsBy, name)
	if ok && srv.Remote() {

		m.needAuth[name] = true
		m.Errors[name] = "authorization required"
	}
	m.mu.Unlock()
	if c != nil {
		c.Close()
	}
	if ok && srv.Remote() {
		_ = m.store.Forget(tokenKey(name, srv))
	}
	m.refreshTools()
	return nil
}

// Add afegeix (o substitueix) un servidor i el connecta.
func (m *Manager) Add(ctx context.Context, name string, srv Server) error {
	m.mu.Lock()
	m.servers[name] = srv
	m.mu.Unlock()
	return m.Reconnect(ctx, name)
}

// Remove treu un servidor (en tanca la connexió, en treu les eines i
// n'oblida el token).
func (m *Manager) Remove(name string) {
	m.mu.Lock()
	srv, ok := m.servers[name]
	c := m.clients[name]
	delete(m.servers, name)
	delete(m.clients, name)
	delete(m.Errors, name)
	delete(m.needAuth, name)
	delete(m.prompts, name)
	delete(m.toolsBy, name)
	m.mu.Unlock()

	if c != nil {
		c.Close()
	}
	if ok && srv.Remote() {
		_ = m.store.Forget(tokenKey(name, srv))
	}
	m.refreshTools()
}

// Names són els servidors configurats.
func (m *Manager) Names() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.servers))
	for n := range m.servers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// --- prompts com a ordres de barra ---

// Prompts són els prompts de tots els servidors connectats.
func (m *Manager) Prompts() []Prompt {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Prompt
	for _, ps := range m.prompts {
		out = append(out, ps...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Command() < out[j].Command() })
	return out
}

// FindPrompt troba un prompt pel nom d'ordre (amb o sense /).
func (m *Manager) FindPrompt(command string) (Prompt, bool) {
	command = strings.TrimPrefix(strings.TrimSpace(command), "/")
	for _, p := range m.Prompts() {
		if p.Command() == command {
			return p, true
		}
	}
	return Prompt{}, false
}

// GetPrompt expandeix un prompt d'un servidor.
func (m *Manager) GetPrompt(ctx context.Context, p Prompt, args map[string]string) (string, error) {
	c := m.client(p.Server)
	if c == nil {
		return "", fmt.Errorf("mcp %s: not connected", p.Server)
	}
	return c.GetPrompt(ctx, p.Name, args)
}

// PromptArgs reparteix el text de després de l'ordre entre els arguments
// del prompt: clau=valor explícits, i la resta en ordre (l'últim argument
// s'emporta tot el que sobra).
func PromptArgs(p Prompt, text string) map[string]string {
	out := map[string]string{}
	var rest []string
	for _, f := range strings.Fields(text) {
		if k, v, ok := strings.Cut(f, "="); ok && k != "" && hasArg(p, k) {
			out[k] = v
			continue
		}
		rest = append(rest, f)
	}
	var free []string
	for _, a := range p.Arguments {
		if _, ok := out[a.Name]; !ok {
			free = append(free, a.Name)
		}
	}
	for i, name := range free {
		if i >= len(rest) {
			break
		}
		if i == len(free)-1 {
			out[name] = strings.Join(rest[i:], " ")
		} else {
			out[name] = rest[i]
		}
	}
	return out
}

func hasArg(p Prompt, name string) bool {
	for _, a := range p.Arguments {
		if a.Name == name {
			return true
		}
	}
	return false
}

// ExpandPromptCommand resol «/mcp__servidor__prompt args» al text del
// prompt. ok=false si el text no és cap prompt MCP.
func (m *Manager) ExpandPromptCommand(ctx context.Context, text string) (expanded string, ok bool, err error) {
	text = strings.TrimSpace(text)
	if m == nil || !strings.HasPrefix(text, "/mcp__") {
		return "", false, nil
	}
	cmd, rest, _ := strings.Cut(text, " ")
	p, found := m.FindPrompt(cmd)
	if !found {
		return "", false, nil
	}
	return m.ExpandPrompt(ctx, cmd, PromptArgs(p, rest))
}

// ExpandPrompt expandeix un prompt pel nom d'ordre amb els arguments ja
// separats.
func (m *Manager) ExpandPrompt(ctx context.Context, command string, args map[string]string) (expanded string, ok bool, err error) {
	p, found := m.FindPrompt(command)
	if m == nil || !found {
		return "", false, nil
	}
	if args == nil {
		args = map[string]string{}
	}
	for _, a := range p.Arguments {

		if a.Required && strings.TrimSpace(args[a.Name]) == "" {
			return "", true, fmt.Errorf("prompt %s: missing argument %q", p.Command(), a.Name)
		}
	}
	out, err := m.GetPrompt(ctx, p, args)
	return out, true, err
}

// --- recursos com a eines ---

// Noms de les eines de recursos (comencen per mcp_: demanen permís com
// la resta d'eines MCP).
const (
	ResourceListTool = "mcp_resource_list"
	ResourceReadTool = "mcp_resource_read"
)

func (m *Manager) registerResourceTools() {
	agent.RegisterExtraCtx(llm.ToolSpec{
		Name:        ResourceListTool,
		Description: "[mcp] List the resources (files, documents, records) that connected MCP servers expose. Optional server filter.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"server": map[string]any{"type": "string", "description": "MCP server name (empty = all)"},
		}},
	}, func(ctx context.Context, args string) (string, error) {
		var a struct {
			Server string `json:"server"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		return m.ResourceList(ctx, a.Server)
	})
	agent.RegisterExtraCtx(llm.ToolSpec{
		Name:        ResourceReadTool,
		Description: "[mcp] Read a resource from an MCP server by URI (see mcp_resource_list).",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"server": map[string]any{"type": "string", "description": "MCP server name"},
			"uri":    map[string]any{"type": "string", "description": "resource URI"},
		}, "required": []string{"server", "uri"}},
	}, func(ctx context.Context, args string) (string, error) {
		var a struct {
			Server string `json:"server"`
			URI    string `json:"uri"`
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", err
		}
		return m.ResourceRead(ctx, a.Server, a.URI)
	})
}

// ResourceList llista els recursos (d'un servidor o de tots).
func (m *Manager) ResourceList(ctx context.Context, server string) (string, error) {
	m.mu.Lock()
	var names []string
	for n, c := range m.clients {
		if c.Caps.Resources && (server == "" || n == server) {
			names = append(names, n)
		}
	}
	m.mu.Unlock()
	if len(names) == 0 {
		return "", fmt.Errorf("no connected MCP server exposes resources%s", map[bool]string{true: " named " + server, false: ""}[server != ""])
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		rs, err := m.client(n).ListResources(ctx)
		if err != nil {
			fmt.Fprintf(&b, "%s: error: %v\n", n, err)
			continue
		}
		for _, r := range rs {
			fmt.Fprintf(&b, "%s\t%s\t%s", n, r.URI, r.Name)
			if r.Description != "" {
				fmt.Fprintf(&b, " — %s", r.Description)
			}
			b.WriteByte('\n')
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = "(no resources)"
	}
	return out, nil
}

// ResourceRead llegeix un recurs.
func (m *Manager) ResourceRead(ctx context.Context, server, uri string) (string, error) {
	c := m.client(server)
	if c == nil {
		return "", fmt.Errorf("mcp %s: not connected", server)
	}
	out, err := c.ReadResource(ctx, uri)
	if err != nil {
		return "", err
	}
	if len(out) > 60000 {
		out = out[:60000] + "\n… (truncated)"
	}
	return out, nil
}
