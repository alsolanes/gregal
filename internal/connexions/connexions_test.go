package connexions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVaultEncryptsPerOwnerAndSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	v := OpenVault(dir)
	ref := NewAuthRef()
	if !strings.HasPrefix(ref, "auth:") {
		t.Fatalf("authRef opac: %q", ref)
	}
	if err := v.Put("anna", ref, []byte("app-password-123")); err != nil {
		t.Fatal(err)
	}
	// Res en clar al disc.
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if strings.Contains(string(b), "app-password-123") || strings.Contains(string(b), "anna") {
			t.Fatalf("%s porta el secret o el nom en clar", f)
		}
	}
	// Un altre procés (mateixa clau a disc) el llegeix.
	got, err := OpenVault(dir).Get("anna", ref)
	if err != nil || string(got) != "app-password-123" {
		t.Fatalf("reobrir: %q %v", got, err)
	}
	// Un altre owner no el té ni copiant-ne el fitxer.
	if _, err := v.Get("bernat", ref); err == nil {
		t.Fatal("un altre owner no pot llegir el secret")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, ownerFile("anna")))
	if err := os.WriteFile(filepath.Join(dir, ownerFile("bernat")), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenVault(dir).Get("bernat", ref); err == nil {
		t.Fatal("un xifrat copiat al fitxer d'un altre owner no s'ha de desxifrar")
	}
	if err := v.Delete("anna", ref); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get("anna", ref); err != ErrNoSecret {
		t.Fatalf("esborrat: %v", err)
	}
}

func TestVaultNeverOverwritesAnUnreadableKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vault.key"), []byte("not-a-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := OpenVault(dir).Put("", NewAuthRef(), []byte("x")); err == nil {
		t.Fatal("amb una clau il·legible s'ha de fallar, no generar-ne una altra")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "vault.key"))
	if string(b) != "not-a-key" {
		t.Fatal("la clau no s'ha de tocar")
	}
}

func TestPendingIsSingleUseAndExpires(t *testing.T) {
	v := OpenVault(t.TempDir())
	type p struct{ Owner, Verifier string }
	if err := v.PutPending("st-1", p{"anna", "verif"}); err != nil {
		t.Fatal(err)
	}
	if !v.HasPending("st-1") || v.HasPending("st-2") {
		t.Fatal("HasPending")
	}
	var got p
	if err := v.TakePending("st-1", &got); err != nil || got.Verifier != "verif" {
		t.Fatalf("take: %+v %v", got, err)
	}
	if err := v.TakePending("st-1", &got); err == nil {
		t.Fatal("d'un sol ús")
	}
	old := PendingTTL
	PendingTTL = time.Millisecond
	t.Cleanup(func() { PendingTTL = old })
	_ = v.PutPending("st-3", p{"anna", "x"})
	time.Sleep(5 * time.Millisecond)
	if err := v.TakePending("st-3", &got); err == nil {
		t.Fatal("caducada")
	}
}

func TestStoreOwnerAndPermissions(t *testing.T) {
	dir := t.TempDir()
	a := Open(dir, "anna")
	c, err := a.Put(Connexio{Service: ServiceMCP, Method: MethodAPI, Provider: "notes", AccountLabel: "feina",
		Capabilities: []string{CapRead, CapSend}})
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(c.ID) || !strings.HasPrefix(c.AuthRef, "auth:") || c.Status != Available || c.Owner != "anna" {
		t.Fatalf("connexió nova: %+v", c)
	}
	if c.Can(CapSend) || !c.Can(CapRead) {
		t.Fatalf("per defecte la persona només autoritza llegir: %v", c.Effective())
	}
	// El mateix fitxer llegit com a un altre owner no dona res.
	if got := Open(dir, "bernat").List(); len(got) != 0 {
		t.Fatalf("un altre owner veu %d connexions", len(got))
	}
	pub := c.Public()
	for _, k := range []string{"id", "owner", "method", "accountLabel", "status", "capabilities", "authRef"} {
		if _, ok := pub[k]; !ok {
			t.Fatalf("falta %s al payload: %v", k, pub)
		}
	}
}
