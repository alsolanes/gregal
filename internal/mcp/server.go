package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// Gregal com a servidor MCP (stdio): `gregal mcp serve` hi registra les
// seves eines (gregal_run, gregal_sessions_list, gregal_session_get) i
// qualsevol client MCP (Claude Code, Cursor, un altre Gregal…) les pot
// cridar. Aquí només hi ha el protocol; les eines viuen al main.

// ServerTool és una eina exposada pel servidor.
type ServerTool struct {
	Name        string
	Description string
	InputSchema map[string]any
	// Handler rep els arguments i torna el text del resultat. Un error
	// surt com a isError (el client el veu com a fallada de l'eina, no
	// com a error de protocol).
	Handler func(ctx context.Context, args map[string]any) (string, error)
}

// ServeOptions configura el servidor.
type ServeOptions struct {
	Name    string
	Version string
	Tools   []ServerTool
	// Resources i Prompts són opcionals: si n'hi ha, el servidor anuncia
	// la capacitat i respon resources/* i prompts/*.
	Resources []ServerResource
	Prompts   []ServerPrompt
	// Log rep diagnòstics (stderr); nil = res.
	Log func(format string, args ...any)
}

// ServerResource és un recurs de text estàtic.
type ServerResource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	Text        string
}

// ServerPrompt és una plantilla de prompt. Text rep els arguments i torna
// el missatge d'usuari.
type ServerPrompt struct {
	Name        string
	Description string
	Arguments   []PromptArgument
	Text        func(args map[string]string) (string, error)
}

type serverMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Serve atén peticions JSON-RPC per línies de r i respon a w fins que r
// s'acaba o ctx mor. Les crides a eines corren en paral·lel (un ping no
// espera un gregal_run de deu minuts) i notifications/cancelled les talla.
func Serve(ctx context.Context, r io.Reader, w io.Writer, o ServeOptions) error {
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	tools := map[string]ServerTool{}
	for _, t := range o.Tools {
		tools[t.Name] = t
	}
	var wmu sync.Mutex
	enc := json.NewEncoder(w)
	send := func(v any) {
		wmu.Lock()
		defer wmu.Unlock()
		if err := enc.Encode(v); err != nil {
			o.Log("mcp serve: write: %v", err)
		}
	}
	reply := func(id json.RawMessage, result any) {
		send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	fail := func(id json.RawMessage, code int, msg string) {
		send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}})
	}
	var (
		cmu     sync.Mutex
		cancels = map[string]context.CancelFunc{}
		wg      sync.WaitGroup
	)
	defer wg.Wait()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lines := make(chan []byte)
	go func() {
		defer close(lines)
		for sc.Scan() {
			b := append([]byte(nil), sc.Bytes()...)
			select {
			case lines <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	for {
		var line []byte
		select {
		case <-ctx.Done():
			cmu.Lock()
			for _, c := range cancels {
				c()
			}
			cmu.Unlock()
			return ctx.Err()
		case l, ok := <-lines:
			if !ok {
				cmu.Lock()
				for _, c := range cancels {
					c()
				}
				cmu.Unlock()
				return sc.Err()
			}
			line = l
		}
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var m serverMsg
		if err := json.Unmarshal(line, &m); err != nil {
			fail(json.RawMessage("null"), -32700, "parse error")
			continue
		}
		isRequest := len(m.ID) > 0 && string(m.ID) != "null"
		switch m.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(m.Params, &p)
			version := p.ProtocolVersion
			switch version {
			case "2024-11-05", "2025-03-26", "2025-06-18":
			default:
				version = ProtocolVersion
			}
			caps := map[string]any{"tools": map[string]any{"listChanged": false}}
			if len(o.Resources) > 0 {
				caps["resources"] = map[string]any{"listChanged": false}
			}
			if len(o.Prompts) > 0 {
				caps["prompts"] = map[string]any{"listChanged": false}
			}
			reply(m.ID, map[string]any{
				"protocolVersion": version,
				"capabilities":    caps,
				"serverInfo":      map[string]any{"name": o.Name, "version": o.Version},
			})
		case "notifications/initialized", "notifications/progress":
		case "notifications/cancelled":
			var p struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			cmu.Lock()
			if c := cancels[string(p.RequestID)]; c != nil {
				c()
			}
			cmu.Unlock()
		case "ping":
			reply(m.ID, map[string]any{})
		case "tools/list":
			list := make([]map[string]any, 0, len(o.Tools))
			for _, t := range o.Tools {
				list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": t.InputSchema})
			}
			reply(m.ID, map[string]any{"tools": list})
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if err := json.Unmarshal(m.Params, &p); err != nil {
				fail(m.ID, -32602, "invalid params")
				continue
			}
			t, ok := tools[p.Name]
			if !ok {
				fail(m.ID, -32602, "unknown tool: "+p.Name)
				continue
			}
			cctx, cancel := context.WithCancel(ctx)
			key := string(m.ID)
			cmu.Lock()
			cancels[key] = cancel
			cmu.Unlock()
			wg.Add(1)
			go func(id json.RawMessage) {
				defer wg.Done()
				defer func() {
					cmu.Lock()
					delete(cancels, key)
					cmu.Unlock()
					cancel()
				}()
				defer func() {
					if rec := recover(); rec != nil {
						reply(id, map[string]any{"content": []any{map[string]any{"type": "text", "text": fmt.Sprintf("internal error: %v", rec)}}, "isError": true})
					}
				}()
				out, err := t.Handler(cctx, p.Arguments)
				if err != nil {
					text := err.Error()
					if out != "" {
						text = out + "\n" + text
					}
					reply(id, map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "isError": true})
					return
				}
				reply(id, map[string]any{"content": []any{map[string]any{"type": "text", "text": out}}})
			}(m.ID)
		case "resources/list":
			list := make([]map[string]any, 0, len(o.Resources))
			for _, r := range o.Resources {
				list = append(list, map[string]any{"uri": r.URI, "name": r.Name, "description": r.Description, "mimeType": r.MimeType})
			}
			reply(m.ID, map[string]any{"resources": list})
		case "resources/read":
			var p struct {
				URI string `json:"uri"`
			}
			_ = json.Unmarshal(m.Params, &p)
			found := false
			for _, r := range o.Resources {
				if r.URI == p.URI {
					reply(m.ID, map[string]any{"contents": []any{map[string]any{"uri": r.URI, "mimeType": r.MimeType, "text": r.Text}}})
					found = true
					break
				}
			}
			if !found {
				fail(m.ID, -32002, "resource not found: "+p.URI)
			}
		case "prompts/list":
			list := make([]map[string]any, 0, len(o.Prompts))
			for _, pr := range o.Prompts {
				args := pr.Arguments
				if args == nil {
					args = []PromptArgument{}
				}
				list = append(list, map[string]any{"name": pr.Name, "description": pr.Description, "arguments": args})
			}
			reply(m.ID, map[string]any{"prompts": list})
		case "prompts/get":
			var p struct {
				Name      string            `json:"name"`
				Arguments map[string]string `json:"arguments"`
			}
			if err := json.Unmarshal(m.Params, &p); err != nil {
				fail(m.ID, -32602, "invalid params")
				continue
			}
			var pr *ServerPrompt
			for i := range o.Prompts {
				if o.Prompts[i].Name == p.Name {
					pr = &o.Prompts[i]
				}
			}
			if pr == nil {
				fail(m.ID, -32602, "unknown prompt: "+p.Name)
				continue
			}
			text, err := pr.Text(p.Arguments)
			if err != nil {
				fail(m.ID, -32602, err.Error())
				continue
			}
			reply(m.ID, map[string]any{"description": pr.Description, "messages": []any{
				map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": text}},
			}})
		default:
			if isRequest {
				fail(m.ID, -32601, "method not found: "+m.Method)
			}
		}
	}
}

// ArgString llegeix un argument string (buit si no hi és).
func ArgString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// ArgBool llegeix un argument booleà.
func ArgBool(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

// ArgInt llegeix un argument numèric.
func ArgInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}
