package tools

import (
	"strings"
	"testing"
)

// Les pàgines que no són UTF-8 s'han de llegir amb la codificació que
// declaren (o windows-1252 si no en declaren cap), no com a «F�tbol».
func TestCosUTF8LlegeixLaCodificacioDeLaPagina(t *testing.T) {
	latin1 := []byte("<html><head><title>F\xfatbol</title></head><body>M\xe9s que un club \xb7 Sud-\xc0frica</body></html>")
	ambMeta := []byte("<html><head><meta charset=\"iso-8859-1\"><title>F\xfatbol</title></head><body>Col\xb7leccions</body></html>")
	cases := []struct {
		nom, ct string
		body    []byte
		vol     []string
	}{
		{"charset a la capçalera", "text/html; charset=ISO-8859-1", latin1, []string{"Fútbol", "Més que un club", "Sud-Àfrica"}},
		{"charset al meta", "text/html", ambMeta, []string{"Fútbol", "Col·leccions"}},
		{"sense declarar", "text/html", latin1, []string{"Fútbol", "Més"}},
		{"ja és UTF-8", "text/html; charset=iso-8859-1", []byte("Fútbol · Més"), []string{"Fútbol · Més"}},
	}
	for _, c := range cases {
		got := cosUTF8(c.ct, c.body)
		for _, v := range c.vol {
			if !strings.Contains(got, v) {
				t.Errorf("%s: esperava %q a %q", c.nom, v, got)
			}
		}
		if strings.Contains(got, "�") {
			t.Errorf("%s: hi ha caràcters de substitució: %q", c.nom, got)
		}
	}
}

func TestCharsetDe(t *testing.T) {
	for ct, vol := range map[string]string{
		"text/html; charset=ISO-8859-1":     "ISO-8859-1",
		`text/html; charset="windows-1252"`: "windows-1252",
		"text/html;charset=utf-8; foo=bar":  "utf-8",
		"text/html":                         "",
	} {
		if got := charsetDe(ct); got != vol {
			t.Errorf("charsetDe(%q) = %q, volia %q", ct, got, vol)
		}
	}
}
