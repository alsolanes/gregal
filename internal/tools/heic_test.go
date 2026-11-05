package tools

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// Una foto HEIC (la de mostra és de github.com/gen2brain/heic, MIT) entra com
// a JPEG: és el que els models accepten.
func TestHEICBecomesJPEG(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample.heic")
	if err != nil {
		t.Fatal(err)
	}
	if !IsHEIF(raw) {
		t.Fatal("la mostra hauria de detectar-se com a HEIF")
	}
	u, err := DataURLForBytes(raw, "foto")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "data:image/jpeg;base64,") {
		t.Fatalf("esperava JPEG, tinc %.40s", u)
	}
	// I també quan arriba com a data URL del navegador o de l'app.
	n, err := NormalizeDataURL("data:image/heic;base64,"+base64.StdEncoding.EncodeToString(raw), "imatge 1")
	if err != nil || !strings.HasPrefix(n, "data:image/jpeg;") {
		t.Fatalf("data URL HEIC: %v %.40s", err, n)
	}
}

func TestIsHEIFRejectsOthers(t *testing.T) {
	if IsHEIF([]byte("\x89PNG\r\n\x1a\n0000")) || IsHEIF([]byte("\x00\x00\x00\x18ftypmp42")) || IsHEIF(nil) {
		t.Fatal("PNG, MP4 o buit no són HEIF")
	}
}

func TestBrokenHEICIsAClearError(t *testing.T) {
	raw := []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic trencat")
	if _, err := DataURLForBytes(raw, "foto"); err == nil || !strings.Contains(err.Error(), "HEIC") {
		t.Fatalf("esperava un error clar, tinc %v", err)
	}
}
