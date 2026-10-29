package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Els patrons: amb barra manen sobre el camí sencer (i la carpeta), sense
// barra sobre el nom del fitxer. És el que fa intuïtiu escriure el fitxer de
// notes sense haver d'encertar la profunditat de la carpeta.
func TestEncaixaPatrons(t *testing.T) {
	casos := []struct {
		patro, cami string
		vol         bool
	}{
		{"internal/web/*", "internal/web/serve.go", true},
		{"internal/web/*", "internal/web/sub/x.go", true},
		{"internal/web/*", "internal/agent/loop.go", false},
		{"*.kt", "android/app/src/main/java/xyz/solanes/gregal/PrefsScreen.kt", true},
		{"*.kt", "internal/agent/loop.go", false},
		{"internal/agent/loop.go", "internal/agent/loop.go", true},
		{"internal/agent/loop.go", "internal/agent/pathnotes.go", false},
		{"serve.go", "internal/web/serve.go", true},
		{"", "internal/web/serve.go", false},
		{"internal/web/*", "", false},
	}
	for _, c := range casos {
		if got := encaixa(c.patro, c.cami); got != c.vol {
			t.Fatalf("encaixa(%q, %q) = %v, volia %v", c.patro, c.cami, got, c.vol)
		}
	}
}

// Una nota que encaixa s'enganxa al resultat, i NO es repeteix a la segona
// lectura del mateix fitxer: si es repetís a cada `read`, el resultat es
// tornaria sopa i el model pagaria tokens per res.
func TestNotesSInjectenUnCopPerSessio(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".gregal"), 0o755); err != nil {
		t.Fatal(err)
	}
	notes := `notes:
  - match: "internal/web/*"
    note: "les rutes es declaren a handle(), no a extras.go"
  - match: "*.go"
    note: "el codi va amb gofmt i els tests al costat del fitxer"
  - match: "serve.go"
    note: "el bucle no pot cridar el model sense comptar el pas"
  - match: "internal/agent/*"
    note: "quarta nota que no ha de sortir (topall de 3)"
`
	if err := os.WriteFile(filepath.Join(ws, ".gregal", "notes.yaml"), []byte(notes), 0o600); err != nil {
		t.Fatal(err)
	}
	args := `{"path":"internal/web/serve.go"}`

	bloc := NotaNotes("s1", ws, "read", args)
	if !strings.Contains(bloc, "[notes] internal/web/serve.go") {
		t.Fatalf("el bloc ha de començar pel camí, i és %q", bloc)
	}
	if !strings.Contains(bloc, "handle()") || !strings.Contains(bloc, "gofmt") {
		t.Fatalf("hi han de ser les notes que encaixen, i és %q", bloc)
	}
	if strings.Contains(bloc, "topall de 3") {
		t.Fatalf("el topall de %d notes no s'ha respectat: %q", notesMaxPerInjeccio, bloc)
	}
	if segona := NotaNotes("s1", ws, "read", args); segona != "" {
		t.Fatalf("la segona lectura no ha de repetir les notes, i torna %q", segona)
	}
	// Una altra sessió sí que les ha de rebre.
	if altres := NotaNotes("s2", ws, "read", args); altres == "" {
		t.Fatal("una sessió nova ha de rebre les notes")
	}
}

// Les eines que no toquen cap fitxer concret no porten notes, i un workspace
// sense fitxer de notes tampoc no ha de petar.
func TestNotesNomesAmbFitxer(t *testing.T) {
	ws := t.TempDir()
	if got := NotaNotes("s3", ws, "read", `{"path":"x.go"}`); got != "" {
		t.Fatalf("sense .gregal/notes.yaml no hi ha notes, i torna %q", got)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".gregal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".gregal", "notes.yaml"),
		[]byte("notes:\n  - match: \"*.go\"\n    note: \"prova\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, eina := range []string{"grep", "glob", "bash", "delegate"} {
		if got := NotaNotes("s4", ws, eina, `{"pattern":"x","dir":"."}`); got != "" {
			t.Fatalf("%s no ha de portar notes, i torna %q", eina, got)
		}
	}
}

// El .local.yaml és per a coses d'aquesta màquina i s'ajunta amb el versionat.
func TestNotesLocalsSajunten(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".gregal"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(ws, ".gregal", "notes.yaml"),
		[]byte("notes:\n  - match: \"*.go\"\n    note: \"del repo\"\n"), 0o600)
	os.WriteFile(filepath.Join(ws, ".gregal", "notes.local.yaml"),
		[]byte("notes:\n  - match: \"*.go\"\n    note: \"d'aquesta maquina\"\n"), 0o600)
	bloc := NotaNotes("s5", ws, "read", `{"path":"main.go"}`)
	if !strings.Contains(bloc, "del repo") || !strings.Contains(bloc, "aquesta maquina") {
		t.Fatalf("les dues notes hi han de ser, i és %q", bloc)
	}
}

// La prova del cablejat: no n'hi ha prou que NotaNotes torni el bloc, ha
// d'acabar DINS del resultat que rep el model. ExecIn és la porta que fan
// servir el TUI, la web, el headless i el Telegram.
func TestNotesArribenAlResultatDeLEina(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".gregal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "internal", "peça.go"), []byte("package x\n// una línia\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".gregal", "notes.yaml"),
		[]byte("notes:\n  - match: \"internal/*\"\n    note: \"aquesta nota ha d'arribar al model\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := ExecIn("sessio-notes", ws, "read", `{"path":"internal/peça.go"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(out, "[notes]") || !strings.Contains(out, "ha d'arribar al model") {
		t.Fatalf("el resultat de l'eina ha de portar la nota, i és:\n%s", out)
	}
	// I la segona lectura no la repeteix.
	out2, _, err := ExecIn("sessio-notes", ws, "read", `{"path":"internal/peça.go"}`)
	if err != nil {
		t.Fatalf("segon read: %v", err)
	}
	if strings.Contains(out2, "[notes]") {
		t.Fatalf("la segona lectura no ha de repetir la nota, i és:\n%s", out2)
	}
}
