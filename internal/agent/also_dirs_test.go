package agent

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// AlsoDirs existeix perquè un vent pugui treballar a la carpeta de projectes
// (~/gregal/projectes) sense perdre el permís d'escriure al repo del Gregal
// quan la feina és sobre el Gregal mateix.
func TestWritablePathAmbAlsoDirs(t *testing.T) {
	base := t.TempDir()
	proj := filepath.Join(base, "grupal-projectes")
	repo := filepath.Join(base, "grupal-repo")
	p := &Policy{ProjectDir: proj, AlsoDirs: []string{repo}}

	casos := []struct {
		path string
		vol  bool
	}{
		{filepath.Join(proj, "joc", "index.html"), true},
		{filepath.Join(repo, "internal", "web", "preview.go"), true},
		{"/etc/passwd", false},
		{proj + "2/fora.txt", false}, // prefix de cadena, no contenció
		{"", false},
	}
	for _, c := range casos {
		data, err := json.Marshal(map[string]string{"path": c.path, "content": "x"})
		if err != nil {
			t.Fatal(err)
		}
		got := p.writablePath("write", string(data))
		if got != c.vol {
			t.Fatalf("%s: esperava %v, he trobat %v", c.path, c.vol, got)
		}
	}
	// Sense AlsoDirs res canvia respecte d'abans.
	p2 := &Policy{ProjectDir: proj}
	data, err := json.Marshal(map[string]string{"path": filepath.Join(repo, "x.go")})
	if err != nil {
		t.Fatal(err)
	}
	if p2.writablePath("write", string(data)) {
		t.Fatal("sense AlsoDirs el repo no ha de ser escrivible")
	}
	// Una ordre sense ruta no passa per aquí (el bash té el seu classificador).
	if p.writablePath("bash", `{"command":"ls"}`) {
		t.Fatal("bash no té ruta d'escriptura")
	}
}
