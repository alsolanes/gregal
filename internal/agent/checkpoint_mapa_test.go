package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// CheckpointIMapa torna exactament el que tornaria TascaAmbMapa: fer la
// foto alhora no canvia el missatge que rep el model.
func TestCheckpointIMapaTornaElMateixQueTascaAmbMapa(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "TASK.md"), []byte("# feina\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc F() {}\n"), 0o644)
	task := "Llegeix TASK.md i fes la feina"
	want := TascaAmbMapa(ModeCode, dir, task, nil)
	if got := CheckpointIMapa("", dir, ModeCode, 0, task, nil); got != want {
		t.Fatalf("diferent:\n%s\n---\n%s", got, want)
	}
	if got := CheckpointIMapa("", dir, ModeChat, 0, task, nil); got != task {
		t.Fatalf("mode xat sense mapa: %q", got)
	}
}
