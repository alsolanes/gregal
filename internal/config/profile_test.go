package config

import (
	"path/filepath"
	"testing"

	"gregal/internal/profile"
)

// El config, les claus i la memòria de Gregal Jocs van a la seva carpeta;
// els de Gregal, on sempre; GREGAL_AUTH i GREGAL_MEMORY manen igualment.
func TestRutesDelPerfilPerProducte(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GREGAL_AUTH", "")
	t.Setenv("GREGAL_MEMORY", "")
	defer profile.SetProduct(profile.Product())

	profile.SetProduct("")
	g := filepath.Join(home, ".config", "gregal")
	if DefaultPath() != filepath.Join(g, "config.yaml") || AuthPath() != filepath.Join(g, "auth.yaml") || UserMemoryPath() != filepath.Join(g, "memory.md") {
		t.Fatalf("Gregal: %s %s %s", DefaultPath(), AuthPath(), UserMemoryPath())
	}
	profile.SetProduct("jocs")
	j := filepath.Join(home, ".config", "gregal-jocs")
	if DefaultPath() != filepath.Join(j, "config.yaml") || AuthPath() != filepath.Join(j, "auth.yaml") || UserMemoryPath() != filepath.Join(j, "memory.md") {
		t.Fatalf("Jocs: %s %s %s", DefaultPath(), AuthPath(), UserMemoryPath())
	}
	t.Setenv("GREGAL_AUTH", filepath.Join(home, "a.yaml"))
	t.Setenv("GREGAL_MEMORY", filepath.Join(home, "m.md"))
	if AuthPath() != filepath.Join(home, "a.yaml") || UserMemoryPath() != filepath.Join(home, "m.md") {
		t.Fatal("les variables d'entorn han de manar")
	}
}
