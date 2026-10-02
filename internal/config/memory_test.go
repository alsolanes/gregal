package config

import (
	"strings"
	"testing"
)

func memTmp(t *testing.T) {
	t.Helper()
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
}

func TestRememberRecupera(t *testing.T) {
	memTmp(t)
	if err := Remember("m'agrada el verd"); err != nil {
		t.Fatal(err)
	}
	if err := Remember("m'agrada el verd"); err == nil {
		t.Fatal("duplicat hauria de fallar")
	}
	lines := Recall("verd")
	if len(lines) != 1 || !strings.Contains(lines[0], "m'agrada el verd") {
		t.Fatalf("recall: %v", lines)
	}
	if got := LoadUserMemory(); !strings.Contains(got, "m'agrada el verd") {
		t.Fatalf("load: %q", got)
	}
}

func TestForgetEsborra(t *testing.T) {
	memTmp(t)
	Remember("el cotxe és blau")
	Remember("la casa és verda")
	n, err := Forget("cotxe")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if lines := Recall(""); len(lines) != 1 || !strings.Contains(lines[0], "casa") {
		t.Fatalf("queden: %v", lines)
	}
	if _, err := Forget(""); err == nil {
		t.Fatal("oblidar sense terme hauria de fallar")
	}
}

func TestSystemPromptInclouMemoria(t *testing.T) {
	memTmp(t)
	c := &Config{}
	if strings.Contains(c.SystemPrompt(), "MEMÒRIA D'USUARI") {
		t.Fatal("sense memòria no hi ha bloc")
	}
	Remember("fato estable")
	if !strings.Contains(c.SystemPrompt(), "fato estable") {
		t.Fatal("el prompt ha d'incloure la memòria")
	}
}

func TestNotesProjecte(t *testing.T) {
	memTmp(t)
	cwd := t.TempDir()
	if err := Note(cwd, "deploy amb ./ship.sh"); err != nil {
		t.Fatal(err)
	}
	if err := Note("", "x"); err == nil {
		t.Fatal("sense cwd hauria de fallar")
	}
	lines := Notes(cwd, "ship")
	if len(lines) != 1 || !strings.Contains(lines[0], "ship.sh") {
		t.Fatalf("notes: %v", lines)
	}
	n, err := ForgetNote(cwd, "ship")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if lines := Notes(cwd, ""); len(lines) != 0 {
		t.Fatalf("resten: %v", lines)
	}
}
