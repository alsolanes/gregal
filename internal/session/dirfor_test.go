package session

import (
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/identity"
)

// TestDirForUsuari: cada usuari té la seva carpeta i el cas «sense usuari»
// torna la carpeta de sempre. El parany: slugify("") torna "sessio", i si
// la comprovació es fa després, la carpeta del servidor local acaba sent
// sessions/sessio.
func TestDirForUsuari(t *testing.T) {
	base := DefaultDir()
	if got := DirFor(""); got != base {
		t.Fatalf("sense usuari hauria de ser %s, és %s", base, got)
	}
	if got := DirFor("   "); got == base {
		t.Fatal("una identitat no buida no pot apuntar a l'estat local")
	}
	cas := map[string]string{"userb": identity.Key("userb"), "Userb": identity.Key("Userb"), "Userb M.": identity.Key("Userb M.")}
	for in, want := range cas {
		got := DirFor(in)
		if filepath.Base(got) != want {
			t.Fatalf("DirFor(%q) = %s, esperava %s", in, got, want)
		}
		if !strings.HasPrefix(got, base+string(filepath.Separator)) {
			t.Fatalf("DirFor(%q) hauria de ser dins %s: %s", in, base, got)
		}
	}
	// Dues persones diferents no poden compartir carpeta.
	if DirFor("userb") == DirFor("usera") {
		t.Fatal("dos usuaris no poden tenir la mateixa carpeta")
	}
}
