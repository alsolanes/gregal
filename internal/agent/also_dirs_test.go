package agent

import (
	"path/filepath"
	"testing"
)

// AlsoDirs existeix perquè un vent pugui treballar a la carpeta de projectes
// (~/gregal/projectes) sense perdre el permís d'escriure al repo del Gregal
// quan la feina és sobre el Gregal mateix.
func TestWritablePathAmbAlsoDirs(t *testing.T) {
	proj := "/tmp/grupal-projectes"
	repo := "/tmp/grupal-repo"
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
		got := p.writablePath("write", `{"path":"`+c.path+`","content":"x"}`)
		if got != c.vol {
			t.Fatalf("%s: esperava %v, he trobat %v", c.path, c.vol, got)
		}
	}
	// Sense AlsoDirs res canvia respecte d'abans.
	p2 := &Policy{ProjectDir: proj}
	if p2.writablePath("write", `{"path":"`+filepath.Join(repo, "x.go")+`"}`) {
		t.Fatal("sense AlsoDirs el repo no ha de ser escrivible")
	}
	// Una ordre sense ruta no passa per aquí (el bash té el seu classificador).
	if p.writablePath("bash", `{"command":"ls"}`) {
		t.Fatal("bash no té ruta d'escriptura")
	}
}
