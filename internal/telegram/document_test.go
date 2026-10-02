package telegram

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSafeFileNameTreutravessers: el nom del fitxer el tria qui envia el
// fitxer, i amb un nom com «../../.ssh/authorized_keys» s'escriuria fora de
// la carpeta. Mai ha de quedar cap separador ni cap punt de navegacio.
func TestSafeFileNameTreutravessers(t *testing.T) {
	casos := map[string]string{
		"../../.ssh/authorized_keys": "authorized_keys.pdf",
		"/etc/passwd":                "passwd.pdf",
		`C:\Windows\system32\cfg`:    "cfg.pdf",
		// Sense nom utilitzable queda el file_id, i com que no hi ha punt,
		// s'hi afegeix l'extensio del tipus.
		"..":               "document-fileid-123.pdf",
		"":                 "document-fileid-123.pdf",
		"informe 2026.pdf": "informe 2026.pdf",
		"nota.txt":         "nota.txt",
	}
	for in, want := range casos {
		got := safeFileName(in, "fileid-123", "application/pdf")
		if got != want {
			t.Fatalf("safeFileName(%q) = %q, esperava %q", in, got, want)
		}
		if strings.ContainsAny(got, `/\`) || strings.HasPrefix(got, ".") {
			t.Fatalf("safeFileName(%q) ha deixat un nom perillos: %q", in, got)
		}
	}
}

// TestSafeFileNamePosaExtensio: sense extensio, les eines del sistema
// (pdftotext, unzip) no saben que fer-hi. Si el client envia el tipus, se'n
// dedueix l'extensio.
func TestSafeFileNamePosaExtensio(t *testing.T) {
	if got := safeFileName("informe", "id", "application/pdf"); got != "informe.pdf" {
		t.Fatalf("PDF sense extensio: %q", got)
	}
	if got := safeFileName("full", "id", "text/plain"); got != "full.txt" {
		t.Fatalf("text sense extensio: %q", got)
	}
	if got := safeFileName("ja-te.pdf", "id", "application/pdf"); got != "ja-te.pdf" {
		t.Fatalf("amb extensio no se n'ha d'afegir cap: %q", got)
	}
}

// TestSaveDocumentDinsLaCarpeta: el fitxer ha d'acabar dins la carpeta del
// xat, amb permisos de nomes l'amo, i el torn ha de rebre una ruta que
// existeix.
func TestSaveDocumentDinsLaCarpeta(t *testing.T) {
	dir := t.TempDir()
	b := &Bot{cwd: dir}
	doc := &Document{FileID: "abc123", FileName: "../../fora.pdf", MimeType: "application/pdf", FileSize: 4}
	st := &chatState{}
	abs, err := b.saveDocument(st, 42, doc, []byte("hola"))
	if err != nil {
		t.Fatal(err)
	}
	esperat := filepath.Join(dir, ".gregal", "documents", "42", "fora.pdf")
	if abs != esperat {
		t.Fatalf("ruta = %s, esperava %s", abs, esperat)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("el fitxer no s'ha desat: %v", err)
	}
	if string(raw) != "hola" {
		t.Fatalf("contingut = %q", raw)
	}
	// Els permisos POSIX no existeixen a Windows: allà Stat().Perm() torna
	// 0666 encara que el fitxer es creï amb 0600. La garantia és de Unix.
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(abs); fi.Mode().Perm() != 0o600 {
			t.Fatalf("permisos = %v, esperava 0600", fi.Mode().Perm())
		}
	}
	// I res no ha d'haver sortit de la carpeta.
	if _, err := os.Stat(filepath.Join(dir, "..", "fora.pdf")); err == nil {
		t.Fatal("s'ha escrit fora de la carpeta!")
	}
	if rel := b.relative(st, abs); strings.HasPrefix(rel, "..") {
		t.Fatalf("la ruta relativa no hauria de sortir: %q", rel)
	}
	// Amb compte lligat, els documents van a la carpeta de l'usuari.
	stUser := &chatState{user: "userb", dir: dir}
	if got := b.documentDir(stUser, 42); got != filepath.Join(dir, ".gregal", "documents", "usuari-userb") {
		t.Fatalf("documentDir usuari = %q", got)
	}
}

// TestHumanBytes: el missatge de límit de mida s'ha de llegir bé.
func TestHumanBytes(t *testing.T) {
	casos := map[int]string{0: "0 B", 512: "512 B", 2048: "2.0 KB", 21 << 20: "21.0 MB"}
	for in, want := range casos {
		if got := humanBytes(in); got != want {
			t.Fatalf("humanBytes(%d) = %q, esperava %q", in, got, want)
		}
	}
}
