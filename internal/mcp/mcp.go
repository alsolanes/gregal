// Package mcp és el client del Model Context Protocol de Gregal: parla amb
// servidors locals (stdio) i remots (Streamable HTTP, amb l'SSE antic de
// reserva i OAuth 2.1), n'exposa les tools a l'agent i en llegeix els
// recursos i els prompts. server.go és la banda contrària: Gregal com a
// servidor MCP (`gregal mcp serve`).
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gregal/internal/llm"
)

// ProtocolVersion és la versió que es demana als servidors remots; els
// locals reben la 2024-11-05, que entenen tots (el servidor contesta amb
// la que suporta i és la que es fa servir).
const (
	ProtocolVersion      = "2025-06-18"
	ProtocolVersionStdio = "2024-11-05"
)

// ClientVersion surt al clientInfo de l'initialize.
var ClientVersion = "1.8"

// callTimeout és el temps màxim d'una crida sense context propi.
var callTimeout = 60 * time.Second

// Server és la configuració d'un servidor MCP.
type Server struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
	// URL i Transport: remot per HTTP. Transport "" = stdio si hi ha
	// command; http (amb SSE antic de reserva) si hi ha url.
	URL       string            `yaml:"url"`
	Transport string            `yaml:"transport"`
	Headers   map[string]string `yaml:"headers"`
	OAuth     *OAuthConfig      `yaml:"oauth"`
	// ReadOnly: les eines del servidor no demanen permís (config readonly).
	ReadOnly bool `yaml:"readonly"`
}

// OAuthConfig fixa el client OAuth (sense client_id: registre dinàmic).
type OAuthConfig struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`
}

// Remote diu si el servidor és per HTTP.
func (s Server) Remote() bool {
	switch s.Transport {
	case "http", "sse":
		return true
	case "stdio":
		return false
	}
	return strings.TrimSpace(s.URL) != "" && strings.TrimSpace(s.Command) == ""
}

// TransportName és el transport efectiu per mostrar-lo.
func (s Server) TransportName() string {
	if !s.Remote() {
		return "stdio"
	}
	if s.Transport == "sse" {
		return "sse"
	}
	return "http"
}

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcNotif struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcMsg és qualsevol missatge que arriba del servidor: resposta,
// petició o notificació.
type rpcMsg struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// idIgual compara l'id d'un missatge amb el nostre (numèric o string).
func idIgual(raw json.RawMessage, id int64) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.Trim(string(raw), `"`) == strconv.FormatInt(id, 10)
}

func (m rpcMsg) esResposta() bool { return len(m.ID) > 0 && m.Method == "" }

// transport mou missatges JSON-RPC amb un servidor.
type transport interface {
	request(ctx context.Context, id int64, method string, params any) (json.RawMessage, error)
	notify(ctx context.Context, method string, params any) error
	close()
}

// AuthRequiredError: el servidor remot demana OAuth (401).
type AuthRequiredError struct {
	ResourceMetadata string
	Scope            string
	Detail           string
}

func (e *AuthRequiredError) Error() string {
	msg := "authorization required"
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	return msg
}

// IsAuthRequired diu si l'error és un 401 que demana autorització.
func IsAuthRequired(err error) bool {
	var a *AuthRequiredError
	return errors.As(err, &a)
}

// Capabilities són les capacitats que el servidor anuncia.
type Capabilities struct {
	Tools     bool
	Resources bool
	Prompts   bool
}

// Client parla amb un servidor MCP.
type Client struct {
	name     string
	tr       transport
	next     atomic.Int64
	Caps     Capabilities
	Protocol string
	Info     string
}

// DialOpts són les dependències opcionals de la connexió remota.
type DialOpts struct {
	// Tokens dona el bearer (OAuth) i el refresca després d'un 401.
	Tokens TokenSource
	// HTTP és el client HTTP (tests); nil = un per defecte.
	HTTP HTTPDoer
}

// Dial connecta (o arrenca) el servidor i fa el handshake initialize.
func Dial(name string, srv Server) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	return DialCtx(ctx, name, srv, DialOpts{})
}

