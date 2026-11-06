package web

import (
	"os"
	"path/filepath"
	"testing"
)

// La carpeta d'usuari, la que les conté i l'arrel d'un disc no són projectes:
// els vents i les colles hi treballen a la carpeta de projectes.
func TestBroadFoldersAreNotProjects(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	for _, dir := range []string{home, filepath.Dir(home), filepath.VolumeName(home) + string(filepath.Separator)} {
		if !esCarpetaMassaAmpla(dir) {
			t.Fatalf("%s no és un projecte", dir)
		}
	}
	if esCarpetaMassaAmpla(filepath.Join(home, "projectes", "web-cv")) || esCarpetaMassaAmpla(t.TempDir()) {
		t.Fatal("una carpeta de projecte sí que ho és")
	}
	t.Setenv("GREGAL_PROJECTES", t.TempDir())
	s := &Server{cwd: home}
	if ws, _ := s.ventWorkspace(); ws != projectesDir() {
		t.Fatalf("amb la carpeta d'usuari, a la de projectes: %s", ws)
	}
}
