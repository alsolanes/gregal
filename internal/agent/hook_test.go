package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostEditHookAnota(t *testing.T) {
	defer SetPostEditHook("")
	SetPostEditHook("echo toc:{file}")
	dir := t.TempDir()
	p := filepath.Join(dir, "hola.txt")
	out, _, err := Exec("write", `{"path":`+quoteJSON(p)+`,"content":"hola"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "toc:"+p) {
		t.Fatalf("hook sense anotar: %q", out)
	}
}

func TestPostEditHookBuitNoFaRes(t *testing.T) {
	defer SetPostEditHook("")
	SetPostEditHook("")
	dir := t.TempDir()
	p := filepath.Join(dir, "net.txt")
	out, _, err := Exec("write", `{"path":`+quoteJSON(p)+`,"content":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "[hook]") {
		t.Fatalf("sense hook no hi ha d'haver anotació: %q", out)
	}
}

func TestPostEditHookQueFallaNoTrenca(t *testing.T) {
	defer SetPostEditHook("")
	SetPostEditHook("sortir-amb-error-42")
	dir := t.TempDir()
	p := filepath.Join(dir, "falla.txt")
	out, _, err := Exec("write", `{"path":`+quoteJSON(p)+`,"content":"x"}`)
	if err != nil {
		t.Fatalf("el hook no pot fer fallar l'eina: %v", err)
	}
	if !strings.Contains(out, "[hook] avís") {
		t.Fatalf("cal anotar l'avís: %q", out)
	}
	raw, _ := os.ReadFile(p)
	if string(raw) != "x" {
		t.Fatal("el fitxer s'ha d'haver escrit igualment")
	}
}

// quoteJSON escapa de debò: les rutes de t.TempDir() a Windows porten
// barres invertides i un ReplaceAll de cometes en feia un JSON invàlid.
func quoteJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
