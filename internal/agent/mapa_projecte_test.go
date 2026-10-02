package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func projecteProva(t *testing.T, git bool) string {
	t.Helper()
	dir := t.TempDir()
	for nom, cos := range map[string]string{
		"TASK.md":       "# Tasca\nFes que Suma sumi.\n",
		"main.go":       "package main\n",
		"pkg/util.go":   "package pkg\n",
		"docs/notes.md": "notes\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(nom))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(cos), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if git {
		for _, a := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "x"}} {
			c := exec.Command("git", a...)
			c.Dir = dir
			if out, err := c.CombinedOutput(); err != nil {
				t.Skipf("sense git: %v %s", err, out)
			}
		}
	}
	return dir
}

// El torn arrenca sabent quins fitxers hi ha, l'estat de git i el
// contingut dels fitxers petits que la tasca anomena.
func TestMapaProjecteLlistaIAdjunta(t *testing.T) {
	dir := projecteProva(t, true)
	c := MapaProjecte(dir, "Llegeix TASK.md i fes el que diu.", nil)
	for _, vol := range []string{MarcaContext, "main.go", "pkg/util.go", "Estat de git", "--- TASK.md", "Fes que Suma sumi."} {
		if !strings.Contains(c, vol) {
			t.Fatalf("hi falta %q:\n%s", vol, c)
		}
	}
	if strings.Contains(c, "--- main.go") {
		t.Fatal("només s'adjunten els fitxers que la tasca anomena")
	}
}

// Sense git, un recorregut que salta el que no és del projecte.
func TestMapaProjecteSenseGit(t *testing.T) {
	dir := projecteProva(t, false)
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "x", "i.js"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := MapaProjecte(dir, "mira-ho", nil)
	if !strings.Contains(c, "pkg/util.go") || strings.Contains(c, "node_modules") {
		t.Fatalf("recorregut sense git:\n%s", c)
	}
	if strings.Contains(c, "Estat de git") {
		t.Fatal("sense git no hi ha estat de git")
	}
}

// En una sessió llarga no es repeteix si no ha canviat; si canvia, sí.
func TestMapaProjecteNoEsRepeteix(t *testing.T) {
	dir := projecteProva(t, true)
	c := MapaProjecte(dir, "mira-ho", nil)
	hist := []llm.Message{{Role: "user", Content: "mira-ho\n\n" + c}, {Role: "assistant", Content: "fet"}}
	if again := MapaProjecte(dir, "i ara això", hist); again != "" {
		t.Fatalf("igual que l'anterior, no es repeteix:\n%s", again)
	}
	if err := os.WriteFile(filepath.Join(dir, "nou.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if again := MapaProjecte(dir, "i ara això", hist); !strings.Contains(again, "nou.go") {
		t.Fatalf("amb un fitxer nou, el context torna:\n%s", again)
	}
}

// Un projecte gran es resumeix per carpetes.
func TestMapaProjecteGranEsResumeix(t *testing.T) {
	var fs []string
	for i := 0; i < 400; i++ {
		fs = append(fs, "internal/p"+string(rune('a'+i%26))+"/f.go")
	}
	fs = append(fs, "go.mod", "README.md")
	r := resumFitxers(fs)
	if !strings.Contains(r, "internal/ (400)") || !strings.Contains(r, "go.mod") {
		t.Fatalf("resum per carpetes:\n%s", r)
	}
	if strings.Count(r, "\n") > 10 {
		t.Fatalf("massa llarg per a 402 fitxers:\n%s", r)
	}
}

func TestAnomenatParaulaSencera(t *testing.T) {
	cas := []struct {
		tasca, f string
		vol      bool
	}{
		{"Llegeix TASK.md i implementa-ho", "TASK.md", true},
		{"Llegeix TASK.md.", "TASK.md", true},
		{"mira MYTASK.md", "TASK.md", false},
		{"obre pkg/util.go", "pkg/util.go", true},
		{"a main hi ha", "main", false},
	}
	for _, c := range cas {
		if got := anomenat(c.tasca, c.f); got != c.vol {
			t.Errorf("anomenat(%q, %q) = %v", c.tasca, c.f, got)
		}
	}
}
