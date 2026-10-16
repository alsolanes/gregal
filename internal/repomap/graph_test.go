package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// El fitxer que la tasca esmenta fa servir un tipus definit en un altre
// fitxer que no comparteix cap paraula amb la tasca: el graf l'ha de dur
// al mapa per davant de fitxers aliens.
func TestRankBringsDependencies(t *testing.T) {
	dir := t.TempDir()
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644)
	}
	write("checkout.go", "package shop\n\nfunc ProcessCheckout(c Cart) error {\n\treturn LedgerWriter{}.Record(c.Total)\n}\n")
	write("ledger.go", "package shop\n\ntype LedgerWriter struct{}\n\nfunc (LedgerWriter) Record(v int) error { return nil }\n")
	write("cart.go", "package shop\n\ntype Cart struct{ Total int }\n")
	for i := 0; i < 12; i++ {
		// Noms anteriors alfabèticament i més API pública: sense el graf,
		// guanyarien l'empat a ledger.go.
		l := string(rune('A' + i))
		write(filepath.Join("aux", "u"+l+".go"), "package aux\n\nfunc HelperNumber"+l+"() int { return 1 }\nfunc OtherThing"+l+"() int { return 2 }\nfunc ThirdThing"+l+"() int { return 3 }\n")
	}
	snap := Build(dir, nil, Options{})
	r := Rank(snap, "fix ProcessCheckout when the total is negative", RankOptions{Tokens: 150})
	if !strings.Contains(r.Text, "checkout.go") {
		t.Fatalf("falta el fitxer directe:\n%s", r.Text)
	}
	iLedger := strings.Index(r.Text, "ledger.go")
	iMisc := strings.Index(r.Text, "aux/")
	if iLedger < 0 {
		t.Fatalf("la dependència no hi és:\n%s", r.Text)
	}
	if iMisc >= 0 && iMisc < iLedger {
		t.Fatalf("un fitxer aliè surt abans que la dependència:\n%s", r.Text)
	}
}
