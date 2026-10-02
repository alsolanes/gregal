package tui

import (
	"strings"
	"testing"
)

// La conversa té tres veus i s'han de poder distingir sense llegir-les:
// el teu missatge porta la marca TU i fons propi, la feina de l'agent va
// resposta no porta cap adorn. Abans «TU › » i «GREGAL › » eren bessones i
// una crida d'eina tenia el mateix pes visual que la resposta.
func TestTresVeusEsDistingeixen(t *testing.T) {
	meu := stripANSI(userLine("revisa el projecte"))
	if !strings.Contains(meu, "TU") {
		t.Fatalf("el teu missatge ha de portar la marca: %q", meu)
	}
	if strings.Contains(meu, "│") || strings.Contains(meu, "▌") {
		t.Fatalf("el teu missatge no porta cap línia vertical: %q", meu)
	}
	if !strings.HasPrefix(meu, "\n") {
		t.Fatalf("un torn nou s'ha de separar de l'anterior: %q", meu)
	}

	feina := []string{
		stripANSI(thinkLine("ha pensat 3,5k caràcters")),
		stripANSI(toolCallLine("glob", `{"pattern":"*.md"}`)),
		stripANSI(toolResultLine("glob", "README.md", false)),
		stripANSI(toolResultLine("read", "ERROR: no existeix", true)),
	}
	for _, f := range feina {
		for _, l := range strings.Split(f, "\n") {
			if !strings.HasPrefix(l, "│ ") {
				t.Fatalf("tota la feina va al rail: %q", l)
			}
		}
	}

	resposta := stripANSI(assistantMD("Dos fitxers a l'arrel.", 76))
	if strings.Contains(resposta, "│") || strings.Contains(resposta, "▌") {
		t.Fatalf("la resposta no porta rail ni barra: %q", resposta)
	}
}

// Un missatge teu de diverses línies duu la marca «TU» només a la
// primera, i va precedit d'una línia en blanc que el separa del torn
// anterior.
func TestMarcaNomesALaPrimeraLiniaDelMissatgeTeu(t *testing.T) {
	linies := strings.Split(stripANSI(userLine("primera\nsegona\ntercera")), "\n")
	if len(linies) != 4 || linies[0] != "" {
		t.Fatalf("volia una línia en blanc i tres de text, tinc %d: %q", len(linies), linies)
	}
	if !strings.Contains(linies[1], "TU") {
		t.Fatalf("la primera línia porta la marca: %q", linies[1])
	}
	for i, l := range linies[2:] {
		if strings.Contains(l, "TU") {
			t.Fatalf("la marca TU només va a la primera línia (línia %d): %q", i+2, l)
		}
	}
}

// Glamour omple cada línia fins a l'amplada amb espais pintats. Es treuen:
// en fosc són un bloc de fons i qualsevol còpia se'ls endú.
func TestLaRespostaNoPortaCuaDEspais(t *testing.T) {
	out := assistantMD("## Títol\n\nUn paràgraf curt.", 76)
	for _, l := range strings.Split(out, "\n") {
		if plain := stripANSI(l); plain != strings.TrimRight(plain, " \t") {
			t.Fatalf("línia amb cua d'espais: %q", plain)
		}
	}
}
