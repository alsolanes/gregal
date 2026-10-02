package telegram

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestRegistreDeMissatges comprova que el bot deixa rastre del que rep i respon:
// sense això, un bot headless és indiagnosticable (silenci = no se sap si hi ha
// hagut error, si no ha arribat res o si la resposta ha sortit bé).
func TestRegistreDeMissatges(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("hola!"))
	bot, _, _ := testBot(t, tg, llm, nil)

	var linies []string
	bot.logf = func(format string, args ...any) {
		linies = append(linies, fmt.Sprintf(format, args...))
	}

	bot.handleMessage(context.Background(), msg(1234567, "/whoami"))
	if len(linies) == 0 {
		t.Fatal("cap línia de registre")
	}
	rebut, respost := false, false
	for _, l := range linies {
		if strings.Contains(l, "rebut: usuari 1234567") {
			rebut = true
		}
		if strings.Contains(l, "respost a 1234567") {
			respost = true
		}
	}
	if !rebut {
		t.Fatalf("no s'ha registrat el missatge: %v", linies)
	}
	if !respost {
		t.Fatalf("no s'ha registrat la resposta: %v", linies)
	}

	// L'usuari no autoritzat també ha de quedar registrat.
	linies = nil
	bot.handleMessage(context.Background(), msg(999, "hola"))
	trobat := false
	for _, l := range linies {
		if strings.Contains(l, "ignorat: usuari 999") {
			trobat = true
		}
	}
	if !trobat {
		t.Fatalf("no s'ha registrat el rebuig: %v", linies)
	}
}
