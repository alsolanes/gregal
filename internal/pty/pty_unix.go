//go:build linux || darwin

package pty

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

type unixPTY struct {
	master *os.File
	cmd    *exec.Cmd

	waitOnce sync.Once
	exit     int
	waitErr  error
}

// Start engega spec amb un pseudoterminal com a terminal de control.
func Start(spec Spec) (PTY, error) {
	cols, rows := clamp(spec.Cols, spec.Rows)
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	name, err := openSlave(master)
	if err != nil {
		master.Close()
		return nil, err
	}
	slave, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, err
	}
	defer slave.Close()
	_ = unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})

	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		master.Close()
		return nil, err
	}
	return &unixPTY{master: master, cmd: cmd}, nil
}

func (p *unixPTY) Read(b []byte) (int, error) {
	n, err := p.master.Read(b)
	// Linux torna EIO quan l'esclau es tanca: és el final de la sortida.
	if err != nil && errors.Is(err, syscall.EIO) {
		return n, os.ErrClosed
	}
	return n, err
}

func (p *unixPTY) Write(b []byte) (int, error) { return p.master.Write(b) }
func (p *unixPTY) Pid() int                    { return p.cmd.Process.Pid }

func (p *unixPTY) Resize(cols, rows int) error {
	cols, rows = clamp(cols, rows)
	return unix.IoctlSetWinsize(int(p.master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})
}

func (p *unixPTY) Kill() error {
	// El grup sencer: el TUI i les ordres que hagi engegat.
	if err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

func (p *unixPTY) Wait() (int, error) {
	p.waitOnce.Do(func() {
		err := p.cmd.Wait()
		var ee *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &ee):
			p.exit = ee.ExitCode()
		default:
			p.waitErr = err
		}
	})
	return p.exit, p.waitErr
}

func (p *unixPTY) Close() error { return p.master.Close() }
