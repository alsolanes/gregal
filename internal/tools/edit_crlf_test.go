package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fitxer(t *testing.T, contingut string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(p, []byte(contingut), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Un fitxer sortit d'un git amb core.autocrlf a Windows —aquest repositori
// mateix— té \r\n, i el model escriu el bloc amb \n, que és el que fa
// tothom. La comparació exacta fallava sempre, i el que rebia el model era
// «bloc no trobat»: ni una pista, així que tornava a provar-ho igual o
// reescrivia el fitxer sencer.
func TestEditFuncionaEnFitxersAmbFinalsDeLiniaDeWindows(t *testing.T) {
	p := fitxer(t, "package a\r\n\r\nfunc F() int {\r\n\treturn 1\r\n}\r\n")
	if err := Edit(p, "func F() int {\n\treturn 1\n}", "func F() int {\n\treturn 2\n}"); err != nil {
		t.Fatalf("editar un fitxer CRLF amb un bloc en LF: %v", err)
	}
	raw, _ := os.ReadFile(p)
	got := string(raw)
	if !strings.Contains(got, "return 2") {
		t.Errorf("el canvi no s'ha aplicat: %q", got)
	}
	// I el fitxer s'ha de quedar com era: passar-lo a \n faria un diff de
	// totes les línies del fitxer per un canvi d'una.
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("el fitxer ha perdut els \r\n: %q", got)
	}
	if n := strings.Count(got, "\r\n"); n != 5 {
		t.Errorf("volem 5 línies CRLF, n'hi ha %d: %q", n, got)
	}
}

// I al revés: un fitxer en LF amb un bloc que porta \r\n.
func TestEditFuncionaAmbElBlocEnCRLFIElFitxerEnLF(t *testing.T) {
	p := fitxer(t, "package a\n\nfunc F() int {\n\treturn 1\n}\n")
	if err := Edit(p, "func F() int {\r\n\treturn 1\r\n}", "func F() int {\r\n\treturn 2\r\n}"); err != nil {
		t.Fatalf("%v", err)
	}
	raw, _ := os.ReadFile(p)
	if got := string(raw); strings.Contains(got, "\r") || !strings.Contains(got, "return 2") {
		t.Errorf("el fitxer havia de quedar-se en LF i amb el canvi fet: %q", got)
	}
}

// «bloc no trobat» a seques no deixa saber si el text no hi és, si hi és
// amb un altre sagnat o si te l'has inventat, i cadascun es resol d'una
// manera diferent.
func TestElBlocNoTrobatDiuPerQue(t *testing.T) {
	p := fitxer(t, "package a\n\nfunc F() int {\n\t\treturn 1\n}\n")

	err := Edit(p, "func F() int {\n    return 1\n}", "x")
	if err == nil || !strings.Contains(err.Error(), "espais o sagnat") {
		t.Errorf("amb el sagnat canviat ho ha de dir: %v", err)
	}

	err = Edit(p, "func F() int {\n\t\treturn 1\n\tfmt.Println(\"res\")\n}", "x")
	if err == nil || !strings.Contains(err.Error(), "hi és, però el bloc de context") {
		t.Errorf("amb una línia bona i context dolent ho ha de dir: %v", err)
	}

	err = Edit(p, "func Z() string {\n\treturn \"res de res\"\n}", "x")
	if err == nil || !strings.Contains(err.Error(), "res semblant") {
		t.Errorf("amb un bloc que no hi és ho ha de dir: %v", err)
	}
}

// El bloc ambigu ha de dir com resoldre'l, no només que ho és.
func TestElBlocAmbiguDiuComResoldreHo(t *testing.T) {
	p := fitxer(t, "a := 1\nb := 2\na := 1\n")
	err := Edit(p, "a := 1", "a := 3")
	if err == nil || !strings.Contains(err.Error(), "més context") {
		t.Errorf("%v", err)
	}
}

// patch tenia el mateix forat que edit, i pitjor: aplica diverses edicions
// seguides i n'hi ha prou que en falli una perquè no se'n faci cap.
func TestPatchFuncionaEnFitxersAmbFinalsDeLiniaDeWindows(t *testing.T) {
	p := fitxer(t, "package a\r\n\r\nvar x = 1\r\nvar y = 2\r\n")
	err := Patch(p, []PatchOp{
		{Old: "var x = 1", New: "var x = 10"},
		{Old: "var y = 2", New: "var y = 20"},
	})
	if err != nil {
		t.Fatalf("patch sobre un fitxer CRLF: %v", err)
	}
	raw, _ := os.ReadFile(p)
	got := string(raw)
	if !strings.Contains(got, "var x = 10") || !strings.Contains(got, "var y = 20") {
		t.Errorf("les dues edicions s'han d'haver aplicat: %q", got)
	}
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("el fitxer ha perdut els \r\n: %q", got)
	}
}

// I el bloc no trobat de patch també ha de dir per què.
func TestElPatchNoTrobatDiuPerQue(t *testing.T) {
	p := fitxer(t, "package a\n\nvar x = 1\n")
	err := Patch(p, []PatchOp{{Old: "var    x    =    1", New: "var x = 2"}})
	if err == nil || !strings.Contains(err.Error(), "espais o sagnat") {
		t.Errorf("%v", err)
	}
}
