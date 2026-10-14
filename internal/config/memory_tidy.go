package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// Higiene de les notes del projecte (.gregal/memory.md): la consolidació
// (gregal memory tidy, botó de la web) proposa una llista nova, l'usuari
// en veu el diff i només llavors s'escriu. ReplaceNotes és l'única
// escriptura i NotesHash evita trepitjar una nota afegida entremig.

// NotesHash és l'empremta del fitxer de notes tal com és ara ("" si no
// n'hi ha).
func NotesHash(cwd string) string {
	p := ProjectMemoryPath(cwd)
	if p == "" {
		return ""
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// ReplaceNotes reescriu les notes del projecte amb aquestes línies (una
// per nota, format "- <data>: <text>"). Les buides s'ignoren.
func ReplaceNotes(cwd string, lines []string) error {
	p := ProjectMemoryPath(cwd)
	if p == "" {
		return errSenseProjecte
	}
	var keep []string
	for _, l := range lines {
		l = strings.TrimSpace(strings.ReplaceAll(l, "\n", " "))
		if l != "" {
			keep = append(keep, l)
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	body := ""
	if len(keep) > 0 {
		body = strings.Join(keep, "\n") + "\n"
	}
	return os.WriteFile(p, []byte(body), 0o600)
}

// IsDuplicateNote diu si l'error de Note/Remember és «ja hi era».
func IsDuplicateNote(err error) bool { return err == errRecordaDuplicat }
