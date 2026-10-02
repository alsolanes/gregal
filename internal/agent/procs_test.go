package agent

import (
	"strings"
	"testing"
)

// Les eines de segon pla existeixen i tenen la política que toca.
func TestPoliticaBashBackground(t *testing.T) {
	if d, _ := PolicyFor("bash_background", `{"command":"npm run dev"}`); d != "allow" {
		t.Fatalf("bash_background=%s (engegar el dev és feina normal)", d)
	}
	if d, _ := PolicyFor("bash_background", `{"command":"npm install express"}`); d != "ask" {
		t.Fatalf("bash_background=%s (instal·lar ha de demanar permís)", d)
	}
	if d, _ := PolicyFor("bash_background", `{"command":"sudo rm -rf /"}`); d != "deny" {
		t.Fatalf("bash_background perillós=%s", d)
	}
	if d, _ := PolicyFor("bash_output", `{"id":"p1"}`); d != "allow" {
		t.Fatalf("bash_output=%s", d)
	}
	if d, _ := PolicyFor("bash_kill", `{}`); d != "allow" {
		t.Fatalf("bash_kill=%s", d)
	}
}

// En modes de només lectura no s'engeguen processos.
func TestModesReadOnlyNoEngeguenProcessos(t *testing.T) {
	p := DefaultPolicy()
	for _, mode := range []string{ModeChat, ModeGoal, ModeInspect} {
		if d, why := p.Decide(mode, "bash_background", `{"command":"ls"}`); d != "deny" {
			t.Fatalf("%s: %s (%s)", mode, d, why)
		}
	}
}

// El cicle complet: engega, llegeix, atura.
func TestCicleBashBackground(t *testing.T) {
	out, _, err := Exec("bash_background", `{"command":"echo hola-fons"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hola-fons") && !strings.Contains(out, "id p") {
		t.Fatalf("engegar: %q", out)
	}
	long, _, err := Exec("bash_background", `{"command":"sleep 20"}`)
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, f := range strings.Fields(long) {
		if strings.HasPrefix(f, "p") && len(f) <= 4 {
			id = f
		}
	}
	if id == "" {
		t.Fatalf("no he trobat l'id a %q", long)
	}
	status, _, err := Exec("bash_output", `{"id":"`+id+`"}`)
	if err != nil || !strings.Contains(status, "EN MARXA") {
		t.Fatalf("output: %q %v", status, err)
	}
	if _, _, err := Exec("bash_kill", `{"id":"`+id+`"}`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Exec("bash_output", `{"id":"no-existeix"}`); err == nil {
		t.Fatal("un id inventat ha de fallar")
	}
}
