package web

import "testing"

func TestFeinaDeVeritat(t *testing.T) {
	cases := []struct {
		nom          string
		jA, jB       int
		dA, dB       string
		volVerificar bool
	}{
		{"pregunta en repo net", 3, 3, "", "", false},
		{"pregunta en repo amb canvis pendents", 3, 3, "diff X", "diff X", false},
		{"edició (journal)", 3, 4, "", "", true},
		{"bash que canvia fitxers", 3, 3, "diff X", "diff X+Y", true},
		{"bash que desfà els canvis", 3, 3, "diff X", "", true},
	}
	for _, c := range cases {
		if got := feinaDeVeritat(c.jA, c.jB, c.dA, c.dB); got != c.volVerificar {
			t.Errorf("%s: %v, volia %v", c.nom, got, c.volVerificar)
		}
	}
}
