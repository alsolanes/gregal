package agent

import (
	"os"
	"strings"
	"testing"

	"gregal/internal/lsp"
)

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

// Amb un servidor de llenguatge disponible, les regles del mode porten la
// línia que diu QUAN triar lsp; sense servidor, no en prometen cap.
func TestReglesDelModeDiuenQuanUsarLSP(t *testing.T) {
	lsp.Configure([]lsp.Server{{Lang: "fake", LanguageID: "fake", Commands: [][]string{{os.Args[0]}}, Extensions: []string{".fk"}}}, false)
	defer lsp.Configure(nil, false)
	for _, mode := range []string{ModeCode, ModeAutonomous, ModeDebug, ModeInspect, ModeChat} {
		if r := PromptFor("base", mode); !strings.Contains(r, "`lsp`") || !strings.Contains(r, "references") {
			t.Errorf("%s: sense la línia de lsp", mode)
		}
	}
	if r := PromptFor("base", ModeCode); !strings.Contains(r, "rename") {
		t.Error("code: la línia ha de dir el rename")
	}
	if r := PromptFor("base", ModeInspect); strings.Contains(r, "rename") {
		t.Error("inspect no pot dir el rename: es denega")
	}
	if r := PromptFor("base", ModeGoal); strings.Contains(r, "`lsp`") {
		t.Error("goal no usa eines d'edició ni d'exploració pròpies")
	}
	lsp.Configure(nil, true) // lsp.mode: off
	if r := PromptFor("base", ModeCode); strings.Contains(r, "`lsp`") {
		t.Error("amb lsp apagat no s'ha de prometre l'eina")
	}
}
