package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// stdioTransport parla amb un procés fill per línies JSON-RPC. Un sol
// lector reparteix les respostes per id: abans cada crida engegava una
// goroutine que llegia del mateix scanner i, després d'un timeout, la
// següent crida en tenia dues de vives llegint alhora.
type stdioTransport struct {
	name  string
	cmd   *exec.Cmd
	stdin io.WriteCloser

	wmu     sync.Mutex
	mu      sync.Mutex
	pending map[int64]chan rpcMsg
	done    chan struct{}
	err     error
	once    sync.Once
	stderr  *cuaText
}

// cuaText guarda els últims bytes que el servidor escriu a stderr. Abans
// anaven directes a la terminal: dins del TUI, una sola línia («gregal mcp
// demo: dataset ready…») desquadrava el pintat i la pantalla quedava amb
// files repetides. Ara es guarden i surten amb l'error si el servidor cau.
type cuaText struct {
	mu  sync.Mutex
	buf []byte
}

const maxCuaText = 4096

func (c *cuaText) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf = append(c.buf, p...)
	if len(c.buf) > maxCuaText {
		c.buf = append([]byte(nil), c.buf[len(c.buf)-maxCuaText:]...)
	}
	return len(p), nil
}

// Ultimes torna les darreres línies no buides.
func (c *cuaText) Ultimes(n int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(c.buf)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return strings.Join(out, " | ")
}

// SelfExecutable és el binari de gregal que corre ara (els tests el poden
// substituir).
var SelfExecutable = os.Executable

// ResolveCommand tradueix la paraula «gregal» al binari actual i repara les
// entrades antigues de la base de dades de demo que apuntaven a un camí que
// ja no existeix: l'app portable es descomprimeix cada cop a una carpeta
// temporal diferent, i el camí desat deixava de valer amb la versió següent.
func ResolveCommand(command string, args []string) string {
	self := func() string {
		if exe, err := SelfExecutable(); err == nil && exe != "" {
			return exe
		}
		return command
	}
	if strings.EqualFold(strings.TrimSpace(command), "gregal") {
		return self()
	}
	if len(args) >= 2 && args[0] == "mcp" && args[1] == "demo" {
		if _, err := os.Stat(command); err != nil {
			return self()
		}
	}
	return command
}

// ChildEnv marca els processos engegats com a servidor MCP: gregal no obre
// el TUI si la porta (evita que un connector «gregal …» s'engegui a si
// mateix en bucle).
const ChildEnv = "GREGAL_MCP_CHILD"

func startStdio(name string, srv Server) (*stdioTransport, error) {
	cmd := exec.Command(ResolveCommand(srv.Command, srv.Args), srv.Args...)
	cmd.Env = append(os.Environ(), ChildEnv+"=1")
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
	cua := &cuaText{}
	cmd.Stderr = cua
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %s: %w", name, err)
	}
	t := &stdioTransport{name: name, cmd: cmd, stdin: stdin, pending: map[int64]chan rpcMsg{}, done: make(chan struct{}), stderr: cua}
	go t.read(stdout)
	return t, nil
}

func (t *stdioTransport) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var m rpcMsg
		if json.Unmarshal(line, &m) != nil {
			continue // soroll: ignora
		}
		if m.esResposta() {
			t.dispatch(m)
			continue
		}
		if m.Method != "" && len(m.ID) > 0 {
			t.answerServer(m)
		}
	}
	t.fail(fmt.Errorf("mcp %s: the server closed stdout", t.name))
}

func (t *stdioTransport) dispatch(m rpcMsg) {
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

// answerServer respon les peticions del servidor: ping sí, la resta no
// la suportem (no declarem cap capacitat de client).
func (t *stdioTransport) answerServer(m rpcMsg) {
	resp := map[string]any{"jsonrpc": "2.0", "id": m.ID}
	if m.Method == "ping" {
		resp["result"] = map[string]any{}
	} else {
		resp["error"] = map[string]any{"code": -32601, "message": "method not supported by client"}
	}
	_ = t.write(resp)
}

func (t *stdioTransport) fail(err error) {
	if t.stderr != nil {
		if ult := t.stderr.Ultimes(3); ult != "" {
			err = fmt.Errorf("%w (stderr: %s)", err, ult)
		}
	}
	t.once.Do(func() {
		t.mu.Lock()
		t.err = err
		t.mu.Unlock()
		close(t.done)
	})
}

func (t *stdioTransport) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.wmu.Lock()
	defer t.wmu.Unlock()
	_, err = t.stdin.Write(append(b, '\n'))
	return err
}

func (t *stdioTransport) request(ctx context.Context, id int64, method string, params any) (json.RawMessage, error) {
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
	if err := t.write(rpcReq{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		drop()
		return nil, err
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return nil, fmt.Errorf("mcp %s: %s", t.name, m.Error.Message)
		}
		return m.Result, nil
	case <-t.done:
		drop()
		t.mu.Lock()
		err := t.err
		t.mu.Unlock()
		return nil, err
	case <-ctx.Done():
		drop()
		_ = t.write(rpcNotif{JSONRPC: "2.0", Method: "notifications/cancelled", Params: map[string]any{"requestId": id, "reason": "cancelled by client"}})
		return nil, ctx.Err()
	}
}

func (t *stdioTransport) notify(_ context.Context, method string, params any) error {
	return t.write(rpcNotif{JSONRPC: "2.0", Method: method, Params: params})
}

func (t *stdioTransport) close() {
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		go t.cmd.Wait()
	}
	t.fail(fmt.Errorf("mcp %s: closed", t.name))
}
