package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// serveProva engega Serve sobre canonades i torna un enviador de línies i
// un lector de respostes.
func serveProva(t *testing.T, tools []ServerTool) (send func(string), recv func() map[string]any, stop func()) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Serve(ctx, inR, outW, ServeOptions{Name: "gregal", Version: "test", Tools: tools})
		outW.Close()
		close(done)
	}()
	sc := bufio.NewScanner(outR)
	lines := make(chan string, 16)
	go func() {
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	send = func(s string) { inW.Write([]byte(s + "\n")) }
	recv = func() map[string]any {
		select {
		case l := <-lines:
			var m map[string]any
			if err := json.Unmarshal([]byte(l), &m); err != nil {
				t.Fatalf("resposta il·legible %q", l)
			}
			return m
		case <-time.After(5 * time.Second):
			t.Fatal("sense resposta")
		}
		return nil
	}
	stop = func() { cancel(); inW.Close(); <-done }
	return
}

func TestServeProtocol(t *testing.T) {
	started := make(chan struct{})
	tools := []ServerTool{
		{Name: "eco", Description: "echo", InputSchema: map[string]any{"type": "object"}, Handler: func(_ context.Context, a map[string]any) (string, error) {
			return "ECO:" + ArgString(a, "text"), nil
		}},
		{Name: "falla", InputSchema: map[string]any{"type": "object"}, Handler: func(context.Context, map[string]any) (string, error) {
			return "", errors.New("no puc")
		}},
		{Name: "lenta", InputSchema: map[string]any{"type": "object"}, Handler: func(ctx context.Context, _ map[string]any) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		}},
	}
	send, recv, stop := serveProva(t, tools)
	defer stop()
	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t"}}}`)
	init := recv()
	res := init["result"].(map[string]any)
	if res["protocolVersion"] != "2025-06-18" || res["serverInfo"].(map[string]any)["name"] != "gregal" {
		t.Fatalf("initialize: %v", init)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	list := recv()["result"].(map[string]any)["tools"].([]any)
	if len(list) != 3 {
		t.Fatalf("tools: %v", list)
	}
	send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"eco","arguments":{"text":"hola"}}}`)
	r := recv()["result"].(map[string]any)
	if r["content"].([]any)[0].(map[string]any)["text"] != "ECO:hola" || r["isError"] == true {
		t.Fatalf("call: %v", r)
	}
	send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"falla","arguments":{}}}`)
	r = recv()["result"].(map[string]any)
	if r["isError"] != true || !strings.Contains(r["content"].([]any)[0].(map[string]any)["text"].(string), "no puc") {
		t.Fatalf("error d'eina = isError: %v", r)
	}
	// Cancel·lació: la crida lenta acaba quan arriba notifications/cancelled.
	send(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"lenta","arguments":{}}}`)
	<-started
	send(`{"jsonrpc":"2.0","id":6,"method":"ping"}`)
	if p := recv(); p["id"].(float64) != 6 {
		t.Fatalf("el ping no espera la crida lenta: %v", p)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":5}}`)
	if c := recv(); c["id"].(float64) != 5 || c["result"].(map[string]any)["isError"] != true {
		t.Fatalf("cancel·lada: %v", c)
	}
	send(`{"jsonrpc":"2.0","id":7,"method":"nope"}`)
	if e := recv(); e["error"] == nil {
		t.Fatalf("mètode desconegut: %v", e)
	}
}