// DialCtx és Dial amb context i dependències.
func DialCtx(ctx context.Context, name string, srv Server, o DialOpts) (*Client, error) {
	if !srv.Remote() {
		if strings.TrimSpace(srv.Command) == "" {
			return nil, fmt.Errorf("mcp %s: no command", name)
		}
		tr, err := startStdio(name, srv)
		if err != nil {
			return nil, err
		}
		c := &Client{name: name, tr: tr}
		if err := c.initialize(ctx, ProtocolVersionStdio); err != nil {
			tr.close()
			return nil, err
		}
		return c, nil
	}
	if strings.TrimSpace(srv.URL) == "" {
		return nil, fmt.Errorf("mcp %s: no url", name)
	}
	hc := o.HTTP
	if hc == nil {
		hc = defaultHTTP()
	}
	headers := expandHeaders(srv.Headers)
	if srv.Transport != "sse" {
		ht := &httpTransport{name: name, url: srv.URL, hc: hc, headers: headers, tokens: o.Tokens}
		c := &Client{name: name, tr: ht}
		err := c.initialize(ctx, ProtocolVersion)
		if err == nil {
			return c, nil
		}
		ht.close()
		// Compatibilitat enrere (spec 2025-03-26): un servidor SSE antic
		// respon 4xx al POST de l'initialize; es prova el GET de l'stream.
		var se *httpStatusError
		if srv.Transport == "http" || !errors.As(err, &se) || (se.Code != 400 && se.Code != 404 && se.Code != 405) {
			return nil, err
		}
	}
	st, err := dialSSE(ctx, name, srv.URL, hc, headers, o.Tokens)
	if err != nil {
		return nil, err
	}
	c := &Client{name: name, tr: st}
	if err := c.initialize(ctx, ProtocolVersionStdio); err != nil {
		st.close()
		return nil, err
	}
	return c, nil
}

func (c *Client) initialize(ctx context.Context, version string) error {
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools     json.RawMessage `json:"tools"`
			Resources json.RawMessage `json:"resources"`
			Prompts   json.RawMessage `json:"prompts"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "gregal", "version": ClientVersion},
	}, &res); err != nil {
		if IsAuthRequired(err) {
			return err
		}
		return fmt.Errorf("mcp %s initialize: %w", c.name, err)
	}
	present := func(r json.RawMessage) bool { return len(r) > 0 && string(r) != "null" }
	c.Caps = Capabilities{Tools: present(res.Capabilities.Tools), Resources: present(res.Capabilities.Resources), Prompts: present(res.Capabilities.Prompts)}
	// Servidors mínims que no declaren res: es prova tools igualment.
	if !c.Caps.Tools && !c.Caps.Resources && !c.Caps.Prompts {
		c.Caps.Tools = true
	}
	c.Protocol = res.ProtocolVersion
	c.Info = strings.TrimSpace(res.ServerInfo.Name + " " + res.ServerInfo.Version)
	if ht, ok := c.tr.(*httpTransport); ok {
		ht.setProtocol(res.ProtocolVersion)
	}
	return c.tr.notify(ctx, "notifications/initialized", nil)
}

// call fa una crida JSON-RPC i en desa el resultat a out.
func (c *Client) call(ctx context.Context, method string, params, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callTimeout)
		defer cancel()
	}
	id := c.next.Add(1)
	raw, err := c.tr.request(ctx, id, method, params)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("mcp %s: timeout waiting for %s", c.name, method)
		}
		return err
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Tool és una eina remota (spec llesta per l'agent).
type Tool struct {
	Spec   llm.ToolSpec
	Call   func(argsJSON string) (string, error)
	server string
	remote string
}

// Server és el servidor d'on ve l'eina.
func (t Tool) Server() string { return t.server }

// RemoteName és el nom de l'eina al servidor (sense el prefix mcp_).
func (t Tool) RemoteName() string { return t.remote }

// ToolName és el nom que veu l'agent: mcp_<servidor>_<eina>.
func ToolName(server, tool string) string { return "mcp_" + sanitize(server) + "_" + sanitize(tool) }

// Tools llista les tools del servidor (amb paginació).
func (c *Client) Tools() ([]Tool, error) { return c.ToolsCtx(context.Background()) }

// ToolsCtx és Tools amb context.
func (c *Client) ToolsCtx(ctx context.Context) ([]Tool, error) {
	var out []Tool
	cursor := ""
	for page := 0; page < 50; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var res struct {
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := c.call(ctx, "tools/list", params, &res); err != nil {
			return nil, err
		}
		for _, t := range res.Tools {
			t := t
			schema := t.InputSchema
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			out = append(out, Tool{
				Spec:   llm.ToolSpec{Name: ToolName(c.name, t.Name), Description: "[mcp:" + c.name + "] " + t.Description, Parameters: schema},
				server: c.name,
				remote: t.Name,
				Call: func(argsJSON string) (string, error) {
					return c.CallTool(context.Background(), t.Name, argsJSON)
				},
			})
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return out, nil
}

// CallTool crida una tool del servidor.
func (c *Client) CallTool(ctx context.Context, tool, argsJSON string) (string, error) {
	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("unreadable arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	var res struct {
		Content           []contentBlock `json:"content"`
		StructuredContent any            `json:"structuredContent"`
		IsError           bool           `json:"isError"`
	}
	if err := c.call(ctx, "tools/call", map[string]any{"name": tool, "arguments": args}, &res); err != nil {
		return "", err
	}
	out := textOf(res.Content)
	if out == "" && res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		out = string(b)
	}
	if res.IsError {
		return out, fmt.Errorf("mcp %s/%s: %s", c.name, tool, out)
	}
	return out, nil
}

type contentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType"`
	URI      string `json:"uri"`
	Resource *struct {
		URI      string `json:"uri"`
		Text     string `json:"text"`
		MimeType string `json:"mimeType"`
	} `json:"resource"`
}

