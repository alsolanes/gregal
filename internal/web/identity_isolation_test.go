package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQ1UserStateExactIdentity(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	seen := map[string]string{}
	for _, name := range []string{"anna_b", "anna-b", "Anna", "anna", "../anna", "à", "a", "", " ", "anna ", strings.Repeat("A", 512)} {
		dir := userStateDir(cfg, name)
		if other, ok := seen[dir]; ok {
			t.Errorf("identities %q and %q share %s", other, name, dir)
		}
		seen[dir] = name
		if dir != userStateDir(cfg, name) {
			t.Errorf("unstable directory for %q", name)
		}
		rel, err := filepath.Rel(filepath.Join(filepath.Dir(cfg), "users"), dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			t.Errorf("unsafe user directory for %q: %s (%v)", name, dir, err)
		}
		if len(filepath.Base(dir)) > 255 {
			t.Errorf("identity exceeds filesystem component limit: %q", name)
		}
	}
}

func TestQ1AmbiguousLegacyStateIsNotClaimed(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	legacy := filepath.Join(filepath.Dir(cfg), "users", "anna-b")
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(legacy, "memory.md")
	if err := os.WriteFile(path, []byte("unverified-owner"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"anna_b", "anna-b"} {
		if dir := userStateDir(cfg, name); dir == legacy {
			t.Errorf("ambiguous old directory assigned to %q", name)
		}
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "unverified-owner" {
		t.Fatalf("legacy data must remain untouched for manual recovery: %q %v", raw, err)
	}
}
