package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeUpdaterCommands(t *testing.T, dirty bool) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixtures")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	os.Mkdir(bin, 0700)
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module gregal\n"), 0600)
	status := ""
	if dirty {
		status = " M user-work.go"
	}
	git := "#!/bin/sh\nif [ \"$1\" = '-C' ]; then shift 2; fi\ncase \"$1\" in\nstatus) printf '%s\\n' '" + status + "';;\nrev-parse) printf '%s\\n' 'abcdef1234567890';;\nesac\n"
	marker := filepath.Join(root, "build-was-called")
	os.WriteFile(filepath.Join(bin, "git"), []byte(git), 0700)
	os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nprintf invoked > '"+marker+"'\nexit 42\n"), 0700)
	t.Setenv("PATH", bin)
	t.Setenv("HOME", root)
	return root, marker
}

func TestCheckWithForceNeverBuildsOrInstalls(t *testing.T) {
	root, marker := fakeUpdaterCommands(t, false)
	result, err := Executa(context.Background(), Opcions{Repo: root, NomesComprova: true, Forca: true})
	if err != nil {
		t.Fatalf("check-only attempted update: %v", err)
	}
	if result == nil || !result.AlDia {
		t.Fatal("expected up-to-date result")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("check-only invoked build: %v", err)
	}
}

func TestForceRebuildDoesNotBypassDirtyTree(t *testing.T) {
	root, marker := fakeUpdaterCommands(t, true)
	_, err := Executa(context.Background(), Opcions{Repo: root, Forca: true})
	if err == nil || !strings.Contains(err.Error(), "sense cometre") {
		t.Fatalf("dirty tree was not refused: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("dirty tree reached build: %v", err)
	}
}
