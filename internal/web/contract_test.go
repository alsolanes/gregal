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
	// La secció de Gregal Jocs és d'un altre binari: la comprova
	// jocs/server (TestContracteJocs).
	doc, _ := splitContract(string(raw), "## Gregal Jocs")
	for _, tok := range strings.Fields(strings.ReplaceAll(doc, "`", " ")) {
		tok = strings.Trim(tok, ",.;:()|")
		if !strings.HasPrefix(tok, "/api/") || strings.ContainsAny(tok, "{?*") {
			continue
		}
		if !viva[tok] {
			t.Errorf("el contracte documenta %s, que no existeix", tok)
		}
	}
}

// splitContract separa del document la secció que comença amb heading (fins
// al títol ## següent): torna la resta i la secció.
func splitContract(doc, heading string) (rest, section string) {
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	i := strings.Index(doc, "\n"+heading)
	if i < 0 {
		return doc, ""
	}
	j := strings.Index(doc[i+1:], "\n## ")
	if j < 0 {
		return doc[:i], doc[i:]
	}
	return doc[:i] + doc[i+1+j:], doc[i : i+1+j]
}

// La secció de Gregal Jocs hi ha de ser (la comprova jocs/server) i, a la
// resta del document, cap ruta dels jocs.
func TestContracteJocsApart(t *testing.T) {
	raw, err := os.ReadFile("../../docs/api-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	rest, jocs := splitContract(string(raw), "## Gregal Jocs")
	if !strings.Contains(jocs, "`/api/arena`") || !strings.Contains(jocs, "`/api/football`") {
		t.Fatal("falta la secció «## Gregal Jocs» amb les rutes dels jocs")
	}
	if strings.Contains(rest, "/api/arena") || strings.Contains(rest, "/api/football") {
		t.Fatal("les rutes dels jocs només van a la secció de Gregal Jocs")
	}
}
