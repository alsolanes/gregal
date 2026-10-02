package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Mesurat amb el model real: de dotze passos, set se'n van anar buscant on
// era el compilador de Go perquè no era al PATH. El cas que ha de sortir
// clar al prompt no és «hi és» sinó «hi és però no és al PATH».

func TestFetsEntornAvisaQuanLEinaNoEsAlPath(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "einafalsa")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := descriuEina(einaTrobada{Nom: "einafalsa", Ruta: exe})
	if !strings.Contains(got, "NO és al PATH") || !strings.Contains(got, filepath.ToSlash(exe)) {
		t.Errorf("ha de dir que no és al PATH i on és:\n%s", got)
	}
	// Amb %q les contrabarres es doblaven i el que llegia el model era
	// «C:\\Program Files\\Go\\bin», que no és cap ruta.
	if strings.Contains(got, `\\`) {
		t.Errorf("contrabarres doblades al prompt:\n%s", got)
	}
	if got := descriuEina(einaTrobada{Nom: "git", Ruta: "/usr/bin/git", AlPath: true}); !strings.Contains(got, "al PATH") || strings.Contains(got, "NO") {
		t.Errorf("una eina al PATH no ha de fer soroll:\n%s", got)
	}
}

// resolCandidat ha d'acceptar un directori contenidor (els JDK viuen a
// «…/Java/jdk-21/bin/java.exe», i el que sabem és el «…/Java»).
func TestResolCandidatEntraAlsDirectorisContenidors(t *testing.T) {
	dir := t.TempDir()
	bin := "java"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	nia := filepath.Join(dir, "jdk-21", "bin")
	if err := os.MkdirAll(nia, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(nia, bin)
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolCandidat("java", dir); got != exe {
		t.Errorf("resolCandidat = %q, volem %q", got, exe)
	}
	if got := resolCandidat("java", filepath.Join(dir, "no-hi-es")); got != "" {
		t.Errorf("el que no hi és no s'inventa: %q", got)
	}
}

// El bloc s'ha de poder enganxar al system prompt tal com surt, i no pot
// ser buit: sense sistema ni shell, a Windows l'agent escrivia ordres de
// cmd que fallaven.
func TestFetsEntornDiuSistemaIShell(t *testing.T) {
	f := FetsEntorn()
	if !strings.Contains(f, "SISTEMA: "+runtime.GOOS) {
		t.Errorf("falta el sistema:\n%s", f)
	}
	if runtime.GOOS == "windows" && !strings.Contains(f, "no cmd ni PowerShell") {
		t.Errorf("a Windows cal dir amb què s'executen les ordres:\n%s", f)
	}
}

// Es calcula un sol cop: la resposta no canvia i cada torn la demanaria
// altra vegada, amb les seves crides al disc.
func TestLesEinesNoEsTornenABuscar(t *testing.T) {
	a := Eines()
	b := Eines()
	if len(a) != len(b) {
		t.Fatalf("dues cerques diferents: %d i %d", len(a), len(b))
	}
	if len(a) > 0 && &a[0] != &b[0] {
		t.Error("s'ha tornat a cercar en comptes de fer servir el que ja teníem")
	}
}
