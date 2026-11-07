package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Les credencials d'un túnel només valen per al seu propi ID: la config no
// pot barrejar credencials del túnel A amb l'ID del túnel B (cloudflared no
// autenticaria mai, i l'error parlaria de connexió, no de credencials).
func TestCredencialsNoEsBarregenEntreTunnels(t *testing.T) {
	dir := t.TempDir()
	vents := filepath.Join(dir, "vents.json")
	cv := filepath.Join(dir, "cv.json")
	if err := os.WriteFile(vents, []byte(`{"AccountTag":"x","TunnelSecret":"s","TunnelID":"a7558e19-1e93-4a79-bc64-fc564c6ca527"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cv, []byte(`{"AccountTag":"x","TunnelSecret":"s","TunnelID":"187d5e74-003c-4e91-b4dc-b03bdf926a6d"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	ingress := []tunnelRecord{
		{Hostname: "cv.solanes.xyz", Target: "http://127.0.0.1:8096"},
	}

	// Credencials del vents + túnel del CV: no s'han d'escriure.
	t.Setenv("GREGAL_CF_CREDENTIALS", vents)
	cfg := namedTunnelConfig("187d5e74-003c-4e91-b4dc-b03bdf926a6d", ingress)
	if strings.Contains(cfg, "credentials-file") {
		t.Fatalf("credencials del vents escrites a la config del túnel del CV:\n%s", cfg)
	}
	if !strings.Contains(cfg, "tunnel: 187d5e74-003c-4e91-b4dc-b03bdf926a6d") {
		t.Fatalf("falta l'ID del túnel:\n%s", cfg)
	}
	if !strings.Contains(cfg, "hostname: cv.solanes.xyz") || !strings.Contains(cfg, "http_status:404") {
		t.Fatalf("falten ingress o catch-all:\n%s", cfg)
	}

	// Credencials correctes: sí.
	t.Setenv("GREGAL_CF_CREDENTIALS", cv)
	cfg = namedTunnelConfig("187d5e74-003c-4e91-b4dc-b03bdf926a6d", ingress)
	if !strings.Contains(cfg, "credentials-file: "+cv) {
		t.Fatalf("credencials del CV absents:\n%s", cfg)
	}

	// Credencials il·legibles (no existeix): no s'escriuen i no peta.
	t.Setenv("GREGAL_CF_CREDENTIALS", filepath.Join(dir, "nope.json"))
	cfg = namedTunnelConfig("187d5e74-003c-4e91-b4dc-b03bdf926a6d", ingress)
	if strings.Contains(cfg, "credentials-file") {
		t.Fatalf("credencials inexistents escrites:\n%s", cfg)
	}
}
