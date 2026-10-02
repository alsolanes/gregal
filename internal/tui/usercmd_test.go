package tui

import (
	"errors"
	"fmt"
	"gregal/internal/llm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Les ordres personalitzades surten a l'autocomplete sense xarxa.
func TestSuggestCustomCommand(t *testing.T) {
	m := planTestModel(t)
	dir := t.TempDir()
	sub := filepath.Join(dir, ".gregal", "commands")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "hola.md"), []byte("# Saluda\nHola {{args}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.cwd = dir
	m.input.SetValue("/hol")
	trobat := false
	for _, s := range m.suggestions() {
		if s.name == "hola" {
			trobat = true
		}
	}
	if !trobat {
		t.Fatalf("autocomplete sense /hola: %+v", m.suggestions())
	}
}

// /compact amb conversa buida avisa sense cridar el model.
func TestCompactBuit(t *testing.T) {
	m := planTestModel(t)
	m.convo = nil
	mod, cmd := m.runCommand("/compact")
	_ = mod
	if cmd != nil {
		t.Fatal("sense conversa no hi ha cmd")
	}
}

// handleCompactDone retalla i desa el resum al system prompt.
func TestHandleCompactDone(t *testing.T) {
	m := planTestModel(t)
	for i := 0; i < 6; i++ {
		text := fmt.Sprintf("missatge-%d", i)
		m.convo = append(m.convo, llm.Message{Role: "user", Content: text})
		m.push(strings.Repeat("línia de farciment ", 40) + text)
	}
	mod, _ := m.Update(compactDoneMsg{summary: "RESUM", before: 6})
	got := mod.(Model)
	if len(got.convo) != 4 {
		t.Fatalf("convo=%d, volia 4", len(got.convo))
	}
	if got.compacted != "RESUM" {
		t.Fatalf("compacted=%q", got.compacted)
	}
	if !strings.Contains(got.sysPrompt(), "RESUM") {
		t.Fatal("el resum ha d'anar al system prompt")
	}
	// El % de la barra surt de promptEst, no de usedTokens: si no es
	// recalcula, després de compactar el mesurador queda ancorat al valor
	// previ. I tots dos han de coincidir just després (el resum ja és dins
	// de sysPrompt: no s'ha de comptar dues vegades).
	if got.promptEst == 0 || got.promptEst != got.usedTokens {
		t.Fatalf("promptEst=%d usedTokens=%d: cal recalcular-los iguals", got.promptEst, got.usedTokens)
	}
	visible := stripANSI(strings.Join(got.lines, "\n"))
	if strings.Contains(visible, "línia de farciment") {
		t.Fatal("la compactació ha de retirar visualment el transcript vell")
	}
	if !strings.Contains(visible, "RESUM") || !strings.Contains(visible, "missatge-5") {
		t.Fatalf("cal mostrar resum + missatges recents:\n%s", visible)
	}
	// Un segon resum substitueix l'anterior: el resumidor ja l'ha rebut.
	mod2, _ := got.Update(compactDoneMsg{summary: "MES", before: 4})
	if mod2.(Model).compacted != "MES" {
		t.Fatal("el resum nou ja incorpora l'anterior i l'ha de substituir")
	}
}

// handleCompactDone amb error no toca res.
func TestHandleCompactDoneError(t *testing.T) {
	m := planTestModel(t)
	m.convo = append(m.convo, llm.Message{Role: "user", Content: "m"})
	mod, _ := m.Update(compactDoneMsg{err: errors.New("xarxa")})
	got := mod.(Model)
	if len(got.convo) != 1 || got.compacted != "" || got.busy {
		t.Fatalf("amb error no es toca: %+v", got.convo)
	}
	// El retall de recanvi també mou el context: la barra s'ha de recalcular.
	if got.promptEst == 0 {
		t.Fatal("el retall d'error ha de recalcular promptEst (barra)")
	}
}

// /attach amb png real afegeix missatge amb imatge a la conversa.
func TestAttachImatge(t *testing.T) {
	m := planTestModel(t)
	// Ruta relativa al repo: abans era absoluta a la màquina de
	// desenvolupament i el test només passava allà (la CI no).
	png := filepath.Join("..", "..", "desktop", "build", "icon128.png")
	if _, err := os.Stat(png); err != nil {
		t.Skip("sense icon128.png al repo")
	}
	mod, _ := m.runCommand("/attach " + png + " descriu-la")
	got := mod.(Model)
	if len(got.convo) != 1 {
		t.Fatalf("convo=%d, volia 1", len(got.convo))
	}
	if len(got.convo[0].Images) != 1 || got.convo[0].Content != "descriu-la" {
		t.Fatalf("missatge malformat: %+v", got.convo[0])
	}
	// Ruta inexistent: sense canvis.
	mod2, _ := got.runCommand("/attach /no/existeix.png")
	if len(mod2.(Model).convo) != 1 {
		t.Fatal("amb error no s'afegeix res")
	}
}