// textOf ajunta el text dels blocs; la resta (imatges, binaris) es nomena.
func textOf(blocks []contentBlock) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "resource":
			if b.Resource != nil && b.Resource.Text != "" {
				parts = append(parts, b.Resource.Text)
			} else if b.Resource != nil {
				parts = append(parts, "[resource "+b.Resource.URI+"]")
			}
		case "resource_link":
			parts = append(parts, "[resource "+b.URI+"]")
		case "image", "audio":
			parts = append(parts, "["+b.Type+" "+b.MimeType+"]")
		}
	}
	return strings.Join(parts, "\n")
}

// Resource és un recurs que el servidor exposa.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListResources llista els recursos del servidor.
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	var out []Resource
	cursor := ""
	for page := 0; page < 50; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var res struct {
			Resources  []Resource `json:"resources"`
			NextCursor string     `json:"nextCursor"`
		}
		if err := c.call(ctx, "resources/list", params, &res); err != nil {
			return nil, err
		}
		out = append(out, res.Resources...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return out, nil
}

// ReadResource llegeix un recurs (el text; els binaris es nomenen).
func (c *Client) ReadResource(ctx context.Context, uri string) (string, error) {
	var res struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
			Blob     string `json:"blob"`
		} `json:"contents"`
	}
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &res); err != nil {
		return "", err
	}
	var parts []string
	for _, ct := range res.Contents {
		if ct.Text != "" || ct.Blob == "" {
			parts = append(parts, ct.Text)
		} else {
			parts = append(parts, fmt.Sprintf("[binary %s, %d bytes base64]", ct.MimeType, len(ct.Blob)))
		}
	}
	return strings.Join(parts, "\n"), nil
}

// Prompt és una plantilla de prompt del servidor.
type Prompt struct {
	Server      string           `json:"server"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument és un argument d'un prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Command és el nom de l'ordre de barra: /mcp__<servidor>__<prompt>.
func (p Prompt) Command() string { return "mcp__" + sanitize(p.Server) + "__" + sanitize(p.Name) }

// ListPrompts llista els prompts del servidor.
func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	var out []Prompt
	cursor := ""
	for page := 0; page < 50; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var res struct {
			Prompts    []Prompt `json:"prompts"`
			NextCursor string   `json:"nextCursor"`
		}
		if err := c.call(ctx, "prompts/list", params, &res); err != nil {
			return nil, err
		}
		for _, p := range res.Prompts {
			p.Server = c.name
			out = append(out, p)
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return out, nil
}

// GetPrompt expandeix un prompt amb els seus arguments: el text dels
// missatges, en ordre.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (string, error) {
	if args == nil {
		args = map[string]string{}
	}
	var res struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := c.call(ctx, "prompts/get", map[string]any{"name": name, "arguments": args}, &res); err != nil {
		return "", err
	}
	var parts []string
	for _, m := range res.Messages {
		var one contentBlock
		if json.Unmarshal(m.Content, &one) == nil && one.Type != "" {
			parts = append(parts, textOf([]contentBlock{one}))
			continue
		}
		var many []contentBlock
		if json.Unmarshal(m.Content, &many) == nil {
			parts = append(parts, textOf(many))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n")), nil
}

// Close tanca la connexió (mata el procés o tanca la sessió HTTP).
func (c *Client) Close() {
	if c != nil && c.tr != nil {
		c.tr.close()
	}
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
