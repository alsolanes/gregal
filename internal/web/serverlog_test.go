package web

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactStripsCredentials(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		bad  []string
	}{
		{"url amb clau", `HTTP 401 a https://api.example.com/v1/chat?api_key=SECRET`, []string{"SECRET", "api.example.com"}},
		{"bearer", `rebutjat: Bearer abcDEF123._-~/+=`, []string{"abcDEF123"}},
		{"sk", `invalid api_key sk-proj-abcdef123456`, []string{"sk-proj"}},
		{"api_key=", `config: api_key = "hunter2"`, []string{"hunter2"}},
		{"token:", `error token: xyz`, []string{"xyz"}},
	} {
		got := redact(tc.in)
		for _, bad := range tc.bad {
			if strings.Contains(got, bad) {
				t.Errorf("%s: filtra %q de %q (got %q)", tc.name, bad, tc.in, got)
			}
		}
	}
	// Text normal intacte: paraules soles sense valor no són secrets.
	for _, in := range []string{"invalid token", "timeout del model", "connexió refusada"} {
		if got := redact(in); got != in {
			t.Errorf("fals positiu: %q -> %q", in, got)
		}
	}
}

func TestPreviewTaskFirstLineCapped(t *testing.T) {
	if got := previewTask("  fes això\nsegona línia secreta"); got != "fes això" {
		t.Fatalf("primera línia: %q", got)
	}
	if got := previewTask(strings.Repeat("a", 200)); len([]rune(got)) > 121 {
		t.Fatalf("sense retall: %d runes", len([]rune(got)))
	}
	if got := previewTask("mira https://x.example/molt/llarg"); strings.Contains(got, "x.example") {
		t.Fatalf("URL al títol: %q", got)
	}
}

func TestTurnOutcomeClassifies(t *testing.T) {
	if turnOutcome(nil) != "done" {
		t.Fatal("nil ha de ser done")
	}
	if turnOutcome(context.Canceled) != "cancelled" {
		t.Fatal("Canceled ha de ser cancelled")
	}
	got := turnOutcome(errors.New("HTTP 500 a https://m.example/k?key=S"))
	if !strings.HasPrefix(got, "failed: ") || strings.Contains(got, "m.example") {
		t.Fatalf("failed sanejador: %q", got)
	}
}

func TestSetupServerLogWritesToFile(t *testing.T) {
	prev := log.Default().Writer()
	defer log.SetOutput(prev)
	path := filepath.Join(t.TempDir(), "gregal.log")
	cleanup, err := SetupServerLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	log.Print("línia de prova del logger")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "línia de prova del logger") {
		t.Fatalf("el fitxer no té la línia: %q", data)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
