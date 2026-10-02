package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// Canviar el projecte d'una sessió ha de canviar on treballen les EINES,
// no només els panells de fitxers. Abans no: bash i les eines de fitxers
// anaven al directori del procés, o sigui que una pestanya que havia
// canviat de projecte llegia i executava a l'altre.
func TestEinesSegueixenElWorkspace(t *testing.T) {
	altre := t.TempDir()
	if err := os.WriteFile(filepath.Join(altre, "marca.txt"), []byte("sóc a l'altre projecte"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: &config.Config{}, cwd: altre, mode: "code", id: "s1"}

	// read amb ruta relativa.
	args, _ := json.Marshal(map[string]string{"path": "marca.txt"})
	if out, _ := s.execTool(crida("read", string(args))); !strings.Contains(out, "altre projecte") {
		t.Errorf("read relatiu no va al workspace: %q", out)
	}

	// bash: en comptes de comparar rutes (pwd escriu en estil POSIX sota
	// Git Bash i el test les té en estil Windows), que llegeixi el fitxer
	// que només existeix allà.
	argsB, _ := json.Marshal(map[string]string{"command": "cat marca.txt"})
	if out, _ := s.execTool(crida("bash", string(argsB))); !strings.Contains(out, "altre projecte") {
		t.Errorf("bash no corre al workspace: %q", out)
	}

	// write relatiu ha de caure al workspace, no al directori del procés.
	argsW, _ := json.Marshal(map[string]string{"path": "nou.txt", "content": "hola"})
	s.execTool(crida("write", string(argsW)))
	if _, err := os.Stat(filepath.Join(altre, "nou.txt")); err != nil {
		t.Errorf("write relatiu no ha caigut al workspace: %v", err)
	}

	// glob sense "dir" ha de cercar al projecte, no al del procés.
	argsG, _ := json.Marshal(map[string]string{"pattern": "*.txt"})
	if out, _ := s.execTool(crida("glob", string(argsG))); !strings.Contains(out, "marca.txt") {
		t.Errorf("glob sense dir no cerca al workspace: %q", out)
	}
}

// Sense workspace (TUI, headless) tot ha de seguir com abans: el
// directori del procés.
func TestSenseWorkspaceNoEsToquenLesRutes(t *testing.T) {
	if got := agent.ResolArgsProva("", `{"path":"a.go"}`); got != `{"path":"a.go"}` {
		t.Fatalf("amb dir buit no s'ha de tocar res: %s", got)
	}
}
