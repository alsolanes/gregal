// Package mcp és un client mínim del Model Context Protocol (transport stdio):
// arrenca servidors, llegeix les seves tools i les crida per JSON-RPC.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gregal/internal/llm"
)

// Server és la configuració d'un servidor MCP (transport stdio).
type Server struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResp struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Client parla amb un servidor MCP via stdin/stdout (línies JSON-RPC).
type Client struct {
	name string
	cmd  *exec.Cmd
	in   *json.Encoder
	scan *bufio.Scanner
	mu   sync.Mutex
	next int
}

// Dial arrenca el procés i fa el handshake initialize.
func Dial(name string, srv Server) (*Client, error) {
	if strings.TrimSpace(srv.Command) == "" {
		return nil, fmt.Errorf("mcp %s: sense command", name)
	}
	cmd := exec.Command(srv.Command, srv.Args...)
	cmd.Env = os.Environ()
	for k, v := range srv.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %s: %w", name, err)
	}
	c := &Client{name: name, cmd: cmd, in: json.NewEncoder(stdin), scan: bufio.NewScanner(stdout)}
	c.scan.Buffer(make([]byte, 1024*1024), 1024*1024)
	var initResp struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := c.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "gregal", "version": "0.1.0"},
	}, &initResp); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("mcp %s initialize: %w", name, err)
	}
	// notificació (sense resposta esperada)
	_ = c.in.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	return c, nil
}

// call fa una crida JSON-RPC (una sola en vol: els servidors stdio són seqüencials).
func (c *Client) call(method string, params, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	id := c.next
	if err := c.in.Encode(rpcReq{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return err
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("mcp %s: timeout esperant %s", c.name, method)
		}
		// lectura amb timeout tou: el scanner bloqueja; el procés viu fa que
		// sigui acceptable (el servidor sempre respon o mor).
		type lineRes struct {
			line string
			ok   bool
		}
		ch := make(chan lineRes, 1)
		go func() {
			if c.scan.Scan() {
				ch <- lineRes{c.scan.Text(), true}
			} else {
				ch <- lineRes{"", false}
			}
		}()
		select {
		case r := <-ch:
			if !r.ok {
				return fmt.Errorf("mcp %s: el servidor ha tancat stdout", c.name)
			}
			var resp rpcResp
			if err := json.Unmarshal([]byte(r.line), &resp); err != nil {
				continue // soroll: ignora
			}
			if resp.ID == nil || *resp.ID != id {
				continue // notificació o resposta aliena: ignora
			}
			if resp.Error != nil {
				return fmt.Errorf("mcp %s: %s", c.name, resp.Error.Message)
			}
			if out != nil {
				return json.Unmarshal(resp.Result, out)
			}
			return nil
		case <-time.After(60 * time.Second):
			return fmt.Errorf("mcp %s: timeout esperant %s", c.name, method)
		}
	}
}

// Tool és una eina remota (spec llesta per l'agent).
type Tool struct {
	Spec   llm.ToolSpec
	Call   func(argsJSON string) (string, error)
	server string
}

// Tools llista les tools del servidor.
func (c *Client) Tools() ([]Tool, error) {
	var res struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := c.call("tools/list", map[string]any{}, &res); err != nil {
		return nil, err
	}
	var out []Tool
	for _, t := range res.Tools {
		t := t
		name := "mcp_" + sanitize(c.name) + "_" + sanitize(t.Name)
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, Tool{
			Spec:   llm.ToolSpec{Name: name, Description: "[mcp:" + c.name + "] " + t.Description, Parameters: schema},
			server: c.name,
			Call: func(argsJSON string) (string, error) {
				return c.callTool(t.Name, argsJSON)
			},
		})
	}
	return out, nil
}

func (c *Client) callTool(tool, argsJSON string) (string, error) {
	var args map[string]any
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("arguments il·legibles: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := c.call("tools/call", map[string]any{"name": tool, "arguments": args}, &res); err != nil {
		return "", err
	}
	var parts []string
	for _, b := range res.Content {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	out := strings.Join(parts, "\n")
	if res.IsError {
		return out, fmt.Errorf("mcp %s/%s: %s", c.name, tool, out)
	}
	return out, nil
}

// Close mata el procés del servidor.
func (c *Client) Close() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
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
