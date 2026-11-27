package tema

import (
	"math"
	"strconv"
	"testing"
)

func lluminancia(t *testing.T, hex string) float64 {
	t.Helper()
	n, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil || len(hex) != 7 {
		t.Fatalf("%q no és #rrggbb", hex)
	}
	canal := func(v uint64) float64 {
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*canal(n>>16&255) + 0.7152*canal(n>>8&255) + 0.0722*canal(n&255)
}

func contrast(t *testing.T, a, b string) float64 {
	la, lb := lluminancia(t, a), lluminancia(t, b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

// El fosc és negre de debò perquè a les pantalles AMOLED els píxels negres
// s'apaguen. Les superfícies només s'aixequen una mica, per tenir jerarquia
// sense tornar al grafit.
func TestFoscEsNegreAmoled(t *testing.T) {
	f := Fosc()
	if f.Fons != "#000000" || f.Abisme != "#000000" {
		t.Fatalf("fons i abisme han de ser #000000; són %s i %s", f.Fons, f.Abisme)
	}
	anterior := 1.05
	for _, s := range []struct{ nom, hex string }{{"Night", f.Night}, {"SuperficieAlta", f.SuperficieAlta}} {
		c := contrast(t, s.hex, "#000000")
		if c <= anterior || c >= 1.4 {
			t.Errorf("%s (%s) fa %.2f:1 amb el negre; ha de pujar per sobre de %.2f i quedar per sota d'1.4", s.nom, s.hex, c, anterior)
		}
		anterior = c
	}
	for _, sup := range []string{f.Fons, f.Night, f.SuperficieAlta} {
		for _, txt := range []struct{ nom, hex string }{{"Text", f.Text}, {"Slate", f.Slate}, {"Faint", f.Faint}} {
			if c := contrast(t, txt.hex, sup); c < 4.5 {
				t.Errorf("%s (%s) sobre %s fa %.2f:1, menys d'AA", txt.nom, txt.hex, sup, c)
			}
		}
	}
	if c := contrast(t, f.BoxIdle, f.Night); c < 3 {
		t.Errorf("BoxIdle sobre Night fa %.2f:1, menys de 3:1", c)
	}
}
