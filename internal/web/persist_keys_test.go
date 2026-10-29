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

// La clau escrita a l'app es retroba després de reiniciar (tornar a
// carregar el config) i desapareix quan l'usuari la buida.
func TestProviderKeySurvivesRestartUntilRemoved(t *testing.T) {
	s := goalTestServer(t)
	auth := filepath.Join(t.TempDir(), "auth.yaml")
	t.Setenv("GREGAL_AUTH", auth)
	t.Setenv("GREGAL_PERSIST_KEYS", "1")
	msg, err := s.providerAction("key", "p", "", "synthetic-key-test-only", "", "", "", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg, "memory") {
		t.Fatalf("message still says memory only: %q", msg)
	}
	cfg, _, err := config.Load(s.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers["p"].APIKey; got != "synthetic-key-test-only" {
		t.Fatalf("key after restart: %q", got)
	}

	if _, err := s.providerAction("key", "p", "", "", "", "", "", false, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := config.LoadAuth()["p"]; ok {
		t.Fatal("removed key still in the auth store")
	}
	cfg, _, err = config.Load(s.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers["p"].APIKey; got != "" {
		t.Fatalf("removed key came back after restart: %q", got)
	}
}

// Passar a ${VAR} també oblida la clau desada: ja no és la que mana.
func TestProviderKeyEnvRefForgetsStoredKey(t *testing.T) {
	s := goalTestServer(t)
	t.Setenv("GREGAL_AUTH", filepath.Join(t.TempDir(), "auth.yaml"))
	t.Setenv("GREGAL_PERSIST_KEYS", "1")
	if _, err := s.providerAction("key", "p", "", "synthetic-key-test-only", "", "", "", false, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.providerAction("key", "p", "", "${GREGAL_TEST_UNSET_KEY}", "", "", "", false, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := config.LoadAuth()["p"]; ok {
		t.Fatal("stored key kept after switching to ${VAR}")
	}
}

// Afegir un provider amb clau literal la desa al magatzem, no al config, i
// esborrar el provider n'esborra la clau.
func TestProviderAddAndRemoveUseKeyStore(t *testing.T) {
	s := goalTestServer(t)
	t.Setenv("GREGAL_AUTH", filepath.Join(t.TempDir(), "auth.yaml"))
	t.Setenv("GREGAL_PERSIST_KEYS", "1")
	if _, err := s.providerAction("add", "extra", "http://127.0.0.1:1/v1", "synthetic-key-test-only", "", "", "", false, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := config.LoadAuth()["extra"]; got != "synthetic-key-test-only" {
		t.Fatalf("auth store: %q", got)
	}
	raw, err := os.ReadFile(s.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "synthetic-key-test-only") {
		t.Fatal("secret written to config.yaml")
	}
	if _, err := s.providerAction("rm", "extra", "", "", "", "", "", false, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := config.LoadAuth()["extra"]; ok {
		t.Fatal("deleted provider's key still in the auth store")
	}
}
