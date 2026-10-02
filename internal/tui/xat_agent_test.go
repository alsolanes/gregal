package tui

import "testing"

// El prompt de xat i de consulta diu "POTS llegir amb read/glob/grep". Perquè
// sigui veritat, tots dos han de passar per l'agent, que és l'única via que
// porta `tools` (la política ja hi denega escriure). Abans anaven per
// ChatStream i DeepSeek, sense esquema, escrivia la crida en DSML com a text.
func TestXatIConsultaVanPerLAgent(t *testing.T) {
	for _, mode := range []string{"chat", "inspect", "code"} {
		m := planTestModel(t)
		m.mode = mode
		mod, _ := m.submitText("revisa el projecte")
		got := mod.(Model)
		if !got.agentActive {
			t.Fatalf("mode %s: el torn havia d'anar per l'agent (amb eines)", mode)
		}
	}
}

// Goal també passa per l'agent: el prompt li promet read/grep/glob i la
// política el deixa només-lectura (com chat). Abans anava per ChatStream
// sense `tools` i el model intentava llegir escrivint la crida en text,
// que no s'executa i matava el torn.
func TestObjectiuVaPerLAgentNomesLectura(t *testing.T) {
	m := planTestModel(t)
	m.mode = "goal"
	mod, _ := m.submitText("vull un objectiu")
	if !mod.(Model).agentActive {
		t.Fatal("goal ha d'anar per l'agent (amb eines de lectura)")
	}
}
