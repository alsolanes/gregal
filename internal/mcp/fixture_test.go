package mcp

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestEcoFixture parla amb el servidor d'exemple del repo (sense fakes
// generats al test): si el protocol canvia, aquest test cau.
func TestEcoFixture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el servidor d'exemple s'executa pel shebang: només POSIX")
	}
	_, this, _, _ := runtime.Caller(0)
	eco := filepath.Join(filepath.Dir(this), "..", "..", "examples", "mcp-eco.py")
	c, err := Dial("eco", Server{Command: eco})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	tools, err := c.Tools()
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Spec.Name != "mcp_eco_eco" {
		t.Fatalf("tools=%+v", tools)
	}
	out, err := tools[0].Call(`{"text":"ping"}`)
	if err != nil || out != "ECO: ping" {
		t.Fatalf("call=%q err=%v", out, err)
	}
}
