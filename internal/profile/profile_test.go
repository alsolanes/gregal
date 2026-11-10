package profile

import (
	"os"
	"path/filepath"
	"testing"
)

// Gregal es queda a les carpetes de sempre; Gregal Jocs té les seves, i
// GREGAL_DATA_DIR mana per damunt de totes dues.
func TestRutesPerProducte(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GREGAL_DATA_DIR", "")
	defer SetProduct(Product())

	SetProduct("")
	if got, want := ConfigDir(), filepath.Join(home, ".config", "gregal"); got != want {
		t.Fatalf("config de Gregal: %s, volia %s", got, want)
	}
	if got, want := DataDir(), filepath.Join(home, ".local", "share", "gregal"); got != want {
		t.Fatalf("dades de Gregal: %s, volia %s", got, want)
	}
	if DirName() != "gregal" {
		t.Fatalf("nom de Gregal: %s", DirName())
	}

	SetProduct("jocs")
	if got, want := ConfigDir(), filepath.Join(home, ".config", "gregal-jocs"); got != want {
		t.Fatalf("config de Jocs: %s, volia %s", got, want)
	}
	if got, want := DataDir(), filepath.Join(home, ".local", "share", "gregal-jocs"); got != want {
		t.Fatalf("dades de Jocs: %s, volia %s", got, want)
	}
	// Les de Gregal es poden llegir des de Jocs (la importació del model).
	if got, want := ConfigDirFor(""), filepath.Join(home, ".config", "gregal"); got != want {
		t.Fatalf("config de Gregal des de Jocs: %s", got)
	}
	if base, err := os.UserConfigDir(); err == nil {
		if got := UserConfigSubdir(); got != filepath.Join(base, "gregal-jocs") {
			t.Fatalf("UserConfigSubdir: %s", got)
		}
	}

	t.Setenv("GREGAL_DATA_DIR", filepath.Join(home, "altre"))
	if got := DataDir(); got != filepath.Join(home, "altre") {
		t.Fatalf("GREGAL_DATA_DIR ha de manar: %s", got)
	}
	if got := DefaultDataDirFor("jocs"); got != filepath.Join(home, ".local", "share", "gregal-jocs") {
		t.Fatalf("DefaultDataDirFor no mira l'entorn: %s", got)
	}
}
