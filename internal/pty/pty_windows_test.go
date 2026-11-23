//go:build windows

package pty

import (
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Si el servidor s'ha engegat en un grup de processos nou (l'app
// d'escriptori, un llançador), Windows li marca «ignora Ctrl+C» i el fill
// ho hereta: el ^C que arriba a la consola no aturava res. Simulem aquest
// pare i comprovem que Ctrl+C atura un procés que no acaba sol.
func TestCtrlCAturaElFillAmbElPareQueIgnoraCtrlC(t *testing.T) {
	setCtrlC := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler")
	if r, _, err := setCtrlC.Call(0, 1); r == 0 {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _, _ = setCtrlC.Call(0, 0) })
	path, err := exec.LookPath("ping.exe")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Start(Spec{Path: path, Args: []string{"-t", "127.0.0.1"}, Env: os.Environ(), Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Kill(); _ = p.Close() })
	go func() { _, _ = io.Copy(io.Discard, p) }()
	time.Sleep(1500 * time.Millisecond)
	if _, err := p.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _, _ = p.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl+C no ha aturat el procés")
	}
}
