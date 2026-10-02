package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficeAppendDocx(t *testing.T) {
	p := filepath.Join(t.TempDir(), "informe.docx")
	if _, err := OfficeCreate(p, "Informe", "Primer paràgraf."); err != nil {
		t.Fatal(err)
	}
	msg, err := OfficeAppend(p, "## Conclusions\n- Punt **fort**\n- Punt *feble*\n\nText final.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "4 paràgrafs") {
		t.Fatalf("resum: %s", msg)
	}
	out, err := OfficeRead(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, vol := range []string{"Primer paràgraf.", "Conclusions", "Punt fort", "Punt feble", "Text final."} {
		if !strings.Contains(out, vol) {
			t.Errorf("falta %q a:\n%s", vol, out)
		}
	}
	if strings.Contains(out, "**") || strings.Contains(out, "## ") {
		t.Fatalf("marcadors markdown impresos:\n%s", out)
	}
	// L'ordre es conserva: el que hi havia va primer.
	if strings.Index(out, "Primer paràgraf.") > strings.Index(out, "Conclusions") {
		t.Fatal("l'afegit ha d'anar al final")
	}
	if _, err := OfficeAppend(p, "   "); err == nil {
		t.Fatal("content buit ha de fallar")
	}
}

func TestOfficeAppendPptx(t *testing.T) {
	p := filepath.Join(t.TempDir(), "deck.pptx")
	if _, err := OfficeCreate(p, "Deck", "Diapositiva u."); err != nil {
		t.Fatal(err)
	}
	if _, err := OfficeAppend(p, "- Nou punt\n- Un altre"); err != nil {
		t.Fatal(err)
	}
	out, _ := OfficeRead(p)
	if !strings.Contains(out, "Nou punt") || !strings.Contains(out, "Un altre") || !strings.Contains(out, "Diapositiva u.") {
		t.Fatalf("pptx:\n%s", out)
	}
}

func TestOfficeXlsxSetRange(t *testing.T) {
	p := testXlsx(t)
	msg, err := OfficeXlsxSetRange(p, "Dades", "B3", "Nom | Edat\nAnna | 34\nPere | 41")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "6 cel·les") || !strings.Contains(msg, "B3:C5") {
		t.Fatalf("resum: %s", msg)
	}
	out, _ := OfficeRead(p)
	for _, vol := range []string{"Nom", "Edat", "Anna", "34", "Pere", "41"} {
		if !strings.Contains(out, vol) {
			t.Errorf("falta %q a:\n%s", vol, out)
		}
	}
	if _, err := OfficeXlsxSetRange(p, "Dades", "3B", "x"); err == nil {
		t.Fatal("referència invàlida ha de fallar")
	}
	if _, err := OfficeXlsxSetRange(p, "Dades", "A1", " | "); err == nil {
		t.Fatal("sense valors ha de fallar")
	}
	if c, r, ok := parseCellRef("AB12"); !ok || c != 28 || r != 12 {
		t.Fatalf("parseCellRef: %d %d %v", c, r, ok)
	}
}
