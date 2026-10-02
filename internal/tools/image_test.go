package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// read_image: png real passa, txt es rebutja, inexistent falla.
func TestReadImage(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "a.png")
	// PNG mínim vàlid (1x1).
	raw := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82,
		0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 144, 119, 83, 222}
	if err := os.WriteFile(png, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	u, err := ReadImageDataURL(png)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "data:image/png;base64,") {
		t.Fatalf("prefix malament: %.30s", u)
	}
	txt := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(txt, []byte("hola"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadImageDataURL(txt); err == nil {
		t.Fatal("un txt no és imatge")
	}
	if _, err := ReadImageDataURL(filepath.Join(dir, "no.png")); err == nil {
		t.Fatal("inexistent ha de fallar")
	}
}

// NormalizeDataURL: vàlida passa, brossa es rebutja.
func TestNormalizeDataURL(t *testing.T) {
	u, err := ReadImageDataURL(filepath.Join("..", "..", "desktop", "build", "icon128.png"))
	if err != nil {
		t.Skip("sense icona de prova")
	}
	n, err := NormalizeDataURL(u, "p")
	if err != nil || n == "" {
		t.Fatalf("vàlida rebutjada: %v", err)
	}
	for _, bad := range []string{"", "hola", "data:text/plain;base64,aGk=", "data:image/png;base64,!!!"} {
		if _, err := NormalizeDataURL(bad, "p"); err == nil {
			t.Fatalf("acceptada brossa: %.30s", bad)
		}
	}
}
