package usercmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCmd(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListIFind(t *testing.T) {
	home := t.TempDir()
	setHomeTest(t, home)
	cwd := t.TempDir()
	writeCmd(t, filepath.Join(home, ".config", "gregal", "commands"), "hola.md", "# Saluda\nHola {{args}}!")
	writeCmd(t, filepath.Join(cwd, ".gregal", "commands"), "hola.md", "# Saluda fort\nHOLA {{args}}!!")
	writeCmd(t, filepath.Join(cwd, ".gregal", "commands"), "altra.md", "Fes això: {{args}}")
	writeCmd(t, filepath.Join(cwd, ".gregal", "commands"), "MAL NOM.md", "ignorat")
	writeCmd(t, filepath.Join(cwd, ".gregal", "commands"), "nota.txt", "ignorat")

	list := List(cwd)
	if len(list) != 2 {
		t.Fatalf("ordres=%v", list)
	}
	c, ok := Find(cwd, "hola")
	if !ok || c.Source != "projecte" {
		t.Fatalf("projecte ha de guanyar: %+v", c)
	}
	if got := Expand(c, "Món"); got != "HOLA Món!!" {
		t.Fatalf("expand=%q", got)
	}
	if _, ok := Find(cwd, "MAL NOM"); ok {
		t.Fatal("noms amb espais s'ignoren")
	}
	if _, ok := Find(cwd, "inexistent"); ok {
		t.Fatal("inexistent no es troba")
	}
}

func TestDescITrim(t *testing.T) {
	if d := DescOf("# Títol\nCos"); d != "Títol" {
		t.Fatalf("desc=%q", d)
	}
	if d := DescOf("\n\nPrimera línia útil"); d != "Primera línia útil" {
		t.Fatalf("desc=%q", d)
	}
	if got := strings.TrimSpace(Expand(Cmd{Body: "  fes {{args}}  "}, "")); got != "fes" {
		t.Fatalf("expand buit=%q", got)
	}
}

func TestResolve(t *testing.T) {
	home := t.TempDir()
	setHomeTest(t, home)
	cwd := t.TempDir()
	writeCmd(t, filepath.Join(cwd, ".gregal", "commands"), "revisa.md", "# Revisa\nRevisa {{args}} a fons.")
	c, args, ok := Resolve(cwd, "/revisa main.go avui")
	if !ok || c.Name != "revisa" || args != "main.go avui" {
		t.Fatalf("resolve=%+v args=%q ok=%v", c, args, ok)
	}
	if got := Expand(c, args); got != "Revisa main.go avui a fons." {
		t.Fatalf("expand=%q", got)
	}
	for _, txt := range []string{"/inexistent x", "/MAL NOM", "sense barra", "/"} {
		if _, _, ok := Resolve(cwd, txt); ok {
			t.Fatalf("no hauria de resoldre %q", txt)
		}
	}
}
