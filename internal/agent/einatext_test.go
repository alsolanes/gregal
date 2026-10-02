package agent

import (
	"strings"
	"testing"
)

// Vist amb el model real: el torn s'acaba i el que arriba a la pantalla és
// l'XML d'una crida que el client no ha reconegut com a tal.
func TestTreuLaCridaDEinaEscritaComAText(t *testing.T) {
	cru := "El PATH no s'ha afegit bé a la subshell. Ho refaig i creo el `go.mod`.\n\n" +
		"<tool_call>\n<function=write>\n<parameter=path>\n/tmp/x/go.mod\n</parameter>\n" +
		"<parameter=content>\nmodule notes\n</parameter>\n</function>\n</tool_call>"
	net, hiEra := SenseEinaText(cru)
	if !hiEra {
		t.Fatal("no ha vist la crida escrita com a text")
	}
	if strings.Contains(net, "<tool_call") || strings.Contains(net, "<function") || strings.Contains(net, "parameter") {
		t.Errorf("queda XML al text:\n%s", net)
	}
	// La frase d'abans sí que val la pena ensenyar-la.
	if !strings.Contains(net, "Ho refaig") {
		t.Errorf("s'ha menjat el text de debò:\n%s", net)
	}
}

// Una crida tallada a mig escriure (el model s'ha quedat sense tokens)
// també s'ha de veure: és el cas més lleig, perquè l'etiqueta de tancament
// no hi és.
func TestTreuLaCridaTallada(t *testing.T) {
	net, hiEra := SenseEinaText("Ara ho escric.\n<tool_call>\n<function=write>\n<parameter=path>\n/tmp/a")
	if !hiEra || strings.Contains(net, "<tool_call") {
		t.Errorf("hiEra=%v net=%q", hiEra, net)
	}
}

// I el text normal no s'ha de tocar: parlar de <tool_call> en una resposta
// sobre aquest mateix problema no pot fer desaparèixer mig missatge.
func TestElTextNormalNoEsToca(t *testing.T) {
	for _, s := range []string{
		"He afegit el Store amb JSON i els tests passen.",
		"La funció `function=write` no existeix; el que hi ha és Write().",
		"",
	} {
		net, hiEra := SenseEinaText(s)
		if hiEra || net != s {
			t.Errorf("ha tocat text normal: %q → %q (hiEra=%v)", s, net, hiEra)
		}
	}
}
