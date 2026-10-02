package agent

import (
	"gregal/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMem(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadProjectMemory(t *testing.T) {
	dir := t.TempDir()
	if got := LoadProjectMemory(dir); got != "" {
		t.Fatalf("sense fitxer ha de ser buit: %q", got)
	}
	writeMem(t, dir, "AGENTS.md", "# Regles\nNo facis X.")
	got := LoadProjectMemory(dir)
	if !strings.Contains(got, "AGENTS.md") || !strings.Contains(got, "No facis X.") {
		t.Fatalf("hauria d'incloure el contingut: %q", got)
	}
	if !strings.Contains(got, dir) {
		t.Fatalf("hauria d'ancorar el directori: %q", got)
	}
	// CLAUDE.md també val si no hi ha AGENTS.md.
	dir2 := t.TempDir()
	writeMem(t, dir2, "CLAUDE.md", "Fes Y.")
	if got := LoadProjectMemory(dir2); !strings.Contains(got, "Fes Y.") {
		t.Fatalf("CLAUDE.md hauria de valer: %q", got)
	}
	// AGENTS.md guanya.
	writeMem(t, dir2, "AGENTS.md", "Mana AGENTS.")
	if got := LoadProjectMemory(dir2); !strings.Contains(got, "Mana AGENTS.") {
		t.Fatalf("AGENTS.md té prioritat: %q", got)
	}
	// Buits i cwd buit no aporten res.
	dir3 := t.TempDir()
	writeMem(t, dir3, "AGENTS.md", "   \n ")
	if got := LoadProjectMemory(dir3); got != "" {
		t.Fatalf("fitxer buit = sense memòria: %q", got)
	}
	if got := LoadProjectMemory(""); got != "" {
		t.Fatalf("cwd buit = sense memòria: %q", got)
	}
	// Topall de mida.
	dir4 := t.TempDir()
	writeMem(t, dir4, "AGENTS.md", strings.Repeat("regla ", 2000))
	got4 := LoadProjectMemory(dir4)
	if len([]rune(got4)) > MaxMemoryChars+200 || !strings.Contains(got4, "retallat") {
		t.Fatalf("hauria de retallar: %d runes", len([]rune(got4)))
	}
}

func TestLoadProjectMemoryAmbNotes(t *testing.T) {
	dir := t.TempDir()
	if err := config.Note(dir, "fet de projecte PROVA456"); err != nil {
		t.Fatal(err)
	}
	got := LoadProjectMemory(dir)
	if !strings.Contains(got, "NOTES DEL PROJECTE") || !strings.Contains(got, "PROVA456") {
		t.Fatalf("falta el bloc de notes: %q", got)
	}
}
