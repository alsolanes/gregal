package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
)

// Amb GREGAL_PERSIST_KEYS=1 (el servidor local d'Android) la clau escrita
// per l'API sobreviu al reinici: va a auth.yaml i mai al config.yaml.
func TestProviderKeyPersistsWhenAsked(t *testing.T) {
	s := goalTestServer(t)
	auth := filepath.Join(t.TempDir(), "auth.yaml")
	t.Setenv("GREGAL_AUTH", auth)
	t.Setenv("GREGAL_PERSIST_KEYS", "1")
	name := ""
	for n := range s.cfg.Providers {
		name = n
		break
	}
	if name == "" {
		t.Skip("config without providers")
	}
	if _, err := s.providerAction("key", name, "", "synthetic-key-test-only", "", "", "", false, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := config.LoadAuth()[name]; got != "synthetic-key-test-only" {
		t.Fatalf("auth store: %q", got)
	}
	raw, err := os.ReadFile(s.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "synthetic-key-test-only") {
		t.Fatal("secret written to config.yaml")
	}
}

func TestProviderKeyStaysInMemoryByDefault(t *testing.T) {
	s := goalTestServer(t)
	auth := filepath.Join(t.TempDir(), "auth.yaml")
	t.Setenv("GREGAL_AUTH", auth)
	t.Setenv("GREGAL_PERSIST_KEYS", "")
	for name := range s.cfg.Providers {
		if _, err := s.providerAction("key", name, "", "synthetic-key-test-only", "", "", "", false, 0, 0); err != nil {
			t.Fatal(err)
		}
		break
	}
	if _, err := os.Stat(auth); !os.IsNotExist(err) {
		t.Fatalf("auth store written without opt-in: %v", err)
	}
}
