package web

import (
	"os"
	"strings"
	"testing"
)

// El contracte HTTP s'ha de poder llegir sense mirar el codi: cada ruta
// /api registrada ha de sortir a docs/api-contract.md. Fins a la 1.0 el
// document anava per darrere del servidor; aquest test ho impedeix.
func TestContracteAPIDocumentat(t *testing.T) {
	s := hubTestServer(t)
	h := s.Hub()
	h.mux()
	if len(routes) < 20 {
		t.Fatalf("només %d rutes registrades: el mux no s'ha construït", len(routes))
	}
	raw, err := os.ReadFile("../../docs/api-contract.md")
	if err != nil {
		t.Fatalf("no puc llegir el contracte: %v", err)
	}
	doc := string(raw)
	var falten []string
	for _, r := range routes {
		if !strings.Contains(doc, r) {
			falten = append(falten, r)
		}
	}
	if len(falten) > 0 {
		t.Fatalf("rutes sense documentar a docs/api-contract.md: %s", strings.Join(falten, ", "))
	}
}

// I a l'inrevés: el document no ha de prometre rutes que no existeixen.
func TestContracteSenseRutesInventades(t *testing.T) {
	s := hubTestServer(t)
	s.Hub().mux()
	viva := map[string]bool{}
	for _, r := range routes {
		viva[r] = true
	}
	raw, _ := os.ReadFile("../../docs/api-contract.md")
	for _, tok := range strings.Fields(strings.ReplaceAll(string(raw), "`", " ")) {
		tok = strings.Trim(tok, ",.;:()|")
		if !strings.HasPrefix(tok, "/api/") || strings.ContainsAny(tok, "{?*") {
			continue
		}
		if !viva[tok] {
			t.Errorf("el contracte documenta %s, que no existeix", tok)
		}
	}
}
