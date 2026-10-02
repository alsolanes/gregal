package web

import (
	"strings"
	"testing"
)

// Les llibreries del render Office han d'anar incrustades al binari
// (go:embed app): si falten, el panell cau al text sense avís visible.
func TestVendorOfficeIncrustat(t *testing.T) {
	fitxers := map[string]string{
		"app/vendor/jszip.min.js":        "JSZip",
		"app/vendor/docx-preview.min.js": "renderAsync",
		"app/vendor/xlsx.mini.min.js":    "sheet_to_json",
	}
	for nom, marca := range fitxers {
		raw, err := assets.ReadFile(nom)
		if err != nil {
			t.Fatalf("%s no incrustat: %v", nom, err)
		}
		if len(raw) < 10000 {
			t.Fatalf("%s sospitosament petit (%d bytes)", nom, len(raw))
		}
		if !strings.Contains(string(raw), marca) {
			t.Fatalf("%s no conté %q", nom, marca)
		}
	}
}
