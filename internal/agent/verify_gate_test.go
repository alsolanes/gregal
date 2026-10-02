package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecVerificacioOk(t *testing.T) {
	codi, sortida := execVerificacio(t.Context(), "", "echo hola-verify")
	if codi != 0 {
		t.Fatalf("codi %d, volia 0", codi)
	}
	if !strings.Contains(sortida, "hola-verify") {
		t.Fatalf("sortida %q sense l'eco", sortida)
	}
}

func TestExecVerificacioKo(t *testing.T) {
	codi, _ := execVerificacio(t.Context(), "", "exit 3")
	if codi != 3 {
		t.Fatalf("codi %d, volia 3", codi)
	}
}

func TestExecVerificacioInexistent(t *testing.T) {
	codi, sortida := execVerificacio(t.Context(), "", "comanda-que-no-existeix-xyz")
	if codi == 0 {
		t.Fatal("una comanda inexistent no pot donar 0")
	}
	if strings.TrimSpace(sortida) == "" {
		t.Fatal("hauria d'explicar què ha fallat")
	}
}

func TestExecVerificacioRetalla(t *testing.T) {
	cmd := "seq 1 3000"
	if runtime.GOOS == "windows" {
		cmd = "for /L %i in (1,1,3000) do @echo %i"
	}
	codi, sortida := execVerificacio(t.Context(), "", cmd)
	if codi != 0 {
		t.Fatalf("codi %d, volia 0", codi)
	}
	if !strings.Contains(sortida, "sortida retallada") {
		t.Fatal("la sortida llarga s'ha de retallar amb marca")
	}
}

func TestExecVerificacioWorkdir(t *testing.T) {
	cmd := "pwd"
	if runtime.GOOS == "windows" {
		cmd = "cd"
	}
	codi, sortida := execVerificacio(t.Context(), t.TempDir(), cmd)
	if codi != 0 || strings.TrimSpace(sortida) == "" {
		t.Fatalf("codi %d sortida %q", codi, sortida)
	}
}

func TestVerifyEnvTeGo(t *testing.T) {
	trobats := []string{}
	for _, d := range []string{"sdk/go/bin", "go/bin", ".local/bin"} {
		if st, err := os.Stat(filepath.Join(os.Getenv("HOME"), d)); err == nil && st.IsDir() {
			trobats = append(trobats, d)
		}
	}
	if len(trobats) == 0 {
		t.Skip("sense toolchains coneguts en aquesta màquina")
	}
	env := verifyEnv()
	var path string
	for _, kv := range env {
		if strings.HasPrefix(strings.ToUpper(kv), "PATH=") {
			path = kv
		}
	}
	for _, d := range trobats {
		if !strings.Contains(path, filepath.FromSlash(d)) {
			t.Fatalf("el PATH del gate no inclou %s", d)
		}
	}
}

func TestBuildTascaReparacioConteTot(t *testing.T) {
	tasca := buildTascaReparacio("fes X", "go test ./...", 1, "FAIL: tot malament", "deia que anava bé")
	for _, peça := range []string{"fes X", "go test ./...", "1", "FAIL: tot malament", "deia que anava bé", "No afirmis"} {
		if !strings.Contains(tasca, peça) {
			t.Fatalf("a la tasca hi falta %q", peça)
		}
	}
}
