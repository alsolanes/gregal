// Package pty engega un procés dins d'un pseudoterminal: ConPTY a Windows,
// /dev/ptmx a Linux i macOS. El fa servir el mode Kanban del web, on cada
// targeta és un TUI de Gregal viu. Sense dependències noves: tot surt de
// golang.org/x/sys.
package pty

import (
	"errors"
	"io"
)

// Spec descriu el procés a engegar.
type Spec struct {
	Path string   // executable (camí absolut)
	Args []string // arguments sense el nom del programa
	Dir  string   // directori de treball
	Env  []string // entorn complet (KEY=valor)
	Cols int
	Rows int
}

// ErrUnsupported vol dir que aquesta plataforma no té pseudoterminals.
var ErrUnsupported = errors.New("pty: pseudoterminals not supported on this platform")

// PTY és un procés que corre dins d'un pseudoterminal.
type PTY interface {
	io.Reader // sortida del terminal (VT)
	io.Writer // entrada (tecles, com les enviaria un terminal)
	Resize(cols, rows int) error
	// Kill atura el procés (i el que hagi engegat dins de la consola).
	Kill() error
	// Wait espera que el procés acabi i en torna el codi de sortida.
	Wait() (int, error)
	// Close allibera el pseudoterminal; Read torna io.EOF després.
	Close() error
	Pid() int
}

func clamp(cols, rows int) (int, int) {
	if cols < 20 {
		cols = 20
	}
	if rows < 5 {
		rows = 5
	}
	if cols > 500 {
		cols = 500
	}
	if rows > 300 {
		rows = 300
	}
	return cols, rows
}
