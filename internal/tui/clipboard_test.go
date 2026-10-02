package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Un backend present però espatllat no ha d'impedir provar el següent. És el
// cas habitual a Linux quan wl-copy existeix però la sessió actual és X11.
func TestCopyClipboardFallsBackBetweenLinuxBackends(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("els backends falsos són scripts #!/bin/sh: no s'executen a Windows")
	}
	dir := t.TempDir()
	writeExecutable := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable("wl-copy", "exit 1")
	writeExecutable("xclip", "v=; while IFS= read -r line || [ -n \"$line\" ]; do v=\"${v}${line}\"; done; printf %s \"$v\" > \"$CLIP_CAPTURE\"")
	t.Setenv("PATH", dir)
	capture := filepath.Join(dir, "captured")
	t.Setenv("CLIP_CAPTURE", capture)

	how, err := copyToClipboard("text amb accents: gràcies")
	if err != nil {
		t.Fatal(err)
	}
	if how != "xclip" {
		t.Fatalf("backend=%q, vull xclip", how)
	}
	got, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "text amb accents: gràcies" {
		t.Fatalf("clipboard=%q", got)
	}
}
