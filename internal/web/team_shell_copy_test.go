package web

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sense bwrap, el shell de la colla corre a la còpia del seient: hi pot
// executar proves i escriure-hi, però no instal·lar ni tocar el sistema.
func TestCrewShellInCopyRunsInTheSeatCopy(t *testing.T) {
	if crewShellConfined() {
		t.Skip("aquesta màquina té bwrap: s'hi fa servir el shell aïllat")
	}
	copyDir := t.TempDir()
	b := crewToolBoundary{workspace: copyDir, allowed: []string{"out.txt"}}
	out, _, err := b.execute(context.Background(), "bash", `{"command":"echo hola > out.txt && cat out.txt"}`)
	if err != nil {
		t.Fatalf("shell: %v (%s)", err, out)
	}
	if !strings.Contains(out, "hola") {
		t.Fatalf("sortida = %q", out)
	}
	if raw, err := os.ReadFile(filepath.Join(copyDir, "out.txt")); err != nil || !strings.Contains(string(raw), "hola") {
		t.Fatalf("havia d'escriure a la còpia: %v %q", err, raw)
	}
	if _, _, err := b.execute(context.Background(), "bash", `{"command":"npm install -g cowsay"}`); err == nil || !strings.Contains(err.Error(), "NOT RUN") {
		t.Fatalf("instal·lar global s'havia de denegar: %v", err)
	}
	ro := crewToolBoundary{workspace: copyDir, readOnly: true}
	if _, _, err := ro.execute(context.Background(), "bash", `{"command":"echo x"}`); err == nil {
		t.Fatal("un seient de només lectura no té shell sense aïllament")
	}
}

func TestCrewShellEnvDropsCredentials(t *testing.T) {
	env := crewShellEnv([]string{"PATH=/bin", "OPENAI_API_KEY=x", "GH_TOKEN=y", "GREGAL_API_TOKEN=z", "DB_PASSWORD=w", "HOME=/h"})
	if strings.Join(env, ",") != "PATH=/bin,HOME=/h" {
		t.Fatalf("env = %v", env)
	}
}
