package pty

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Un procés de debò dins del pseudoterminal: escriu, rep la mida i acaba
// amb el seu codi de sortida.
func TestStartEchoesAndExits(t *testing.T) {
	var spec Spec
	switch runtime.GOOS {
	case "windows":
		path, err := exec.LookPath("cmd.exe")
		if err != nil {
			t.Skip("sense cmd.exe")
		}
		spec = Spec{Path: path, Args: []string{"/c", "echo kanban-ok & exit 3"}}
	case "linux", "darwin":
		spec = Spec{Path: "/bin/sh", Args: []string{"-c", "echo kanban-ok; stty size; exit 3"}}
	default:
		t.Skip("plataforma sense pty")
	}
	spec.Env, spec.Cols, spec.Rows = os.Environ(), 90, 20
	p, err := Start(spec)
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := p.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				out <- b.String()
				return
			}
		}
	}()
	code, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Fatalf("codi %d", code)
	}
	_ = p.Close()
	select {
	case s := <-out:
		if !strings.Contains(s, "kanban-ok") {
			t.Fatalf("sortida sense l'eco: %q", s)
		}
		if runtime.GOOS != "windows" && !strings.Contains(s, "20 90") {
			t.Fatalf("mida no aplicada: %q", s)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("la sortida no s'ha tancat")
	}
}

func TestReadAfterCloseEnds(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip()
	}
	_, err := Start(Spec{Path: "/no/existeix/gregal", Cols: 80, Rows: 24})
	if err == nil {
		t.Fatal("hauria de fallar amb un executable inexistent")
	}
	if errors.Is(err, io.EOF) {
		t.Fatal("error inesperat")
	}
}
