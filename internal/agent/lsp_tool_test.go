package agent

import "testing"

// Consultar és lectura (allow a tot arreu); el rename escriu: demana
// permís en mode code, passa sol en autònom i es denega als modes de només
// lectura.
func TestPoliticaLSP(t *testing.T) {
	p := &Policy{}
	consulta := `{"operation":"references","path":"a.go","line":3,"symbol":"Suma"}`
	rename := `{"operation":"rename","path":"a.go","line":3,"symbol":"Suma","new_name":"Add"}`
	cases := []struct {
		mode, args, volia string
	}{
		{ModeCode, consulta, "allow"},
		{ModeInspect, consulta, "allow"},
		{ModeCode, rename, "ask"},
		{ModeAutonomous, rename, "allow"},
		{ModeInspect, rename, "deny"},
		{ModeChat, rename, "deny"},
	}
	for _, c := range cases {
		if d, rao := p.Decide(c.mode, "lsp", c.args); d != c.volia {
			t.Errorf("%s %s: %s (%s), volia %s", c.mode, c.args, d, rao, c.volia)
		}
	}
}
