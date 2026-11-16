package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Una cerca des de la carpeta personal no ha d'entrar a Nextcloud (obrir-hi
// fitxers els baixa) ni obrir fotos; si la cerca és a dins, sí que hi busca.
func TestGrepNoEntraAlNuvolNiObreFotos(t *testing.T) {
	home := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("notes/llista.txt", "comprar pa\n")
	write("Nextcloud/.Contacts-Backup/a.vcf", "comprar llet\n")
	write("OneDrive - Feina/doc.txt", "comprar cafè\n")
	write("fotos/IMG_1.jpg", "comprar ous\n") // text, però amb nom de foto

	got, err := Grep("comprar", home, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "llista.txt") {
		t.Fatalf("ha de trobar el fitxer normal:\n%s", got)
	}
	for _, no := range []string{"a.vcf", "doc.txt", "IMG_1.jpg"} {
		if strings.Contains(got, no) {
			t.Fatalf("no havia d'obrir %s:\n%s", no, got)
		}
	}
	// Demanat expressament dins de la carpeta del núvol: hi busca.
	got, _ = Grep("llet", filepath.Join(home, "Nextcloud"), "", 50)
	if !strings.Contains(got, "a.vcf") {
		t.Fatalf("dins del núvol, si s'hi demana, sí:\n%s", got)
	}
	if !isCloudDir("OneDrive - Empresa") || isCloudDir("projectes") {
		t.Fatal("isCloudDir")
	}
}
