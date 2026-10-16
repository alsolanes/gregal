package xlsxdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

const modelSpec = `{
 "theme": "default",
 "sheets": [{
  "name": "Model",
  "rows": [
   ["Concepte", "2025", "2026", "2027"],
   ["Vendes", 1000, "=B2*(1+$B$7)", "=C2*(1+$B$7)"],
   ["Costos", 600, "=B3*1.05", "=C3*1.05"],
   ["Marge", "=B2-B3", "=C2-C3", "=D2-D3"],
   ["Marge %", "=B4/B2", "=C4/C2", "=D4/D2"],
   [null],
   ["Creixement", 0.1],
   ["Ràtio dolenta", "=B2/0"]
  ],
  "number_formats": {"B2:D4": "#,##0 €", "B5:D5": "0.0%", "B7": "0.0%"},
  "roles": {"B2:B3": "input", "B7": "key", "A4:D4": "total"},
  "comments": {"B7": "Hipòtesi: creixement anual de vendes."},
  "charts": [{"type": "col", "title": "Vendes i costos", "categories": "Model!$B$1:$D$1", "series": [{"name": "Model!$A$2", "values": "Model!$B$2:$D$2"}, {"name": "Model!$A$3", "values": "Model!$B$3:$D$3"}]}]
 }]
}`

func TestCreateModelAndRecalc(t *testing.T) {
	dir := t.TempDir()
	if out := os.Getenv("GREGAL_OOXML_OUT"); out != "" {
		dir = out
	}
	path := filepath.Join(dir, "model.xlsx")
	os.Remove(path)
	sp, err := ParseSpec(modelSpec)
	if err != nil {
		t.Fatal(err)
	}
	warns, err := Create(path, "", sp)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) > 0 {
		t.Fatalf("avisos: %v", warns)
	}
	rep, err := Recalc(path)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Formulas != 11 {
		t.Errorf("fórmules: %d", rep.Formulas)
	}
	if got := rep.Errors["#DIV/0!"]; len(got) != 1 || got[0] != "Model!B8" {
		t.Errorf("errors: %v", rep.Errors)
	}
	txt, err := Read(path, 4, 60, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, "=B2-B3 → 400") {
		t.Errorf("lectura sense fórmula avaluada:\n%s", txt)
	}
	f, _ := excelize.OpenFile(path)
	defer f.Close()
	st, _ := f.GetCellStyle("Model", "B2")
	s, _ := f.GetStyle(st)
	if s.Font == nil || !strings.EqualFold(s.Font.Color, "0000FF") || s.CustomNumFmt == nil {
		t.Errorf("B2 hauria de ser entrada blava amb format: %+v", s)
	}
}

func TestEditRealLibreOfficeXlsx(t *testing.T) {
	src, err := os.ReadFile("../ooxml/testdata/lo.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "lo.xlsx")
	os.WriteFile(path, src, 0o644)
	if _, err := SetCells(path, "", map[string]string{"B2": "15", "D1": "Triple", "D2": "=B2*3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SetRange(path, "", "A6", "| Gamma | 7 | =B6*2 |\n| Delta | 1.234,5 | 12% |"); err != nil {
		t.Fatal(err)
	}
	rep, err := Recalc(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Errors) > 0 {
		t.Fatalf("errors: %v", rep)
	}
	f, _ := excelize.OpenFile(path)
	defer f.Close()
	if v, _ := f.CalcCellValue(f.GetSheetList()[0], "D2"); v != "45" {
		t.Errorf("D2=%q", v)
	}
	if v, _ := f.GetCellValue(f.GetSheetList()[0], "B7", excelize.Options{RawCellValue: true}); v != "1234.5" {
		t.Errorf("B7=%q", v)
	}
	// La capçalera original es conserva.
	if v, _ := f.GetCellValue(f.GetSheetList()[0], "A1"); v != "Nom" {
		t.Errorf("A1=%q", v)
	}
}
