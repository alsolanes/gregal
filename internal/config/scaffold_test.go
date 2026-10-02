package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "config.yaml")
	got, err := InitConfig(p, false)
	if err != nil || got != p {
		t.Fatalf("init=%q err=%v", got, err)
	}
	if _, _, err := Load(p); err != nil {
		t.Fatalf("el generat no valida: %v", err)
	}
	if _, err := InitConfig(p, false); err == nil {
		t.Fatal("sense --force hauria de refusar")
	}
	if _, err := InitConfig(p, true); err != nil {
		t.Fatalf("amb --force: %v", err)
	}
	_ = os.Remove(p)
}
