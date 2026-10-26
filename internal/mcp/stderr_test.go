package mcp

import (
	"errors"
	"strings"
	"testing"
)

// TestStderrDelServidorNoVaALaTerminal: el que el servidor escriu a stderr
// es guarda (no surt per la terminal del TUI) i acompanya l'error si cau.
func TestStderrDelServidorNoVaALaTerminal(t *testing.T) {
	c := &cuaText{}
	c.Write([]byte("gregal mcp demo: dataset ready in 10ms; serving on stdio\n"))
	c.Write([]byte(strings.Repeat("x", maxCuaText) + "\npanic: boom\n"))
	if len(c.buf) > maxCuaText {
		t.Fatalf("la cua no té límit: %d", len(c.buf))
	}
	tr := &stdioTransport{name: "demo", done: make(chan struct{}), stderr: c}
	tr.fail(errors.New("mcp demo: the server closed stdout"))
	if !strings.Contains(tr.err.Error(), "panic: boom") {
		t.Fatalf("l'error ha de portar l'última sortida: %v", tr.err)
	}
}
