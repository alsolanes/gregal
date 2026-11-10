package config

import (
	"os"
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

// ReadProfileModel llegeix el model d'un altre perfil: el del rol inicial,
// el triat a la UI si n'hi ha, i la clau del config o, si és buida, del
// seu auth.yaml. No escriu res.
func TestReadProfileModel(t *testing.T) {
	dir := t.TempDir()
	yaml := "mode: code\nproviders:\n  a: {base_url: 'https://a.example/v1', api_key: ''}\n  b: {base_url: 'https://b.example/v1', api_key: 'sk-b'}\n" +
		"roles:\n  chat: {provider: a, model: m1}\n  code: {provider: a, model: m1, context_window: 4096}\n  reviewer: {provider: a, model: m1}\n"
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600)
	os.WriteFile(filepath.Join(dir, "auth.yaml"), []byte("keys:\n  a: sk-a\n"), 0o600)
	m, err := ReadProfileModel(dir, nil)
	if err != nil || m.Provider != "a" || m.Model != "m1" || m.Key != "sk-a" || m.Role.ContextWindow != 4096 || m.Settings.BaseURL != "https://a.example/v1" || m.Settings.APIKey != "" {
		t.Fatalf("del config: %+v %v", m, err)
	}
	m, _ = ReadProfileModel(dir, map[string]string{"code": "b/m2"})
	if m.Provider != "b" || m.Model != "m2" || m.Key != "sk-b" {
		t.Fatalf("triat a la UI: %+v", m)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 2 {
		t.Fatalf("no ha d'escriure res: %d fitxers", len(ents))
	}
	if _, err := ReadProfileModel(t.TempDir(), nil); err == nil {
		t.Fatal("sense config, error")
	}
}
